package defaultcloud

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/gocrane/fadvisor/pkg/consts"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cache"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprovider"
	"github.com/gocrane/fadvisor/pkg/util"
)

const Name = "default"

type DefaultCloud struct {
	priceConfig *cloudprovider.PriceConfig

	cache cache.Cache
}

func (tc *DefaultCloud) OnNodeDelete(node *v1.Node) error {
	return nil
}

func (tc *DefaultCloud) OnNodeAdd(node *v1.Node) error {
	return nil
}

func (tc *DefaultCloud) OnNodeUpdate(old, new *v1.Node) error {
	return nil
}

func (tc *DefaultCloud) IsVirtualNode(node *v1.Node) bool {
	return false
}

func (tc *DefaultCloud) WarmUp() error {
	return nil
}

func (tc *DefaultCloud) Refresh() {
}

func NewDefaultCloud(config *cloudprovider.PriceConfig, cache cache.Cache) cloudprovider.CloudPrice {
	return &DefaultCloud{
		priceConfig: config,
		cache:       cache,
	}
}

// UpdateConfigFromConfigMap update CustomPricing from configmap
func (tc *DefaultCloud) UpdateConfigFromConfigMap(conf map[string]string) (*cloudprovider.CustomPricing, error) {
	return tc.priceConfig.UpdateConfigFromConfigMap(conf)
}

// GetConfig return CustomPricing
func (tc *DefaultCloud) GetConfig() (*cloudprovider.CustomPricing, error) {
	return tc.priceConfig.GetConfig()
}

func (tc *DefaultCloud) getDefaultNodePrice(cfg *cloudprovider.CustomPricing, node *v1.Node) (*cloudprovider.Node, error) {
	usageType := "Default"
	insType, _ := util.GetInstanceType(node.Labels)
	region := cfg.Region
	cpuCores := node.Status.Capacity[v1.ResourceCPU]
	memory := node.Status.Capacity[v1.ResourceMemory]
	cpu := float64(cpuCores.Value())
	mem := float64(memory.Value())
	return &cloudprovider.Node{
		BaseInstancePrice: cloudprovider.BaseInstancePrice{
			Cost:             fmt.Sprintf("%v", cfg.CpuHourlyPrice*cpu+cfg.RamGBHourlyPrice*mem/consts.GB),
			Cpu:              fmt.Sprintf("%v", cpu),
			CpuHourlyCost:    fmt.Sprintf("%v", cfg.CpuHourlyPrice),
			Ram:              fmt.Sprintf("%v", mem/consts.GB),
			RamBytes:         fmt.Sprintf("%v", mem),
			RamGBHourlyCost:  fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
			DefaultCpuPrice:  fmt.Sprintf("%v", cfg.CpuHourlyPrice),
			DefaultRamPrice:  fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
			UsageType:        usageType,
			UsesDefaultPrice: true,
			InstanceType:     insType,
			ProviderID:       node.Spec.ProviderID,
			Region:           region,
		},
	}, nil
}

func (tc *DefaultCloud) GetNodesCost() (map[string]*cloudprovider.Node, error) {
	nodes := make(map[string]*cloudprovider.Node)
	cfg, err := tc.GetConfig()
	if err != nil {
		return nodes, err
	}
	if cfg == nil {
		return nodes, fmt.Errorf("provider config is null")
	}

	nodeList := tc.cache.GetNodes()
	for _, node := range nodeList {

		newCnode, err := tc.computeNodeBreakdownCost(cfg, node)
		if err != nil {
			continue
		}
		nodes[node.Name] = newCnode
	}
	return nodes, nil
}

func (tc *DefaultCloud) computeNodeBreakdownCost(cfg *cloudprovider.CustomPricing, node *v1.Node) (*cloudprovider.Node, error) {
	return tc.getDefaultNodePrice(cfg, node)
}

func (tc *DefaultCloud) GetPodsCost() (map[string]*cloudprovider.Pod, error) {
	pods := make(map[string]*cloudprovider.Pod)

	cfg, err := tc.GetConfig()
	if err != nil {
		return pods, err
	}
	if cfg == nil {
		return pods, fmt.Errorf("provider config is null")
	}

	nodeList := tc.cache.GetNodes()
	nodesMap := make(map[string]*v1.Node)
	for _, node := range nodeList {
		nodesMap[node.Name] = node
	}
	podList := tc.cache.GetPods()
	for _, pod := range podList {
		key := klog.KObj(pod).String()

		nodeName := pod.Spec.NodeName
		node := nodesMap[nodeName]

		nodePrice, err := tc.computeNodeBreakdownCost(cfg, node)
		if err != nil {
			klog.Errorf("Failed to computeNodeBreakdownCost pod: %v, node: %v", klog.KObj(pod), klog.KObj(node))
			continue
		}
		podPrice := &cloudprovider.Pod{
			BaseInstancePrice: nodePrice.BaseInstancePrice,
		}
		pods[key] = podPrice
	}
	return pods, nil
}

func (tc *DefaultCloud) GetNodesPricing() (map[string]*cloudprovider.Price, error) {
	results := make(map[string]*cloudprovider.Price)
	return results, nil
}
