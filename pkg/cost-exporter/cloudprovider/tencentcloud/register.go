package tencentcloud

import (
	"fmt"
	"io"
	"time"

	gcfg "gopkg.in/gcfg.v1"

	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog/v2"

	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/consts"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/credential"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cache"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprovider"
)

func registerTencent(cloudConfig io.Reader, priceConfig *cloudprovider.PriceConfig, cache *cache.Cache) (cloudprovider.CloudPrice, error) {
	var qcloudClientConfig *qcloud.QCloudClientConfig
	var err error
	if qcloudClientConfig, err = buildClientConfig(cloudConfig); err != nil {
		return nil, err
	}
	if qcloudClientConfig.Region == "" {
		if cache == nil {
			return nil, fmt.Errorf("client cache should not be empty")
		}
		nodes := (*cache).GetNodes()
		for _, node := range nodes {
			region := cloudprovider.DetectRegion(node)
			qcloudClientConfig.Region = region
			break
		}
	}
	if qcloudClientConfig.Region == "" {
		return nil, fmt.Errorf("no region info found. must specify region for provider %v", qcloudClientConfig.Region)
	}
	klog.V(4).Infof("Cloud config detail: %+v", qcloudClientConfig.QCloudClientProfile)
	p := NewTencentCloud(qcloudClientConfig, priceConfig, *cache)
	return p, nil
}

func buildClientConfig(cloudConfig io.Reader) (*qcloud.QCloudClientConfig, error) {
	var cfg CloudConfig
	if err := gcfg.FatalOnly(gcfg.ReadInto(&cfg, cloudConfig)); err != nil {
		klog.Errorf("Failed to read TencentCloud configuration file: %v", err)
		return nil, err
	}
	qccp := qcloud.QCloudClientProfile{
		Debug:           cfg.Debug,
		DefaultLanguage: cfg.DefaultLanguage,
		DefaultLimit:    cfg.DefaultLimit,
		DefaultTimeout:  time.Duration(cfg.DefaultTimeoutSeconds) * time.Second,
		Region:          cfg.Region,
		DomainSuffix:    cfg.DomainSuffix,
		Scheme:          cfg.Scheme,
	}

	cred := credential.NewQCloudCredential(cfg.ClusterId, cfg.AppId, cfg.SecretId, cfg.SecretKey, 1*time.Hour)
	qcc := &qcloud.QCloudClientConfig{
		RateLimiter:         flowcontrol.NewTokenBucketRateLimiter(5, 1),
		DefaultRetryCnt:     consts.MAXRETRY,
		QCloudClientProfile: qccp,
		Credential:          cred,
	}
	return qcc, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(Name, registerTencent)
}
