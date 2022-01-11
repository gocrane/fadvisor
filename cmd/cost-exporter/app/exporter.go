package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprice/defaultcloud"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprice/tencentcloud"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog/v2"

	"github.com/spf13/cobra"

	"github.com/gocrane/fadvisor/cmd/cost-exporter/app/options"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/consts"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/credential"
	cost_exporter "github.com/gocrane/fadvisor/pkg/cost-exporter"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cache"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprice"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/costmodel"
	storeprometheus "github.com/gocrane/fadvisor/pkg/cost-exporter/store/prometheus"
	"github.com/gocrane/fadvisor/pkg/util"
)

// NewManagerCommand creates a *cobra.Command object with default parameters
func NewExporterCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "cost-exporter",
		Long: `cost-exporter used to export cost metrics to storage store such as prometheus`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Complete(); err != nil {
				klog.Errorf("opts complete failed,exit: %v", err)
				os.Exit(255)
			}
			if err := opts.Validate(); err != nil {
				klog.Errorf("opts validate failed,exit: %v", err)
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

// Run runs the cost-exporter with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {

	qccp := qcloud.QCloudClientProfile{
		Debug:           opts.Debug,
		DefaultLanguage: opts.DefaultLanguage,
		DefaultLimit:    opts.DefaultLimit,
		DefaultTimeout:  opts.DefaultTimeout,
		Region:          opts.Region,
		DomainSuffix:    opts.DomainSuffix,
		Scheme:          opts.Scheme,
	}

	cred := credential.NewQCloudCredential(opts.ClusterId, opts.AppId, opts.SecretId, opts.SecretKey, 1*time.Hour)
	qcc := &qcloud.QCloudClientConfig{
		RateLimiter:         flowcontrol.NewTokenBucketRateLimiter(5, 1),
		DefaultRetryCnt:     consts.MAXRETRY,
		QCloudClientProfile: qccp,
		Credential:          cred,
	}

	creator, err := util.CreateK8sClient(opts.ClientConfig, opts.MaxIdleConnsPerClient)
	if err != nil {
		return err
	}

	kubeClient := creator("cost-exporter")

	kubeEventClient := creator("cost-exporter-event")

	k8sCache := cache.NewCache(kubeClient)
	k8sCache.WaitForCacheSync(ctx.Done())

	providerConfig := cloudprice.NewProviderConfig(&opts.CustomPrice)

	nodes := k8sCache.GetNodes()
	provider := opts.Provider
	if qcc.Region == "" {
		for _, node := range nodes {
			region := cloudprice.DetectRegion(node)
			qcc.Region = region
			break
		}
	}

	if provider != string(cloudprice.DefaultCloud) {
		if qcc.Region == "" {
			return fmt.Errorf("no region info found. must specify region for provider %v", qcc.Region)
		}
	}

	klog.Infof("qcc: %+v", qcc.QCloudClientProfile)

	cloudPrice := defaultcloud.NewDefaultCloud(providerConfig, k8sCache)

	if provider == string(cloudprice.TencentCloud) {
		cloudPrice = tencentcloud.NewTencentCloud(qcc, providerConfig, k8sCache)
	}

	err = cloudPrice.WarmUp()
	if err != nil {
		return err
	}
	go wait.Until(cloudPrice.Refresh, 30*time.Minute, ctx.Done())
	model := costmodel.NewCostModel(k8sCache, cloudPrice)

	metricEmitter := storeprometheus.NewCostMetricEmitter(model, opts.MetricUpdateInterval, ctx.Done())

	// metrics do not allow multiple instances at the same time
	run := func(ctx context.Context) {
		go metricEmitter.Start()

		server := cost_exporter.NewServer(model, opts.BindAddr, opts.Debugging)
		server.RegisterHandlers()
		serverStopedCh := server.Serve(ctx.Done())

		<-serverStopedCh
	}

	eventRecorder := events.NewEventBroadcasterAdapter(kubeEventClient).NewRecorder("cost-exporter-event")

	leadElectCfg, err := util.CreateLeaderElectionConfig("cost-exporter", kubeClient, eventRecorder, opts.LeaderElection)
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
