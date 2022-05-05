package estimator

import (
	"github.com/gocrane/crane/pkg/common"
	"github.com/gocrane/fadvisor/pkg/spec"
)

type Estimator interface {
	// Given a time series, then return a statistic estimation, an estimation is all the statistic data of the time series, such as P95, P99, avg, max, min, median, variance
	Estimation(ts *common.TimeSeries, estimateConfig map[string]interface{}) (*spec.Statistic, error)
}
