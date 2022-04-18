package qmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/monitor/v20180724"

	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/credential"
	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/metrics"
)

const (
	REQUEST_TIME_LAYOUT = "2006-01-02T15:15:50"
)

var (
	// Namesapce
	MetricsNamespace = "QCE/TKE"
	MetricsModule    = "monitor"

	/**
	Metrics
	*/
	// workload view
	K8sWorkloadCpuCoreUsedMetric     = "K8sWorkloadCpuCoreUsed"
	K8sWorkloadMemUsageBytesMetric   = "K8sWorkloadMemUsageBytes"
	K8sWorkloadMemNoCacheBytesMetric = "K8sWorkloadMemNoCacheBytes"
	K8sWorkloadReplicasMetric        = "K8sWorkloadPodTotal"
	K8sWorkloadCpuRequestsMetric     = "K8sWorkloadCpuRequests"
	K8sWorkloadMemRequestsMetric     = "K8sWorkloadMemRequests"
	K8sWorkloadCpuLimitsMetric       = "K8sWorkloadCpuLimits"
	K8sWorkloadMemLimitsMetric       = "K8sWorkloadMemLimits"

	// node view

	// pod view
	K8sPodCpuCoreUsedMetric     = "K8sPodCpuCoreUsed"
	K8sPodMemUsageBytesMetric   = "K8sPodMemUsageBytes"
	K8sPodMemNoCacheBytesMetric = "K8sPodMemNoCacheBytes"

	// container view
	K8sContainerCpuCoreUsedMetric     = "K8sContainerCpuCoreUsed"
	K8sContainerMemUsageBytesMetric   = "K8sContainerMemUsageBytes"
	K8sContainerMemNoCacheBytesMetric = "K8sContainerMemNoCacheBytes"
	K8sContainerCpuCoreLimitMetric    = "K8sContainerCpuCoreLimit"
	K8sContainerCpuCoreRequestMetric  = "K8sContainerCpuCoreRequest"
	K8sContainerMemLimitMetric        = "K8sContainerMemLimit"
	K8sContainerMemRequestMetric      = "K8sContainerMemRequest"

	// 下面都是比例，目前还没得 limit/request 绝对值
	K8sContainerRateCpuCoreUsedLimitMetric   = "K8sContainerRateCpuCoreUsedLimit"
	K8sContainerRateCpuCoreUsedRequestMetric = "K8sContainerRateCpuCoreUsedRequest"
	K8sContainerRateMemUsageLimitMetric      = "K8sContainerRateMemUsageLimit"
	K8sContainerRateMemUsageRequestMetric    = "K8sContainerRateMemUsageRequest"
	K8sContainerRateMemNoCacheLimitMetric    = "K8sContainerRateMemNoCacheLimit"
	K8sContainerRateMemNoCacheRequestMetric  = "K8sContainerRateMemNoCacheRequest"

	// Dimension
	LabelAppId         = "appid"
	LabelContainerId   = "container_id"
	LabelContainerName = "container_name"
	LabelNamespace     = "namespace"
	LabelNode          = "node"
	LabelNodeRole      = "node_role"
	LabelPodName       = "pod_name"
	LabelRegion        = "region"
	LabelClusterId     = "tke_cluster_instance_id"
	LabelUnInstanceId  = "un_instance_id"
	LabelWorkloadKind  = "workload_kind"
	LabelWorkloadName  = "workload_name"

	// View
	ViewK8sContainer = "k8s_container"
	ViewK8sCluster   = "k8s_cluster"
	ViewK8sComponent = "k8s_component"
	ViewK8sNode      = "k8s_node"
	ViewK8sPod       = "k8s_pod"
	ViewK8sWorkload  = "k8s_workload"
)

type QCloudMonitorClient struct {
	clientLock sync.Mutex
	client     *cm.Client
	config     *qcloud.QCloudClientConfig
}

type retryFunc func(request interface{}) (interface{}, error)

func NewQCloudMonitorClient(qcc *qcloud.QCloudClientConfig) *QCloudMonitorClient {
	return &QCloudMonitorClient{
		config: qcc,
	}
}

func (qcc *QCloudMonitorClient) UpdateCredential(cred credential.QCloudCredential) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Credential = cred
}

func (qcc *QCloudMonitorClient) ExponentialRetryCall(retryCnt int, f retryFunc, request interface{}) (interface{}, error) {
	var err error
	var resp interface{}
	//ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Minute)
	//defer cancelFunc()

	// blocking
	qcc.config.RateLimiter.Accept()

	resp, err = f(request)
	if err == nil {
		return resp, nil
	}
	for i := 1; i <= retryCnt; i++ {
		randInt := rand.IntnRange(2<<i, 2<<(i+1)+1)
		sleepTime := time.Duration(randInt) * time.Second
		klog.Errorf("qcloudClient tencent cloud api failed, retrying %v times, qps:%v, err: %v", i, qcc.config.RateLimiter.QPS(), err)
		time.Sleep(sleepTime)
		qcc.config.RateLimiter.Accept()
		resp, err = f(request)
		if err == nil {
			return resp, nil
		}
	}
	return nil, fmt.Errorf("qcloudClient tencent cloud api retry failed after retry %v times, err: %s", retryCnt, err)
}

func (qcc *QCloudMonitorClient) UpdateCred(cred credential.QCloudCredential) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Credential = cred
}

func (qcc *QCloudMonitorClient) UpdateCustomCredential(id, secret string) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Credential.UpdateQCloudCustomCredential(id, secret)
}

func (qcc *QCloudMonitorClient) EnableDebug() bool {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()
	qcc.config.Debug = true
	return qcc.config.Debug
}

func (qcc *QCloudMonitorClient) DebugModeNoLock() bool {
	return qcc.config.Debug
}

// getQCloudCredential return credential assumed from norm or user custom
func (qcc *QCloudMonitorClient) getQCloudCredential() *common.Credential {
	return qcc.config.Credential.GetQCloudCredential()
}

func (qcc *QCloudMonitorClient) getClient() (*cm.Client, error) {
	qcc.clientLock.Lock()
	defer qcc.clientLock.Unlock()

	cred := qcc.getQCloudCredential()
	var err error
	if qcc.client == nil {
		prof := profile.NewClientProfile()
		prof.Language = qcc.config.DefaultLanguage
		prof.Debug = qcc.config.Debug
		prof.HttpProfile.Endpoint = qcc.getQMonitorDomain()
		prof.HttpProfile.Scheme = qcc.config.Scheme
		qcc.client, err = cm.NewClient(cred, qcc.config.Region, prof)
		if err != nil {
			return qcc.client, err
		}
	}
	if qcc.config.Debug {
		SecretId := cred.GetSecretId()
		SecretKey := cred.GetSecretKey()
		Token := cred.GetToken()
		klog.Infof("qcloudClient qmonitor region: %v, scheme: %v, domain: %v client credential: %s, %s, %s\n", qcc.config.Region, qcc.config.Scheme, qcc.getQMonitorDomain(), SecretId, SecretKey, Token)
	}
	return qcc.client, nil
}

func (qcc *QCloudMonitorClient) getQMonitorDomain() string {
	return fmt.Sprintf("%v.%v", "monitor", qcc.config.DomainSuffix)
}

func (qcc *QCloudMonitorClient) DescribeBaseMetricsWithRetry(cli *cm.Client, req *cm.DescribeBaseMetricsRequest) (*cm.DescribeBaseMetricsResponse, error) {
	resp, err := qcc.ExponentialRetryCall(qcc.config.DefaultRetryCnt, func(request interface{}) (interface{}, error) {
		req := request.(*cm.DescribeBaseMetricsRequest)
		start := time.Now()
		resp, err := cli.DescribeBaseMetrics(req)
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
	return resp.(*cm.DescribeBaseMetricsResponse), nil
}

func (qcc *QCloudMonitorClient) GetMonitorDataWithRetry(cli *cm.Client, req *cm.GetMonitorDataRequest) (*cm.GetMonitorDataResponse, error) {
	resp, err := qcc.ExponentialRetryCall(qcc.config.DefaultRetryCnt, func(request interface{}) (interface{}, error) {
		req := request.(*cm.GetMonitorDataRequest)
		start := time.Now()
		resp, err := cli.GetMonitorData(req)
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
	return resp.(*cm.GetMonitorDataResponse), nil
}

// cloud monitor recommended to fetch container metrics by this api
func (qcc *QCloudMonitorClient) DescribeStatisticDataWithRetry(cli *cm.Client, req *cm.DescribeStatisticDataRequest) (*cm.DescribeStatisticDataResponse, error) {
	resp, err := qcc.ExponentialRetryCall(qcc.config.DefaultRetryCnt, func(request interface{}) (interface{}, error) {
		req := request.(*cm.DescribeStatisticDataRequest)
		start := time.Now()
		resp, err := cli.DescribeStatisticData(request.(*cm.DescribeStatisticDataRequest))
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
	return resp.(*cm.DescribeStatisticDataResponse), nil
}

func (qcc *QCloudMonitorClient) PutMonitorDataWithRetry(cli *cm.Client, req *cm.PutMonitorDataRequest) (*cm.PutMonitorDataResponse, error) {
	resp, err := qcc.ExponentialRetryCall(qcc.config.DefaultRetryCnt, func(request interface{}) (interface{}, error) {
		req := request.(*cm.PutMonitorDataRequest)
		start := time.Now()
		resp, err := cli.PutMonitorData(req)
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
	return resp.(*cm.PutMonitorDataResponse), nil
}

func (qcc *QCloudMonitorClient) GetMonitorData(ctx context.Context, req *cm.GetMonitorDataRequest) (*cm.GetMonitorDataResponse, error) {
	cli, err := qcc.getClient()
	if err != nil {
		return nil, err
	}
	return qcc.GetMonitorDataWithRetry(cli, req)
}

func (qcc *QCloudMonitorClient) DescribeStatisticData(ctx context.Context, req *GetDataParam) (*GetDataResult, error) {
	cli, err := qcc.getClient()
	if err != nil {
		return nil, err
	}
	resp, err := qcc.DescribeStatisticDataWithRetry(cli, req.Convert2SDKRequest())
	if err != nil {
		return nil, err
	}
	result := &GetDataResult{}
	if resp.Response == nil {
		klog.Error(fmt.Errorf("qcloudClient tencent cloud api nil response, req: %+v, resp: %v", *req, resp.ToJsonString()))
		result.Data = ConvertSDKMetricData([]*cm.MetricData{})
		result.StartTime = req.StartTime
		result.EndTime = req.EndTime
		result.Period = req.Period
		return result, nil
	}
	if resp.Response.Period != nil {
		result.Period = *resp.Response.Period
	}
	if resp.Response.StartTime != nil {
		result.StartTime = *resp.Response.StartTime
	}
	if resp.Response.EndTime != nil {
		result.EndTime = *resp.Response.EndTime
	}
	if resp.Response.Data != nil {
		result.Data = ConvertSDKMetricData(resp.Response.Data)
	}
	return result, nil
}

func (qcc *QCloudMonitorClient) PutMonitorData(ctx context.Context, req *cm.PutMonitorDataRequest) (*cm.PutMonitorDataResponse, error) {
	cli, err := qcc.getClient()
	if err != nil {
		return nil, err
	}
	return qcc.PutMonitorDataWithRetry(cli, req)
}

type GetDataParam struct {
	Module      string
	Namespace   string
	MetricNames []string
	Conditions  []MidQueryCondition
	Period      uint64
	StartTime   string
	EndTime     string
	GroupBys    []string
}

type PutDataParam struct {
	// 一组指标和数据
	Metrics []MetricDatum `json:"Metrics,omitempty" name:"Metrics"`

	// 上报时自行指定的 IP
	AnnounceIp string `json:"AnnounceIp,omitempty" name:"AnnounceIp"`

	// 上报时自行指定的时间戳
	AnnounceTimestamp uint64 `json:"AnnounceTimestamp,omitempty" name:"AnnounceTimestamp"`

	// 上报时自行指定的 IP 或 产品实例ID
	AnnounceInstance string `json:"AnnounceInstance,omitempty" name:"AnnounceInstance"`
}

type MidQueryCondition struct {
	Key      string
	Operator string
	Value    []string
}

type GetDataResult struct {
	Period    uint64
	StartTime string
	EndTime   string
	Data      []MetricDatum
}

type MetricDatum struct {
	MetricName string
	Points     []MetricDataPoint
}

type MetricDataPoint struct {
	Dimensions []Dimension
	Values     []Point
}

type Dimension struct {
	Name  string
	Value string
}

type Point struct {
	Timestamp *uint64
	Value     *float64
}

func (mdp *MetricDataPoint) Dimensions2Map() map[string]interface{} {
	results := make(map[string]interface{})
	for _, dim := range mdp.Dimensions {
		results[dim.Name] = dim.Value
	}
	return results
}

func (mdp *MetricDataPoint) Dimensions2List() []Dimension {
	return mdp.Dimensions
}

func ConvertSDKMetricData(datas []*cm.MetricData) []MetricDatum {
	results := make([]MetricDatum, len(datas))
	for i, data := range datas {
		if data.MetricName != nil {
			results[i].MetricName = *data.MetricName
		}
		points := make([]MetricDataPoint, len(data.Points))
		for j, point := range data.Points {
			dims := make([]Dimension, len(point.Dimensions))
			for k, dim := range point.Dimensions {
				if dim.Name != nil {
					dims[k].Name = *dim.Name
				}
				if dim.Value != nil {
					dims[k].Value = *dim.Value
				}
			}
			vals := make([]Point, len(point.Values))
			for l, val := range point.Values {
				if val.Timestamp != nil {
					vals[l].Timestamp = val.Timestamp
				}
				vals[l].Value = val.Value
			}
			points[j].Dimensions = dims
			points[j].Values = vals
		}
		results[i].Points = points
	}
	return results
}

func (p *GetDataParam) AppendCondition(key, operator string, value []string) {
	p.Conditions = append(p.Conditions, MidQueryCondition{
		key, operator, value,
	})
}

func (p *GetDataParam) SetTimeRange(startTime time.Time, endTime time.Time, period time.Duration) {
	p.StartTime = startTime.Format(time.RFC3339)
	p.EndTime = endTime.Format(time.RFC3339)
	p.Period = uint64(period.Seconds())
}

func (p *GetDataParam) Convert2SDKRequest() *cm.DescribeStatisticDataRequest {
	req := cm.NewDescribeStatisticDataRequest()
	req.StartTime = &p.StartTime
	req.EndTime = &p.EndTime
	req.Period = &p.Period
	req.Module = &p.Module
	req.Namespace = &p.Namespace
	metricNames := []*string{}
	for i := range p.MetricNames {
		metricNames = append(metricNames, &p.MetricNames[i])
	}
	req.MetricNames = metricNames
	groupBys := []*string{}
	for i := range p.GroupBys {
		metricNames = append(metricNames, &p.GroupBys[i])
	}
	req.GroupBys = groupBys
	results := []*cm.MidQueryCondition{}
	for i, cond := range p.Conditions {
		vals := []*string{}
		for j := range cond.Value {
			vals = append(vals, &cond.Value[j])
		}
		results = append(results, &cm.MidQueryCondition{Key: &p.Conditions[i].Key, Operator: &p.Conditions[i].Operator, Value: vals})
	}
	req.Conditions = results
	return req
}

// 查询过滤条件
type QueryConditions struct {
	Conditions []MidQueryCondition
}

func NewQueryConditions() *QueryConditions {
	return &QueryConditions{
		Conditions: make([]MidQueryCondition, 0),
	}
}

func (qc *QueryConditions) AppendCondition(key, operator string, value []string) {
	qc.Conditions = append(qc.Conditions, MidQueryCondition{Key: key, Operator: operator, Value: value})
}

func (qc *QueryConditions) ToMidQueryConditions() []*cm.MidQueryCondition {
	results := []*cm.MidQueryCondition{}
	for i, cond := range qc.Conditions {
		vals := []*string{}
		for j := range cond.Value {
			vals = append(vals, &cond.Value[j])
		}
		results = append(results, &cm.MidQueryCondition{Key: &qc.Conditions[i].Key, Operator: &qc.Conditions[i].Operator, Value: vals})
	}
	return results
}

type DescDimensionValueReq struct {
	// 固定值为"monitor"
	Module string
	// 命名空间
	Namespace string
	// 视图名
	ViewName string
	// 要查询的维度
	DimensionKey string
	// 查询过滤条件
	Conditions []MidQueryCondition
	// 起始时间
	StartTime int64
	// 结束时间
	EndTime int64
	// 查询的中台地域
	QueryRegion string
}

func (ddvr *DescDimensionValueReq) AppendCondition(key, operator string, value []string) {
	ddvr.Conditions = append(ddvr.Conditions, MidQueryCondition{
		key, operator, value,
	})
}
