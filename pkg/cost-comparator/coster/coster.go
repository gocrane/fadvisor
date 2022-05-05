package coster

import (
	"github.com/gocrane/fadvisor/pkg/cloud"
	"k8s.io/apimachinery/pkg/types"

	"github.com/gocrane/fadvisor/pkg/spec"
)

// CosterContext used to compute cost given workload and node spec portrait, and time span, cloud pricer
type CosterContext struct {
	// time seconds
	TimeSpanSeconds  int64
	Discount         *float64
	PodsSpec         map[string]spec.CloudPodSpec
	NodesSpec        map[string]spec.CloudNodeSpec
	WorkloadsSpec    map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ spec.CloudPodSpec
	WorkloadsRecSpec map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ *spec.WorkloadRecommendedData
	Pricer           cloud.Pricer
}

type Cost struct {
	TotalCost              float64
	ServerfulCost          float64
	ServerlessCost         float64
	ServerfulPlatformCost  float64
	ServerlessPlatformCost float64
}

type RecommendedCost struct {
	TotalCost    float64
	PlatformCost float64
	WorkloadCost float64
}

// Coster to compute cost by given context, different cloud with different platform has different pricings.
// tke, eks, ack, ask. or hybrid
// There are two available Costers now, serverless and serverful Coster.
// 1. serverful coster only use NodesSpec to compute costs.
// 2. serverlesss coster only use WorkloadsSpec to compute costs.
// todo: later may consider using a building pattern way to construct Coster, decorator pattern by functional programming is better way, because this is no state computing

func (c *Cost) Add(other Cost) {
	c.TotalCost += other.TotalCost
	c.ServerfulCost += other.ServerfulCost
	c.ServerlessCost += other.ServerlessCost
	c.ServerlessPlatformCost += other.ServerlessPlatformCost
	c.ServerfulPlatformCost += other.ServerfulPlatformCost
}

func (rc *RecommendedCost) Add(other *RecommendedCost) {
	rc.TotalCost += other.TotalCost
	rc.PlatformCost += other.PlatformCost
	rc.WorkloadCost += other.WorkloadCost
}
