package coster

import (
	"math"
	"strconv"
	"time"

	"github.com/gocrane/fadvisor/pkg/cloud"

	"k8s.io/klog/v2"
)

// tke or ack
type serverful struct {
}

func NewServerfulCoster() *serverful {
	return &serverful{}

}

func (s *serverful) TotalCost(costerCtx *CosterContext) Cost {
	nodeTotalCost := 0.
	var realNodesNum int32 = 0
	timespanInHour := float64(costerCtx.TimeSpanSeconds) / time.Hour.Seconds()
	for name, nodeSpec := range costerCtx.NodesSpec {
		if nodeSpec.VirtualNode {
			continue
		}
		realNodesNum++
		nodePricing, err := costerCtx.Pricer.NodePrice(nodeSpec)
		if err != nil {
			klog.Errorf("Failed to get node %v price: %v", name, err)
			continue
		}
		var nodePrice float64
		if nodePricing.Cost != "" {
			nodePrice, err = strconv.ParseFloat(nodePricing.Cost, 64)
			if err != nil {
				klog.V(3).Infof("Could not parse total node price, node: %v, key: %v", name)
				continue
			}
		}
		if math.IsNaN(nodePrice) {
			klog.V(3).Infof("NodePrice is NaN. Setting to 0. node: %v, key: %v", name)
			nodePrice = 0
		}
		nodeTotalCost += nodePrice * timespanInHour
	}

	serverlessPodsTotalCost := 0.
	for name, podSpec := range costerCtx.PodsSpec {
		if !podSpec.Serverless {
			continue
		}

		podPricing, err := costerCtx.Pricer.ServerlessPodPrice(podSpec)
		if err != nil {
			klog.Errorf("Failed to get pod %v ServerlessPodPrice: %v", klog.KObj(podSpec.PodRef), err)
			continue
		}
		var podPrice float64
		if podPricing.Cost != "" {
			podPrice, err = strconv.ParseFloat(podPricing.Cost, 64)
			if err != nil {
				klog.V(3).Infof("Could not parse pod total cost price, pod: %v, err: %v", klog.KObj(podSpec.PodRef), err)
				continue
			}
		}
		if math.IsNaN(podPrice) {
			klog.V(3).Infof("NodePrice is NaN. Setting to 0. node: %v, key: %v", name)
			podPrice = 0
		}
		serverlessPodsTotalCost += podPrice * timespanInHour
	}
	serverfulPlatformCost := costerCtx.Pricer.PlatformPrice(cloud.PlatformParameter{Nodes: &realNodesNum, Platform: cloud.ServerfulKind})
	serverlessPlatformCost := costerCtx.Pricer.PlatformPrice(cloud.PlatformParameter{Nodes: &realNodesNum, Platform: cloud.ServerlessKind})

	return Cost{
		TotalCost:              nodeTotalCost + serverlessPodsTotalCost + serverfulPlatformCost.TotalPrice + serverlessPlatformCost.TotalPrice,
		ServerfulCost:          nodeTotalCost,
		ServerlessCost:         serverlessPodsTotalCost,
		ServerfulPlatformCost:  serverfulPlatformCost.TotalPrice,
		ServerlessPlatformCost: serverlessPlatformCost.TotalPrice,
	}
}
