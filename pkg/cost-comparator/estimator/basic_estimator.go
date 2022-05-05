package estimator

import (
	"fmt"
	"strconv"

	"github.com/montanaflynn/stats"

	"github.com/gocrane/crane/pkg/common"
	"github.com/gocrane/fadvisor/pkg/spec"
)

var _ Estimator = &StatisticEstimator{}

// StatisticEstimator based on statistic such as histogram, mean,max,min,median and so on
type StatisticEstimator struct {
}

func NewStatisticEstimator() *StatisticEstimator {
	return &StatisticEstimator{}
}

func (s *StatisticEstimator) Estimation(ts *common.TimeSeries, estimateConfig map[string]interface{}) (*spec.Statistic, error) {
	result := &spec.Statistic{}
	data := TimeSeries2Float64Data(ts)
	percent := 0.95
	if perts, ok := estimateConfig["percentile"]; ok {
		percent, ok = perts.(float64)
		if !ok {
			return nil, fmt.Errorf("percentile param is not valid")
		}
	}
	marginFraction := 1.25
	if margin, ok := estimateConfig["marginFraction"]; ok {
		marginFraction, ok = margin.(float64)
		if !ok {
			return nil, fmt.Errorf("marginFraction param is not valid")
		}
	}

	gotPertValue, err := data.Percentile(percent * 100)
	if err != nil {
		return result, err
	}
	var errs []error
	max, err := data.Max()
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%v", errs)
	}

	recommends := gotPertValue * marginFraction

	maxRecommended := max * marginFraction
	return &spec.Statistic{
		Percentile:     &gotPertValue,
		Max:            &max,
		MaxRecommended: &maxRecommended,
		Recommended:    &recommends,
	}, nil
}

//nolint:unused
func (s *StatisticEstimator) percentsValues(data stats.Float64Data, percentList []string) (map[string]float64, error) {
	result := make(map[string]float64)
	for _, pertStr := range percentList {
		pert, err := strconv.ParseFloat(pertStr, 64)
		if err != nil {
			return result, err
		}
		gotPertValue, err := data.Percentile(pert * 100)
		if err != nil {
			return result, err
		}
		result[pertStr] = gotPertValue
	}
	return result, nil
}

func TimeSeries2Float64Data(ts *common.TimeSeries) stats.Float64Data {
	var data stats.Float64Data
	for _, sample := range ts.Samples {
		data = append(data, sample.Value)
	}
	return data
}
