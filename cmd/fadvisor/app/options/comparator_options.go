package options

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/gocrane/fadvisor/pkg/cloud"
	comparatorcfg "github.com/gocrane/fadvisor/pkg/cost-comparator/config"
	"github.com/gocrane/fadvisor/pkg/datasource"
)

// ComparatorOptions used for fadvisor cost comparator
type ComparatorOptions struct {
	Config      comparatorcfg.Config
	DataSource  string
	CustomPrice cloud.CustomPricing
	CloudConfig cloud.CloudConfig
	// DataSourcePromConfig is the prometheus datasource config
	DataSourcePromConfig datasource.PromConfig
	// DataSourceQMonitorConfig is the tencent cloud monitor datasource config
	DataSourceQMonitorConfig datasource.QCloudMonitorConfig
}

func NewComparatorOptions() *ComparatorOptions {
	return &ComparatorOptions{}
}

func (o *ComparatorOptions) Complete() error {
	return nil
}

func (o *ComparatorOptions) Validate() []error {
	var errors []error
	return errors
}

func (o *ComparatorOptions) AddFlags(fs *pflag.FlagSet) {
	if fs == nil {
		return
	}

	fs.DurationVar(&o.Config.History.Length, "comparator-analyze-history-length", 24*time.Hour, "analyze history length")
	fs.DurationVar(&o.Config.History.Step, "comparator-analyze-step", 5*time.Minute, "analyze history step")
	fs.StringVar(&o.Config.History.EndTime, "comparator-analyze-end", "", "analyze history end time, if no specified, default is from now")
	fs.Int64Var(&o.Config.TimeSpanSeconds, "comparator-timespan-seconds", 3600, "")
	fs.StringVar(&o.Config.ClusterName, "comparator-cluster-name", "default", "cluster name the comparator running base on")
	fs.StringVar(&o.Config.ClusterId, "comparator-cluster-id", "default", "cluster id the comparator running base on")
	fs.Float64Var(&o.Config.Discount, "comparator-discount", 1.0, "discount used to compute costs")
	fs.StringVar(&o.Config.OutputMode, "comparator-output-mode", "", "results output mode, includes stdout, csv. if no specified, both will output. including csv file and a table print")
	fs.BoolVar(&o.Config.EnableContainerCheckpoint, "comparator-enable-container-ts-checkpoint", false, "enable container time series data checkpoint")
	fs.BoolVar(&o.Config.EnableWorkloadTimeSeries, "comparator-enable-workload-ts", false, "enable workload time series fetching, it will fetch workload time series data")
	fs.BoolVar(&o.Config.EnableWorkloadCheckpoint, "comparator-enable-workload-ts-checkpoint", false, "enable workload time series data checkpoint")
	fs.StringVar(&o.Config.DataPath, "comparator-data-path", ".", "data path of the report and checkpoint data stored")

	fs.StringVar(&o.DataSource, "datasource", "prom", "data source of the estimator, prom, qmonitor is available")
	fs.StringVar(&o.DataSourcePromConfig.Address, "prometheus-address", "", "prometheus address")
	fs.StringVar(&o.DataSourcePromConfig.Auth.Username, "prometheus-auth-username", "", "prometheus auth username")
	fs.StringVar(&o.DataSourcePromConfig.Auth.Password, "prometheus-auth-password", "", "prometheus auth password")
	fs.StringVar(&o.DataSourcePromConfig.Auth.BearerToken, "prometheus-auth-bearertoken", "", "prometheus auth bearertoken")
	fs.IntVar(&o.DataSourcePromConfig.QueryConcurrency, "prometheus-query-concurrency", 10, "prometheus query concurrency")
	fs.BoolVar(&o.DataSourcePromConfig.InsecureSkipVerify, "prometheus-insecure-skip-verify", false, "prometheus insecure skip verify")
	fs.DurationVar(&o.DataSourcePromConfig.KeepAlive, "prometheus-keepalive", 60*time.Second, "prometheus keep alive")
	fs.DurationVar(&o.DataSourcePromConfig.Timeout, "prometheus-timeout", 3*time.Minute, "prometheus timeout")
	fs.BoolVar(&o.DataSourcePromConfig.BRateLimit, "prometheus-bratelimit", false, "prometheus bratelimit")
	fs.IntVar(&o.DataSourcePromConfig.MaxPointsLimitPerTimeSeries, "prometheus-maxpoints", 11000, "prometheus max points limit per time series")
	fs.BoolVar(&o.DataSourcePromConfig.FederatedClusterScope, "prometheus-federated-cluster-scope", false, "prometheus support federated clusters query")
	fs.BoolVar(&o.DataSourcePromConfig.ThanosPartial, "prometheus-thanos-partial", false, "prometheus api to query thanos data source, hacking way, denote the thanos partial response query")
	fs.BoolVar(&o.DataSourcePromConfig.ThanosDedup, "prometheus-thanos-dedup", false, "prometheus api to query thanos data source, hacking way, denote the thanos deduplicate query")

}
