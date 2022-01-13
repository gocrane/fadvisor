package cvm

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"

	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/consts"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/credential"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/metrics"
)

type CVM interface {
	DescribeZoneInstanceConfigInfos() ([]*cvm.InstanceTypeQuotaItem, error)
}

type CVMClient struct {
	clientLock    sync.Mutex
	client        *cvm.Client
	defaultClient *cvm.Client
	config        *qcloud.QCloudClientConfig
}

type retryFunc func(request interface{}) (interface{}, error)

func NewCVMClient(qcc *qcloud.QCloudClientConfig) *CVMClient {

	return &CVMClient{
		config: qcc,
	}
}

func (qcc *CVMClient) getCVMDomain() string {
	return fmt.Sprintf("%v.%v", "cvm", qcc.config.DomainSuffix)
}

func (qcc *CVMClient) UpdateCredential(cred credential.QCloudCredential) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Credential = cred
}

func (qcc *CVMClient) ExponentialRetryCall(retryCnt int, f retryFunc, request interface{}) (interface{}, error) {
	var err error
	var resp interface{}

	// blocking
	qcc.config.RateLimiter.Accept()

	resp, err = f(request)
	if err == nil {
		return resp, nil
	}
	for i := 1; i <= retryCnt; i++ {
		klog.Errorf("qcloudClient tencent cloud api failed, retrying %v times, qps:%v, err: %v", i, qcc.config.RateLimiter.QPS(), err)
		randInt := rand.IntnRange(2<<i, 2<<(i+1)+1)
		sleepTime := time.Duration(randInt) * time.Second
		time.Sleep(sleepTime)
		qcc.config.RateLimiter.Accept()
		resp, err = f(request)
		if err == nil {
			return resp, nil
		}
	}
	return nil, fmt.Errorf("qcloudClient tencent cloud api retry failed after retry %v times, err: %s", retryCnt, err)
}

func (qcc *CVMClient) UpdateCred(cred credential.QCloudCredential) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Credential = cred
}

func (qcc *CVMClient) UpdateCustomCredential(id, secret string) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Credential.UpdateQCloudCustomCredential(id, secret)
}

func (qcc *CVMClient) EnableDebug() bool {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Debug = true
	return qcc.config.Debug
}

func (qcc *CVMClient) DebugModeNoLock() bool {
	return qcc.config.Debug
}

// getQCloudCredential return credential assumed from norm or user custom
func (qcc *CVMClient) getQCloudCredential() *common.Credential {
	return qcc.config.Credential.GetQCloudCredential()
}

// GetAllRegions
func (qcc *CVMClient) getAllRegions() ([]*cvm.RegionInfo, error) {
	req := cvm.NewDescribeRegionsRequest()
	var regions []*cvm.RegionInfo
	cli, err := qcc.getDefaultClient()
	if err != nil {
		return regions, err
	}
	resp, err := cli.DescribeRegions(req)
	if err != nil {
		return regions, err
	}
	return resp.Response.RegionSet, nil
}

func (qcc *CVMClient) getDefaultClient() (*cvm.Client, error) {
	cred := qcc.getQCloudCredential()
	var err error

	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()

	if qcc.defaultClient == nil {
		prof := profile.NewClientProfile()
		prof.Language = qcc.config.DefaultLanguage
		prof.Debug = qcc.config.Debug
		prof.HttpProfile.Endpoint = qcc.getCVMDomain()
		prof.HttpProfile.Scheme = qcc.config.Scheme
		qcc.defaultClient, err = cvm.NewClient(cred, "", prof)
		if err != nil {
			return qcc.defaultClient, err
		}
	}
	qcc.defaultClient.WithCredential(cred)
	if qcc.config.Debug {
		SecretId := cred.GetSecretId()
		SecretKey := cred.GetSecretKey()
		Token := cred.GetToken()
		klog.Infof("qcloudClient cvm region: %v, scheme: %v, domain: %v client credential: %s, %s, %s\n", qcc.config.Region, qcc.config.Scheme, qcc.getCVMDomain(), SecretId, SecretKey, Token)
	}
	return qcc.defaultClient, nil
}

func (qcc *CVMClient) getClient() (*cvm.Client, error) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()

	cred := qcc.getQCloudCredential()
	var err error
	if qcc.client == nil {
		prof := profile.NewClientProfile()
		prof.Language = qcc.config.DefaultLanguage
		prof.Debug = qcc.config.Debug
		prof.HttpProfile.Endpoint = qcc.getCVMDomain()
		prof.HttpProfile.Scheme = qcc.config.Scheme
		qcc.client, err = cvm.NewClient(cred, qcc.config.Region, prof)
		if err != nil {
			return qcc.client, err
		}
	}
	if qcc.config.Debug {
		SecretId := cred.GetSecretId()
		SecretKey := cred.GetSecretKey()
		Token := cred.GetToken()
		klog.Infof("qcloudClient cvm region: %v, scheme: %v, domain: %v client credential: %s, %s, %s\n", qcc.config.Region, qcc.config.Scheme, qcc.getCVMDomain(), SecretId, SecretKey, Token)
	}
	return qcc.client, nil
}

func (qcc *CVMClient) InquiryDescribeInstancesWithRetry(cli *cvm.Client, req *cvm.DescribeInstancesRequest) (*cvm.DescribeInstancesResponse, error) {
	resp, err := qcc.ExponentialRetryCall(qcc.config.DefaultRetryCnt, func(request interface{}) (interface{}, error) {
		req := request.(*cvm.DescribeInstancesRequest)
		start := time.Now()
		resp, err := cli.DescribeInstances(req)
		if err != nil {
			metrics.ComponentWrongRequestStatics(req.GetDomain(), req.GetAction(), err.Error(), req.GetVersion())
		} else {
			metrics.ComponentRequestStatics(req.GetDomain(), req.GetAction(), 200, "", req.GetVersion(), start)
		}
		if qcc.config.Debug {
			if resp != nil {
				out, _ := json.Marshal(resp)
				klog.Info(string(out))
			}
		}
		return resp, err
	}, req)
	if err != nil {
		return nil, err
	}
	return resp.(*cvm.DescribeInstancesResponse), nil
}

func (qcc *CVMClient) InquiryPriceRunInstancesWithRetry(cli *cvm.Client, req *cvm.InquiryPriceRunInstancesRequest) (*cvm.InquiryPriceRunInstancesResponse, error) {
	resp, err := qcc.ExponentialRetryCall(qcc.config.DefaultRetryCnt, func(request interface{}) (interface{}, error) {
		req := request.(*cvm.InquiryPriceRunInstancesRequest)
		start := time.Now()
		resp, err := cli.InquiryPriceRunInstances(req)
		if err != nil {
			metrics.ComponentWrongRequestStatics(req.GetDomain(), req.GetAction(), err.Error(), req.GetVersion())
		} else {
			metrics.ComponentRequestStatics(req.GetDomain(), req.GetAction(), 200, "", req.GetVersion(), start)
		}
		if qcc.config.Debug {
			if resp != nil {
				out, _ := json.Marshal(resp)
				klog.Info(string(out))
			}
		}
		return resp, err
	}, req)
	if err != nil {
		return nil, err
	}
	return resp.(*cvm.InquiryPriceRunInstancesResponse), nil
}

func (qcc *CVMClient) DescribeZoneInstanceConfigInfosWithRetry(cli *cvm.Client, req *cvm.DescribeZoneInstanceConfigInfosRequest) (*cvm.DescribeZoneInstanceConfigInfosResponse, error) {
	resp, err := qcc.ExponentialRetryCall(qcc.config.DefaultRetryCnt, func(request interface{}) (interface{}, error) {
		req := request.(*cvm.DescribeZoneInstanceConfigInfosRequest)
		start := time.Now()
		resp, err := cli.DescribeZoneInstanceConfigInfos(req)
		if err != nil {
			metrics.ComponentWrongRequestStatics(req.GetDomain(), req.GetAction(), err.Error(), req.GetVersion())
		} else {
			metrics.ComponentRequestStatics(req.GetDomain(), req.GetAction(), 200, "", req.GetVersion(), start)
		}
		return resp, err
	}, req)
	if err != nil {
		return nil, err
	}
	return resp.(*cvm.DescribeZoneInstanceConfigInfosResponse), nil
}

// DescribeZoneInstanceConfigInfos return zone instance config, which include instance type family, charge type;
// DescribeZoneInstanceConfigInfos only has cpu/ram machine price, it do not include machine disk and network price.
// zone + instanceType + chargeType can identify an instance price
func (qcc *CVMClient) DescribeZoneInstanceConfigInfos() ([]*cvm.InstanceTypeQuotaItem, error) {
	instanceConfs := []*cvm.InstanceTypeQuotaItem{}
	cli, err := qcc.getClient()
	if err != nil {
		return instanceConfs, err
	}
	req := cvm.NewDescribeZoneInstanceConfigInfosRequest()
	resp, err := qcc.DescribeZoneInstanceConfigInfosWithRetry(cli, req)
	if err != nil {
		return instanceConfs, err
	}
	instanceConfs = append(instanceConfs, resp.Response.InstanceTypeQuotaSet...)
	return instanceConfs, nil
}

// GetAllZoneInstanceConfigInfos return all regions all zones instance config, which include instance type family, charge type;
// zone + instanceType + chargeType can identify an instance price, it is an standard price not the real costmodel in cloud billing
// todo: we support standard price only now, because billing data is hard to get for security policy, and it has T+1 delay. you can not upload billing data in current time.
func (qcc *CVMClient) GetAllZoneInstanceConfigInfos() ([]*cvm.InstanceTypeQuotaItem, error) {
	var instanceConfs []*cvm.InstanceTypeQuotaItem
	regionInfos, err := qcc.getAllRegions()
	if err != nil {
		return instanceConfs, err
	}
	var wg sync.WaitGroup
	wg.Add(len(regionInfos))
	insConfsCh := make(chan []*cvm.InstanceTypeQuotaItem, len(regionInfos))
	errorCh := make(chan error, len(regionInfos))
	for _, region := range regionInfos {
		go func(region *cvm.RegionInfo) {
			defer wg.Done()
			defer utilruntime.HandleCrash()
			if region.Region == nil {
				return
			}
			insConfs, err := qcc.DescribeZoneInstanceConfigInfos()
			if err != nil {
				errorCh <- err
				return
			}

			insConfsCh <- insConfs
		}(region)
	}

	go func() {
		defer utilruntime.HandleCrash()

		wg.Wait()
		close(errorCh)
		close(insConfsCh)
	}()

	for insList := range insConfsCh {
		instanceConfs = append(instanceConfs, insList...)
	}

	errors := []error{}
	for err := range errorCh {
		klog.Errorf("qcloudClient unable to get instanceConfigInfos: %s", err)
		errors = append(errors, err)
	}

	// Return error if no volumes are returned
	if len(errors) > 0 {
		return instanceConfs, fmt.Errorf("qcloudClient %d error(s) retrieving instanceConfigInfos: %v", len(errors), errors)
	}
	return instanceConfs, nil
}

func (qcc *CVMClient) GetCVMInstances(instanceIds []*string) ([]*cvm.Instance, error) {
	cvms := []*cvm.Instance{}
	var limit int64 = qcc.config.DefaultLimit
	var offset int64 = 0

	client, err := qcc.getClient()
	if err != nil {
		return cvms, err
	}
	length := len(instanceIds)
	for s := 0; s < length; s += int(limit) {
		e := s + int(limit)
		if e > length {
			e = length
		}
		request := cvm.NewDescribeInstancesRequest()
		request.InstanceIds = instanceIds[s:e]
		request.Limit = &limit
		request.Offset = &offset

		resp, err := qcc.InquiryDescribeInstancesWithRetry(client, request)
		if err != nil {
			return cvms, err
		}
		cvms = append(cvms, resp.Response.InstanceSet...)
	}
	// page by page, but this is not guaranteed we can fetch complete items if there are some inserts between pages we fetch
	//for resp != nil && resp.Response != nil && resp.Response.TotalCount != nil &&
	//	(offset+int64(len(resp.Response.InstanceSet))) < *resp.Response.TotalCount {
	//	request := cvm.NewDescribeInstancesRequest()
	//	request.InstanceIds = instanceIds
	//	offset = offset + limit
	//	request.Limit = &limit
	//	request.Offset = &offset
	//	resp, err = qcc.InquiryDescribeInstancesWithRetry(client, request)
	//	if err != nil {
	//		return cvms, err
	//	}
	//	cvms = append(cvms, resp.Response.InstanceSet...)
	//}
	return cvms, nil
}

type QCloudInstancePrice struct {
	Instance *cvm.Instance
	Price    *cvm.Price
}

func (qcc *CVMClient) GetCVMInstancesPrice(cvmInstances []*cvm.Instance) ([]*QCloudInstancePrice, error) {
	qcloudPrices := []*QCloudInstancePrice{}
	client, err := qcc.getClient()
	if err != nil {
		return qcloudPrices, err
	}
	for _, ins := range cvmInstances {
		req := cvm.NewDescribeZoneInstanceConfigInfosRequest()

		zone := ""
		insType := ""
		insChargeType := ""
		if ins.Placement != nil && ins.Placement.Zone != nil {
			zone = *ins.Placement.Zone
		}
		if ins.InstanceType != nil {
			insType = *ins.InstanceType
		}
		if ins.InstanceChargeType != nil {
			insChargeType = *ins.InstanceChargeType
		}

		req.Filters = []*cvm.Filter{
			{
				Name:   common.StringPtr("instance-type"),
				Values: common.StringPtrs([]string{insType}),
			},
			{
				Name:   common.StringPtr("instance-charge-type"),
				Values: common.StringPtrs([]string{insChargeType}),
			},
			{
				Name:   common.StringPtr("zone"),
				Values: common.StringPtrs([]string{zone}),
			},
		}

		resp, err := qcc.DescribeZoneInstanceConfigInfosWithRetry(client, req)
		if err != nil {
			klog.Warningf("Failed to Get Config, error: %v", err)
			continue
		}

		if qcc.config.Debug {
			klog.V(6).Infof("insId: %v, zone: %v, insType: %v, insChargeType: %v", *ins.InstanceId, zone, insType, insChargeType)
		}

		if resp.Response != nil && len(resp.Response.InstanceTypeQuotaSet) > 0 {
			item := resp.Response.InstanceTypeQuotaSet[0]
			if item.Price != nil {
				if qcc.config.Debug {
					out, _ := json.Marshal(item.Price)
					klog.V(6).Infof("item: %+v", string(out))
				}
				qcloudPrices = append(qcloudPrices, &QCloudInstancePrice{Instance: ins, Price: &cvm.Price{InstancePrice: item.Price}})
			}
		}

	}
	return qcloudPrices, nil
}

func (qcc *CVMClient) GetCVMInstancesInquiryPrice(cvmInstances []*cvm.Instance) ([]*QCloudInstancePrice, error) {

	qcloudPrices := []*QCloudInstancePrice{}
	client, err := qcc.getClient()
	if err != nil {
		return qcloudPrices, err
	}
	var months int64 = 1
	// inqury instance price must has instance type and charge model, if you has some other disk params, it will include disk and net price.
	for _, instance := range cvmInstances {
		req := cvm.NewInquiryPriceRunInstancesRequest()
		req.InstanceName = instance.InstanceName
		req.ImageId = instance.ImageId
		req.Placement = instance.Placement
		req.InstanceChargeType = instance.InstanceChargeType
		//req.DataDisks = instance.DataDisks
		//req.SystemDisk = instance.SystemDisk
		if instance.InstanceChargeType != nil && *instance.InstanceChargeType == consts.INSTANCECHARGETYPE_PREPAID {
			req.InstanceChargePrepaid = &cvm.InstanceChargePrepaid{
				Period:    &months,
				RenewFlag: instance.RenewFlag,
			}
		}
		req.InstanceType = instance.InstanceType
		resp, err := qcc.InquiryPriceRunInstancesWithRetry(client, req)
		if err != nil {
			return qcloudPrices, err
		}
		qcloudPrices = append(qcloudPrices, &QCloudInstancePrice{Instance: instance, Price: resp.Response.Price})
	}
	return qcloudPrices, nil
}
