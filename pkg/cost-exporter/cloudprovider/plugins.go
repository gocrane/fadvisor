package cloudprovider

import (
	"fmt"
	"io"
	"os"
	"sync"

	"k8s.io/klog/v2"

	"github.com/gocrane/fadvisor/pkg/cost-exporter/cache"
)

// Factory is a function that returns a cloudsdk.Interface.
// The config parameter provides an io.Reader handler to the factory in
// order to load specific configurations. If no configuration is provided
// the parameter is nil.
type Factory func(cloudConfig io.Reader, priceConfig *PriceConfig, cache *cache.Cache) (CloudPrice, error)

// All registered price providers.
var (
	providersMutex sync.Mutex
	providers      = make(map[string]Factory)
)

// RegisterCloudProvider registers a cloudsdk.Factory by name.  This
// is expected to happen during app startup.
func RegisterCloudProvider(name string, cloud Factory) {
	providersMutex.Lock()
	defer providersMutex.Unlock()
	if _, found := providers[name]; found {
		klog.Fatalf("price provider %q was registered twice", name)
	}
	klog.V(1).Infof("Registered price provider %q", name)
	providers[name] = cloud
}

// GetCloudProvider creates an instance of the named price provider, or nil if
// the name is unknown.  The error return is only used if the named provider
// was known but failed to initialize. The config parameter specifies the
// io.Reader handler of the configuration file for the price provider, or nil
// for no configuration.
func GetCloudProvider(name string, cloudConfig io.Reader, priceConfig *PriceConfig, cache *cache.Cache) (CloudPrice, error) {
	providersMutex.Lock()
	defer providersMutex.Unlock()
	f, found := providers[name]
	if !found {
		return nil, nil
	}
	return f(cloudConfig, priceConfig, cache)
}

// InitPriceProvider creates a price provider instance.
func InitPriceProvider(CloudOpts CloudConfig, priceConfig *PriceConfig, cache *cache.Cache) (CloudPrice, error) {
	var cloud CloudPrice
	var err error

	if CloudOpts.CloudConfigFile != "" {
		var cloudConfig *os.File
		cloudConfig, err = os.Open(CloudOpts.CloudConfigFile)
		if err != nil {
			klog.Fatalf("Couldn't open cloud provider configuration %s: %#v",
				CloudOpts.CloudConfigFile, err)
		}

		defer cloudConfig.Close()
		cloud, err = GetCloudProvider(CloudOpts.Provider, cloudConfig, priceConfig, cache)
	} else {
		// Pass explicit nil so plugins can actually check for nil. See
		// "Why is my nil error value not equal to nil?" in golang.org/doc/faq.
		cloud, err = GetCloudProvider(CloudOpts.Provider, nil, priceConfig, cache)
	}
	if err != nil {
		return nil, fmt.Errorf("could not init price provider %q: %v", CloudOpts.Provider, err)
	}
	if cloud == nil {
		return nil, fmt.Errorf("unknown price provider %q", CloudOpts.Provider)
	}

	return cloud, nil
}
