package estimator

import (
	"github.com/gocrane/crane/pkg/common"
	"github.com/gocrane/fadvisor/pkg/spec"
)

var _ Estimator = &VpaEstimator{}

// VpaEstimator based on vpa decaying exponential moving window algorithm to estimate resource
type VpaEstimator struct {
}

func (v VpaEstimator) Estimation(ts *common.TimeSeries, estimateConfig map[string]interface{}) (*spec.Statistic, error) {
	panic("implement me")
}
