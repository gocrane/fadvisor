package qcloudmonitor

import (
	"github.com/gocrane/fadvisor/pkg/metricquery"
	"github.com/gocrane/fadvisor/pkg/querybuilder"
)

var _ querybuilder.Builder = &builder{}

type builder struct {
	metric *metricquery.Metric
}

func NewQCloudMonitorQueryBuilder(metric *metricquery.Metric) querybuilder.Builder {
	return &builder{
		metric: metric,
	}
}

func (b builder) BuildQuery(behavior querybuilder.BuildQueryBehavior) (*metricquery.Query, error) {
	return qcloudMonitorQuery(&metricquery.QCloudMonitorQuery{Metric: b.metric}), nil
}

func qcloudMonitorQuery(query *metricquery.QCloudMonitorQuery) *metricquery.Query {
	return &metricquery.Query{
		Type:          metricquery.QCloudMonitorMetricSource,
		QCloudMonitor: query,
	}
}

func init() {
	querybuilder.RegisterBuilderFactory(metricquery.QCloudMonitorMetricSource, NewQCloudMonitorQueryBuilder)
}
