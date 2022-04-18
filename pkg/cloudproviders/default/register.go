package defaultcloud

import (
	"io"

	"github.com/gocrane/fadvisor/pkg/cache"
	"github.com/gocrane/fadvisor/pkg/cloud"
)

func registerDefault(_ io.Reader, priceConfig *cloud.PriceConfig, cache *cache.Cache) (cloud.Cloud, error) {
	p := DefaultCloud{
		priceConfig: priceConfig,
		cache:       *cache,
	}
	return &p, nil
}

func init() {
	cloud.RegisterCloudProvider(Name, registerDefault)
}
