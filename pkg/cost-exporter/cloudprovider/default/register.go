package defaultcloud

import (
	"io"

	"github.com/gocrane/fadvisor/pkg/cost-exporter/cache"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprovider"
)

func registerDefault(_ io.Reader, priceConfig *cloudprovider.PriceConfig, cache *cache.Cache) (cloudprovider.CloudPrice, error) {
	p := DefaultCloud{
		priceConfig: priceConfig,
		cache:       *cache,
	}
	return &p, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(Name, registerDefault)
}
