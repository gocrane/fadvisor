package spec

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type PodFilter interface {
	// IsServerlessPod return if the pod is serverless pod, which means this pod is in virtual kubelet. It is controlled by kubernetes control panel.
	// for example, TencentCloud eks pod, or AliCloud eci pod
	IsServerlessPod(pod *v1.Pod) bool
}

type NodeFilter interface {
	// IsVirtualNode return the node is running virtual kubelet
	// for example, TencentCloud eks eklet, or AliCloud virtual kubelet
	IsVirtualNode(node *v1.Node) bool
}

type CloudPodSpec struct {
	// only pod template is useful, ignore name, now we have not completely know the struct like
	PodRef   *v1.Pod
	Workload *unstructured.Unstructured
	Cpu      resource.Quantity
	Mem      resource.Quantity
	CpuLimit resource.Quantity
	MemLimit resource.Quantity
	Zone     string
	// v100，t4，amd，Default：intel
	MachineArch   string
	Gpu           resource.Quantity
	PodChargeType string
	// time span
	TimeSpan uint64
	// replicas, for pod value is 1, for workload, value is replicas of the workload spec
	GoodsNum uint64
	// serverless pod or not
	Serverless bool

	QoSClass v1.PodQOSClass
}

type CloudNodeSpec struct {
	NodeRef      *v1.Node
	Cpu          resource.Quantity
	Mem          resource.Quantity
	Gpu          resource.Quantity
	GpuType      string
	InstanceType string
	ChargeType   string
	Zone         string
	Region       string
	// virtual node or not
	VirtualNode bool
}

type WorkloadRecommendedData struct {
	RecommendedSpec          CloudPodSpec
	PercentRecommendedSpec   *CloudPodSpec
	MaxRecommendedSpec       *CloudPodSpec
	MaxMarginRecommendedSpec *CloudPodSpec
	Containers               map[string]*ContainerRecommendedData
}

type Statistic struct {
	Percentile     *float64
	Max            *float64
	MaxRecommended *float64
	Recommended    *float64
}

type ContainerRecommendedData struct {
	Cpu *Statistic
	Mem *Statistic
}
