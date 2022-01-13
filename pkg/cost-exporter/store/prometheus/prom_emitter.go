package prometheus

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/gocrane/fadvisor/pkg/cost-exporter/cloudcost"
)

var metricsInit sync.Once

var (
	nodeCpuCostGv   *prometheus.GaugeVec
	nodeRamCostGv   *prometheus.GaugeVec
	nodeTotalCostGv *prometheus.GaugeVec

	//containerRamAllocGv *prometheus.GaugeVec
	//containerCpuAllocGv *prometheus.GaugeVec
)

func init() {
	metricsInit.Do(func() {
		nodeCpuCostGv = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "node_cpu_hourly_cost",
			Help: "node_cpu_hourly_cost hourly cost for each cpu on the node",
		}, []string{"instance", "node", "instance_type", "region", "provider_id"})

		nodeRamCostGv = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "node_ram_hourly_cost",
			Help: "node_ram_hourly_cost hourly cost for each GB of ram on the node",
		}, []string{"instance", "node", "instance_type", "region", "provider_id"})

		nodeTotalCostGv = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "node_total_hourly_cost",
			Help: "node_total_hourly_cost total node cost per hour",
		}, []string{"instance", "node", "instance_type", "region", "provider_id"})

		//containerCpuAllocGv = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		//	Name: "container_cpu_allocation",
		//	Help: "container_cpu_allocation of container CPU used in a minute",
		//}, []string{"namespace", "pod", "container", "instance", "node"})
		//
		//containerRamAllocGv = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		//	Name: "container_memory_allocation_bytes",
		//	Help: "container_memory_allocation_bytes Bytes of container RAM used",
		//}, []string{"namespace", "pod", "container", "instance", "node"})

		prometheus.MustRegister(nodeCpuCostGv, nodeRamCostGv, nodeTotalCostGv)
		//prometheus.MustRegister(containerCpuAllocGv, containerRamAllocGv)

	})
}

// CostMetricEmitter export cost metric
type CostMetricEmitter struct {
	costModel cloudcost.CostModel

	nodeCpuCostGv   *prometheus.GaugeVec
	nodeRamCostGv   *prometheus.GaugeVec
	nodeTotalCostGv *prometheus.GaugeVec

	//containerRamAllocGv *prometheus.GaugeVec
	//containerCpuAllocGv *prometheus.GaugeVec

	updateInterval time.Duration
	stopCh         <-chan struct{}
}

func NewCostMetricEmitter(costModel cloudcost.CostModel, updateInterval time.Duration, stopCh <-chan struct{}) *CostMetricEmitter {
	return &CostMetricEmitter{
		costModel:       costModel,
		updateInterval:  updateInterval,
		stopCh:          stopCh,
		nodeCpuCostGv:   nodeCpuCostGv,
		nodeRamCostGv:   nodeRamCostGv,
		nodeTotalCostGv: nodeTotalCostGv,
	}
}

func (cme *CostMetricEmitter) Start() {
	ticker := time.NewTicker(cme.updateInterval)
	defer ticker.Stop()

	nodesLastSeen := make(map[string]bool)
	getKeyFromLabelStrings := func(labels ...string) string {
		return strings.Join(labels, ",")
	}
	getLabelStringsFromKey := func(key string) []string {
		return strings.Split(key, ",")
	}

	for {
		cfg, err := cme.costModel.GetConfig()
		if err != nil {
			klog.Errorf("Failed to get provider config: %v", err)
			continue
		}
		nodes, err := cme.costModel.GetNodesCost()
		if err != nil {
			klog.Errorf("Failed to get provider config: %v", err)
			continue
		}
		klog.V(3).Info("Setting node metrics")
		for nodeName, node := range nodes {
			cpuCost, _ := strconv.ParseFloat(node.CpuHourlyCost, 64)
			if math.IsNaN(cpuCost) || math.IsInf(cpuCost, 0) {
				cpuCost = cfg.CpuHourlyPrice
			}
			cpu, _ := strconv.ParseFloat(node.Cpu, 64)
			if math.IsNaN(cpu) || math.IsInf(cpu, 0) {
				cpu = 1
			}
			ramCost, _ := strconv.ParseFloat(node.RamGBHourlyCost, 64)
			if math.IsNaN(ramCost) || math.IsInf(ramCost, 0) {
				ramCost = cfg.RamGBHourlyPrice
				if math.IsNaN(ramCost) || math.IsInf(ramCost, 0) {
					ramCost = 0
				}
			}
			ram, _ := strconv.ParseFloat(node.Ram, 64)
			if math.IsNaN(ram) || math.IsInf(ram, 0) {
				ram = 0
			}

			nodeType := node.InstanceType
			nodeRegion := node.Region

			totalCost := cpu*cpuCost + ramCost*ram

			cme.nodeCpuCostGv.WithLabelValues(nodeName, nodeName, nodeType, nodeRegion, node.ProviderID).Set(cpuCost)
			cme.nodeRamCostGv.WithLabelValues(nodeName, nodeName, nodeType, nodeRegion, node.ProviderID).Set(ramCost)
			cme.nodeTotalCostGv.WithLabelValues(nodeName, nodeName, nodeType, nodeRegion, node.ProviderID).Set(totalCost)

			labelKey := getKeyFromLabelStrings(nodeName, nodeName, nodeType, nodeRegion, node.ProviderID)
			nodesLastSeen[labelKey] = true
		}

		for labelString, seen := range nodesLastSeen {
			if !seen {
				klog.V(3).Infof("Removing from nodes, labelString: %v", labelString)
				labels := getLabelStringsFromKey(labelString)
				ok := cme.nodeTotalCostGv.DeleteLabelValues(labels...)
				if !ok {
					klog.Errorf("Failed to remove totalcost, labelString: %v", labelString)
				}
				ok = cme.nodeCpuCostGv.DeleteLabelValues(labels...)
				if !ok {
					klog.Errorf("Failed to remove cpucost, labelString: %v", labelString)
				}
				ok = cme.nodeRamCostGv.DeleteLabelValues(labels...)
				if !ok {
					klog.Errorf("Failed to remove ramcost, labelString: %v", labelString)
				}
				delete(nodesLastSeen, labelString)
			} else {
				// reset to false to be used in next loop, if node still exists, it will be set to true
				nodesLastSeen[labelString] = false
			}
		}

		select {
		case <-cme.stopCh:
			klog.Infoln("Emitter stop...")
			return
		case <-ticker.C:
		}
	}

}
