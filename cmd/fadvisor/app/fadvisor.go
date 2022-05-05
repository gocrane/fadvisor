package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/gcfg.v1"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/scale"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/gocrane/fadvisor/cmd/fadvisor/app/options"
	"github.com/gocrane/fadvisor/pkg/cache"
	"github.com/gocrane/fadvisor/pkg/cloud"
	_ "github.com/gocrane/fadvisor/pkg/cloudproviders/default"
	_ "github.com/gocrane/fadvisor/pkg/cloudproviders/qcloud"
	costcomparator "github.com/gocrane/fadvisor/pkg/cost-comparator"
	exporter "github.com/gocrane/fadvisor/pkg/cost-exporter"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudcost"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/store/prometheus"
	"github.com/gocrane/fadvisor/pkg/datasource"
	"github.com/gocrane/fadvisor/pkg/datasource-providers/metricserver"
	"github.com/gocrane/fadvisor/pkg/datasource-providers/prom"
	"github.com/gocrane/fadvisor/pkg/datasource-providers/qcloudmonitor"
	_ "github.com/gocrane/fadvisor/pkg/querybuilder-providers/metricserver"
	_ "github.com/gocrane/fadvisor/pkg/querybuilder-providers/prometheus"
	_ "github.com/gocrane/fadvisor/pkg/querybuilder-providers/qcloudmonitor"
	"github.com/gocrane/fadvisor/pkg/util"
	"github.com/gocrane/fadvisor/pkg/util/target"
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
			if errs := opts.Validate(); len(errs) > 0 {
				klog.Errorf("opts validate failed, exit: %v", errs)
				os.Exit(255)

			}

			if opts.ComparatorMode {
				if err := RunComparator(ctx, opts); err != nil {
					klog.Errorf("run comparator failed, exit: %v", err)
					os.Exit(255)
				}
			} else {
				if err := Run(ctx, opts); err != nil {
					klog.Errorf("run failed, exit: %v", err)
					os.Exit(255)
				}
			}
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(cmd.Flags())
	return cmd
}

// once task analyze
func RunComparator(ctx context.Context, opts *options.Options) error {
	creator, err := util.CreateK8sClient(opts.ClientConfig, opts.MaxIdleConnsPerClient)
	if err != nil {
		return err
	}

	kubeClient := creator("fadvisor-comparator")

	k8sCache := cache.NewCache(kubeClient)
	k8sCache.WaitForCacheSync(ctx.Done())

	// initialize cloud provider with the cloud provider name and config file provided
	opts.ComparatorOptions.CustomPrice = opts.CustomPrice
	opts.ComparatorOptions.CloudConfig = opts.CloudConfig
	var cfg datasource.QCloudMonitorConfig
	cloudConfigFile, err := os.Open(opts.CloudConfig.CloudConfigFile)
	defer cloudConfigFile.Close()
	if err != nil {
		return fmt.Errorf("couldn't open cloud provider configuration %s: %#v",
			opts.CloudConfig.CloudConfigFile, err)
	}
	if err := gcfg.FatalOnly(gcfg.ReadInto(&cfg, cloudConfigFile)); err != nil {
		klog.Errorf("Failed to read TencentCloud configuration file: %v", err)
		return err
	}
	opts.ComparatorOptions.DataSourceQMonitorConfig = cfg

	priceConfig := cloud.NewProviderConfig(&opts.ComparatorOptions.CustomPrice)
	cloudProvider, err := cloud.InitCloudProvider(opts.ComparatorOptions.CloudConfig, priceConfig, &k8sCache)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if cloudProvider == nil {
		klog.Fatalf("Failed to initialize cloud provider")
	}

	if err = cloudProvider.WarmUp(); err != nil {
		return err
	}
	restConfig, err := util.NewK8sConfig(opts.ClientConfig, opts.MaxIdleConnsPerClient)
	if err != nil {
		return err
	}
	dynamicKubeClient, err := dynamic.NewForConfig(rest.AddUserAgent(restConfig, "fadvisor-comparator-dynamic"))
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		klog.Exit(err, "Unable to create discover client")
	}
	restMapper, err := apiutil.NewDynamicRESTMapper(restConfig)
	if err != nil {
		klog.Exit(err, "Unable to create rest mapper")
	}

	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(discoveryClient)
	scaleClient := scale.New(
		discoveryClient.RESTClient(), restMapper,
		dynamic.LegacyAPIPathResolverFunc,
		scaleKindResolver,
	)
	targetFetcher := target.NewTargetInfoFetcher(restMapper, scaleClient, kubeClient)

	_, _, hybrid := initializationDataSource(opts, restConfig)

	fmt.Println(opts.ComparatorOptions.Config)
	comparator := costcomparator.NewComparator(opts.ComparatorOptions.Config,
		dynamicKubeClient,
		discoveryClient,
		restMapper,
		targetFetcher,
		k8sCache,
		cloudProvider,
		hybrid)

	comparator.Init()
	comparator.DoAnalysis()
	return nil
}

func initializationDataSource(opts *options.Options, restConfig *rest.Config) (datasource.RealTime, datasource.History, datasource.Interface) {
	var realtimeDataSource datasource.RealTime
	var historyDataSource datasource.History
	var hybridDataSource datasource.Interface
	datasourceStr := opts.ComparatorOptions.DataSource
	switch strings.ToLower(datasourceStr) {
	case "metricserver", "ms":
		provider, err := metricserver.NewProvider(restConfig)
		if err != nil {
			klog.Exitf("unable to create datasource provider %v, err: %v", datasourceStr, err)
		}
		realtimeDataSource = provider
	case "qmonitor", "qcloudmonitor", "qm":
		provider, err := qcloudmonitor.NewProvider(&opts.ComparatorOptions.DataSourceQMonitorConfig)
		if err != nil {
			klog.Exitf("unable to create datasource provider %v, err: %v", datasourceStr, err)
		}
		hybridDataSource = provider
		realtimeDataSource = provider
		historyDataSource = provider
	case "prometheus", "prom":
		fallthrough
	default:
		// default is prom
		provider, err := prom.NewProvider(&opts.ComparatorOptions.DataSourcePromConfig)
		if err != nil {
			klog.Exitf("unable to create datasource provider %v, err: %v", datasourceStr, err)
		}
		hybridDataSource = provider
		realtimeDataSource = provider
		historyDataSource = provider
	}
	return realtimeDataSource, historyDataSource, hybridDataSource
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
	priceConfig := cloud.NewProviderConfig(&opts.CustomPrice)
	cloudPrice, err := cloud.InitCloudProvider(opts.CloudConfig, priceConfig, &k8sCache)
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
