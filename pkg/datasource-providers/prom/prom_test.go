package prom

import (
	"testing"

	"github.com/gocrane/fadvisor/pkg/datasource"
)

func TestNewPrometheusClient(t *testing.T) {
	config := &datasource.PromConfig{
		Address:                     "",
		QueryConcurrency:            10,
		BRateLimit:                  true,
		MaxPointsLimitPerTimeSeries: 11000,
	}
	_, err := NewPrometheusClient(config)
	if err != nil {
		t.Fatal(err)
	}

}
