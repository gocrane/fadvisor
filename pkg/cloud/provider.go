package cloud

import (
	v1 "k8s.io/api/core/v1"
)

type PriceItem struct {
	UnitPrice                   *float64 `json:"unitPrice,omitempty" name:"unitPrice"`
	ChargeUnit                  *string  `json:"chargeUnit,omitempty" name:"chargeUnit"`
	OriginalPrice               *float64 `json:"originalPrice,omitempty" name:"originalPrice"`
	DiscountPrice               *float64 `json:"discountPrice,omitempty" name:"discountPrice"`
	Discount                    *float64 `json:"discount,omitempty" name:"discount"`
	UnitPriceDiscount           *float64 `json:"unitPriceDiscount,omitempty" name:"unitPriceDiscount"`
	UnitPriceSecondStep         *float64 `json:"unitPriceSecondStep,omitempty" name:"unitPriceSecondStep"`
	UnitPriceDiscountSecondStep *float64 `json:"unitPriceDiscountSecondStep,omitempty" name:"unitPriceDiscountSecondStep"`
	UnitPriceThirdStep          *float64 `json:"unitPriceThirdStep,omitempty" name:"unitPriceThirdStep"`
	UnitPriceDiscountThirdStep  *float64 `json:"unitPriceDiscountThirdStep,omitempty" name:"unitPriceDiscountThirdStep"`
	OriginalPriceThreeYear      *float64 `json:"originalPriceThreeYear,omitempty" name:"originalPriceThreeYear"`
	DiscountPriceThreeYear      *float64 `json:"discountPriceThreeYear,omitempty" name:"discountPriceThreeYear"`
	DiscountThreeYear           *float64 `json:"discountThreeYear,omitempty" name:"discountThreeYear"`
	OriginalPriceFiveYear       *float64 `json:"originalPriceFiveYear,omitempty" name:"originalPriceFiveYear"`
	DiscountPriceFiveYear       *float64 `json:"discountPriceFiveYear,omitempty" name:"discountPriceFiveYear"`
	DiscountFiveYear            *float64 `json:"discountFiveYear,omitempty" name:"discountFiveYear"`
	OriginalPriceOneYear        *float64 `json:"originalPriceOneYear,omitempty" name:"originalPriceOneYear"`
	DiscountPriceOneYear        *float64 `json:"discountPriceOneYear,omitempty" name:"discountPriceOneYear"`
	DiscountOneYear             *float64 `json:"discountOneYear,omitempty" name:"discountOneYear"`
}

type Price struct {
	InstanceType string     `json:"instanceType"`
	ChargeType   string     `json:"chargeType"`
	Memory       string     `json:"memory"`
	VCpu         string     `json:"vcpu"`
	CvmPrice     *PriceItem `json:"cvmPrice,omitempty"`
}

// cross cloud pricing
type CloudPrice interface {
	// UpdateConfigFromConfigMap update CustomPricing from configmap
	UpdateConfigFromConfigMap(map[string]string) (*CustomPricing, error)
	// GetConfig return CustomPricing
	GetConfig() (*CustomPricing, error)
	// GetNodeCost a model to compute each node cpu and ram unit price cost.
	/**
	  This is an idea from FinOps, because the traditional billing and pricing system for cloud resource is not adaptive to cloud native resource.
	  cost model is a way to estimate and breakdown the resource price to each container or pod.
	  !!! Note cost model is just used to estimate cost not to replace the billing, because real billing depends on the billing system.
	  !!! model is an experimental implementation of the cost allocation and showback & chargeback from the FinOps.

	     1. The simplest cost model is to estimate a resource price of all nodes or pods by the same price.
	         for example, when compute costs, you can assume all container's cpu & ram unit price is the same, 2$ Core/Hour, 0.3$ Gib/Hour

	     2. Advanced cost model is to estimate a resource price by cost breakdown.
	     this theory is based on each cloud machine instance is different price with different instance type and charge type.
	     so the containers in different node type or eks pod has different price
	*/
	// GetNodesCost, key is node name
	// GetNodesCost get all the real nodes price of kubernetes cluster.
	GetNodesCost() (map[string]*Node, error)
	// GetPodsCost get the eks or tke pod price.
	// if the pod is in the real node of kubernetes cluster, then its price is computed from the instance backed the node by cost breakdown.
	// if the pod is in virtual node of kubernetes cluster, then its price came from the pod billing directly or the virtual machine instance price backed the the pod.
	// Note!!! In distributed cloud, the cluster master maybe in one cloud provider, but the nodes in the cluster maybe in multiple clouds from different cloud datasource-providers
	// so the node and pod pricing is crossing clouds, currently do not support it.
	// GetPodsCost, key is namespace/name
	// This interface is better for unified real node or vk node, because we get pod costs, then we get container costs too.
	GetPodsCost() (map[string]*Pod, error)

	// OnNodeDelete
	OnNodeDelete(node *v1.Node) error
	// OnNodeAdd
	OnNodeAdd(node *v1.Node) error
	// OnNodeUpdate
	OnNodeUpdate(old, new *v1.Node) error

	// IsVirtualNode detects the node is virtual node.
	IsVirtualNode(node *v1.Node) bool

	WarmUp() error

	Refresh()

	GetNodesPricing() (map[string]*Price, error)
}

type ChargeType string

type BaseInstancePrice struct {
	DiscountedCost   string `json:"discountedHourlyCost"`
	Cost             string `json:"hourlyCost"`
	Cpu              string `json:"cpu"`
	CpuHourlyCost    string `json:"cpuHourlyCost"`
	Ram              string `json:"ram"`
	RamBytes         string `json:"ramBytes"`
	RamGBHourlyCost  string `json:"ramGBHourlyCost"`
	UsesDefaultPrice bool   `json:"usesDefaultPrice"`
	// Used to compute an implicit CPU Core/Hr price when CPU pricing is not provided.
	DefaultCpuPrice string `json:"defaultCpuPrice"`
	// Used to compute an implicit RAM GB/Hr price when RAM pricing is not provided.
	DefaultRamPrice string `json:"defaultRamPrice"`
	// Default or ChargeType
	UsageType    string `json:"usageType"`
	InstanceType string `json:"instanceType,omitempty"`
	Region       string `json:"region,omitempty"`
	ProviderID   string `json:"providerID,omitempty"`
}

type Node struct {
	BaseInstancePrice
}

type Pod struct {
	BaseInstancePrice
}
