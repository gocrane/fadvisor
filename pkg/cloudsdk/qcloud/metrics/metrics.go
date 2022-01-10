package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	ComponentRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "component_request_total",
			Help: "norm request total",
		},
		[]string{"module", "action", "status_code", "result_code", "version"},
	)
	ComponentRequestDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "component_request_duration_millisecond",
		Help:       "norm request durations in millisecond",
		Objectives: map[float64]float64{0.25: 0.05, 0.5: 0.05, 0.75: 0.01, 0.9: 0.01, 0.99: 0.001},
	}, []string{"module", "action", "status_code", "result_code", "version"})
)

func init() {
	prometheus.MustRegister(ComponentRequestTotal)
	prometheus.MustRegister(ComponentRequestDuration)
}

func ComponentRequestStatics(module string, action string, statusCode int, resultCode string, version string, startTime time.Time) {
	lan := float64(time.Since(startTime)) / float64(time.Millisecond)
	ComponentRequestTotal.WithLabelValues(module, action, fmt.Sprint(statusCode), resultCode, version).Inc()
	ComponentRequestDuration.WithLabelValues(module, action, fmt.Sprint(statusCode), resultCode, version).Observe(lan)
}

func ComponentWrongRequestStatics(module string, action string, errMsg string, version string) {
	ComponentRequestTotal.WithLabelValues(module, action, "0", errMsg, version).Inc()
}
