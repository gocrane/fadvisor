package util

import (
	v1 "k8s.io/api/core/v1"
)

func GetRegion(labels map[string]string) (string, bool) {
	if _, ok := labels[v1.LabelTopologyRegion]; ok { // Label as of 1.17
		return labels[v1.LabelTopologyRegion], true
	} else if _, ok := labels[v1.LabelZoneRegion]; ok { // deprecated label
		return labels[v1.LabelZoneRegion], true
	} else {
		return "", false
	}
}

func GetZone(labels map[string]string) (string, bool) {
	if _, ok := labels[v1.LabelTopologyZone]; ok { // Label as of 1.17
		return labels[v1.LabelTopologyZone], true
	} else if _, ok := labels[v1.LabelZoneFailureDomain]; ok { // deprecated label
		return labels[v1.LabelZoneFailureDomain], true
	} else {
		return "", false
	}
}

func GetInstanceType(labels map[string]string) (string, bool) {
	if _, ok := labels[v1.LabelInstanceType]; ok {
		return labels[v1.LabelInstanceType], true
	} else if _, ok := labels[v1.LabelInstanceTypeStable]; ok {
		return labels[v1.LabelInstanceTypeStable], true
	} else {
		return "", false
	}
}

func GetOperatingSystem(labels map[string]string) (string, bool) {
	betaLabel := "beta.kubernetes.io/os"
	if _, ok := labels[v1.LabelOSStable]; ok {
		return labels[v1.LabelOSStable], true
	} else if _, ok := lables[betaLabel]; ok {
		return lables[betaLabel], true
	} else {
		return "", false
	}
}
