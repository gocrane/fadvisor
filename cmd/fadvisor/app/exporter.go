package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/klog/v2"

	"github.com/gocrane/fadvisor/cmd/fadvisor/app/options"
	exporter "github.com/gocrane/fadvisor/pkg/cost-exporter"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cache"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudcost"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprovider"
	_ "github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprovider/default"
	_ "github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprovider/tencentcloud"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/store/prometheus"
	"github.com/gocrane/fadvisor/pkg/util"
)

// NewExporterCommand creates a *cobra.Command object with default parameters
func NewExporterCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "fadvisor",
		Long: `fadvisor used to export cost metrics to storage store such as prometheus`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Complete(); err != nil {
				klog.Errorf("opts complete failed, exit: %v", err)
				os.Exit(255)
			}
			if err := opts.Validate(); err != nil {
				klog.Errorf("opts validate failed, exit: %v", err)
				os.Exit(255)

			}

			if err := Run(ctx, opts); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(cmd.Flags())
	return cmd
}

// Run runs the fadvisor with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {

	creator, err := util.CreateK8sClient(opts.ClientConfig, opts.MaxIdleConnsPerClient)
	if err != nil {
		return err
	}

	kubeClient := creator("fadvisor")
	kubeEventClient := creator("fadvisor-event")

	k8sCache := cache.NewCache(kubeClient)
	k8sCache.WaitForCacheSync(ctx.Done())

	// initialize cloud provider with the cloud provider name and config file provided
	priceConfig := cloudprovider.NewProviderConfig(&opts.CustomPrice)
	cloudPrice, err := cloudprovider.InitPriceProvider(opts.CloudConfig, priceConfig, &k8sCache)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if cloudPrice == nil {
		klog.Fatalf("Failed to initialize cloud price")
	}

	if err = cloudPrice.WarmUp(); err != nil {
		return err
	}
	go wait.Until(cloudPrice.Refresh, 30*time.Minute, ctx.Done())
	model := cloudcost.NewCloudCost(k8sCache, cloudPrice)

	metricEmitter := prometheus.NewCostMetricEmitter(model, opts.MetricUpdateInterval, ctx.Done())

	// metrics do not allow multiple instances at the same time
	run := func(ctx context.Context) {
		go metricEmitter.Start()

		server := exporter.NewServer(model, opts.BindAddr, opts.Debugging)
		server.RegisterHandlers()
		serverStopedCh := server.Serve(ctx.Done())

		<-serverStopedCh
	}

	eventRecorder := events.NewEventBroadcasterAdapter(kubeEventClient).NewRecorder("fadvisor-event")
	leadElectCfg, err := util.CreateLeaderElectionConfig("fadvisor", kubeClient, eventRecorder, opts.LeaderElection)
	if err != nil {
		return fmt.Errorf("couldn't create leader elector config: %v", err)
	}

	// If leader election is enabled, runCommand via LeaderElector until done and exit.
	if leadElectCfg != nil {
		leadElectCfg.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Info("leader election lost")
			},
		}

		leaderElector, err := leaderelection.NewLeaderElector(*leadElectCfg)
		if err != nil {
			return fmt.Errorf("couldn't create leader elector: %v", err)
		}
		leaderElector.Run(ctx)

		return fmt.Errorf("lost lease")
	}
	run(ctx)

	return nil
}
