package cloud

import (
	"reflect"
	"strings"
	"testing"
)

func TestPriceConfig_UpdateConfigFromConfigMap(t *testing.T) {
	tests := []struct {
		name      string
		priceConf map[string]string
		pc        *PriceConfig
		want      *CustomPricing
		wantErr   bool
	}{
		{
			name:      "base",
			priceConf: map[string]string{},
			pc: &PriceConfig{
				customPricing: &CustomPricing{},
			},
			want:    &CustomPricing{},
			wantErr: false,
		},
		{
			name: "SetCustomPricing raise an error",
			priceConf: map[string]string{
				"CpuHourlyPrice": "",
			},
			pc: &PriceConfig{
				customPricing: &CustomPricing{},
			},
			want:    &CustomPricing{},
			wantErr: true,
		},
		{
			name: "SetCustomPricing ok",
			priceConf: map[string]string{
				"CpuHourlyPrice": "1.23",
			},
			pc: &PriceConfig{
				customPricing: &CustomPricing{},
			},
			want: &CustomPricing{
				CpuHourlyPrice: 1.23,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.pc.UpdateConfigFromConfigMap(tt.priceConf)
			gotErr := err != nil
			if gotErr != tt.wantErr {
				t.Errorf("PriceConfig.UpdateConfigFromConfigMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PriceConfig.UpdateConfigFromConfigMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetCustomPricing(t *testing.T) {
	tests := []struct {
		name       string
		obj        *CustomPricing
		filedName  string
		fieldValue string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "base",
			obj:        &CustomPricing{},
			wantErr:    true,
			wantErrMsg: "no such field:  in obj",
		},
		{
			name:      "value is not float type",
			filedName: "CpuHourlyPrice",
			obj: &CustomPricing{
				CpuHourlyPrice: 1.23,
			},
			wantErr:    true,
			wantErrMsg: "invalid syntax",
		},
		{
			name:       "value is float type",
			filedName:  "CpuHourlyPrice",
			fieldValue: "1.23",
			obj: &CustomPricing{
				CpuHourlyPrice: 1.23,
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name:       "value is string type",
			filedName:  "Provider",
			fieldValue: string(TencentCloud),
			obj: &CustomPricing{
				Provider: "qcloud",
			},
			wantErr:    false,
			wantErrMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetCustomPricing(tt.obj, tt.filedName, tt.fieldValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetCustomPricing() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("SetCustomPricing() error = %v, wantErrMsg %v", err.Error(), tt.wantErrMsg)
			}
		})
	}
}
