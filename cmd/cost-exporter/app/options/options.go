package options

import (
	"time"

	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/gocrane/fadvisor/pkg/consts"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprovider"
)

// Options hold the command-line options about crane manager
type Options struct {
	// LeaderElection hold the configurations for manager leader election.
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	// BindAddr is The address the probe endpoint binds to.
	BindAddr string

	ClientConfig componentbaseconfig.ClientConnectionConfiguration
	// SecureServing specifies the server configurations to set up a HTTPS server.
	SecureServing options.SecureServingOptionsWithLoopback
	CloudConfig   cloudprovider.CloudConfig

	Debugging componentbaseconfig.DebuggingConfiguration

	MaxIdleConnsPerClient int

	MetricUpdateInterval time.Duration

	CustomPrice cloudprovider.CustomPricing
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: consts.CraneNamespace,
			ResourceName:      consts.CostExporterName,
			LeaseDuration:     metav1.Duration{Duration: consts.DefaultLeaseDuration},
			RenewDeadline:     metav1.Duration{Duration: consts.DefaultRenewDeadline},
			RetryPeriod:       metav1.Duration{Duration: consts.DefaultRetryPeriod},
		},
	}
}

// Complete completes all the required options.
func (o *Options) Complete() error {

	return nil
}

// Validate all required options.
func (o *Options) Validate() error {
	return nil
}

func (o *Options) ApplyTo() {

}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.BindAddr, "bind-address", ":8081", "The address the probe endpoint binds to.")
	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.DurationVar(&o.LeaderElection.LeaseDuration.Duration, "lease-duration", 15*time.Second,
		"Specifies the expiration period of lease.")
	flags.DurationVar(&o.LeaderElection.RetryPeriod.Duration, "lease-retry-period", 2*time.Second,
		"Specifies the lease renew interval.")
	flags.DurationVar(&o.LeaderElection.RenewDeadline.Duration, "lease-renew-period", 10*time.Second,
		"Specifies the lease renew interval.")
	flags.StringVar(&o.LeaderElection.ResourceLock, "leader-elect-resource-lock", o.LeaderElection.ResourceLock, ""+
		"The type of resource object that is used for locking during "+
		"leader election. Supported options are `leases` (default), `endpoints` and `configmaps`.")
	flags.StringVar(&o.LeaderElection.ResourceName, "leader-elect-resource-name", o.LeaderElection.ResourceName, ""+
		"The name of resource object that is used for locking during "+
		"leader election.")
	flags.StringVar(&o.LeaderElection.ResourceNamespace, "leader-elect-resource-namespace", o.LeaderElection.ResourceNamespace, ""+
		"The namespace of resource object that is used for locking during "+
		"leader election.")

	flags.StringVar(&o.CloudConfig.Provider, "provider", "default", "cloud provider the cost-exporter running on, now support default and qcloud only.")
	flags.StringVar(&o.CloudConfig.CloudConfigFile, "cloudConfigFile", "", "cloudConfigFile specifies path for the cloud configuration.")

	flags.StringVar(&o.ClientConfig.Kubeconfig, "kubeconfig",
		o.ClientConfig.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	flags.StringVar(&o.ClientConfig.ContentType, "kube-api-content-type",
		o.ClientConfig.ContentType, "Content type of requests sent to apiserver.")
	flags.Float32Var(&o.ClientConfig.QPS, "kube-api-qps",
		o.ClientConfig.QPS, "QPS to use while talking with kubernetes apiserver.")
	flags.Int32Var(&o.ClientConfig.Burst, "kube-api-burst",
		o.ClientConfig.Burst, "Burst to use while talking with kubernetes apiserver.")
	flags.IntVar(&o.MaxIdleConnsPerClient, "kube-client-max-idle-conns",
		o.MaxIdleConnsPerClient, "MaxIdleConnsPerHost of each k8s or custom clients")

	flags.BoolVar(&o.Debugging.EnableProfiling, "profiling",
		true, "Enable profiling via web interface host:port/debug/pprof/")
	flags.BoolVar(&o.Debugging.EnableContentionProfiling, "contention-profiling",
		true, "Enable lock contention profiling, if profiling is enabled")

	flags.DurationVar(&o.MetricUpdateInterval, "metric-update-interval", 5*time.Minute, "metric update interval for prometheus")

	flags.StringVar(&o.CustomPrice.Description, "custom-price-desc", "default pricing", "custom pricing config description")
	flags.StringVar(&o.CustomPrice.Provider, "custom-price-provider", "default", "custom pricing config provider")
	flags.Float64Var(&o.CustomPrice.CpuHourlyPrice, "custom-price-cpu", 0.031611, "cpu hourly unit price of one core")
	flags.Float64Var(&o.CustomPrice.RamGBHourlyPrice, "custom-price-ram", 0.004237, "ram gb hourly unit price")
}
