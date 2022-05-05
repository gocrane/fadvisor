package qcloud

import (
	"regexp"
	"time"

	"k8s.io/klog/v2"

	v1 "k8s.io/api/core/v1"

	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"

	sdkcvm "github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/cvm"
)

var QCloudProviderIdRegex = regexp.MustCompile("qcloud:///([^/]+)/([^/]+)") // It's of the form qcloud:///800005/ins-2jv4wpmr and we want ins-2jv4wpmr, if it exists

// fill the charge type. because tencent cloud provider tke do not label the k8s node its charge type directly, must query from cloud cvm service.
// but you can not query it each time because of flow control of cloud provider and performance issue, so we cached the instance and update periodically.

func (tc *TencentCloud) refreshPricingCache() error {
	now := time.Now()
	defer func() {
		klog.Infof("refreshPricingCache consumed: %v", time.Since(now))
	}()

	items, err := tc.cvm.GetAllZoneInstanceConfigInfos()
	if err != nil {
		klog.Errorf("UpdateCachedInstancesStandardPrice failed: %v", err)
		return err
	}
	func() {
		tc.lock.Lock()
		defer tc.lock.Unlock()
		for _, item := range items {
			zone := *item.Zone
			insType := *item.InstanceType
			insChargeType := *item.InstanceChargeType
			key := zone + "," + insType + "," + insChargeType
			tc.standardPricing[key] = item
		}
		klog.V(3).Infof("UpdateCachedInstancesStandardPrice success")
	}()
	return nil
}

func (pc *TencentCloud) refreshInstancePricingCache(nodeList []*v1.Node) error {
	now := time.Now()
	defer func() {
		klog.Infof("refreshInstancePricingCache consumed: %v", time.Since(now))
	}()

	// a cluster only can be in one region
	inputInstances := make([]*string, 0)
	nodeIds := make(map[string]bool)
	for _, n := range nodeList {
		id := ParseID(n.Spec.ProviderID)
		if id == "" {
			// eklet
			klog.V(3).Infof("refreshInstancePricingCache node %v ProviderID id is null, ignore it", n.Name)
			continue
		}
		nodeIds[id] = true
		inputInstances = append(inputInstances, &id)
		klog.V(4).Infof("refreshInstancePricingCache node instanceId: %v, node: %v", id, n.Name)
	}
	instances, err := pc.cvm.GetCVMInstances(inputInstances)
	if err != nil {
		return err
	}
	insPrices, err := pc.cvm.GetCVMInstancesPrice(instances)
	if err != nil {
		return err
	}

	pc.instanceLock.Lock()
	for _, insPrice := range insPrices {
		ins := insPrice.Instance
		price := insPrice.Price
		if ins != nil && ins.InstanceId != nil {
			pc.instances[*ins.InstanceId] = &sdkcvm.QCloudInstancePrice{
				Instance: ins,
				Price:    price,
			}
		}
	}
	for insid := range pc.instances {
		if _, ok := nodeIds[insid]; !ok {
			delete(pc.instances, insid)
		}
	}
	pc.instanceLock.Unlock()
	return nil
}

// providerID: qcloud:///800005/ins-2jv4wpmr
func ParseID(id string) string {
	// It's of the form qcloud:///800005/ins-2jv4wpmr and we want ins-2jv4wpmr, if it exists
	match := QCloudProviderIdRegex.FindStringSubmatch(id)
	if len(match) < 3 {
		if id != "" {
			klog.V(3).Infof("TencentCloud ParseID: failed to parse %s", id)
		}
		return id
	}

	return match[2]
}

func (tc *TencentCloud) WarmUp() error {
	nodes := tc.cache.GetNodes()
	klog.Info("refreshPricingCache")
	err := tc.refreshPricingCache()
	if err != nil {
		return err
	}
	klog.Info("refreshInstancePricingCache")
	err = tc.refreshInstancePricingCache(nodes)
	if err != nil {
		return err
	}
	return err
}

func (pc *TencentCloud) Refresh() {
	nodes := pc.cache.GetNodes()
	err := pc.refreshPricingCache()
	if err != nil {
		klog.Errorf("Failed to refresh: %v", err)
		return
	}
	err = pc.refreshInstancePricingCache(nodes)
	if err != nil {
		klog.Errorf("Failed to refresh: %v", err)
		return
	}
	return
}

func (pc *TencentCloud) GetInstancePrice(instanceid string) *sdkcvm.QCloudInstancePrice {
	pc.instanceLock.RLock()
	defer pc.instanceLock.RUnlock()
	return pc.instances[instanceid]
}

func (pc *TencentCloud) getInstanceById(instanceid string) *cvm.Instance {
	pc.instanceLock.Lock()
	defer pc.instanceLock.Unlock()

	ins, ok := pc.instances[instanceid]
	if !ok {
		return nil
	}
	return ins.Instance
}

func (pc *TencentCloud) OnNodeDelete(node *v1.Node) error {
	pc.instanceLock.Lock()
	defer pc.instanceLock.Unlock()

	if node != nil {
		id := ParseID(node.Spec.ProviderID)
		delete(pc.instances, id)
	}
	return nil
}

func (pc *TencentCloud) OnNodeAdd(node *v1.Node) error {
	if pc.IsVirtualNode(node) {
		klog.Warningf("node %v is virtual node", node.Name)
		return nil
	}
	if node != nil {
		id := ParseID(node.Spec.ProviderID)
		ids := []*string{&id}
		instances, err := pc.cvm.GetCVMInstances(ids)
		if err != nil {
			return err
		}
		prices, err := pc.cvm.GetCVMInstancesPrice(instances)
		if err != nil {
			return err
		}
		if len(prices) > 0 {
			func() {
				pc.instanceLock.Lock()
				defer pc.instanceLock.Unlock()
				pc.instances[id] = prices[0]
			}()
		}
	}
	return nil
}

func (pc *TencentCloud) OnNodeUpdate(old, new *v1.Node) error {
	// do nothing now.
	return nil
}
