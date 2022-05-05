package qcloud

import (
	"github.com/gocrane/fadvisor/pkg/cloud"
)

type PriceStageFeeUnit struct {
	Nodes       int32
	PriceHourly float64
}

var (
	// 最大管理节点规模	定价 （元/小时）
	// https://cloud.tencent.com/document/product/457/68803
	defaultClusterPriceModel = []PriceStageFeeUnit{
		{5, 0.13},
		{20, 0.40},
		{50, 0.73},
		{100, 1.22},
		{200, 2.55},
		{500, 5.11},
		{1000, 9.30},
		{3000, 15.60},
		{5000, 28.04},
	}
)

type TKEPlatform struct {
}

func (tp *TKEPlatform) PlatformCost(cp cloud.PlatformParameter) *cloud.Prices {
	if cp.Nodes == nil {
		return &cloud.Prices{
			TotalPrice: defaultClusterPriceModel[0].PriceHourly,
		}
	}
	clusterRealNodes := *cp.Nodes
	for _, unit := range defaultClusterPriceModel {
		if clusterRealNodes <= unit.Nodes {
			return &cloud.Prices{
				TotalPrice: unit.PriceHourly,
			}
		}
	}
	return &cloud.Prices{
		TotalPrice: defaultClusterPriceModel[len(defaultClusterPriceModel)-1].PriceHourly,
	}
}
