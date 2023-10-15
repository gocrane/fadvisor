package qcloud

import (
	"math"
	"reflect"
	"testing"

	"github.com/gocrane/fadvisor/pkg/cloud"
)

func TestTKEPlatform_PlatformCost(t *testing.T) {
	nodes := int32(1)
	nodes2 := int32(math.MaxInt32)
	tests := []struct {
		name string
		tp   *TKEPlatform
		cp   cloud.PlatformParameter
		want *cloud.Prices
	}{
		{
			name: "base",
			cp:   cloud.PlatformParameter{},
			want: &cloud.Prices{
				TotalPrice: defaultClusterPriceModel[0].PriceHourly,
			},
		},
		{
			name: "clusterRealNodes is less than default unit nodes",
			cp: cloud.PlatformParameter{
				Nodes: &nodes,
			},
			want: &cloud.Prices{
				TotalPrice: defaultClusterPriceModel[0].PriceHourly,
			},
		},
		{
			name: "clusterRealNodes is larger than default unit nodes",
			cp: cloud.PlatformParameter{
				Nodes: &nodes2,
			},
			want: &cloud.Prices{
				TotalPrice: defaultClusterPriceModel[len(defaultClusterPriceModel)-1].PriceHourly,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tp.PlatformCost(tt.cp); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TKEPlatform.PlatformCost() = %v, want %v", got, tt.want)
			}
		})
	}
}
