package coster

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gocrane/fadvisor/pkg/cloud"

	"k8s.io/klog/v2"
)

// eks or ask eci
type serverless struct {
}

func NewServerlessCoster() *serverless {
	return &serverless{}
}

func (e *serverless) TotalCost(costerCtx *CosterContext) Cost {
	serverlessPodsTotalCost := 0.
	timespanInHour := float64(costerCtx.TimeSpanSeconds) / time.Hour.Seconds()

	workloadKindTotalCost := map[string]float64{}
	for kind, workloadsSpec := range costerCtx.WorkloadsSpec {
		workloadKindTotalCost[kind] = 0
		if strings.ToLower(kind) == "daemonset" {
			continue
		}
		for nn, workloadSpec := range workloadsSpec {
			workloadPricing, err := costerCtx.Pricer.ServerlessPodPrice(workloadSpec)
			if err != nil {
				klog.Errorf("Failed to get ServerlessPodPrice for workload: %v, kind: %v, err: %v", nn, kind, err)
				continue
			}
			var workloadPrice float64
			if workloadPricing.Cost != "" {
				workloadPrice, err = strconv.ParseFloat(workloadPricing.Cost, 64)
				if err != nil {
					klog.V(3).Infof("Could not parse pod total cost price, workload: %v, kind: %v, err: %v", nn, kind, err)
					continue
				}
			}
			if math.IsNaN(workloadPrice) {
				klog.V(3).Infof("PodPrice is NaN. Setting to 0. workload: %v, kind: %v", nn, kind)
				workloadPrice = 0
			}
			workloadCost := workloadPrice * timespanInHour

			workloadKindTotalCost[kind] += workloadCost
			serverlessPodsTotalCost += workloadCost
		}
	}

	platformCost := costerCtx.Pricer.PlatformPrice(cloud.PlatformParameter{Platform: cloud.ServerlessKind})

	return Cost{
		TotalCost:              serverlessPodsTotalCost + platformCost.TotalPrice,
		ServerfulCost:          0,
		ServerlessCost:         serverlessPodsTotalCost,
		ServerfulPlatformCost:  0,
		ServerlessPlatformCost: platformCost.TotalPrice,
	}
}
