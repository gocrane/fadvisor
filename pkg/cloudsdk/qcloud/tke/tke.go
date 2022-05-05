package tke

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tke "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"

	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/credential"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/metrics"
)

type TKEClient struct {
	clientLock sync.Mutex
	client     *tke.Client
	config     *qcloud.QCloudClientConfig
}

type retryFunc func(request interface{}) (interface{}, error)

func NewTKEClient(qcc *qcloud.QCloudClientConfig) *TKEClient {

	return &TKEClient{
		config: qcc,
	}
}

func (qcc *TKEClient) getTKEDomain() string {
	return fmt.Sprintf("%v.%v", "tke", qcc.config.DomainSuffix)
}

func (qcc *TKEClient) UpdateCredential(cred credential.QCloudCredential) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Credential = cred
}

func (qcc *TKEClient) ExponentialRetryCall(retryCnt int, f retryFunc, request interface{}) (interface{}, error) {
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

func (qcc *TKEClient) UpdateCred(cred credential.QCloudCredential) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Credential = cred
}

func (qcc *TKEClient) UpdateCustomCredential(id, secret string) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Credential.UpdateQCloudCustomCredential(id, secret)
}

func (qcc *TKEClient) EnableDebug() bool {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Debug = true
	return qcc.config.Debug
}

func (qcc *TKEClient) DebugMode() bool {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	return qcc.config.Debug
}

// getQCloudCredential return credential assumed from norm or user custom
func (qcc *TKEClient) getQCloudCredential() *common.Credential {
	return qcc.config.Credential.GetQCloudCredential()
}

func (qcc *TKEClient) getClient() (*tke.Client, error) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()

	cred := qcc.getQCloudCredential()
	var err error
	if qcc.client == nil {
		prof := profile.NewClientProfile()
		prof.Language = qcc.config.DefaultLanguage
		prof.Debug = qcc.config.Debug
		prof.HttpProfile.Endpoint = qcc.getTKEDomain()
		prof.HttpProfile.Scheme = qcc.config.Scheme
		qcc.client, err = tke.NewClient(cred, qcc.config.Region, prof)
		if err != nil {
			return qcc.client, err
		}
	}
	if qcc.config.Debug {
		SecretId := cred.GetSecretId()
		SecretKey := cred.GetSecretKey()
		Token := cred.GetToken()
		klog.Infof("qcloudClient tke region: %v, scheme: %v, domain: %v client credential: %s, %s, %s\n", qcc.config.Region, qcc.config.Scheme, qcc.getTKEDomain(), SecretId, SecretKey, Token)
	}
	return qcc.client, nil
}

func (qcc *TKEClient) getPodSpecificationWithRetry(cli *tke.Client, req *tke.GetPodSpecificationRequest) (*tke.GetPodSpecificationResponse, error) {
	resp, err := qcc.ExponentialRetryCall(qcc.config.DefaultRetryCnt, func(request interface{}) (interface{}, error) {
		req := request.(*tke.GetPodSpecificationRequest)
		start := time.Now()
		resp, err := cli.GetPodSpecification(req)
		if err != nil {
			metrics.ComponentWrongRequestStatics(req.GetDomain(), req.GetAction(), err.Error(), req.GetVersion())
		} else {
			metrics.ComponentRequestStatics(req.GetDomain(), req.GetAction(), 200, "", req.GetVersion(), start)
		}
		if qcc.DebugMode() {
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
	return resp.(*tke.GetPodSpecificationResponse), nil
}

func (qcc *TKEClient) getEKSPodPriceWithRetry(cli *tke.Client, req *tke.GetPriceRequest) (*tke.GetPriceResponse, error) {
	resp, err := qcc.ExponentialRetryCall(qcc.config.DefaultRetryCnt, func(request interface{}) (interface{}, error) {
		req := request.(*tke.GetPriceRequest)
		start := time.Now()
		resp, err := cli.GetPrice(req)
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
	return resp.(*tke.GetPriceResponse), nil
}

func (qcc *TKEClient) GetEKSPodPrice(req *tke.GetPriceRequest) (*tke.GetPriceResponse, error) {
	cli, err := qcc.getClient()
	if err != nil {
		return nil, err
	}
	return qcc.getEKSPodPriceWithRetry(cli, req)
}

func (qcc *TKEClient) GetPodSpecification(req *tke.GetPodSpecificationRequest) (*tke.GetPodSpecificationResponse, error) {
	cli, err := qcc.getClient()
	if err != nil {
		return nil, err
	}
	return qcc.getPodSpecificationWithRetry(cli, req)
}

func (qcc *TKEClient) Pod2EKSSpecConverter(pod *v1.Pod) (v1.ResourceList, error) {
	cli, err := qcc.getClient()
	if err != nil {
		return nil, err
	}

	req := tke.NewGetPodSpecificationRequest()

	var requirements []string
	for _, container := range pod.Spec.Containers {
		requireStr, err := json.Marshal(container.Resources)
		if err != nil {
			continue
		}
		requirements = append(requirements, string(requireStr))
	}
	var reqRequirements []*string
	for i := range requirements {
		reqRequirements = append(reqRequirements, &requirements[i])
	}
	machineType := EKSPodCpuType(pod)
	if ok, gpuType := EKSPodGpuType(pod); ok {
		machineType = gpuType
	}
	req.Type = &machineType
	req.ResourceRequirements = reqRequirements

	resp, err := qcc.getPodSpecificationWithRetry(cli, req)
	if err != nil {
		return nil, err
	}

	cpuQuantity, err := resource.ParseQuantity(*resp.Response.Cpu)
	if err != nil {
		return nil, err
	}
	memQuantity, err := resource.ParseQuantity(*resp.Response.Memory)
	if err != nil {
		return nil, err
	}
	return v1.ResourceList{
		v1.ResourceCPU:    cpuQuantity,
		v1.ResourceMemory: memQuantity,
	}, nil
}

const (
	// https://cloud.tencent.com/document/product/457/53030
	EKSAnnoCpuType = "eks.tke.cloud.tencent.com/cpu-type"
	//EKSAnnoCpuQuantity     = "eks.tke.cloud.tencent.com/cpu"
	//EKSAnnoMemQuantity     = "eks.tke.cloud.tencent.com/mem"
	EKSAnnoGpuType = "eks.tke.cloud.tencent.com/gpu-type"
	//EKSAnnoGpuCount        = "eks.tke.cloud.tencent.com/gpu-count"

	EKSCpuTypeValue_Intel = "intel"
	//EKSCpuTypeValue_Amd    = "amd"
	//EKSGpuTypeValue_V100   = "V100"
	//EKSGpuTypeValue_1_4_T4 = "1/4*T4"
	//EKSGpuTypeValue_1_2_T4 = "1/2*T4"
	//EKSGpuTypeValue_T4     = "T4"
)

func EKSPodCpuType(pod *v1.Pod) string {
	if pod.Annotations == nil {
		//default
		return EKSCpuTypeValue_Intel
	}
	return pod.Annotations[EKSAnnoCpuType]
}

func EKSPodGpuType(pod *v1.Pod) (bool, string) {
	if pod.Annotations == nil {
		return false, ""
	}
	t, ok := pod.Annotations[EKSAnnoGpuType]
	return ok, t
}
