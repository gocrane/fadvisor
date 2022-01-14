package tencentcloud

import (
	"strings"
	"testing"
	"time"
)

var config = `
[credentials]
clusterId=cluster1
appId=app1
secretId=id1
secretKey=key1
[clientProfile]
debug=true
defaultLimit=1
defaultLanguage=CH
defaultTimeoutSeconds=10
region=shanghai
domainSuffix=cloud.tencent.com
scheme=http
`

func TestRegisterTencent(t *testing.T) {
	//cloudConfig io.Reader, priceConfig *cloudprovider.PriceConfig, cache *cache.Cache
	reader := strings.NewReader(config)
	clientConf, err := buildClientConfig(reader)
	if err != nil {
		t.Errorf("Got unexpected error %v", err)
	}
	if clientConf.Region != "shanghai" {
		t.Errorf("Expected region to be shanghai, got %s", clientConf.Region)
	}
	if clientConf.DefaultTimeout != 10*time.Second {
		t.Errorf("Expected timeout to be 10s, got %s", clientConf.DefaultTimeout)
	}
	if clientConf.Credential.GetQCloudCredential().SecretId != "id1" {
		t.Errorf("Expected secretId to be id1, got %s", clientConf.Credential.GetQCloudCredential().SecretKey)
	}
}
