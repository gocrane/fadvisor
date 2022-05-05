package util

import (
	v1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	resourcehelper "k8s.io/kubernetes/pkg/api/v1/resource"

	"github.com/gocrane/fadvisor/pkg/spec"
)

type PodFilterFunc func(pod *v1.Pod) bool

func PodsRequestsAndLimitsTotal(pods []*v1.Pod, filter PodFilterFunc, reverse bool) (v1.ResourceList, v1.ResourceList) {
	var reqMemTotal, reqCpuTotal, limMemTotal, limCpuTotal resource.Quantity
	for _, pod := range pods {
		if reverse {
			if !filter(pod) {
				continue
			}
		} else {
			if filter(pod) {
				continue
			}
		}
		req, limit := resourcehelper.PodRequestsAndLimits(pod)
		cpuReq := req[v1.ResourceCPU]
		reqCpuTotal.Add(cpuReq)
		memReq := req[v1.ResourceMemory]
		reqMemTotal.Add(memReq)
		cpuLim := limit[v1.ResourceCPU]
		limCpuTotal.Add(cpuLim)
		memLim := limit[v1.ResourceMemory]
		limMemTotal.Add(memLim)
	}
	reqTotal := v1.ResourceList{
		v1.ResourceCPU:    reqCpuTotal,
		v1.ResourceMemory: reqMemTotal,
	}
	limTotal := v1.ResourceList{
		v1.ResourceCPU:    limCpuTotal,
		v1.ResourceMemory: limMemTotal,
	}
	return reqTotal, limTotal

}

type NodeFilterFunc func(node *v1.Node) bool

func NodesResourceTotal(nodes []*v1.Node, filter NodeFilterFunc, reverse bool) v1.ResourceList {
	var cpuTotal, memTotal resource.Quantity
	for _, node := range nodes {
		if reverse {
			if !filter(node) {
				continue
			}
		} else {
			if filter(node) {
				continue
			}
		}

		cpu := node.Status.Capacity[v1.ResourceCPU]
		cpuTotal.Add(cpu)
		mem := node.Status.Capacity[v1.ResourceMemory]
		memTotal.Add(mem)
	}
	return v1.ResourceList{
		v1.ResourceCPU:    cpuTotal,
		v1.ResourceMemory: memTotal,
	}
}

func PodsSpecRequestsAndLimitsTotal(specs []spec.CloudPodSpec) (v1.ResourceList, v1.ResourceList) {
	var cpuReqTotal, memReqTotal, cpuLimTotal, memLimTotal resource.Quantity
	for _, pod := range specs {
		cpuReqTotal.Add(pod.Cpu)
		memReqTotal.Add(pod.Mem)
		cpuLimTotal.Add(pod.CpuLimit)
		memLimTotal.Add(pod.MemLimit)
	}
	reqTotal := v1.ResourceList{
		v1.ResourceCPU:    cpuReqTotal,
		v1.ResourceMemory: memReqTotal,
	}
	limTotal := v1.ResourceList{
		v1.ResourceCPU:    cpuLimTotal,
		v1.ResourceMemory: memLimTotal,
	}
	return reqTotal, limTotal
}
