package coster

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gocrane/fadvisor/pkg/cloud"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/gocrane/fadvisor/pkg/spec"
)

// eks or ask eci
type recommender struct {
}

func NewRecommenderCoster() *recommender {
	return &recommender{}
}

func (e *recommender) TotalCost(costerCtx *CosterContext) (RecommendedCost, RecommendedCost, RecommendedCost, RecommendedCost) {
	serverlessPodsTotalCost := 0.
	timespanInHour := float64(costerCtx.TimeSpanSeconds) / time.Hour.Seconds()

	workloadKindTotalCost := map[string]float64{}
	for kind, workloadsSpec := range costerCtx.WorkloadsSpec {
		workloadKindTotalCost[kind] = 0
		for nn, workloadSpec := range workloadsSpec {
			workloadPrice := workloadCosting(costerCtx.Pricer, timespanInHour, workloadSpec, nn, kind)
			workloadCost := workloadPrice * timespanInHour

			workloadKindTotalCost[kind] += workloadCost
			serverlessPodsTotalCost += workloadCost
		}
	}

	recWorkloadKindTotalCost := map[string]float64{}
	recServerlessPodsTotalCost := 0.
	maxRecServerlessPodsTotalCost := 0.
	maxMarginServerlessPodsTotalCost := 0.
	percentServerlessPodsTotalCost := 0.

	for kind, workloadsRecSpec := range costerCtx.WorkloadsRecSpec {
		recWorkloadKindTotalCost[kind] = 0
		if strings.ToLower(kind) == "daemonset" {
			continue
		}
		for nn, workloadRecSpec := range workloadsRecSpec {
			recWorkloadPrice := workloadCosting(costerCtx.Pricer, timespanInHour, workloadRecSpec.RecommendedSpec, nn, kind)
			workloadCost := recWorkloadPrice * timespanInHour
			recWorkloadKindTotalCost[kind] += workloadCost
			recServerlessPodsTotalCost += workloadCost

			if workloadRecSpec.MaxRecommendedSpec != nil {
				recMaxWorkloadPrice := workloadCosting(costerCtx.Pricer, timespanInHour, *workloadRecSpec.MaxRecommendedSpec, nn, kind)
				recMaxWorkloadCost := recMaxWorkloadPrice * timespanInHour
				maxRecServerlessPodsTotalCost += recMaxWorkloadCost
			}

			if workloadRecSpec.MaxMarginRecommendedSpec != nil {
				recMaxMarginWorkloadPrice := workloadCosting(costerCtx.Pricer, timespanInHour, *workloadRecSpec.MaxMarginRecommendedSpec, nn, kind)
				recMaxMarginWorkloadCost := recMaxMarginWorkloadPrice * timespanInHour
				maxMarginServerlessPodsTotalCost += recMaxMarginWorkloadCost
			}

			if workloadRecSpec.PercentRecommendedSpec != nil {
				percentWorkloadPrice := workloadCosting(costerCtx.Pricer, timespanInHour, *workloadRecSpec.PercentRecommendedSpec, nn, kind)
				percentWorkloadPriceCost := percentWorkloadPrice * timespanInHour
				percentServerlessPodsTotalCost += percentWorkloadPriceCost
			}
		}
	}

	platformCost := costerCtx.Pricer.PlatformPrice(cloud.PlatformParameter{Platform: cloud.ServerlessKind})

	recCost := RecommendedCost{
		TotalCost:    recServerlessPodsTotalCost + platformCost.TotalPrice,
		WorkloadCost: recServerlessPodsTotalCost,
		PlatformCost: platformCost.TotalPrice,
	}

	percentCost := RecommendedCost{
		TotalCost:    percentServerlessPodsTotalCost + platformCost.TotalPrice,
		WorkloadCost: percentServerlessPodsTotalCost,
		PlatformCost: platformCost.TotalPrice,
	}

	maxRecCost := RecommendedCost{
		TotalCost:    maxRecServerlessPodsTotalCost + platformCost.TotalPrice,
		WorkloadCost: maxRecServerlessPodsTotalCost,
		PlatformCost: platformCost.TotalPrice,
	}

	maxMarginCost := RecommendedCost{
		TotalCost:    maxMarginServerlessPodsTotalCost + platformCost.TotalPrice,
		WorkloadCost: maxMarginServerlessPodsTotalCost,
		PlatformCost: platformCost.TotalPrice,
	}

	return recCost, percentCost, maxRecCost, maxMarginCost
}

func workloadCosting(pricer cloud.Pricer, timespanInHour float64, recommendedSpec spec.CloudPodSpec, nn types.NamespacedName, kind string) float64 {
	workloadPricing, err := pricer.ServerlessPodPrice(recommendedSpec)
	if err != nil {
		klog.Errorf("Failed to get ServerlessPodPrice for workload: %v, kind: %v, err: %v", nn, kind, err)
		return 0
	}
	var workloadPrice float64
	if workloadPricing.Cost != "" {
		workloadPrice, err = strconv.ParseFloat(workloadPricing.Cost, 64)
		if err != nil {
			klog.V(3).Infof("Could not parse pod total cost price, workload: %v, kind: %v, err: %v", nn, kind, err)
			return 0
		}
	}
	if math.IsNaN(workloadPrice) {
		klog.V(3).Infof("workloadPrice is NaN. Setting to 0. workload: %v, kind: %v", nn, kind)
		workloadPrice = 0
	}
	workloadCost := workloadPrice * timespanInHour
	return workloadCost
}
