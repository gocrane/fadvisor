package cloudprice

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
	"github.com/gocrane/fadvisor/pkg/util"
	v1 "k8s.io/api/core/v1"
)

type CustomPricing struct {
	Region           string  `json:"region"`
	Provider         string  `json:"provider"`
	Description      string  `json:"description"`
	CpuHourlyPrice   float64 `json:"cpuHourlyPrice"`
	RamGBHourlyPrice float64 `json:"ramGBHourlyPrice"`
}

type ProviderConfig struct {
	lock          sync.Mutex
	customPricing *CustomPricing
}

type ProviderType string

const (
	TencentCloud ProviderType = "qcloud"
	DefaultCloud ProviderType = "default"
)

func DetectRegion(node *v1.Node) string {
	regionStr, _ := util.GetRegion(node.Labels)
	provider := DetectProvider(node)
	if provider == TencentCloud {
		if regionStruct, ok := qcloud.ShortName2region[regionStr]; ok {
			return regionStruct.Region
		} else {
			return ""
		}
	}
	return regionStr
}

func DetectProvider(node *v1.Node) ProviderType {
	provider := node.Spec.ProviderID
	if strings.Contains(provider, "qcloud") {
		return TencentCloud
	} else {
		return DefaultCloud
	}
}

func NewProviderConfig(customPricing *CustomPricing) *ProviderConfig {
	return &ProviderConfig{
		customPricing: customPricing,
	}
}

// UpdateConfigFromConfigMap update CustomPricing from configmap
func (pc *ProviderConfig) UpdateConfigFromConfigMap(priceConf map[string]string) (*CustomPricing, error) {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	for k, v := range priceConf {
		kUpper := strings.Title(k)
		err := SetCustomPricing(pc.customPricing, kUpper, v)
		if err != nil {
			return pc.customPricing, err
		}
	}
	return pc.customPricing, nil
}

// GetConfig return CustomPricing
func (pc *ProviderConfig) GetConfig() (*CustomPricing, error) {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	return pc.customPricing, nil
}

func SetCustomPricing(obj *CustomPricing, name string, value string) error {
	structValue := reflect.ValueOf(obj).Elem()
	structFieldValue := structValue.FieldByName(name)

	if !structFieldValue.IsValid() {
		return fmt.Errorf("no such field: %s in obj", name)
	}

	if !structFieldValue.CanSet() {
		return fmt.Errorf("cannot set %s field value", name)
	}

	structFieldType := structFieldValue.Type()
	val := reflect.ValueOf(value)
	if structFieldType != val.Type() {
		return fmt.Errorf("provided value type didn't match custom pricing field type")
	}
	if structFieldValue.Kind() == reflect.Float64 || structFieldValue.Kind() == reflect.Float32 {
		t, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		val = reflect.ValueOf(t)
	}
	structFieldValue.Set(val)
	return nil
}
