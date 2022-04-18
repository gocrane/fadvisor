package qcloud

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/gocrane/fadvisor/pkg/cloud"
)

type Pod2EKSSpecConverter interface {
	Pod2EKSSpecConverter(pod *v1.Pod) (v1.ResourceList, error)
}

type EKSPlatform struct {
}

// no platform cost now for eks
func (ep *EKSPlatform) PlatformCost(cp cloud.PlatformParameter) *cloud.Prices {
	return &cloud.Prices{
		TotalPrice:    0,
		DiscountPrice: pointer.Float64(0),
	}
}

const (
	// https://cloud.tencent.com/document/product/457/53030
	ValueNodeTypeEKLet = "eklet"

	labelNodeInstanceVersion   = "eks.tke.cloud.tencent.com/version"
	valueNodeInstanceVersionV2 = "v2"

	//labelEKSPodType = "tke.cloud.tencent.com/pod-type"

	EKSAnnoCpuType     = "eks.tke.cloud.tencent.com/cpu-type"
	EKSAnnoCpuQuantity = "eks.tke.cloud.tencent.com/cpu"
	EKSAnnoMemQuantity = "eks.tke.cloud.tencent.com/mem"
	EKSAnnoGpuType     = "eks.tke.cloud.tencent.com/gpu-type"
	EKSAnnoGpuCount    = "eks.tke.cloud.tencent.com/gpu-count"

	EKSCpuTypeValue_Intel = "intel"
	//EKSCpuTypeValue_Amd    = "amd"
	//EKSGpuTypeValue_V100   = "V100"
	//EKSGpuTypeValue_1_4_T4 = "1/4*T4"
	//EKSGpuTypeValue_1_2_T4 = "1/2*T4"
	//EKSGpuTypeValue_T4     = "T4"
)

func EKSPodCpuType(pod *v1.Pod) string {
	if pod.Annotations == nil {
		//default
		return EKSCpuTypeValue_Intel
	}
	return pod.Annotations[EKSAnnoCpuType]
}

func EKSPodGpuType(pod *v1.Pod) (bool, string) {
	if pod.Annotations == nil {
		return false, ""
	}
	t, ok := pod.Annotations[EKSAnnoGpuType]
	return ok, t
}

func EKSPodGpuCount(pod *v1.Pod) (bool, string) {
	if pod.Annotations == nil {
		return false, ""
	}
	t, ok := pod.Annotations[EKSAnnoGpuCount]
	return ok, t
}

func EKSPodCpuValue(pod *v1.Pod) (bool, string) {
	if pod.Annotations == nil {
		return false, ""
	}
	res, ok := pod.Annotations[EKSAnnoCpuQuantity]
	return ok, res
}

func EKSPodMemValue(pod *v1.Pod) (bool, string) {
	if pod.Annotations == nil {
		return false, ""
	}
	res, ok := pod.Annotations[EKSAnnoMemQuantity]
	return ok, res
}
