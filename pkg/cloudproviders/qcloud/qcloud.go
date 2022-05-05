package qcloud

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
	resourcehelper "k8s.io/kubernetes/pkg/api/v1/resource"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper/qos"

	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	tke "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"

	"github.com/gocrane/fadvisor/pkg/cache"
	qcloudsdk "github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
	sdkcvm "github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/cvm"
	sdktke "github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/tke"

	"github.com/gocrane/fadvisor/pkg/cloud"
	"github.com/gocrane/fadvisor/pkg/consts"
	"github.com/gocrane/fadvisor/pkg/spec"
	"github.com/gocrane/fadvisor/pkg/util"
)

type CloudConfig struct {
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

type qcloudKey struct {
	SpotLabelName  string
	SpotLabelValue string
	Labels         map[string]string
	ProviderID     string
	Zone           string
	ChargeType     string
}

func (k *qcloudKey) GPUType() string {
	return ""
}

func (k *qcloudKey) ID() string {
	provIdRx := regexp.MustCompile("qcloud:///([^/]+)/([^/]+)") // It's of the form qcloud:///800005/ins-2jv4wpmr and we want ins-2jv4wpmr, if it exists
	for matchNum, group := range provIdRx.FindStringSubmatch(k.ProviderID) {
		if matchNum == 2 {
			return group
		}
	}
	klog.V(3).Infof("TencentCloud Could not find instance ID, ProviderID: %v", k.ProviderID)
	return ""
}

func (k *qcloudKey) Features() string {
	instanceType, _ := util.GetInstanceType(k.Labels)
	// note: tke node zone is zoneId, convert it to zone
	//zoneId, _ := util.GetZone(k.Labels)

	key := k.Zone + "," + instanceType + "," + k.ChargeType
	return key
}

func (k *qcloudKey) Region() string {
	regionShortName, _ := util.GetRegion(k.Labels)
	region, ok := qcloudsdk.ShortName2region[regionShortName]
	if !ok {
		return ""
	}
	return region.Region
}

var _ cloud.Cloud = &TencentCloud{}

type TencentCloud struct {
	cache cache.Cache
	cvm   *sdkcvm.CVMClient
	tke   *sdktke.TKEClient

	priceConfig *cloud.PriceConfig

	lock sync.Mutex
	// cached standard price quota
	// this price is from standard inquiry instance price, it is just a reference because each customer has different adjustments for the instance in real world
	// key is (zone + instanceType + instanceChargeType) for node;
	standardPricing map[string]*cvm.InstanceTypeQuotaItem

	// cached instances
	instanceLock sync.RWMutex
	// key is ins id
	instances map[string]*sdkcvm.QCloudInstancePrice

	eksPlatformer *EKSPlatform
	tkePlatformer *TKEPlatform
	eksConverter  Pod2EKSSpecConverter
}

func NewTencentCloud(qcloudConf *qcloudsdk.QCloudClientConfig, config *cloud.PriceConfig, cache cache.Cache) cloud.Cloud {
	cvmClient := sdkcvm.NewCVMClient(qcloudConf)
	tkeClient := sdktke.NewTKEClient(qcloudConf)
	return &TencentCloud{
		cvm:             cvmClient,
		tke:             tkeClient,
		priceConfig:     config,
		cache:           cache,
		standardPricing: make(map[string]*cvm.InstanceTypeQuotaItem),
		instances:       make(map[string]*sdkcvm.QCloudInstancePrice),
		eksPlatformer:   &EKSPlatform{},
		tkePlatformer:   &TKEPlatform{},
		eksConverter:    tkeClient,
	}
}

func (q *TencentCloud) PlatformPrice(cp cloud.PlatformParameter) *cloud.Prices {
	switch cp.Platform {
	case cloud.ServerfulKind:
		return q.tkePlatformer.PlatformCost(cp)
	case cloud.ServerlessKind:
		return q.eksPlatformer.PlatformCost(cp)
	default:
		klog.Warningf("unknown platform kind, only support serverless/serverful", cp.Platform)
		return q.tkePlatformer.PlatformCost(cp)
	}
}

func (tc *TencentCloud) NodePrice(spec spec.CloudNodeSpec) (*cloud.Node, error) {
	cfg, err := tc.priceConfig.GetConfig()
	if err != nil {
		return nil, err
	}
	newCnode, err := tc.computeNodeBreakdownCost(cfg, spec.NodeRef)
	if err != nil {
		return nil, err
	}
	return newCnode, nil
}

func CloudPodSpec2EKSPriceRequest(spec spec.CloudPodSpec) *tke.GetPriceRequest {
	cpu := float64(spec.Cpu.MilliValue()) / 1000.0
	mem := float64(spec.Mem.Value()) / consts.GB
	req := tke.NewGetPriceRequest()
	req.Cpu = &cpu
	req.Mem = &mem
	req.TimeSpan = &spec.TimeSpan
	req.GoodsNum = &spec.GoodsNum
	if spec.MachineArch != "" {
		req.Type = &spec.MachineArch
	}
	if spec.PodChargeType != "" {
		req.PodType = &spec.PodChargeType
	}
	if spec.Zone != "" {
		req.Zone = &spec.Zone
	}
	return req
}

func (tc *TencentCloud) ServerlessPodPrice(spec spec.CloudPodSpec) (*cloud.Pod, error) {
	price, err := tc.tke.GetEKSPodPrice(CloudPodSpec2EKSPriceRequest(spec))
	if err != nil {
		return nil, err
	}
	cfg, err := tc.priceConfig.GetConfig()
	if err != nil {
		return nil, err
	}

	cpu := float64(spec.Cpu.MilliValue()) / 1000.
	ram := float64(spec.Mem.Value())

	var cost, discountCost float64
	// price unit is cent
	if price.Response != nil && price.Response.Cost != nil {
		discountCost = float64(*price.Response.Cost) / 100.
		cost = float64(*price.Response.TotalCost) / 100.
	}
	newCnode := &cloud.Pod{
		BaseInstancePrice: cloud.BaseInstancePrice{
			DiscountedCost: fmt.Sprintf("%f", discountCost),
			Cost:           fmt.Sprintf("%f", cost),
			Cpu:            fmt.Sprintf("%f", cpu),
			Ram:            fmt.Sprintf("%f", ram/consts.GB),
			RamBytes:       fmt.Sprintf("%f", ram),
		},
	}

	defaultCPU := cfg.CpuHourlyPrice

	if math.IsNaN(defaultCPU) {
		klog.V(3).Infof("DefaultCPU parsed as NaN. Setting to 0. key: %v", klog.KObj(spec.PodRef))
		defaultCPU = 0
	}

	defaultRAM := cfg.RamGBHourlyPrice

	if math.IsNaN(defaultRAM) {
		klog.V(3).Infof("DefaultRAM parsed as NaN. Setting to 0. key: %v", klog.KObj(spec.PodRef))
		defaultRAM = 0
	}

	cpuToRAMRatio := defaultCPU / defaultRAM
	if math.IsNaN(cpuToRAMRatio) {
		klog.V(3).Infof("cpuToRAMRatio is NaN. Setting to 0. defaultCPU: %v, defaultRAM: %v, key: %v", defaultCPU, defaultRAM, klog.KObj(spec.PodRef))
		cpuToRAMRatio = 0
	}

	ramGB := ram / consts.GB
	newCnode.Ram = fmt.Sprintf("%f", ramGB)
	if math.IsNaN(ramGB) {
		klog.V(3).Infof("ramGB is NaN. Setting to 0. key: %v", klog.KObj(spec.PodRef))
		ramGB = 0
	}

	ramMultiple := cpu*cpuToRAMRatio + ramGB
	if math.IsNaN(ramMultiple) {
		klog.V(3).Infof("ramMultiple is NaN. Setting to 0. key: %v", klog.KObj(spec.PodRef))
		ramMultiple = 0
	}

	var podPrice float64
	if newCnode.Cost != "" {
		podPrice, err = strconv.ParseFloat(newCnode.Cost, 64)
		if err != nil {
			klog.V(3).Infof("Could not parse total pod price, key: %v", klog.KObj(spec.PodRef))
			return nil, err
		}
	} else {
		// default use cpu price to represent
		podPrice, err = strconv.ParseFloat(newCnode.CpuHourlyCost, 64)
		if err != nil {
			klog.V(3).Infof("Could not parse total pod cpu price, key: %v", klog.KObj(spec.PodRef))
			return nil, err
		}
	}

	if math.IsNaN(podPrice) {
		klog.V(3).Infof("nodePrice is NaN. Setting to 0. key: %v", klog.KObj(spec.PodRef))
		podPrice = 0
	}

	ramPrice := podPrice / ramMultiple
	if math.IsNaN(ramPrice) {
		klog.V(3).Infof("ramPrice[podPrice / ramMultiple] parsed as NaN. Setting to 0. podPrice: %v, ramMultiple: %v, key: %v", podPrice, ramMultiple, klog.KObj(spec.PodRef))
		ramPrice = 0
	}

	cpuPrice := ramPrice * cpuToRAMRatio

	if defaultRAM != 0 {
		newCnode.CpuHourlyCost = fmt.Sprintf("%f", cpuPrice)
		newCnode.RamGBHourlyCost = fmt.Sprintf("%f", ramPrice)
	} else {
		if cpu != 0 {
			newCnode.CpuHourlyCost = fmt.Sprintf("%f", podPrice/cpu)
		} else {
			newCnode.CpuHourlyCost = fmt.Sprintf("%f", podPrice)
		}
	}

	klog.V(6).Infof("ServerlessPodPrice for key %v, price %+v", klog.KObj(spec.PodRef), *newCnode)
	return newCnode, nil
}

func (tc *TencentCloud) PodPrice(spec spec.CloudPodSpec) (*cloud.Pod, error) {
	panic("implement me")
}

func (tc *TencentCloud) IsVirtualNode(node *v1.Node) bool {
	if node == nil {
		return false
	}
	if len(node.Labels) == 0 {
		return false
	}

	t, ok := node.Labels[v1.LabelInstanceTypeStable]
	if !ok || t == "" {
		t, ok = node.Labels[v1.LabelInstanceType]
		if !ok || t == "" {
			return false
		}
	}

	if t != ValueNodeTypeEKLet {
		return false
	}

	v, ok := node.Labels[labelNodeInstanceVersion]
	if !ok || v == "" {
		return false
	}
	if v != valueNodeInstanceVersionV2 {
		return false
	}

	return true
}

func (tc *TencentCloud) IsServerlessPod(pod *v1.Pod) bool {
	nodeList := tc.cache.GetNodes()
	nodesMap := make(map[string]*v1.Node)
	for _, node := range nodeList {
		nodesMap[node.Name] = node
	}
	if node, ok := nodesMap[pod.Spec.NodeName]; ok && tc.IsVirtualNode(node) {
		return true
	}
	return false
}

// convert pod to serverless pod spec, no matter the pod is in real node or virtual node.
func (tc *TencentCloud) Pod2ServerlessSpec(pod *v1.Pod) spec.CloudPodSpec {
	reqs, lims := resourcehelper.PodRequestsAndLimits(pod)
	machineType := EKSPodCpuType(pod)
	exists, gpuType := EKSPodGpuType(pod)
	if exists {
		machineType = gpuType
	}
	qosClass := qos.GetPodQOS(pod)
	refs := pod.GetOwnerReferences()
	// eks not support daemonset now. so there is no daemonset pod resource, return zero
	if len(refs) > 0 && strings.ToLower(refs[0].Kind) == "daemonset" {
		return spec.CloudPodSpec{
			PodRef:      pod,
			Cpu:         reqs[v1.ResourceCPU],
			Mem:         reqs[v1.ResourceMemory],
			CpuLimit:    lims[v1.ResourceCPU],
			MemLimit:    lims[v1.ResourceMemory],
			GoodsNum:    0,
			TimeSpan:    3600,
			MachineArch: machineType,
			Serverless:  false,
			QoSClass:    qosClass,
		}
	}
	if qosClass == v1.PodQOSBestEffort {
		// BestEffort is 1C2G in eks by default
		memorySize := resource.NewQuantity(2*1024*1024*1024, resource.BinarySI)
		cores := resource.NewMilliQuantity(1000, resource.DecimalSI)
		reqs[v1.ResourceCPU] = *cores
		reqs[v1.ResourceMemory] = *memorySize
		lims[v1.ResourceCPU] = *cores
		lims[v1.ResourceMemory] = *memorySize
	} else {
		resourceList, err := tc.eksConverter.Pod2EKSSpecConverter(pod)
		if err != nil {
			klog.Errorf("Failed to convert pod %v to eks spec: %v, use default sum way", klog.KObj(pod), err)
		}
		for name, value := range resourceList {
			reqs[name] = value
			lims[name] = value
		}
	}
	return spec.CloudPodSpec{
		PodRef:      pod,
		Cpu:         reqs[v1.ResourceCPU],
		Mem:         reqs[v1.ResourceMemory],
		CpuLimit:    lims[v1.ResourceCPU],
		MemLimit:    lims[v1.ResourceMemory],
		GoodsNum:    1,
		TimeSpan:    3600,
		MachineArch: machineType,
		Serverless:  true,
		QoSClass:    qosClass,
	}
}

func (tc *TencentCloud) Pod2Spec(pod *v1.Pod) spec.CloudPodSpec {
	isServerless := false
	reqs, lims := resourcehelper.PodRequestsAndLimits(pod)
	machineType := EKSPodCpuType(pod)
	exists, gpuType := EKSPodGpuType(pod)
	if exists {
		machineType = gpuType
	}
	qosClass := qos.GetPodQOS(pod)
	if qosClass == v1.PodQOSBestEffort {
		// BestEffort is 1C2G in eks by default
		memorySize := resource.NewQuantity(2*1024*1024*1024, resource.BinarySI)
		cores := resource.NewMilliQuantity(1000, resource.DecimalSI)
		reqs[v1.ResourceCPU] = *cores
		reqs[v1.ResourceMemory] = *memorySize
		lims[v1.ResourceCPU] = *cores
		lims[v1.ResourceMemory] = *memorySize
	}
	if tc.IsServerlessPod(pod) {
		isServerless = true
		if qosClass != v1.PodQOSBestEffort {
			resourceList, err := tc.eksConverter.Pod2EKSSpecConverter(pod)
			if err != nil {
				klog.Errorf("Failed to convert pod %v to eks spec: %v, use default sum way", klog.KObj(pod), err)
			}
			for name, value := range resourceList {
				reqs[name] = value
				lims[name] = value
			}
		}
	}
	return spec.CloudPodSpec{
		PodRef:      pod,
		Cpu:         reqs[v1.ResourceCPU],
		Mem:         reqs[v1.ResourceMemory],
		CpuLimit:    lims[v1.ResourceCPU],
		MemLimit:    lims[v1.ResourceMemory],
		GoodsNum:    1,
		TimeSpan:    3600,
		MachineArch: machineType,
		Serverless:  isServerless,
		QoSClass:    qosClass,
	}
}

func (tc *TencentCloud) Node2Spec(node *v1.Node) spec.CloudNodeSpec {
	usageType := "Default"
	insType, _ := util.GetInstanceType(node.Labels)
	region := tc.getNodeRegion(node)
	zone, _ := util.GetZone(node.Labels)
	cpuCores := node.Status.Capacity[v1.ResourceCPU]
	memory := node.Status.Capacity[v1.ResourceMemory]

	insId := ParseID(node.Spec.ProviderID)
	instance := tc.getInstanceById(insId)
	if instance != nil {
		if instance.InstanceChargeType != nil {
			usageType = *instance.InstanceChargeType
		}

		if instance.CPU != nil {
			cpuCores = *resource.NewMilliQuantity(*instance.CPU*1000, resource.DecimalSI)
		}
		if instance.Memory != nil {
			mem := float64(*instance.Memory) * consts.GB
			memory = *resource.NewQuantity(int64(mem), resource.BinarySI)
		}
		if instance.InstanceType != nil {
			insType = *instance.InstanceType
		}
	}

	return spec.CloudNodeSpec{
		NodeRef:      node,
		Cpu:          cpuCores,
		Mem:          memory,
		ChargeType:   usageType,
		InstanceType: insType,
		Zone:         zone,
		Region:       region,
		VirtualNode:  tc.IsVirtualNode(node),
	}
}

// UpdateConfigFromConfigMap update CustomPricing from configmap
func (tc *TencentCloud) UpdateConfigFromConfigMap(conf map[string]string) (*cloud.CustomPricing, error) {
	return tc.priceConfig.UpdateConfigFromConfigMap(conf)
}

// GetConfig return CustomPricing
func (tc *TencentCloud) GetConfig() (*cloud.CustomPricing, error) {
	return tc.priceConfig.GetConfig()
}

func (tc *TencentCloud) getNodeRegion(node *v1.Node) string {
	regionShortName, _ := util.GetRegion(node.Labels)
	if regionStruct, ok := qcloudsdk.ShortName2region[regionShortName]; ok {
		return regionStruct.Region
	} else {
		return regionShortName
	}
}

func (tc *TencentCloud) getDefaultNodePrice(cfg *cloud.CustomPricing, node *v1.Node) (*cloud.Node, error) {
	usageType := "Default"
	insType, _ := util.GetInstanceType(node.Labels)
	region := tc.getNodeRegion(node)
	cpuCores := node.Status.Capacity[v1.ResourceCPU]
	memory := node.Status.Capacity[v1.ResourceMemory]
	cpu := float64(cpuCores.Value())
	mem := float64(memory.Value())
	return &cloud.Node{
		BaseInstancePrice: cloud.BaseInstancePrice{
			Cost:             fmt.Sprintf("%v", cfg.CpuHourlyPrice*cpu+cfg.RamGBHourlyPrice*mem/consts.GB),
			Cpu:              fmt.Sprintf("%v", cpu),
			CpuHourlyCost:    fmt.Sprintf("%v", cfg.CpuHourlyPrice),
			Ram:              fmt.Sprintf("%v", mem/consts.GB),
			RamBytes:         fmt.Sprintf("%v", mem),
			RamGBHourlyCost:  fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
			DefaultCpuPrice:  fmt.Sprintf("%v", cfg.CpuHourlyPrice),
			DefaultRamPrice:  fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
			UsageType:        usageType,
			UsesDefaultPrice: true,
			InstanceType:     insType,
			ProviderID:       node.Spec.ProviderID,
			Region:           region,
		},
	}, nil
}

// do not support virtual node, virtual node has dynamic price depends on its eks pods
func (tc *TencentCloud) getCloudInstancePrice(node *v1.Node) (*cloud.Node, error) {
	nodePrice := &cloud.Node{
		BaseInstancePrice: cloud.BaseInstancePrice{},
	}
	cfg, err := tc.GetConfig()
	if err != nil {
		return nodePrice, err
	}
	if cfg == nil {
		return nodePrice, fmt.Errorf("custompricing config is null")
	}

	usageType := "Default"
	insType, _ := util.GetInstanceType(node.Labels)
	region := tc.getNodeRegion(node)
	cpuCores := node.Status.Capacity[v1.ResourceCPU]
	memory := node.Status.Capacity[v1.ResourceMemory]
	cpu := float64(cpuCores.Value())
	mem := float64(memory.Value())

	insId := ParseID(node.Spec.ProviderID)
	insPrice := tc.GetInstancePrice(insId)
	if insPrice == nil || insPrice.Instance == nil || insPrice.Price.InstancePrice == nil {
		klog.Warningf("node (%v, %v) got no cache price", node.Name, insId)
		return tc.getDefaultNodePrice(cfg, node)
	}

	instance := insPrice.Instance
	if instance.InstanceChargeType != nil {
		usageType = *instance.InstanceChargeType
	}

	price := insPrice.Price.InstancePrice
	if instance.CPU != nil {
		cpu = float64(*instance.CPU)
	}
	if instance.Memory != nil {
		mem = float64(*instance.Memory) * consts.GB
	}
	if instance.InstanceType != nil {
		insType = *instance.InstanceType
	}

	if usageType == qcloudsdk.INSTANCECHARGETYPE_PREPAID {
		// prepaid original price is for one month.
		// todo: we divided by 30*24 hours to compute a avg hourly cost now
		cost := *price.OriginalPrice / float64(30*24)
		return &cloud.Node{
			BaseInstancePrice: cloud.BaseInstancePrice{
				Cost:            fmt.Sprintf("%v", cost),
				Cpu:             fmt.Sprintf("%v", cpu),
				Ram:             fmt.Sprintf("%v", mem/consts.GB),
				RamBytes:        fmt.Sprintf("%v", mem),
				DefaultCpuPrice: fmt.Sprintf("%v", cfg.CpuHourlyPrice),
				DefaultRamPrice: fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
				UsageType:       qcloudsdk.INSTANCECHARGETYPE_PREPAID,
				InstanceType:    insType,
				Region:          region,
				ProviderID:      node.Spec.ProviderID,
			},
		}, nil
	} else if usageType == qcloudsdk.INSTANCECHARGETYPE_POSTPAID_BY_HOUR {
		cost := *price.UnitPrice
		return &cloud.Node{
			BaseInstancePrice: cloud.BaseInstancePrice{
				Cost:            fmt.Sprintf("%v", cost),
				Cpu:             fmt.Sprintf("%v", cpu),
				Ram:             fmt.Sprintf("%v", mem/consts.GB),
				RamBytes:        fmt.Sprintf("%v", mem),
				DefaultCpuPrice: fmt.Sprintf("%v", cfg.CpuHourlyPrice),
				DefaultRamPrice: fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
				UsageType:       qcloudsdk.INSTANCECHARGETYPE_POSTPAID_BY_HOUR,
				InstanceType:    insType,
				Region:          region,
				ProviderID:      node.Spec.ProviderID,
			},
		}, nil
	} else if usageType == qcloudsdk.INSTANCECHARGETYPE_SPOTPAID {
		// now use the unit price too.
		cost := *price.UnitPrice
		return &cloud.Node{
			BaseInstancePrice: cloud.BaseInstancePrice{
				Cost:            fmt.Sprintf("%v", cost),
				Cpu:             fmt.Sprintf("%v", cpu),
				Ram:             fmt.Sprintf("%v", mem/consts.GB),
				RamBytes:        fmt.Sprintf("%v", mem),
				DefaultCpuPrice: fmt.Sprintf("%v", cfg.CpuHourlyPrice),
				DefaultRamPrice: fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
				UsageType:       qcloudsdk.INSTANCECHARGETYPE_SPOTPAID,
				InstanceType:    insType,
				Region:          region,
				ProviderID:      node.Spec.ProviderID,
			},
		}, nil
	} else {
		return tc.getDefaultNodePrice(cfg, node)
	}
}

func (tc *TencentCloud) GetNodesCost() (map[string]*cloud.Node, error) {
	nodes := make(map[string]*cloud.Node)
	cfg, err := tc.GetConfig()
	if err != nil {
		return nodes, err
	}
	if cfg == nil {
		return nodes, fmt.Errorf("provider config is null")
	}

	nodeList := tc.cache.GetNodes()
	for _, node := range nodeList {
		if tc.IsVirtualNode(node) {
			klog.Warningf("Ignore virtual node %v now.", node.Name)
			continue
		}
		newCnode, err := tc.computeNodeBreakdownCost(cfg, node)
		if err != nil {
			continue
		}
		nodes[node.Name] = newCnode
	}
	return nodes, nil
}

func (tc *TencentCloud) computeNodeBreakdownCost(cfg *cloud.CustomPricing, node *v1.Node) (*cloud.Node, error) {
	it, _ := util.GetInstanceType(node.Labels)
	region := tc.getNodeRegion(node)
	cpu := float64(node.Status.Capacity.Cpu().Value())
	ram := float64(node.Status.Capacity.Memory().Value())

	if tc.IsVirtualNode(node) {
		return &cloud.Node{
			BaseInstancePrice: cloud.BaseInstancePrice{
				Cost:            "0",
				CpuHourlyCost:   "0",
				Cpu:             fmt.Sprintf("%f", cpu),
				Ram:             fmt.Sprintf("%f", ram/consts.GB),
				RamBytes:        fmt.Sprintf("%f", ram),
				RamGBHourlyCost: "0",
				InstanceType:    it,
				Region:          region,
				ProviderID:      node.Spec.ProviderID,
			},
		}, nil
	}
	// real node
	cnodePrice, err := tc.getCloudInstancePrice(node)
	if err != nil {
		klog.Errorf("Failed to get node pricing, node: %v, key: %v, err: %v", node.Name, "key", tc.GetKey(node).Features(), err)
		if cnodePrice != nil {
			return cnodePrice, err
		} else {
			klog.Errorf("Failed to get node pricing, pricing is null. node: %v, key: %v, err: %v", node.Name, "key", tc.GetKey(node).Features(), err)
			return cnodePrice, err
		}
	}

	newCnode := *cnodePrice
	if newCnode.InstanceType == "" {
		newCnode.InstanceType = it

	}
	if newCnode.Region == "" {
		newCnode.Region = region
	}
	if newCnode.ProviderID == "" {
		newCnode.ProviderID = node.Spec.ProviderID
	}

	if newCnode.Cpu == "" {
		newCnode.Cpu = node.Status.Capacity.Cpu().String()
	} else {
		cpu, err = strconv.ParseFloat(newCnode.Cpu, 64)
		if err != nil {
			klog.V(3).Infof("Parsing VCPU value as float64 Cpu: %v", newCnode.Cpu)
		}
	}
	if math.IsNaN(cpu) {
		klog.V(3).Info("Cpu parsed as NaN. Setting to 0.")
		cpu = 0
	}

	if newCnode.Ram == "" {
		newCnode.Ram = node.Status.Capacity.Memory().String()
	}
	if math.IsNaN(ram) {
		klog.Info("Ram parsed as NaN. Setting to 0.")
		ram = 0
	}

	newCnode.RamBytes = fmt.Sprintf("%f", ram)

	if !cnodePrice.UsesDefaultPrice {
		klog.V(3).Infof("Need to calculating node price... node: %v, key: %v", node.Name, tc.GetKey(node).Features())

		defaultCPU := cfg.CpuHourlyPrice

		if math.IsNaN(defaultCPU) {
			klog.V(3).Infof("DefaultCPU parsed as NaN. Setting to 0. node: %v, key: %v", node.Name, tc.GetKey(node).Features())
			defaultCPU = 0
		}

		defaultRAM := cfg.RamGBHourlyPrice

		if math.IsNaN(defaultRAM) {
			klog.V(3).Infof("DefaultRAM parsed as NaN. Setting to 0. node: %v, key: %v", node.Name, tc.GetKey(node).Features())
			defaultRAM = 0
		}

		cpuToRAMRatio := defaultCPU / defaultRAM
		if math.IsNaN(cpuToRAMRatio) {
			klog.V(3).Infof("cpuToRAMRatio is NaN. Setting to 0. defaultCPU: %v, defaultRAM: %v, node: %v, key: %v", defaultCPU, defaultRAM, node.Name, tc.GetKey(node).Features())
			cpuToRAMRatio = 0
		}

		ramGB := ram / consts.GB
		newCnode.Ram = fmt.Sprintf("%f", ramGB)
		if math.IsNaN(ramGB) {
			klog.V(3).Infof("ramGB is NaN. Setting to 0. node: %v, key: %v", node.Name, tc.GetKey(node).Features())
			ramGB = 0
		}

		ramMultiple := cpu*cpuToRAMRatio + ramGB
		if math.IsNaN(ramMultiple) {
			klog.V(3).Infof("ramMultiple is NaN. Setting to 0. node: %v, key: %v", node.Name, tc.GetKey(node).Features())
			ramMultiple = 0
		}

		var nodePrice float64
		if newCnode.Cost != "" {
			nodePrice, err = strconv.ParseFloat(newCnode.Cost, 64)
			if err != nil {
				klog.V(3).Infof("Could not parse total node price, node: %v, key: %v", node.Name, tc.GetKey(node).Features())
				return nil, err
			}
		} else {
			// default use cpu price to represent
			nodePrice, err = strconv.ParseFloat(newCnode.CpuHourlyCost, 64)
			if err != nil {
				klog.V(3).Infof("Could not parse total node cpu price, node: %v, key: %v", node.Name, tc.GetKey(node).Features())
				return nil, err
			}
		}

		if math.IsNaN(nodePrice) {
			klog.V(3).Infof("nodePrice is NaN. Setting to 0. node: %v, key: %v", node.Name, tc.GetKey(node).Features())
			nodePrice = 0
		}

		ramPrice := nodePrice / ramMultiple
		if math.IsNaN(ramPrice) {
			klog.V(3).Infof("ramPrice[nodePrice / ramMultiple] parsed as NaN. Setting to 0. nodePrice: %v, ramMultiple: %v, node: %v, key: %v", nodePrice, ramMultiple, node.Name, tc.GetKey(node).Features())
			ramPrice = 0
		}

		cpuPrice := ramPrice * cpuToRAMRatio

		if defaultRAM != 0 {
			newCnode.CpuHourlyCost = fmt.Sprintf("%f", cpuPrice)
			newCnode.RamGBHourlyCost = fmt.Sprintf("%f", ramPrice)
		} else {
			if cpu != 0 {
				newCnode.CpuHourlyCost = fmt.Sprintf("%f", nodePrice/cpu)
			} else {
				newCnode.CpuHourlyCost = fmt.Sprintf("%f", nodePrice)
			}
		}

		klog.V(3).Infof("Computed Node Cost cost: %v, node: %v, key: %v", node.Name, newCnode.RamGBHourlyCost, tc.GetKey(node).Features())
	}
	return &newCnode, nil
}

func (tc *TencentCloud) GetPodsCost() (map[string]*cloud.Pod, error) {
	pods := make(map[string]*cloud.Pod)

	cfg, err := tc.GetConfig()
	if err != nil {
		return pods, err
	}
	if cfg == nil {
		return pods, fmt.Errorf("provider config is null")
	}

	nodeList := tc.cache.GetNodes()
	nodesMap := make(map[string]*v1.Node)
	for _, node := range nodeList {
		nodesMap[node.Name] = node
	}
	podList := tc.cache.GetPods()
	for _, pod := range podList {
		key := klog.KObj(pod).String()

		nodeName := pod.Spec.NodeName
		node := nodesMap[nodeName]
		if tc.IsVirtualNode(node) {
			klog.V(3).Infof("pod is in virtual node, ignore temporarily pod: %v, node: %v", klog.KObj(pod), klog.KObj(node))
			continue
		}

		nodePrice, err := tc.computeNodeBreakdownCost(cfg, node)
		if err != nil {
			klog.Errorf("Failed to computeNodeBreakdownCost pod: %v, node: %v", klog.KObj(pod), klog.KObj(node))
			continue
		}
		podPrice := &cloud.Pod{
			BaseInstancePrice: nodePrice.BaseInstancePrice,
		}
		pods[key] = podPrice
	}
	return pods, nil
}

func (tc *TencentCloud) GetKey(node *v1.Node) *qcloudKey {
	key := &qcloudKey{
		Labels:     node.Labels,
		ProviderID: node.Spec.ProviderID,
	}

	insID := key.ID()
	ins := tc.getInstanceById(insID)
	if ins == nil {
		klog.Warningf("TencentCloud instances cache missed! instanceId: %v", insID)
		return key
	}
	if ins.InstanceChargeType != nil {
		key.ChargeType = *ins.InstanceChargeType
	} else {
		klog.Warningf("TencentCloud instance InstanceChargeType missed instanceId: %v", insID)
	}
	if ins.Placement != nil && ins.Placement.Zone != nil {
		key.Zone = *ins.Placement.Zone
	} else {
		klog.Warningf("TencentCloud instance Placement missed instanceId: %v", insID)
	}
	return key
}

func (tc *TencentCloud) GetNodesPricing() (map[string]*cloud.Price, error) {
	tc.instanceLock.RLock()
	defer tc.instanceLock.RUnlock()
	results := make(map[string]*cloud.Price)
	for id, insPrice := range tc.instances {
		qPrice := insPrice.Price.InstancePrice
		results[id] = &cloud.Price{
			InstanceType: *insPrice.Instance.InstanceType,
			ChargeType:   *insPrice.Instance.InstanceChargeType,
			VCpu:         fmt.Sprintf("%v", *insPrice.Instance.CPU),
			Memory:       fmt.Sprintf("%v", *insPrice.Instance.Memory),
			CvmPrice: &cloud.PriceItem{
				UnitPrice:                   qPrice.UnitPrice,
				ChargeUnit:                  qPrice.ChargeUnit,
				OriginalPrice:               qPrice.OriginalPrice,
				DiscountPrice:               qPrice.DiscountPrice,
				Discount:                    qPrice.Discount,
				UnitPriceDiscount:           qPrice.UnitPriceDiscount,
				UnitPriceSecondStep:         qPrice.UnitPriceSecondStep,
				UnitPriceDiscountSecondStep: qPrice.UnitPriceDiscountSecondStep,
				UnitPriceThirdStep:          qPrice.UnitPriceThirdStep,
				UnitPriceDiscountThirdStep:  qPrice.UnitPriceDiscountThirdStep,
				OriginalPriceThreeYear:      qPrice.OriginalPriceThreeYear,
				DiscountPriceThreeYear:      qPrice.DiscountPriceThreeYear,
				DiscountThreeYear:           qPrice.DiscountThreeYear,
				OriginalPriceFiveYear:       qPrice.OriginalPriceFiveYear,
				DiscountPriceFiveYear:       qPrice.DiscountPriceFiveYear,
				DiscountFiveYear:            qPrice.DiscountFiveYear,
				OriginalPriceOneYear:        qPrice.OriginalPriceOneYear,
				DiscountPriceOneYear:        qPrice.DiscountPriceOneYear,
				DiscountOneYear:             qPrice.DiscountOneYear,
			},
		}
	}
	return results, nil
}
