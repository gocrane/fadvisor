package defaultcloud

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/gocrane/fadvisor/pkg/consts"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cache"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprice"
	"github.com/gocrane/fadvisor/pkg/util"
)

type DefaultCloud struct {
	providerConfig *cloudprice.ProviderConfig

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

func NewDefaultCloud(config *cloudprice.ProviderConfig, cache cache.Cache) cloudprice.CloudPrice {
	return &DefaultCloud{
		providerConfig: config,
		cache:          cache,
	}
}

// UpdateConfigFromConfigMap update CustomPricing from configmap
func (tc *DefaultCloud) UpdateConfigFromConfigMap(conf map[string]string) (*cloudprice.CustomPricing, error) {
	return tc.providerConfig.UpdateConfigFromConfigMap(conf)
}

// GetConfig return CustomPricing
func (tc *DefaultCloud) GetConfig() (*cloudprice.CustomPricing, error) {
	return tc.providerConfig.GetConfig()
}

func (tc *DefaultCloud) getDefaultNodePrice(cfg *cloudprice.CustomPricing, node *v1.Node) (*cloudprice.Node, error) {
	usageType := "Default"
	insType, _ := util.GetInstanceType(node.Labels)
	region := cfg.Region
	cpuCores := node.Status.Capacity[v1.ResourceCPU]
	memory := node.Status.Capacity[v1.ResourceMemory]
	cpu := float64(cpuCores.Value())
	mem := float64(memory.Value())
	return &cloudprice.Node{
		BaseInstancePrice: cloudprice.BaseInstancePrice{
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

func (tc *DefaultCloud) GetNodesCost() (map[string]*cloudprice.Node, error) {
	nodes := make(map[string]*cloudprice.Node)
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

func (tc *DefaultCloud) computeNodeBreakdownCost(cfg *cloudprice.CustomPricing, node *v1.Node) (*cloudprice.Node, error) {
	return tc.getDefaultNodePrice(cfg, node)
}

func (tc *DefaultCloud) GetPodsCost() (map[string]*cloudprice.Pod, error) {
	pods := make(map[string]*cloudprice.Pod)

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
		podPrice := &cloudprice.Pod{
			BaseInstancePrice: nodePrice.BaseInstancePrice,
		}
		pods[key] = podPrice
	}
	return pods, nil
}

func (tc *DefaultCloud) GetNodesPricing() (map[string]*cloudprice.Price, error) {
	results := make(map[string]*cloudprice.Price)
	return results, nil
}
