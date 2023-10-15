package cloud

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/gocrane/fadvisor/pkg/cache"
)

func TestRegisterCloudProvider(t *testing.T) {
	defer func() {
		// clear up
		defer providersMutex.Unlock()
		providers = make(map[ProviderKind]Factory)
	}()
	RegisterCloudProvider(TencentCloud, mockFactory)
	providersMutex.Lock()
	_, ok := providers[TencentCloud]
	if !ok {
		t.Errorf("RegisterCloudProvider() = not found registered cloud")
	}
}

type mockCloud struct {
	Cloud
}

type mockCache struct {
	cache.Cache
}

func mockFactory(cloudConfig io.Reader, priceConfig *PriceConfig, cache *cache.Cache) (Cloud, error) {
	return mockCloud{}, nil
}

func TestGetCloudProvider(t *testing.T) {
	tests := []struct {
		name        string
		kindName    ProviderKind
		cloudConfig io.Reader
		priceConfig *PriceConfig
		cache       cache.Cache
		want        Cloud
		wantErr     bool
		PreRegister bool
	}{
		{
			name:     "base",
			kindName: TencentCloud,
			cache:    mockCache{},
		},
		{
			name:        "found provider by name",
			kindName:    TencentCloud,
			cloudConfig: strings.NewReader("test"),
			priceConfig: &PriceConfig{},
			cache:       mockCache{},
			want:        mockCloud{},
			wantErr:     false,
			PreRegister: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				// clear up
				providersMutex.Lock()
				defer providersMutex.Unlock()
				providers = make(map[ProviderKind]Factory)
			}()
			if tt.PreRegister {
				RegisterCloudProvider(tt.kindName, mockFactory)
			}
			got, err := GetCloudProvider(tt.kindName, tt.cloudConfig, tt.priceConfig, &tt.cache)
			gotErr := (err != nil)
			if gotErr != tt.wantErr {
				t.Errorf("GetCloudProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCloudProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitCloudProvider(t *testing.T) {
	tests := []struct {
		name        string
		kindName    ProviderKind
		CloudOpts   CloudConfig
		priceConfig *PriceConfig
		cache       cache.Cache
		want        Cloud
		wantErr     bool
		PreRegister bool
	}{
		{
			name: "base",
			CloudOpts: CloudConfig{
				CloudConfigFile: t.TempDir(),
				Provider:        string(TencentCloud),
			},
			wantErr: true,
		},
		{
			name:      "not CloudConfigFile and cloud is nil",
			CloudOpts: CloudConfig{},
			wantErr:   true,
		},
		{
			name:     "base",
			kindName: TencentCloud,
			CloudOpts: CloudConfig{
				Provider: string(TencentCloud),
			},
			want:        mockCloud{},
			wantErr:     false,
			PreRegister: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				// clear up
				providersMutex.Lock()
				defer providersMutex.Unlock()
				providers = make(map[ProviderKind]Factory)
			}()
			if tt.PreRegister {
				RegisterCloudProvider(tt.kindName, mockFactory)
			}
			got, err := InitCloudProvider(tt.CloudOpts, tt.priceConfig, &tt.cache)
			gotErr := (err != nil)
			if gotErr != tt.wantErr {
				t.Errorf("InitCloudProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InitCloudProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}
