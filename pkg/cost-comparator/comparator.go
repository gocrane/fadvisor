package cost_comparator

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	promapiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/gocrane/crane/pkg/common"
	"github.com/gocrane/fadvisor/pkg/cache"
	"github.com/gocrane/fadvisor/pkg/cloud"
	"github.com/gocrane/fadvisor/pkg/consts"
	"github.com/gocrane/fadvisor/pkg/cost-comparator/config"
	"github.com/gocrane/fadvisor/pkg/cost-comparator/coster"
	"github.com/gocrane/fadvisor/pkg/cost-comparator/estimator"
	"github.com/gocrane/fadvisor/pkg/datasource"
	"github.com/gocrane/fadvisor/pkg/metricnaming"
	"github.com/gocrane/fadvisor/pkg/spec"
	ownerutil "github.com/gocrane/fadvisor/pkg/util/owner"
	targetutil "github.com/gocrane/fadvisor/pkg/util/target"
)

// Now do once task analysis, mapped Data in, then reduced Data out
type Comparator struct {
	config              config.Config
	kubeDynamicClient   dynamic.Interface
	kubeDiscoveryClient discovery.DiscoveryInterface
	restMapper          meta.RESTMapper
	targetInfoFetcher   targetutil.TargetInfoFetcher
	// some Intermediate cache data, cache it for other functions reuse.
	clusterCache       cache.Cache
	workloadsSpecCache map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ spec.CloudPodSpec
	// NOTE: workloadsContainerDataCache is memory consuming, so for online service it is not suitable. now just used to do offline task analysis
	containersTimeSeriesDataCache map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ map[string] /*container*/ *RawContainerTimeSeriesData
	workloadsTimeSeriesDataCache  map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ *RawWorkloadTimeSeriesData

	dataSource     datasource.Interface
	estimateConfig map[string]interface{}
	estimator      estimator.Estimator
	// this is your baseline estimate cloud provider, such as a tencent cloud tke cluster which is your current using cluster
	baselineCloud cloud.Cloud
}

func NewComparator(config config.Config,
	kubeDynamicClient dynamic.Interface,
	kubeDiscoveryClient discovery.DiscoveryInterface,
	restMapper meta.RESTMapper,
	fetcher targetutil.TargetInfoFetcher,
	clusterCache cache.Cache,
	baselineCloud cloud.Cloud,
	dataSource datasource.Interface) *Comparator {
	return &Comparator{
		estimateConfig:      make(map[string]interface{}),
		estimator:           estimator.NewStatisticEstimator(),
		config:              config,
		kubeDynamicClient:   kubeDynamicClient,
		kubeDiscoveryClient: kubeDiscoveryClient,
		restMapper:          restMapper,
		targetInfoFetcher:   fetcher,
		clusterCache:        clusterCache,
		dataSource:          dataSource,
		baselineCloud:       baselineCloud,
	}
}

// Init initialize some cached data and time series data, Must call before DoAnalysis
func (c *Comparator) Init() {
	c.initWorkloadsSpec()
	err := c.ContainerTsDataInit()
	if err != nil {
		klog.Fatalf("Failed to init container time series data: %v", err)
	}
	if c.config.EnableWorkloadTimeSeries {
		err = c.WorkloadTsDataInit()
		if err != nil {
			klog.Fatalf("Failed to init workload time series data: %v", err)
		}
	}
}

// Now it will fetch full data to do once analysis, so it is a time consuming offline computing task, also it will consuming memory because it will do time series analysis.
// todo: refactor to online service model when used for online deploy, split the services to online service & offline computing job.
// ??? offline computing jobs like spark by operator way VS. online service by deployment way
func (c *Comparator) DoAnalysis() {
	podsSpec := c.GetAllPodsSpec()
	nodesSpec := c.GetAllNodesSpec()

	workloadsRecs := c.GetAllWorkloadRecommendedData()

	costerCtx := &coster.CosterContext{
		TimeSpanSeconds:  c.config.TimeSpanSeconds,
		Discount:         &c.config.Discount,
		PodsSpec:         podsSpec,
		NodesSpec:        nodesSpec,
		WorkloadsRecSpec: workloadsRecs,
		WorkloadsSpec:    c.workloadsSpecCache,
		Pricer:           c.baselineCloud,
	}

	c.ReportOriginalResourceSummary()
	c.ReportOriginalCostSummary(costerCtx)
	c.ReportRawServerlessCostSummary(costerCtx)
	c.ReportRecommendedResourceSummary(costerCtx)
	c.ReportRecommendedCostSummary(costerCtx)

	c.ReportOriginalWorkloadsResourceDistribution(costerCtx)
	c.ReportRecommendedWorkloadsResourceDistribution(costerCtx)

}

func Int642Str(a int64) string {
	return fmt.Sprintf("%v", a)
}

func (c *Comparator) getQueryRange() promapiv1.Range {
	end := time.Now()
	var err error
	if c.config.History.EndTime != "" {
		end, err = time.Parse(time.RFC3339, c.config.History.EndTime)
		if err != nil {
			klog.Errorf("parsed config history end time failed, use default now: %v", err)
			end = time.Now()
		}
	}
	start := end.Add(-c.config.History.Length)
	return promapiv1.Range{
		Start: start,
		End:   end,
		Step:  c.config.History.Step,
	}
}

func (c *Comparator) GetAllPodsSpec() map[string] /*namespace-name*/ spec.CloudPodSpec {
	res := make(map[string]spec.CloudPodSpec)
	pods := c.clusterCache.GetPods()
	for _, pod := range pods {
		res[klog.KObj(pod).String()] = c.baselineCloud.Pod2Spec(pod)
	}
	return res
}

func (c *Comparator) GetAllNodesSpec() map[string] /*nodename*/ spec.CloudNodeSpec {
	res := make(map[string]spec.CloudNodeSpec)
	nodes := c.clusterCache.GetNodes()
	for _, node := range nodes {
		nodeSpec := c.baselineCloud.Node2Spec(node)
		res[node.Name] = nodeSpec
	}
	return res
}

// build workloads by inverted-index pods
func (c *Comparator) initWorkloadsSpec() map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ spec.CloudPodSpec {
	workloads := make(map[string]map[types.NamespacedName]spec.CloudPodSpec)
	pods := c.clusterCache.GetPods()
	for _, pod := range pods {
		unstruct, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
		if err != nil {
			klog.V(4).Info(err)
			continue
		}
		rootUnstruct, err := ownerutil.FindRootOwner(context.TODO(), c.restMapper, c.kubeDynamicClient, &unstructured.Unstructured{Object: unstruct})
		if err != nil {
			klog.V(4).Info(err)
			continue
		}
		kind := rootUnstruct.GetKind()
		nnworklod, ok := workloads[kind]
		if !ok {
			nnworklod = make(map[types.NamespacedName]spec.CloudPodSpec)
			workloads[kind] = nnworklod
		}
		nn := types.NamespacedName{Namespace: rootUnstruct.GetNamespace(), Name: rootUnstruct.GetName()}

		// ignore if already fetched
		if _, exists := nnworklod[nn]; exists {
			continue
		}
		podSpec := c.baselineCloud.Pod2Spec(pod)
		if strings.ToLower(kind) != "pod" {
			// because of job & cronjob is very special workload, it is once task, we can not estimate is directly
			if strings.ToLower(kind) == "cronjob" || strings.ToLower(kind) == "job" {
				klog.Warningf("Ignore %v %v: %v", rootUnstruct.GetAPIVersion(), kind, nn)
				continue
			}
			desiredReplicas, _, err := c.targetInfoFetcher.FetchReplicas(&v1.ObjectReference{
				Name:       rootUnstruct.GetName(),
				Kind:       rootUnstruct.GetKind(),
				Namespace:  rootUnstruct.GetNamespace(),
				APIVersion: rootUnstruct.GetAPIVersion(),
			})
			if err != nil {
				klog.Errorf("Failed to fetch target info for kind %v, nn: %v, err: %v", kind, nn, err)
				continue
			}
			podSpec.GoodsNum = uint64(desiredReplicas)
		}
		podSpec.Workload = rootUnstruct
		nnworklod[nn] = podSpec
	}
	c.workloadsSpecCache = workloads
	return workloads
}

func (c *Comparator) GetAllWorkloads() []*unstructured.Unstructured {
	var workloads []*unstructured.Unstructured
	pods := c.clusterCache.GetPods()
	for _, pod := range pods {
		unstruct, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
		if err != nil {
			klog.V(4).Infof("Failed to convert pod %v: %v", klog.KObj(pod), err)
			continue
		}
		rootUnstruct, err := ownerutil.FindRootOwner(context.TODO(), c.restMapper, c.kubeDynamicClient, &unstructured.Unstructured{Object: unstruct})
		if err != nil {
			klog.V(4).Infof("Failed to FindRootOwner pod %v: %v", klog.KObj(pod), err)
			continue
		}
		workloads = append(workloads, rootUnstruct)

	}
	return workloads
}

type RawContainerTimeSeriesData struct {
	Cpu         []*common.TimeSeries
	Mem         []*common.TimeSeries
	CpuRequests []*common.TimeSeries
	MemRequests []*common.TimeSeries
	CpuLimits   []*common.TimeSeries
	MemLimits   []*common.TimeSeries
}

type RawWorkloadTimeSeriesData struct {
	Cpu         []*common.TimeSeries
	Mem         []*common.TimeSeries
	CpuRequests []*common.TimeSeries
	MemRequests []*common.TimeSeries
	CpuLimits   []*common.TimeSeries
	MemLimits   []*common.TimeSeries
	Replicas    []*common.TimeSeries
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (c *Comparator) ContainerTsDataInit() error {
	data, err := c.LoadContainerTimeSeriesDataFromCheckpoint()
	if err == nil {
		c.containersTimeSeriesDataCache = data
		return nil
	}
	klog.Errorf("Failed to load workload container data from check point: %v, init from datasource", err)
	data = c.fetchContainerData()
	c.containersTimeSeriesDataCache = data
	klog.V(2).Infof("Succeed to load workload container data from data source, checkpointing it")
	if c.config.EnableContainerCheckpoint {
		err = c.ContainerDataCheckpoint()
		return err
	}
	return nil
}

func (c *Comparator) WorkloadTsDataInit() error {
	data, err := c.LoadWorkloadTimeSeriesDataFromCheckpoint()
	if err == nil {
		c.workloadsTimeSeriesDataCache = data
		return nil
	}
	klog.Errorf("Failed to load workload time series data from check point: %v, init from datasource", err)
	data = c.fetchWorkloadMetricData()
	c.workloadsTimeSeriesDataCache = data
	klog.V(2).Infof("Succeed to load workload time series data from data source, checkpointing it")
	if c.config.EnableWorkloadCheckpoint {
		err = c.workloadTimeSeriesDataCheckpoint()
	}
	return err
}

func (c *Comparator) LoadContainerTimeSeriesDataFromCheckpoint() (map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ map[string] /*container*/ *RawContainerTimeSeriesData, error) {
	checkPointFile := c.containerTimeSeriesDataCheckpointName()
	result := make(map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ map[string] /*container*/ *RawContainerTimeSeriesData)

	fileData, err := os.Open(checkPointFile)
	if err != nil {
		return result, err
	}
	reader := csv.NewReader(fileData)
	reader.Comma = '\t'
	lines, err := reader.ReadAll()
	if err != nil {
		return result, err
	}

	if len(lines) == 0 || len(lines[0]) <= 4 {
		return nil, fmt.Errorf("wrong fmt csv, header is wrong")
	}

	cpuUsageHeaders := make([]string, 0)
	memUsageHeaders := make([]string, 0)
	cpuRequestHeaders := make([]string, 0)
	memRequestHeaders := make([]string, 0)
	cpuLimitHeaders := make([]string, 0)
	memLimitHeaders := make([]string, 0)

	headers := lines[0][0:4]

	for _, field := range lines[0] {
		if strings.HasPrefix(field, "CpuUsage-") {
			cpuUsageHeaders = append(cpuUsageHeaders, field)
		}
		if strings.HasPrefix(field, "MemUsage-") {
			memUsageHeaders = append(memUsageHeaders, field)
		}
		if strings.HasPrefix(field, "CpuRequest-") {
			cpuRequestHeaders = append(cpuRequestHeaders, field)
		}
		if strings.HasPrefix(field, "MemRequest-") {
			memRequestHeaders = append(memRequestHeaders, field)
		}
		if strings.HasPrefix(field, "CpuLimit-") {
			cpuLimitHeaders = append(cpuLimitHeaders, field)
		}
		if strings.HasPrefix(field, "MemLimit-") {
			memLimitHeaders = append(memLimitHeaders, field)
		}
	}

	headers = append(headers, cpuUsageHeaders...)
	headers = append(headers, memUsageHeaders...)
	headers = append(headers, cpuRequestHeaders...)
	headers = append(headers, memRequestHeaders...)
	headers = append(headers, cpuLimitHeaders...)
	headers = append(headers, memLimitHeaders...)

	// skip header
	for line, row := range lines[1:] {
		if len(row) < len(headers) {
			klog.Warningf("wrong length row")
		}
		kind := row[0]
		namespace := row[1]
		name := row[2]
		container := row[3]

		kindNN, ok := result[kind]
		if !ok {
			kindNN = make(map[types.NamespacedName]map[string]*RawContainerTimeSeriesData)
			result[kind] = kindNN
		}
		nn := types.NamespacedName{Namespace: namespace, Name: name}
		containersMap, ok := kindNN[nn]
		if !ok {
			containersMap = make(map[string]*RawContainerTimeSeriesData)
			kindNN[nn] = containersMap
		}

		cpuTs := common.NewTimeSeries()
		memTs := common.NewTimeSeries()
		CpuRequestTs := common.NewTimeSeries()
		MemRequestTs := common.NewTimeSeries()
		CpuLimitTs := common.NewTimeSeries()
		MemLimitTs := common.NewTimeSeries()

		row := row[4:]
		indexStart := 0
		for i, colName := range cpuUsageHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			cpuTs.AppendSample(ts, val)
		}
		indexStart += len(cpuUsageHeaders)
		for i, colName := range memUsageHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			memTs.AppendSample(ts, val)
		}
		indexStart += len(memUsageHeaders)
		for i, colName := range cpuRequestHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			CpuRequestTs.AppendSample(ts, val)
		}
		indexStart += len(cpuRequestHeaders)
		for i, colName := range memRequestHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			MemRequestTs.AppendSample(ts, val)
		}
		indexStart += len(memRequestHeaders)
		for i, colName := range cpuLimitHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			CpuLimitTs.AppendSample(ts, val)
		}
		indexStart += len(cpuLimitHeaders)
		for i, colName := range memLimitHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			MemLimitTs.AppendSample(ts, val)
		}
		containersMap[container] = &RawContainerTimeSeriesData{
			Cpu:         []*common.TimeSeries{cpuTs},
			Mem:         []*common.TimeSeries{memTs},
			CpuRequests: []*common.TimeSeries{CpuRequestTs},
			MemRequests: []*common.TimeSeries{MemRequestTs},
			CpuLimits:   []*common.TimeSeries{CpuLimitTs},
			MemLimits:   []*common.TimeSeries{MemLimitTs},
		}
	}

	return result, nil
	//file, err := os.Open(checkPointFile)
	//defer file.Close()
	//decoder := json.NewDecoder(file)
	//// Read the array open bracket
	//decoder.Token()
	//for decoder.More() {
	//	decoder.Decode(&data)
	//}
	//return data, nil
}

func (c *Comparator) LoadWorkloadTimeSeriesDataFromCheckpoint() (map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ *RawWorkloadTimeSeriesData, error) {
	checkPointFile := c.workloadTimeSeriesDataCheckpointName()
	result := make(map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ *RawWorkloadTimeSeriesData)

	fileData, err := os.Open(checkPointFile)
	if err != nil {
		return result, err
	}
	reader := csv.NewReader(fileData)
	reader.Comma = '\t'
	lines, err := reader.ReadAll()
	if err != nil {
		return result, err
	}

	if len(lines) == 0 || len(lines[0]) <= 4 {
		return nil, fmt.Errorf("wrong fmt csv, header is wrong")
	}

	cpuUsageHeaders := make([]string, 0)
	memUsageHeaders := make([]string, 0)
	cpuRequestHeaders := make([]string, 0)
	memRequestHeaders := make([]string, 0)
	cpuLimitHeaders := make([]string, 0)
	memLimitHeaders := make([]string, 0)
	replicaHeaders := make([]string, 0)

	headers := lines[0][0:3]

	for _, field := range lines[0] {
		if strings.HasPrefix(field, "CpuUsage-") {
			cpuUsageHeaders = append(cpuUsageHeaders, field)
		}
		if strings.HasPrefix(field, "MemUsage-") {
			memUsageHeaders = append(memUsageHeaders, field)
		}
		if strings.HasPrefix(field, "CpuRequest-") {
			cpuRequestHeaders = append(cpuRequestHeaders, field)
		}
		if strings.HasPrefix(field, "MemRequest-") {
			memRequestHeaders = append(memRequestHeaders, field)
		}
		if strings.HasPrefix(field, "CpuLimit-") {
			cpuLimitHeaders = append(cpuLimitHeaders, field)
		}
		if strings.HasPrefix(field, "MemLimit-") {
			memLimitHeaders = append(memLimitHeaders, field)
		}
		if strings.HasPrefix(field, "Replicas-") {
			replicaHeaders = append(replicaHeaders, field)
		}
	}

	headers = append(headers, cpuUsageHeaders...)
	headers = append(headers, memUsageHeaders...)
	headers = append(headers, cpuRequestHeaders...)
	headers = append(headers, memRequestHeaders...)
	headers = append(headers, cpuLimitHeaders...)
	headers = append(headers, memLimitHeaders...)
	headers = append(headers, replicaHeaders...)

	// skip header
	for line, row := range lines[1:] {
		if len(row) < len(headers) {
			klog.Warningf("wrong length row")
		}
		kind := row[0]
		namespace := row[1]
		name := row[2]

		kindNN, ok := result[kind]
		if !ok {
			kindNN = make(map[types.NamespacedName]*RawWorkloadTimeSeriesData)
			result[kind] = kindNN
		}
		nn := types.NamespacedName{Namespace: namespace, Name: name}

		cpuTs := common.NewTimeSeries()
		memTs := common.NewTimeSeries()
		CpuRequestTs := common.NewTimeSeries()
		MemRequestTs := common.NewTimeSeries()
		CpuLimitTs := common.NewTimeSeries()
		MemLimitTs := common.NewTimeSeries()
		ReplicaTs := common.NewTimeSeries()

		row := row[3:]
		indexStart := 0
		for i, colName := range cpuUsageHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			cpuTs.AppendSample(ts, val)
		}
		indexStart += len(cpuUsageHeaders)
		for i, colName := range memUsageHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			memTs.AppendSample(ts, val)
		}
		indexStart += len(memUsageHeaders)
		for i, colName := range cpuRequestHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			CpuRequestTs.AppendSample(ts, val)
		}
		indexStart += len(cpuRequestHeaders)
		for i, colName := range memRequestHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			MemRequestTs.AppendSample(ts, val)
		}
		for i, colName := range cpuLimitHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			CpuLimitTs.AppendSample(ts, val)
		}
		indexStart += len(cpuLimitHeaders)
		for i, colName := range memLimitHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			MemLimitTs.AppendSample(ts, val)
		}
		indexStart += len(memLimitHeaders)
		for i, colName := range replicaHeaders {
			var ts int64
			var err error
			splits := strings.Split(colName, "-")
			if len(splits) >= 3 {
				ts, err = strconv.ParseInt(splits[2], 10, 64)
				if err != nil {
					klog.Warningf("line %v col %v parsed failed", line, colName)
				}
			}
			// null value ignored
			if row[indexStart+i] == "null" {
				continue
			}
			val, err := strconv.ParseFloat(row[indexStart+i], 64)
			if err != nil {
				continue
			}
			ReplicaTs.AppendSample(ts, val)
		}
		kindNN[nn] = &RawWorkloadTimeSeriesData{
			Cpu:         []*common.TimeSeries{cpuTs},
			Mem:         []*common.TimeSeries{memTs},
			CpuRequests: []*common.TimeSeries{CpuRequestTs},
			MemRequests: []*common.TimeSeries{MemRequestTs},
			CpuLimits:   []*common.TimeSeries{CpuLimitTs},
			MemLimits:   []*common.TimeSeries{MemLimitTs},
			Replicas:    []*common.TimeSeries{ReplicaTs},
		}
	}

	return result, nil
}

func (c *Comparator) containerTimeSeriesDataCheckpointName() string {
	return filepath.Join(c.config.DataPath, c.config.ClusterId+"-workloads-container-timeseries.csv")
}

func (c *Comparator) workloadTimeSeriesDataCheckpointName() string {
	return filepath.Join(c.config.DataPath, c.config.ClusterId+"-workloads-timeseries.csv")
}

func (c *Comparator) ContainerDataCheckpoint() error {
	//checkPointFile := c.checkpointName()
	//var json = jsoniter.ConfigCompatibleWithStandardLibrary
	//file, _ := os.OpenFile(checkPointFile, os.O_CREATE, os.ModePerm)
	//defer file.Close()
	//encoder := json.NewEncoder(file)
	//return encoder.Encode(data)

	maxHeaderLen := 0
	maxCpuUsageHeaderLen := 0
	maxMemUsageHeaderLen := 0
	maxCpuRequestHeaderLen := 0
	maxMemRequestHeaderLen := 0
	maxCpuLimitHeaderLen := 0
	maxMemLimitHeaderLen := 0
	for _, kindWorkloads := range c.containersTimeSeriesDataCache {
		for _, workloadContainers := range kindWorkloads {
			for _, data := range workloadContainers {

				cpuUsageLen := 0
				memUsageLen := 0
				cpuRequestLen := 0
				memRequestLen := 0
				cpuLimitLen := 0
				memLimitLen := 0

				if len(data.Cpu) > 0 && data.Cpu[0] != nil {
					cpuUsageLen = len(data.Cpu[0].Samples)
				}
				if len(data.Mem) > 0 && data.Mem[0] != nil {
					memUsageLen = len(data.Mem[0].Samples)
				}
				if len(data.CpuRequests) > 0 && data.CpuRequests[0] != nil {
					cpuRequestLen = len(data.CpuRequests[0].Samples)
				}
				if len(data.MemRequests) > 0 && data.MemRequests[0] != nil {
					memRequestLen = len(data.MemRequests[0].Samples)
				}
				if len(data.CpuLimits) > 0 && data.CpuLimits[0] != nil {
					cpuLimitLen = len(data.CpuLimits[0].Samples)
				}
				if len(data.MemLimits) > 0 && data.MemLimits[0] != nil {
					memLimitLen = len(data.MemLimits[0].Samples)
				}

				totalHeaderLen := cpuUsageLen + memUsageLen + cpuRequestLen + memRequestLen + cpuLimitLen + memLimitLen
				if maxHeaderLen < totalHeaderLen {
					maxHeaderLen = totalHeaderLen
				}

				if maxCpuUsageHeaderLen < cpuUsageLen {
					maxCpuUsageHeaderLen = cpuUsageLen
				}
				if maxMemUsageHeaderLen < memUsageLen {
					maxMemUsageHeaderLen = memUsageLen
				}
				if maxCpuRequestHeaderLen < cpuRequestLen {
					maxCpuRequestHeaderLen = cpuRequestLen
				}
				if maxMemRequestHeaderLen < memRequestLen {
					maxMemRequestHeaderLen = memRequestLen
				}
				if maxCpuLimitHeaderLen < cpuLimitLen {
					maxCpuLimitHeaderLen = cpuLimitLen
				}
				if maxMemLimitHeaderLen < memLimitLen {
					maxMemLimitHeaderLen = memLimitLen
				}
			}
		}
	}

	headers := []string{"Kind", "Namespace", "Name", "Container"}
	cpuUsageHeaders := make([]string, maxCpuUsageHeaderLen)
	memUsageHeaders := make([]string, maxMemUsageHeaderLen)
	cpuRequestHeaders := make([]string, maxCpuRequestHeaderLen)
	memRequestHeaders := make([]string, maxMemRequestHeaderLen)
	cpuLimitHeaders := make([]string, maxCpuLimitHeaderLen)
	memLimitHeaders := make([]string, maxMemLimitHeaderLen)

	for i := 0; i < maxCpuUsageHeaderLen; i++ {
		cpuUsageHeaders[i] = fmt.Sprintf("CpuUsage-%v", i)
	}
	for i := 0; i < maxMemUsageHeaderLen; i++ {
		memUsageHeaders[i] = fmt.Sprintf("MemUsage-%v", i)
	}
	for i := 0; i < maxCpuRequestHeaderLen; i++ {
		cpuRequestHeaders[i] = fmt.Sprintf("CpuRequest-%v", i)
	}
	for i := 0; i < maxMemRequestHeaderLen; i++ {
		memRequestHeaders[i] = fmt.Sprintf("MemRequest-%v", i)
	}
	for i := 0; i < maxCpuLimitHeaderLen; i++ {
		cpuLimitHeaders[i] = fmt.Sprintf("CpuLimit-%v", i)
	}
	for i := 0; i < maxMemLimitHeaderLen; i++ {
		memLimitHeaders[i] = fmt.Sprintf("MemLimit-%v", i)
	}

	var data [][]string
	for kind, kindWorkloads := range c.containersTimeSeriesDataCache {
		for nn, workloadContainers := range kindWorkloads {
			for container, containerData := range workloadContainers {
				row := []string{kind, nn.Namespace, nn.Name, container}

				for i := range cpuUsageHeaders {
					if len(containerData.Cpu) > 0 && i < len(containerData.Cpu[0].Samples) {
						sample := containerData.Cpu[0].Samples[i]
						row = append(row, Float642Str(sample.Value))
						splits := strings.Split(cpuUsageHeaders[i], "-")
						cpuUsageHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
					} else {
						row = append(row, "null")
					}
				}
				for i := range memUsageHeaders {
					if len(containerData.Mem) > 0 && i < len(containerData.Mem[0].Samples) {
						sample := containerData.Mem[0].Samples[i]
						row = append(row, Float642Str(sample.Value))
						splits := strings.Split(memUsageHeaders[i], "-")
						memUsageHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
					} else {
						row = append(row, "null")
					}
				}
				for i := range cpuRequestHeaders {
					if len(containerData.CpuRequests) > 0 && i < len(containerData.CpuRequests[0].Samples) {
						sample := containerData.CpuRequests[0].Samples[i]
						row = append(row, Float642Str(sample.Value))
						splits := strings.Split(cpuRequestHeaders[i], "-")
						cpuRequestHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
					} else {
						row = append(row, "null")
					}
				}
				for i := range memRequestHeaders {
					if len(containerData.MemRequests) > 0 && i < len(containerData.MemRequests[0].Samples) {
						sample := containerData.MemRequests[0].Samples[i]
						row = append(row, Float642Str(sample.Value))
						splits := strings.Split(memRequestHeaders[i], "-")
						memRequestHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
					} else {
						row = append(row, "null")
					}
				}
				for i := range cpuLimitHeaders {
					if len(containerData.CpuLimits) > 0 && i < len(containerData.CpuLimits[0].Samples) {
						sample := containerData.CpuLimits[0].Samples[i]
						row = append(row, Float642Str(sample.Value))
						splits := strings.Split(cpuLimitHeaders[i], "-")
						cpuLimitHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
					} else {
						row = append(row, "null")
					}
				}
				for i := range memLimitHeaders {
					if len(containerData.MemLimits) > 0 && i < len(containerData.MemLimits[0].Samples) {
						sample := containerData.MemLimits[0].Samples[i]
						row = append(row, Float642Str(sample.Value))
						splits := strings.Split(memLimitHeaders[i], "-")
						memLimitHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
					} else {
						row = append(row, "null")
					}
				}
				data = append(data, row)
			}
		}
	}

	headers = append(headers, cpuUsageHeaders...)
	headers = append(headers, memUsageHeaders...)
	headers = append(headers, cpuRequestHeaders...)
	headers = append(headers, memRequestHeaders...)
	headers = append(headers, cpuLimitHeaders...)
	headers = append(headers, memLimitHeaders...)

	csvFile, err := os.Create(c.containerTimeSeriesDataCheckpointName())
	if err != nil {
		return err
	}
	csvW := csv.NewWriter(csvFile)
	csvW.Comma = '\t'
	err = csvW.Write(headers)
	if err != nil {
		return err
	}
	err = csvW.WriteAll(data)
	if err != nil {
		return err
	}
	return nil
}

func (c *Comparator) workloadTimeSeriesDataCheckpoint() error {
	maxHeaderLen := 0
	maxCpuUsageHeaderLen := 0
	maxMemUsageHeaderLen := 0
	maxCpuRequestHeaderLen := 0
	maxMemRequestHeaderLen := 0
	maxCpuLimitHeaderLen := 0
	maxMemLimitHeaderLen := 0
	maxReplicaHeaderLen := 0
	for _, kindWorkloads := range c.workloadsTimeSeriesDataCache {
		for _, workloadTSData := range kindWorkloads {

			cpuUsageLen := 0
			memUsageLen := 0
			cpuRequestLen := 0
			memRequestLen := 0
			cpuLimitLen := 0
			memLimitLen := 0
			replicaLen := 0

			if len(workloadTSData.Cpu) > 0 && workloadTSData.Cpu[0] != nil {
				cpuUsageLen = len(workloadTSData.Cpu[0].Samples)
			}
			if len(workloadTSData.Mem) > 0 && workloadTSData.Mem[0] != nil {
				memUsageLen = len(workloadTSData.Mem[0].Samples)
			}
			if len(workloadTSData.CpuRequests) > 0 && workloadTSData.CpuRequests[0] != nil {
				cpuRequestLen = len(workloadTSData.CpuRequests[0].Samples)
			}
			if len(workloadTSData.MemRequests) > 0 && workloadTSData.MemRequests[0] != nil {
				memRequestLen = len(workloadTSData.MemRequests[0].Samples)
			}
			if len(workloadTSData.CpuLimits) > 0 && workloadTSData.CpuLimits[0] != nil {
				cpuLimitLen = len(workloadTSData.CpuLimits[0].Samples)
			}
			if len(workloadTSData.MemLimits) > 0 && workloadTSData.MemLimits[0] != nil {
				memLimitLen = len(workloadTSData.MemLimits[0].Samples)
			}
			if len(workloadTSData.Replicas) > 0 && workloadTSData.Replicas[0] != nil {
				replicaLen = len(workloadTSData.Replicas[0].Samples)
			}

			totalHeaderLen := cpuUsageLen + memUsageLen + cpuRequestLen + memRequestLen + cpuLimitLen + memLimitLen + replicaLen
			if maxHeaderLen < totalHeaderLen {
				maxHeaderLen = totalHeaderLen
			}

			if maxCpuUsageHeaderLen < cpuUsageLen {
				maxCpuUsageHeaderLen = cpuUsageLen
			}
			if maxMemUsageHeaderLen < memUsageLen {
				maxMemUsageHeaderLen = memUsageLen
			}
			if maxCpuRequestHeaderLen < cpuRequestLen {
				maxCpuRequestHeaderLen = cpuRequestLen
			}
			if maxMemRequestHeaderLen < memRequestLen {
				maxMemRequestHeaderLen = memRequestLen
			}
			if maxCpuLimitHeaderLen < cpuLimitLen {
				maxCpuLimitHeaderLen = cpuLimitLen
			}
			if maxMemLimitHeaderLen < memLimitLen {
				maxMemLimitHeaderLen = memLimitLen
			}
			if maxReplicaHeaderLen < replicaLen {
				maxReplicaHeaderLen = replicaLen
			}
		}
	}

	headers := []string{"Kind", "Namespace", "Name"}
	cpuUsageHeaders := make([]string, maxCpuUsageHeaderLen)
	memUsageHeaders := make([]string, maxMemUsageHeaderLen)
	cpuRequestHeaders := make([]string, maxCpuRequestHeaderLen)
	memRequestHeaders := make([]string, maxMemRequestHeaderLen)
	cpuLimitHeaders := make([]string, maxCpuLimitHeaderLen)
	memLimitHeaders := make([]string, maxMemLimitHeaderLen)
	replicaHeaders := make([]string, maxReplicaHeaderLen)

	for i := 0; i < maxCpuUsageHeaderLen; i++ {
		cpuUsageHeaders[i] = fmt.Sprintf("CpuUsage-%v", i)
	}
	for i := 0; i < maxMemUsageHeaderLen; i++ {
		memUsageHeaders[i] = fmt.Sprintf("MemUsage-%v", i)
	}
	for i := 0; i < maxCpuRequestHeaderLen; i++ {
		cpuRequestHeaders[i] = fmt.Sprintf("CpuRequest-%v", i)
	}
	for i := 0; i < maxMemRequestHeaderLen; i++ {
		memRequestHeaders[i] = fmt.Sprintf("MemRequest-%v", i)
	}
	for i := 0; i < maxCpuLimitHeaderLen; i++ {
		cpuLimitHeaders[i] = fmt.Sprintf("CpuLimit-%v", i)
	}
	for i := 0; i < maxMemLimitHeaderLen; i++ {
		memLimitHeaders[i] = fmt.Sprintf("MemLimit-%v", i)
	}
	for i := 0; i < maxReplicaHeaderLen; i++ {
		replicaHeaders[i] = fmt.Sprintf("Replicas-%v", i)
	}

	var data [][]string
	for kind, kindWorkloads := range c.workloadsTimeSeriesDataCache {
		for nn, workload := range kindWorkloads {
			row := []string{kind, nn.Namespace, nn.Name}

			for i := range cpuUsageHeaders {
				if len(workload.Cpu) > 0 && i < len(workload.Cpu[0].Samples) {
					sample := workload.Cpu[0].Samples[i]
					row = append(row, Float642Str(sample.Value))
					splits := strings.Split(cpuUsageHeaders[i], "-")
					cpuUsageHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
				} else {
					row = append(row, "null")
				}
			}
			for i := range memUsageHeaders {
				if len(workload.Mem) > 0 && i < len(workload.Mem[0].Samples) {
					sample := workload.Mem[0].Samples[i]
					row = append(row, Float642Str(sample.Value))
					splits := strings.Split(memUsageHeaders[i], "-")
					memUsageHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
				} else {
					row = append(row, "null")
				}
			}
			for i := range cpuRequestHeaders {
				if len(workload.CpuRequests) > 0 && i < len(workload.CpuRequests[0].Samples) {
					sample := workload.CpuRequests[0].Samples[i]
					row = append(row, Float642Str(sample.Value))
					splits := strings.Split(cpuRequestHeaders[i], "-")
					cpuRequestHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
				} else {
					row = append(row, "null")
				}
			}
			for i := range memRequestHeaders {
				if len(workload.MemRequests) > 0 && i < len(workload.MemRequests[0].Samples) {
					sample := workload.MemRequests[0].Samples[i]
					row = append(row, Float642Str(sample.Value))
					splits := strings.Split(memRequestHeaders[i], "-")
					memRequestHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
				} else {
					row = append(row, "null")
				}
			}
			for i := range cpuLimitHeaders {
				if len(workload.CpuLimits) > 0 && i < len(workload.CpuLimits[0].Samples) {
					sample := workload.CpuLimits[0].Samples[i]
					row = append(row, Float642Str(sample.Value))
					splits := strings.Split(cpuLimitHeaders[i], "-")
					cpuLimitHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
				} else {
					row = append(row, "null")
				}
			}
			for i := range memLimitHeaders {
				if len(workload.MemLimits) > 0 && i < len(workload.MemLimits[0].Samples) {
					sample := workload.MemLimits[0].Samples[i]
					row = append(row, Float642Str(sample.Value))
					splits := strings.Split(memLimitHeaders[i], "-")
					memLimitHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
				} else {
					row = append(row, "null")
				}
			}
			for i := range replicaHeaders {
				if len(workload.Replicas) > 0 && i < len(workload.Replicas[0].Samples) {
					sample := workload.Replicas[0].Samples[i]
					row = append(row, Float642Str(sample.Value))
					splits := strings.Split(replicaHeaders[i], "-")
					replicaHeaders[i] = fmt.Sprintf("%v-%v", splits[0]+"-"+splits[1], sample.Timestamp)
				} else {
					row = append(row, "null")
				}
			}
			data = append(data, row)
		}
	}

	headers = append(headers, cpuUsageHeaders...)
	headers = append(headers, memUsageHeaders...)
	headers = append(headers, cpuRequestHeaders...)
	headers = append(headers, memRequestHeaders...)
	headers = append(headers, cpuLimitHeaders...)
	headers = append(headers, memLimitHeaders...)
	headers = append(headers, replicaHeaders...)

	csvFile, err := os.Create(c.workloadTimeSeriesDataCheckpointName())
	if err != nil {
		return err
	}
	csvW := csv.NewWriter(csvFile)
	csvW.Comma = '\t'
	err = csvW.Write(headers)
	if err != nil {
		return err
	}
	err = csvW.WriteAll(data)
	if err != nil {
		return err
	}
	return nil
}

func (c *Comparator) fetchWorkloadMetricData() map[string]map[types.NamespacedName]*RawWorkloadTimeSeriesData {
	results := make(map[string]map[types.NamespacedName]*RawWorkloadTimeSeriesData)
	workloads := c.workloadsSpecCache
	qRange := c.getQueryRange()
	for kind := range workloads {
		if kindWorkloads, ok := workloads[kind]; ok {
			kindResult, ok := results[kind]
			if !ok {
				kindResult = make(map[types.NamespacedName]*RawWorkloadTimeSeriesData)
				results[kind] = kindResult
			}
			for nn, workload := range kindWorkloads {
				target := &v1.ObjectReference{
					Kind:       workload.Workload.GetKind(),
					Namespace:  workload.Workload.GetNamespace(),
					Name:       workload.Workload.GetName(),
					APIVersion: workload.Workload.GetAPIVersion(),
				}

				workloadCpuUsage := metricnaming.ResourceToWorkloadMetricNamer(c.config.ClusterId, target, v1.ResourceCPU, labels.Everything())
				workloadMemUsage := metricnaming.ResourceToWorkloadMetricNamer(c.config.ClusterId, target, v1.ResourceMemory, labels.Everything())
				workloadCpuReqs := metricnaming.WorkloadMetricNamer(c.config.ClusterId, target, consts.MetricCpuRequest, labels.Everything())
				workloadCpuLims := metricnaming.WorkloadMetricNamer(c.config.ClusterId, target, consts.MetricCpuLimit, labels.Everything())
				workloadMemReqs := metricnaming.WorkloadMetricNamer(c.config.ClusterId, target, consts.MetricMemRequest, labels.Everything())
				workloadMemLims := metricnaming.WorkloadMetricNamer(c.config.ClusterId, target, consts.MetricMemLimit, labels.Everything())
				workloadReplicas := metricnaming.WorkloadMetricNamer(c.config.ClusterId, target, consts.MetricWorkloadReplicas, labels.Everything())

				wkCpuUsageTs, err := c.dataSource.QueryTimeSeries(context.TODO(), workloadCpuUsage, qRange.Start, qRange.End, qRange.Step)
				if err != nil {
					klog.Errorf("Failed to query history for metric %v: %v", workloadCpuUsage.BuildUniqueKey(), err)
					continue
				}
				wkMemUsageTs, err := c.dataSource.QueryTimeSeries(context.TODO(), workloadMemUsage, qRange.Start, qRange.End, qRange.Step)
				if err != nil {
					klog.Errorf("Failed to query history for metric %v: %v", workloadMemUsage.BuildUniqueKey(), err)
					continue
				}
				wkCpuReqTs, err := c.dataSource.QueryTimeSeries(context.TODO(), workloadCpuReqs, qRange.Start, qRange.End, qRange.Step)
				if err != nil {
					klog.Errorf("Failed to query history for metric %v: %v", workloadCpuReqs.BuildUniqueKey(), err)
					continue
				}
				wkCpuLimTs, err := c.dataSource.QueryTimeSeries(context.TODO(), workloadCpuLims, qRange.Start, qRange.End, qRange.Step)
				if err != nil {
					klog.Errorf("Failed to query history for metric %v: %v", workloadCpuLims.BuildUniqueKey(), err)
					continue
				}
				wkMemReqTs, err := c.dataSource.QueryTimeSeries(context.TODO(), workloadMemReqs, qRange.Start, qRange.End, qRange.Step)
				if err != nil {
					klog.Errorf("Failed to query history for metric %v: %v", workloadMemReqs.BuildUniqueKey(), err)
					continue
				}
				wkMemLimTs, err := c.dataSource.QueryTimeSeries(context.TODO(), workloadMemLims, qRange.Start, qRange.End, qRange.Step)
				if err != nil {
					klog.Errorf("Failed to query history for metric %v: %v", workloadMemLims.BuildUniqueKey(), err)
					continue
				}
				wkReplicasTs, err := c.dataSource.QueryTimeSeries(context.TODO(), workloadReplicas, qRange.Start, qRange.End, qRange.Step)
				if err != nil {
					klog.Errorf("Failed to query history for metric %v: %v", workloadReplicas.BuildUniqueKey(), err)
					continue
				}

				kindResult[nn] = &RawWorkloadTimeSeriesData{
					Cpu:         wkCpuUsageTs,
					Mem:         wkMemUsageTs,
					CpuRequests: wkCpuReqTs,
					MemRequests: wkMemReqTs,
					CpuLimits:   wkCpuLimTs,
					MemLimits:   wkMemLimTs,
					Replicas:    wkReplicasTs,
				}
			}
		}
	}
	return results
}

func (c *Comparator) fetchContainerData() map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ map[string] /*container*/ *RawContainerTimeSeriesData {
	results := make(map[string]map[types.NamespacedName]map[string]*RawContainerTimeSeriesData)
	workloads := c.workloadsSpecCache
	qRange := c.getQueryRange()
	for kind := range workloads {
		if kindWorkloads, ok := workloads[kind]; ok {
			kindResult, ok := results[kind]
			if !ok {
				kindResult = make(map[types.NamespacedName]map[string]*RawContainerTimeSeriesData)
				results[kind] = kindResult
			}
			for nn, workload := range kindWorkloads {
				workloadResult, ok := kindResult[nn]
				if !ok {
					workloadResult = make(map[string]*RawContainerTimeSeriesData)
					kindResult[nn] = workloadResult
				}
				for _, container := range workload.PodRef.Spec.Containers {
					cpu := metricnaming.ResourceToContainerMetricNamer(c.config.ClusterId, nn.Namespace, nn.Name, container.Name, v1.ResourceCPU)
					mem := metricnaming.ResourceToContainerMetricNamer(c.config.ClusterId, nn.Namespace, nn.Name, container.Name, v1.ResourceMemory)
					cpuRequest := metricnaming.ContainerMetricNamer(c.config.ClusterId, nn.Namespace, nn.Name, container.Name, consts.MetricCpuRequest, labels.Everything())
					memRequest := metricnaming.ContainerMetricNamer(c.config.ClusterId, nn.Namespace, nn.Name, container.Name, consts.MetricMemRequest, labels.Everything())
					cpuLimit := metricnaming.ContainerMetricNamer(c.config.ClusterId, nn.Namespace, nn.Name, container.Name, consts.MetricCpuLimit, labels.Everything())
					memLimit := metricnaming.ContainerMetricNamer(c.config.ClusterId, nn.Namespace, nn.Name, container.Name, consts.MetricMemLimit, labels.Everything())

					cpuTsList, err := c.dataSource.QueryTimeSeries(context.TODO(), cpu, qRange.Start, qRange.End, qRange.Step)
					if err != nil {
						klog.Errorf("Failed to query history for metric %v: %v", cpu.BuildUniqueKey(), err)
						continue
					}
					memTsList, err := c.dataSource.QueryTimeSeries(context.TODO(), mem, qRange.Start, qRange.End, qRange.Step)
					if err != nil {
						klog.Errorf("Failed to query history for metric %v: %v", mem.BuildUniqueKey(), err)
						continue
					}
					cpuReqTsList, err := c.dataSource.QueryTimeSeries(context.TODO(), cpuRequest, qRange.Start, qRange.End, qRange.Step)
					if err != nil {
						klog.Errorf("Failed to query history for metric %v: %v", cpuRequest.BuildUniqueKey(), err)
						continue
					}
					memReqTsList, err := c.dataSource.QueryTimeSeries(context.TODO(), memRequest, qRange.Start, qRange.End, qRange.Step)
					if err != nil {
						klog.Errorf("Failed to query history for metric %v: %v", memRequest.BuildUniqueKey(), err)
						continue
					}
					cpuLimTsList, err := c.dataSource.QueryTimeSeries(context.TODO(), cpuLimit, qRange.Start, qRange.End, qRange.Step)
					if err != nil {
						klog.Errorf("Failed to query history for metric %v: %v", cpuLimit.BuildUniqueKey(), err)
						continue
					}
					memLimTsList, err := c.dataSource.QueryTimeSeries(context.TODO(), memLimit, qRange.Start, qRange.End, qRange.Step)
					if err != nil {
						klog.Errorf("Failed to query history for metric %v: %v", memLimit.BuildUniqueKey(), err)
						continue
					}
					workloadResult[container.Name] = &RawContainerTimeSeriesData{
						Cpu:         cpuTsList,
						Mem:         memTsList,
						CpuRequests: cpuReqTsList,
						MemRequests: memReqTsList,
						CpuLimits:   cpuLimTsList,
						MemLimits:   memLimTsList,
					}

				}
			}
		}
	}
	return results
}

func (c *Comparator) GetWorkloadContainerData() map[string] /*kind*/ map[types.NamespacedName] /*namespace-name*/ map[string] /*container*/ *RawContainerTimeSeriesData {
	return c.containersTimeSeriesDataCache
}

// NOTE: memory & time consuming, now it is a tool for offline analytics
// todo: For online predicting service, we should use a model updating way not the offline once task way
func (c *Comparator) GetAllWorkloadRecommendedData() map[string]map[types.NamespacedName] /*namespace-name*/ *spec.WorkloadRecommendedData {
	results := make(map[string]map[types.NamespacedName]*spec.WorkloadRecommendedData)
	workloads := c.workloadsSpecCache
	workloadsContainerData := c.GetWorkloadContainerData()
	klog.V(4).Infof("Workloads %v, workloadsContainerData: %v", len(c.workloadsSpecCache), len(workloadsContainerData))

	for kind := range workloads {
		kindResult, ok := results[kind]
		if !ok {
			kindResult = make(map[types.NamespacedName]*spec.WorkloadRecommendedData)
			results[kind] = kindResult
		}
		for nn, workloadPodSpec := range workloads[kind] {
			wrd := &spec.WorkloadRecommendedData{
				Containers: make(map[string]*spec.ContainerRecommendedData),
			}

			recPod := workloadPodSpec.PodRef.DeepCopy()
			pertRecPod := workloadPodSpec.PodRef.DeepCopy()
			maxRecPod := workloadPodSpec.PodRef.DeepCopy()
			maxMarginRecPod := workloadPodSpec.PodRef.DeepCopy()

			for _, container := range workloadPodSpec.PodRef.Spec.Containers {
				kindWorkloadsContainerData, ok := workloadsContainerData[kind]
				if !ok {
					klog.Warningf("No cached data for kind %v", kind)
					continue
				}
				containersData, ok := kindWorkloadsContainerData[nn]
				if !ok {
					klog.Warningf("No cached data for kind %v, workload %v", kind, nn)
					continue
				}
				rawTsData, ok := containersData[container.Name]
				if !ok {
					klog.Warningf("No cached data for kind %v, workload %v, container %v", kind, nn, container.Name)
					continue
				}
				cpuTs := MergeTimeSeriesList(rawTsData.Cpu)
				cpuStatistics, err := c.estimator.Estimation(cpuTs, c.estimateConfig)
				if err != nil {
					klog.Errorf("Failed to estimate cpu for kind %v, workload %v, container %v, err: %v", kind, nn, container.Name, err)
					continue
				}
				memTs := MergeTimeSeriesList(rawTsData.Mem)
				memStatistics, err := c.estimator.Estimation(memTs, c.estimateConfig)
				if err != nil {
					klog.Errorf("Failed to estimate mem for kind %v, workload %v, container %v, err: %v", kind, nn, container.Name, err)
					continue
				}
				wrd.Containers[container.Name] = &spec.ContainerRecommendedData{
					Cpu: cpuStatistics,
					Mem: memStatistics,
				}
				cpuReqLimRatio := 1.0
				memReqLimRatio := 1.0
				if container.Resources.Requests != nil && container.Resources.Limits != nil {
					originalCpuReq, ok1 := container.Resources.Requests[v1.ResourceCPU]
					originalCpuLim, ok2 := container.Resources.Limits[v1.ResourceCPU]
					if ok1 && ok2 {
						cpuReqLimRatio = float64(originalCpuLim.MilliValue()) / float64(originalCpuReq.MilliValue())
					}

					originalMemReq, ok1 := container.Resources.Requests[v1.ResourceCPU]
					originalMemLim, ok2 := container.Resources.Limits[v1.ResourceCPU]
					if ok1 && ok2 {
						memReqLimRatio = float64(originalMemLim.MilliValue()) / float64(originalMemReq.MilliValue())
					}
				}

				for i := range recPod.Spec.Containers {
					if container.Name == recPod.Spec.Containers[i].Name {

						if cpuStatistics.Recommended != nil && memStatistics.Recommended != nil {
							recCpu := resource.NewMilliQuantity(int64(*cpuStatistics.Recommended*1000), resource.DecimalSI)
							recMem := resource.NewQuantity(int64(*memStatistics.Recommended), resource.BinarySI)
							requests := v1.ResourceList{
								v1.ResourceCPU:    *recCpu,
								v1.ResourceMemory: *recMem,
							}
							recPod.Spec.Containers[i].Resources.Requests = requests
							if recPod.Spec.Containers[i].Resources.Limits != nil {
								// keep original resources behavior
								if _, ok := recPod.Spec.Containers[i].Resources.Limits[v1.ResourceCPU]; ok {
									limCpu := resource.NewMilliQuantity(int64(*cpuStatistics.Recommended*cpuReqLimRatio*1000), resource.DecimalSI)
									recPod.Spec.Containers[i].Resources.Limits[v1.ResourceCPU] = *limCpu
								}

								if _, ok := recPod.Spec.Containers[i].Resources.Limits[v1.ResourceMemory]; ok {
									limMem := resource.NewQuantity(int64(*memStatistics.Recommended*memReqLimRatio), resource.BinarySI)
									recPod.Spec.Containers[i].Resources.Limits[v1.ResourceMemory] = *limMem
								}
							}
						}

						if cpuStatistics.Max != nil && memStatistics.Max != nil {
							maxCpu := resource.NewMilliQuantity(int64(*cpuStatistics.Max*1000), resource.DecimalSI)
							maxMem := resource.NewQuantity(int64(*memStatistics.Max), resource.BinarySI)
							resourceList := v1.ResourceList{
								v1.ResourceCPU:    *maxCpu,
								v1.ResourceMemory: *maxMem,
							}
							maxRecPod.Spec.Containers[i].Resources.Requests = resourceList

							if maxRecPod.Spec.Containers[i].Resources.Limits != nil {
								// keep original resources behavior
								if _, ok := maxRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceCPU]; ok {
									limCpu := resource.NewMilliQuantity(int64(*cpuStatistics.Max*cpuReqLimRatio*1000), resource.DecimalSI)
									maxRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceCPU] = *limCpu
								}

								if _, ok := maxRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceMemory]; ok {
									limMem := resource.NewQuantity(int64(*memStatistics.Max*memReqLimRatio), resource.BinarySI)
									maxRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceMemory] = *limMem
								}
							}
						}

						if cpuStatistics.MaxRecommended != nil && memStatistics.MaxRecommended != nil {
							maxMarginRecCpu := resource.NewMilliQuantity(int64(*cpuStatistics.MaxRecommended*1000), resource.DecimalSI)
							maxMarginRecMem := resource.NewQuantity(int64(*memStatistics.MaxRecommended), resource.BinarySI)
							resourceList := v1.ResourceList{
								v1.ResourceCPU:    *maxMarginRecCpu,
								v1.ResourceMemory: *maxMarginRecMem,
							}
							maxMarginRecPod.Spec.Containers[i].Resources.Requests = resourceList

							if maxMarginRecPod.Spec.Containers[i].Resources.Limits != nil {
								// keep original resources behavior
								if _, ok := maxMarginRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceCPU]; ok {
									limCpu := resource.NewMilliQuantity(int64(*cpuStatistics.MaxRecommended*cpuReqLimRatio*1000), resource.DecimalSI)
									maxMarginRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceCPU] = *limCpu
								}

								if _, ok := maxMarginRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceMemory]; ok {
									limMem := resource.NewQuantity(int64(*memStatistics.MaxRecommended*memReqLimRatio), resource.BinarySI)
									maxMarginRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceMemory] = *limMem
								}
							}
						}

						if cpuStatistics.Percentile != nil && memStatistics.Percentile != nil {
							recPercentileCpu := resource.NewMilliQuantity(int64(*cpuStatistics.Percentile*1000), resource.DecimalSI)
							recPercentileMem := resource.NewQuantity(int64(*memStatistics.Percentile), resource.BinarySI)
							resourceList := v1.ResourceList{
								v1.ResourceCPU:    *recPercentileCpu,
								v1.ResourceMemory: *recPercentileMem,
							}
							pertRecPod.Spec.Containers[i].Resources.Requests = resourceList

							if pertRecPod.Spec.Containers[i].Resources.Limits != nil {
								// keep original resources behavior
								if _, ok := pertRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceCPU]; ok {
									limCpu := resource.NewMilliQuantity(int64(*cpuStatistics.Percentile*cpuReqLimRatio*1000), resource.DecimalSI)
									pertRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceCPU] = *limCpu
								}

								if _, ok := pertRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceMemory]; ok {
									limMem := resource.NewQuantity(int64(*memStatistics.Percentile*memReqLimRatio), resource.BinarySI)
									pertRecPod.Spec.Containers[i].Resources.Limits[v1.ResourceMemory] = *limMem
								}
							}
						}
					}
				}

				wrd.RecommendedSpec = c.baselineCloud.Pod2ServerlessSpec(recPod)
				wrd.RecommendedSpec.PodRef = recPod
				wrd.RecommendedSpec.GoodsNum = workloadPodSpec.GoodsNum

				maxRecSpec := c.baselineCloud.Pod2ServerlessSpec(maxRecPod)
				maxRecSpec.PodRef = maxRecPod
				wrd.MaxRecommendedSpec = &maxRecSpec
				wrd.MaxRecommendedSpec.GoodsNum = workloadPodSpec.GoodsNum

				maxMarginRecSpec := c.baselineCloud.Pod2ServerlessSpec(maxMarginRecPod)
				maxMarginRecSpec.PodRef = maxMarginRecPod
				wrd.MaxMarginRecommendedSpec = &maxMarginRecSpec
				wrd.MaxMarginRecommendedSpec.GoodsNum = workloadPodSpec.GoodsNum

				percentRecSpec := c.baselineCloud.Pod2ServerlessSpec(pertRecPod)
				percentRecSpec.PodRef = pertRecPod
				wrd.PercentRecommendedSpec = &percentRecSpec
				wrd.PercentRecommendedSpec.GoodsNum = workloadPodSpec.GoodsNum

			}
			if klog.V(7).Enabled() {
				data, _ := jsoniter.Marshal(wrd)
				klog.V(7).Infof("Workload %v, %s", nn, wrd, string(data))
			}
			kindResult[nn] = wrd
		}
	}
	return results
}

func MergeTimeSeriesList(tsList []*common.TimeSeries) *common.TimeSeries {
	result := common.NewTimeSeries()
	for _, ts := range tsList {
		for _, label := range ts.Labels {
			result.AppendLabel(label.Name, label.Value)
		}
		for _, sample := range ts.Samples {
			result.AppendSample(sample.Timestamp, sample.Value)
		}
	}
	return result
}

func ServerlessWorkloadsResourceTotal(workloadsRecs map[string]map[types.NamespacedName] /*namespace-name*/ *spec.WorkloadRecommendedData) v1.ResourceList {
	var totalCpu resource.Quantity
	var totalMem resource.Quantity
	kindsCpuTotal := make(map[string]resource.Quantity)
	kindsMemTotal := make(map[string]resource.Quantity)
	for kind, kindWorkloadsRecs := range workloadsRecs {
		// serverless is not support kind daemonset
		if strings.ToLower(kind) == "daemonset" {
			continue
		}
		kindCpuTotal, ok := kindsCpuTotal[kind]
		if !ok {
			kindCpuTotal = *resource.NewMilliQuantity(0, resource.DecimalSI)
			kindsCpuTotal[kind] = kindCpuTotal
		}
		kindMemTotal, ok := kindsMemTotal[kind]
		if !ok {
			kindMemTotal = *resource.NewMilliQuantity(0, resource.BinarySI)
			kindsMemTotal[kind] = kindMemTotal
		}
		for _, workloadRecs := range kindWorkloadsRecs {
			totalCpu.Add(workloadRecs.RecommendedSpec.Cpu)
			totalMem.Add(workloadRecs.RecommendedSpec.Mem)
			kindCpuTotal.Add(workloadRecs.RecommendedSpec.Cpu)
			kindMemTotal.Add(workloadRecs.RecommendedSpec.Mem)
		}
	}
	return v1.ResourceList{
		v1.ResourceCPU:    totalCpu,
		v1.ResourceMemory: totalMem,
	}
}
