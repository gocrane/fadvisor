package cloud

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetectRegion(t *testing.T) {
	tests := []struct {
		name string
		node *v1.Node
		want string
	}{
		{
			name: "base",
			node: &v1.Node{},
			want: "",
		},
		{
			name: "qcloud with no node labels",
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "qcloud://01",
				},
			},
			want: "",
		},
		{
			name: "qcloud with node labels",
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "qcloud://01",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.LabelTopologyRegion: "gz",
					},
				},
			},
			want: "ap-guangzhou",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectRegion(tt.node); got != tt.want {
				t.Errorf("DetectRegion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name string
		node *v1.Node
		want ProviderKind
	}{
		{
			name: "base",
			node: &v1.Node{},
			want: "default",
		},
		{
			name: "qcloud",
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "qcloud://01",
				},
			},
			want: "qcloud",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectProvider(tt.node); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DetectProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewProviderConfig(t *testing.T) {
	tests := []struct {
		name          string
		customPricing *CustomPricing
		want          *PriceConfig
	}{
		{
			name:          "base",
			customPricing: &CustomPricing{},
			want: &PriceConfig{
				customPricing: &CustomPricing{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewProviderConfig(tt.customPricing); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewProviderConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPriceConfig_UpdateConfigFromConfigMap(t *testing.T) {
	tests := []struct {
		name      string
		pc        *PriceConfig
		priceConf map[string]string
		want      *CustomPricing
		wantErr   bool
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.pc.UpdateConfigFromConfigMap(tt.priceConf)
			if (err != nil) != tt.wantErr {
				t.Errorf("PriceConfig.UpdateConfigFromConfigMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PriceConfig.UpdateConfigFromConfigMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPriceConfig_GetConfig(t *testing.T) {
	tests := []struct {
		name    string
		pc      *PriceConfig
		want    *CustomPricing
		wantErr error
	}{
		{
			name: "base",
			pc: &PriceConfig{
				customPricing: &CustomPricing{},
			},
			want:    &CustomPricing{},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.pc.GetConfig()
			if err != tt.wantErr {
				t.Errorf("PriceConfig.GetConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PriceConfig.GetConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetCustomPricing(t *testing.T) {
	tests := []struct {
		testName   string
		obj        *CustomPricing
		name       string
		value      string
		wantErr    bool
		wantErrMsg string
	}{
		{
			testName:   "base",
			obj:        &CustomPricing{},
			wantErr:    true,
			wantErrMsg: "no such field:  in obj",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetCustomPricing(tt.obj, tt.name, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetCustomPricing() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err.Error() != tt.wantErrMsg {
				t.Errorf("SetCustomPricing() error = %v, wantErrMsg %v", err.Error(), tt.wantErrMsg)
			}
		})
	}
}
