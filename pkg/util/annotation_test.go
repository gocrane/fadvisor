package util

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestGetRegion(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
		want1  bool
	}{
		{
			name: "base",
			labels: map[string]string{
				v1.LabelTopologyRegion: v1.LabelTopologyRegion,
			},
			want:  v1.LabelTopologyRegion,
			want1: true,
		},
		{
			name: "LabelZoneRegion",
			labels: map[string]string{
				v1.LabelZoneRegion: v1.LabelZoneRegion,
			},
			want:  v1.LabelZoneRegion,
			want1: true,
		},
		{
			name:   "not found",
			labels: map[string]string{},
			want:   "",
			want1:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetRegion(tt.labels)
			if got != tt.want {
				t.Errorf("GetRegion() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetRegion() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetZone(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
		want1  bool
	}{
		{
			name: "base",
			labels: map[string]string{
				v1.LabelTopologyZone: v1.LabelTopologyZone,
			},
			want:  v1.LabelTopologyZone,
			want1: true,
		},
		{
			name: "LabelZoneFailureDomain",
			labels: map[string]string{
				v1.LabelZoneFailureDomain: v1.LabelZoneFailureDomain,
			},
			want:  v1.LabelZoneFailureDomain,
			want1: true,
		},
		{
			name:   "not found",
			labels: map[string]string{},
			want:   "",
			want1:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetZone(tt.labels)
			if got != tt.want {
				t.Errorf("GetZone() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetZone() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetInstanceType(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
		want1  bool
	}{
		{
			name: "base",
			labels: map[string]string{
				v1.LabelInstanceType: v1.LabelInstanceType,
			},
			want:  v1.LabelInstanceType,
			want1: true,
		},
		{
			name: "LabelInstanceTypeStable",
			labels: map[string]string{
				v1.LabelInstanceTypeStable: v1.LabelInstanceTypeStable,
			},
			want:  v1.LabelInstanceTypeStable,
			want1: true,
		},
		{
			name:   "not found",
			labels: map[string]string{},
			want:   "",
			want1:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetInstanceType(tt.labels)
			if got != tt.want {
				t.Errorf("GetInstanceType() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetInstanceType() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetOperatingSystem(t *testing.T) {
	betaLabel := "beta.kubernetes.io/os"
	tests := []struct {
		name   string
		labels map[string]string
		want   string
		want1  bool
	}{
		{
			name: "base",
			labels: map[string]string{
				v1.LabelOSStable: v1.LabelOSStable,
			},
			want:  v1.LabelOSStable,
			want1: true,
		},
		{
			name: "betaLabel",
			labels: map[string]string{
				betaLabel: betaLabel,
			},
			want:  betaLabel,
			want1: true,
		},
		{
			name:   "not found",
			labels: map[string]string{},
			want:   "",
			want1:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetOperatingSystem(tt.labels)
			if got != tt.want {
				t.Errorf("GetOperatingSystem() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetOperatingSystem() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}


import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestGetRegion(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
		want1  bool
	}{
		{
			name: "base",
			labels: map[string]string{
				v1.LabelTopologyRegion: v1.LabelTopologyRegion,
			},
			want:  v1.LabelTopologyRegion,
			want1: true,
		},
		{
			name: "LabelZoneRegion",
			labels: map[string]string{
				v1.LabelZoneRegion: v1.LabelZoneRegion,
			},
			want:  v1.LabelZoneRegion,
			want1: true,
		},
		{
			name:   "not found",
			labels: map[string]string{},
			want:   "",
			want1:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetRegion(tt.labels)
			if got != tt.want {
				t.Errorf("GetRegion() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetRegion() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetZone(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
		want1  bool
	}{
		{
			name: "base",
			labels: map[string]string{
				v1.LabelTopologyZone: v1.LabelTopologyZone,
			},
			want:  v1.LabelTopologyZone,
			want1: true,
		},
		{
			name: "LabelZoneFailureDomain",
			labels: map[string]string{
				v1.LabelZoneFailureDomain: v1.LabelZoneFailureDomain,
			},
			want:  v1.LabelZoneFailureDomain,
			want1: true,
		},
		{
			name:   "not found",
			labels: map[string]string{},
			want:   "",
			want1:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetZone(tt.labels)
			if got != tt.want {
				t.Errorf("GetZone() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetZone() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetInstanceType(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
		want1  bool
	}{
		{
			name: "base",
			labels: map[string]string{
				v1.LabelInstanceType: v1.LabelInstanceType,
			},
			want:  v1.LabelInstanceType,
			want1: true,
		},
		{
			name: "LabelInstanceTypeStable",
			labels: map[string]string{
				v1.LabelInstanceTypeStable: v1.LabelInstanceTypeStable,
			},
			want:  v1.LabelInstanceTypeStable,
			want1: true,
		},
		{
			name:   "not found",
			labels: map[string]string{},
			want:   "",
			want1:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetInstanceType(tt.labels)
			if got != tt.want {
				t.Errorf("GetInstanceType() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetInstanceType() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetOperatingSystem(t *testing.T) {
	betaLabel := "beta.kubernetes.io/os"
	tests := []struct {
		name   string
		labels map[string]string
		want   string
		want1  bool
	}{
		{
			name: "base",
			labels: map[string]string{
				v1.LabelOSStable: v1.LabelOSStable,
			},
			want:  v1.LabelOSStable,
			want1: true,
		},
		{
			name: "betaLabel",
			labels: map[string]string{
				betaLabel: betaLabel,
			},
			want:  betaLabel,
			want1: true,
		},
		{
			name:   "not found",
			labels: map[string]string{},
			want:   "",
			want1:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetOperatingSystem(tt.labels)
			if got != tt.want {
				t.Errorf("GetOperatingSystem() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetOperatingSystem() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
