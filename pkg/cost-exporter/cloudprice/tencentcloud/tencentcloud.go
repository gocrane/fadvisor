package tencentcloud

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"

	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud"
	sdkcvm "github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/cvm"
	"github.com/gocrane/fadvisor/pkg/consts"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cache"
	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudprice"
	"github.com/gocrane/fadvisor/pkg/util"
)

var _ cloudprice.CloudPrice = &TencentCloud{}

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
	region, ok := qcloud.ShortName2region[regionShortName]
	if !ok {
		return ""
	}
	return region.Region
}

type TencentCloud struct {
	cvm *sdkcvm.CVMClient

	providerConfig *cloudprice.ProviderConfig

	lock sync.Mutex
	// cached standard price quota
	// this price is from standard inquiry instance price, it is just a reference because each customer has different adjustments for the instance in real world
	// key is (zone + instanceType + instanceChargeType) for node;
	standardPricing map[string]*cvm.InstanceTypeQuotaItem

	// cached instances
	instanceLock sync.RWMutex
	// key is ins id
	instances map[string]*sdkcvm.QCloudInstancePrice

	cache cache.Cache
}

func NewTencentCloud(qcloudConf *qcloud.QCloudClientConfig, config *cloudprice.ProviderConfig, cache cache.Cache) cloudprice.CloudPrice {
	cvmClient := sdkcvm.NewCVMClient(qcloudConf)
	return &TencentCloud{
		cvm:             cvmClient,
		providerConfig:  config,
		cache:           cache,
		standardPricing: make(map[string]*cvm.InstanceTypeQuotaItem),
		instances:       make(map[string]*sdkcvm.QCloudInstancePrice),
	}
}

// UpdateConfigFromConfigMap update CustomPricing from configmap
func (tc *TencentCloud) UpdateConfigFromConfigMap(conf map[string]string) (*cloudprice.CustomPricing, error) {
	return tc.providerConfig.UpdateConfigFromConfigMap(conf)
}

// GetConfig return CustomPricing
func (tc *TencentCloud) GetConfig() (*cloudprice.CustomPricing, error) {
	return tc.providerConfig.GetConfig()
}

func (tc *TencentCloud) getNodeRegion(node *v1.Node) string {
	regionShortName, _ := util.GetRegion(node.Labels)
	if regionStruct, ok := qcloud.ShortName2region[regionShortName]; ok {
		return regionStruct.Region
	} else {
		return regionShortName
	}
}

func (tc *TencentCloud) getDefaultNodePrice(cfg *cloudprice.CustomPricing, node *v1.Node) (*cloudprice.Node, error) {
	usageType := "Default"
	insType, _ := util.GetInstanceType(node.Labels)
	region := tc.getNodeRegion(node)
	cpuCores := node.Status.Capacity[v1.ResourceCPU]
	memory := node.Status.Capacity[v1.ResourceMemory]
	cpu := float64(cpuCores.Value())
	mem := float64(memory.Value())
	return &cloudprice.Node{
		BaseInstancePrice: cloudprice.BaseInstancePrice{
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
func (tc *TencentCloud) getCloudInstancePrice(node *v1.Node) (*cloudprice.Node, error) {
	nodePrice := &cloudprice.Node{
		BaseInstancePrice: cloudprice.BaseInstancePrice{},
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

	if usageType == qcloud.INSTANCECHARGETYPE_PREPAID {
		// prepaid original price is for one month.
		// todo: we divided by 30*24 hours to compute a avg hourly cost now
		cost := *price.OriginalPrice / float64(30*24)
		return &cloudprice.Node{
			BaseInstancePrice: cloudprice.BaseInstancePrice{
				Cost:            fmt.Sprintf("%v", cost),
				Cpu:             fmt.Sprintf("%v", cpu),
				Ram:             fmt.Sprintf("%v", mem/consts.GB),
				RamBytes:        fmt.Sprintf("%v", mem),
				DefaultCpuPrice: fmt.Sprintf("%v", cfg.CpuHourlyPrice),
				DefaultRamPrice: fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
				UsageType:       qcloud.INSTANCECHARGETYPE_PREPAID,
				InstanceType:    insType,
				Region:          region,
				ProviderID:      node.Spec.ProviderID,
			},
		}, nil
	} else if usageType == qcloud.INSTANCECHARGETYPE_POSTPAID_BY_HOUR {
		cost := *price.UnitPrice
		return &cloudprice.Node{
			BaseInstancePrice: cloudprice.BaseInstancePrice{
				Cost:            fmt.Sprintf("%v", cost),
				Cpu:             fmt.Sprintf("%v", cpu),
				Ram:             fmt.Sprintf("%v", mem/consts.GB),
				RamBytes:        fmt.Sprintf("%v", mem),
				DefaultCpuPrice: fmt.Sprintf("%v", cfg.CpuHourlyPrice),
				DefaultRamPrice: fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
				UsageType:       qcloud.INSTANCECHARGETYPE_POSTPAID_BY_HOUR,
				InstanceType:    insType,
				Region:          region,
				ProviderID:      node.Spec.ProviderID,
			},
		}, nil
	} else if usageType == qcloud.INSTANCECHARGETYPE_SPOTPAID {
		// now use the unit price too.
		cost := *price.UnitPrice
		return &cloudprice.Node{
			BaseInstancePrice: cloudprice.BaseInstancePrice{
				Cost:            fmt.Sprintf("%v", cost),
				Cpu:             fmt.Sprintf("%v", cpu),
				Ram:             fmt.Sprintf("%v", mem/consts.GB),
				RamBytes:        fmt.Sprintf("%v", mem),
				DefaultCpuPrice: fmt.Sprintf("%v", cfg.CpuHourlyPrice),
				DefaultRamPrice: fmt.Sprintf("%v", cfg.RamGBHourlyPrice),
				UsageType:       qcloud.INSTANCECHARGETYPE_SPOTPAID,
				InstanceType:    insType,
				Region:          region,
				ProviderID:      node.Spec.ProviderID,
			},
		}, nil
	} else {
		return tc.getDefaultNodePrice(cfg, node)
	}
}

func (tc *TencentCloud) GetNodesCost() (map[string]*cloudprice.Node, error) {
	nodes := make(map[string]*cloudprice.Node)
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

func (tc *TencentCloud) computeNodeBreakdownCost(cfg *cloudprice.CustomPricing, node *v1.Node) (*cloudprice.Node, error) {
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
		it, _ := util.GetInstanceType(node.Labels)
		newCnode.InstanceType = it
	}
	if newCnode.Region == "" {
		region := tc.getNodeRegion(node)
		newCnode.Region = region
	}
	newCnode.ProviderID = node.Spec.ProviderID

	var cpu float64
	if newCnode.Cpu == "" {
		cpu = float64(node.Status.Capacity.Cpu().Value())
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

	var ram float64
	if newCnode.Ram == "" {
		newCnode.Ram = node.Status.Capacity.Memory().String()
	}
	ram = float64(node.Status.Capacity.Memory().Value())
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
		newCnode.RamBytes = fmt.Sprintf("%f", ram)

		klog.V(3).Infof("Computed Node Cost cost: %v, node: %v, key: %v", node.Name, newCnode.RamGBHourlyCost, tc.GetKey(node).Features())
	}
	return &newCnode, nil
}

func (tc *TencentCloud) GetPodsCost() (map[string]*cloudprice.Pod, error) {
	pods := make(map[string]*cloudprice.Pod)

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
		podPrice := &cloudprice.Pod{
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

func (tc *TencentCloud) GetNodesPricing() (map[string]*cloudprice.Price, error) {
	tc.instanceLock.RLock()
	defer tc.instanceLock.RUnlock()
	results := make(map[string]*cloudprice.Price)
	for id, insPrice := range tc.instances {
		qPrice := insPrice.Price.InstancePrice
		results[id] = &cloudprice.Price{
			InstanceType: *insPrice.Instance.InstanceType,
			ChargeType:   *insPrice.Instance.InstanceChargeType,
			VCpu:         fmt.Sprintf("%v", *insPrice.Instance.CPU),
			Memory:       fmt.Sprintf("%v", *insPrice.Instance.Memory),
			CvmPrice: &cloudprice.PriceItem{
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

const (
	ValueNodeTypeEKLet = "eklet"

	labelNodeInstanceVersion   = "eks.tke.cloud.tencent.com/version"
	valueNodeInstanceVersionV2 = "v2"
)
