package cloud

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"

	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
	"github.com/gocrane/fadvisor/pkg/spec"
	"github.com/gocrane/fadvisor/pkg/util"
)

type ProviderKind string

// todo: move the cloud to a staging src for a common lib for crane community
type Cloud interface {
	Pricer
	PodSpecConverter
	NodeSpecConverter
	spec.NodeFilter
	spec.PodFilter
	CloudCacher
	CloudPrice
}

type CloudCacher interface {
	WarmUp() error
	Refresh()
}

type PodSpecConverter interface {
	Pod2Spec(pod *v1.Pod) spec.CloudPodSpec
	Pod2ServerlessSpec(pod *v1.Pod) spec.CloudPodSpec
}

type NodeSpecConverter interface {
	Node2Spec(node *v1.Node) spec.CloudNodeSpec
}

type CloudConfig struct {
	CloudConfigFile string `json:"cloudConfigFile"`
	Provider        string `json:"provider"`
}

type CustomPricing struct {
	Region           string  `json:"region"`
	Provider         string  `json:"provider"`
	Description      string  `json:"description"`
	CpuHourlyPrice   float64 `json:"cpuHourlyPrice"`
	RamGBHourlyPrice float64 `json:"ramGBHourlyPrice"`
}

type PriceConfig struct {
	lock          sync.Mutex
	customPricing *CustomPricing
}

const (
	TencentCloud ProviderKind = "qcloud"
	DefaultCloud ProviderKind = "default"
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

func DetectProvider(node *v1.Node) ProviderKind {
	provider := node.Spec.ProviderID
	if strings.Contains(provider, "qcloud") {
		return TencentCloud
	} else {
		return DefaultCloud
	}
}

func NewProviderConfig(customPricing *CustomPricing) *PriceConfig {
	return &PriceConfig{
		customPricing: customPricing,
	}
}

// UpdateConfigFromConfigMap update CustomPricing from configmap
func (pc *PriceConfig) UpdateConfigFromConfigMap(priceConf map[string]string) (*CustomPricing, error) {
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
func (pc *PriceConfig) GetConfig() (*CustomPricing, error) {
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
