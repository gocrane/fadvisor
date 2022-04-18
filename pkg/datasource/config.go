package datasource

import (
	"net/http"
	"time"
)

// PromConfig represents the config of prometheus
type PromConfig struct {
	Address            string
	Timeout            time.Duration
	KeepAlive          time.Duration
	InsecureSkipVerify bool
	Auth               ClientAuth

	QueryConcurrency            int
	BRateLimit                  bool
	MaxPointsLimitPerTimeSeries int
	FederatedClusterScope       bool
	// for thanos query, it must when use thanos as query source https://thanos.io/tip/components/query.md/#partial-response
	ThanosPartial bool
	ThanosDedup   bool
}

// ClientAuth holds the HTTP client identity info.
type ClientAuth struct {
	Username    string
	BearerToken string
	Password    string
}

// Apply applies the authentication identity info to the HTTP request headers
func (auth *ClientAuth) Apply(req *http.Request) {
	if auth == nil {
		return
	}

	if auth.BearerToken != "" {
		token := "Bearer " + auth.BearerToken
		req.Header.Add("Authorization", token)
	}

	if auth.Username != "" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}
}

// MockConfig represents the config of an in-memory provider, which is for demonstration or testing purpose.
type MockConfig struct {
	SeedFile string
}

type QCloudMonitorConfig struct {
	Credentials   `name:"credentials" value:"optional"`
	ClientProfile `name:"clientProfile" value:"optional"`
}

// Credentials use user defined SecretId and SecretKey
type Credentials struct {
	ClusterId string
	AppId     string
	SecretId  string
	SecretKey string
}

type ClientProfile struct {
	Debug                 bool
	DefaultLimit          int64
	DefaultLanguage       string
	DefaultTimeoutSeconds int
	Region                string
	DomainSuffix          string
	Scheme                string
}

type DataSourceType string

const (
	MockDataSource          DataSourceType = "mock"
	PrometheusDataSource    DataSourceType = "prom"
	MetricServerDataSource  DataSourceType = "metricserver"
	QCloudMonitorDataSource DataSourceType = "qcloudmonitor"
)
