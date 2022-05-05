package prom

import (
	gocontext "context"
	"time"

	"github.com/gocrane/fadvisor/pkg/querybuilder"

	"k8s.io/klog/v2"

	"github.com/gocrane/crane/pkg/common"
	"github.com/gocrane/fadvisor/pkg/datasource"
	"github.com/gocrane/fadvisor/pkg/metricnaming"
	"github.com/gocrane/fadvisor/pkg/metricquery"
)

type prom struct {
	ctx    *context
	config *datasource.PromConfig
}

// NewProvider return a prometheus data provider
func NewProvider(config *datasource.PromConfig) (datasource.Interface, error) {

	client, err := NewPrometheusClient(config)
	if err != nil {
		return nil, err
	}

	ctx := NewContext(client, config.MaxPointsLimitPerTimeSeries)

	return &prom{ctx: ctx, config: config}, nil
}

func (p *prom) QueryTimeSeries(ctx gocontext.Context, namer metricnaming.MetricNamer, startTime time.Time, endTime time.Time, step time.Duration) ([]*common.TimeSeries, error) {
	promBuilder := namer.QueryBuilder().Builder(metricquery.PrometheusMetricSource)
	promQuery, err := promBuilder.BuildQuery(querybuilder.BuildQueryBehavior{FederatedClusterScope: p.config.FederatedClusterScope})
	if err != nil {
		klog.Errorf("Failed to BuildQuery: %v", err)
		return nil, err
	}
	klog.V(6).Infof("QueryTimeSeries metricNamer %v, timeout: %v, query: %v", namer.BuildUniqueKey(), p.config.Timeout, promQuery.Prometheus.Query)
	timeoutCtx, cancelFunc := gocontext.WithTimeout(ctx, p.config.Timeout)
	defer cancelFunc()
	timeSeries, err := p.ctx.QueryRangeSync(timeoutCtx, promQuery.Prometheus.Query, startTime, endTime, step)
	if err != nil {
		klog.Errorf("Failed to QueryTimeSeries: %v, metricNamer: %v, query: %v", err, namer.BuildUniqueKey(), promQuery.Prometheus.Query)
		return nil, err
	}
	return timeSeries, nil
}

func (p *prom) QueryLatestTimeSeries(ctx gocontext.Context, namer metricnaming.MetricNamer) ([]*common.TimeSeries, error) {
	promBuilder := namer.QueryBuilder().Builder(metricquery.PrometheusMetricSource)
	promQuery, err := promBuilder.BuildQuery(querybuilder.BuildQueryBehavior{FederatedClusterScope: p.config.FederatedClusterScope})
	if err != nil {
		klog.Errorf("Failed to BuildQuery: %v", err)
		return nil, err
	}
	// use range query for latest too. because the queryExpr is an range in crd spec
	//end := time.Now()
	// avoid no data latest. multiply 2
	//start := end.Add(-step * 2)
	klog.V(6).Infof("QueryLatestTimeSeries metricNamer %v, timeout: %v, query: %v", namer.BuildUniqueKey(), p.config.Timeout, promQuery.Prometheus.Query)
	timeoutCtx, cancelFunc := gocontext.WithTimeout(ctx, p.config.Timeout)
	defer cancelFunc()
	timeSeries, err := p.ctx.QuerySync(timeoutCtx, promQuery.Prometheus.Query)
	if err != nil {
		klog.Errorf("Failed to QueryLatestTimeSeries: %v, metricNamer: %v, query: %v", err, namer.BuildUniqueKey(), promQuery.Prometheus.Query)
		return nil, err
	}
	return timeSeries, nil
}
