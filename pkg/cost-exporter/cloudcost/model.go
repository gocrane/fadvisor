package cloudcost

import (
	"fmt"

	"github.com/gocrane/fadvisor/pkg/cache"
	"github.com/gocrane/fadvisor/pkg/cloud"
)

type ContainerAllocation struct {
	Key           string
	Pod           string
	Node          string
	Namespace     string
	CpuAllocation float64
	RamAllocation float64
}

/**
This is an idea from FinOps, because the traditional billing and pricing system for cloud resource is not adaptive to cloud native resource.
cost model is a way to estimate and breakdown the resource price to each container or pod.
!!! Note cost model is just used to estimate cost, not to replace the billing, because real billing depends on the billing system.
!!! model is an experimental implementation of the cost allocation and showback & chargeback from the FinOps.

   1. The simplest cost model is to estimate a resource price of all nodes or pods by the same price.
       for example, when compute costs, you can assume all container's cpu & ram unit price is the same, 2$ Core/Hour, 0.3$ Gib/Hour

   2. Advanced cost model is to estimate a resource price by cost breakdown.
   this theory is based on each cloud machine instance is different price with different instance type and charge type.
   so the containers in different node type or eks pod has different price
*/

// CostModel define a model
type CostModel interface {
	// GetNodesCost get all the real nodes price of kubernetes cluster with name as the key.
	GetNodesCost() (map[string]*cloud.Node, error)
	// GetPodsCost get the eks or tke pod price.
	// if the pod is in the real node of kubernetes cluster, then its price is computed from the instance backed the node by cost breakdown.
	// if the pod is in virtual node of kubernetes cluster, then its price came from the pod billing directly or the virtual machine instance price backed the the pod.
	// Note!!! In distributed cloud, the cluster master maybe in one cloud provider, but the nodes in the cluster maybe in multiple clouds from different cloud datasource-providers
	// so the node and pod pricing is crossing clouds, currently do not support it.
	// GetPodsCost, key is namespace/name
	GetPodsCost() (map[string]*cloud.Pod, error)

	// UpdateConfigFromConfigMap update CustomPricing from configmap
	UpdateConfigFromConfigMap(map[string]string) (*cloud.CustomPricing, error)
	// GetConfig return CustomPricing
	GetConfig() (*cloud.CustomPricing, error)

	// ContainerAllocation return the container resource allocation. resource allocation is max(request, usage)
	ContainerAllocation() (map[string]*ContainerAllocation, error)

	GetNodesPricing() (map[string]*cloud.Price, error)
}

type model struct {
	cache    cache.Cache
	provider cloud.CloudPrice
}

func NewCloudCost(cache cache.Cache, provider cloud.CloudPrice) CostModel {
	return &model{
		cache:    cache,
		provider: provider,
	}
}

func (m *model) GetNodesCost() (map[string]*cloud.Node, error) {
	return m.provider.GetNodesCost()
}

func (m *model) GetPodsCost() (map[string]*cloud.Pod, error) {
	return m.provider.GetPodsCost()
}

func (m *model) UpdateConfigFromConfigMap(cfg map[string]string) (*cloud.CustomPricing, error) {
	return m.provider.UpdateConfigFromConfigMap(cfg)
}

func (m *model) GetConfig() (*cloud.CustomPricing, error) {
	return m.provider.GetConfig()
}

//todo: this must first fetch container resource usage and request metric from prom. then compute the max of the two.
func (m *model) ContainerAllocation() (map[string]*ContainerAllocation, error) {
	return nil, fmt.Errorf("not implement")
}

func (m *model) GetNodesPricing() (map[string]*cloud.Price, error) {
	return m.provider.GetNodesPricing()
}
