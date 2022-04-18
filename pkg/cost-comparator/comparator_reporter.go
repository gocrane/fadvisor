package cost_comparator

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/olekukonko/tablewriter"
	v1 "k8s.io/api/core/v1"
	resourcehelper "k8s.io/kubernetes/pkg/api/v1/resource"

	"github.com/gocrane/fadvisor/pkg/consts"
	"github.com/gocrane/fadvisor/pkg/cost-comparator/config"
	"github.com/gocrane/fadvisor/pkg/cost-comparator/coster"
	"github.com/gocrane/fadvisor/pkg/util"
)

func (c *Comparator) ReportOriginalWorkloadsResourceDistribution(costerCtx *coster.CosterContext) {
	data := [][]string{}
	for kind, kindWorkloads := range costerCtx.WorkloadsSpec {
		for nn, workload := range kindWorkloads {

			cpuReqFloat64Cores := float64(workload.Cpu.MilliValue()) / 1000.
			memReqFloat64GB := float64(workload.Mem.Value()) / consts.GB
			cpuLimFloat64Cores := float64(workload.CpuLimit.MilliValue()) / 1000.
			memLimFloat64GB := float64(workload.MemLimit.Value()) / consts.GB

			labels := workload.Workload.GetLabels()
			labelsStr := ""
			if labels != nil {
				labelsBytes, _ := json.Marshal(labels)
				labelsStr = string(labelsBytes)
			}

			// non serverless, use original pod template resource requirements
			if !workload.Serverless {
				req, lim := resourcehelper.PodRequestsAndLimits(workload.PodRef)
				reqCpu := req[v1.ResourceCPU]
				reqMem := req[v1.ResourceMemory]
				limCpu := lim[v1.ResourceCPU]
				limMem := lim[v1.ResourceMemory]
				cpuReqFloat64Cores = float64(reqCpu.MilliValue()) / 1000.
				memReqFloat64GB = float64(reqMem.Value()) / consts.GB
				cpuLimFloat64Cores = float64(limCpu.MilliValue()) / 1000.
				memLimFloat64GB = float64(limMem.Value()) / consts.GB
			}

			data = append(data,
				[]string{kind, nn.Namespace, nn.Name, Float642Str(cpuReqFloat64Cores), Float642Str(memReqFloat64GB), Float642Str(cpuLimFloat64Cores), Float642Str(memLimFloat64GB), fmt.Sprintf("%v", workload.GoodsNum), fmt.Sprintf("%v", workload.Serverless), string(workload.QoSClass), labelsStr},
			)
		}
	}

	fmt.Println("Reporting, Original Workloads Resource Distribution.....................................................................................................")
	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeStdOut {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeaderLine(true)
		table.SetAutoFormatHeaders(false)
		table.SetHeader([]string{"Kind", "Namespace", "Name", "CpuReq", "MemReq", "CpuLim", "MemLim", "Replicas", "Serverless", "K8SQoS", "Labels"})
		table.SetBorder(false) // Set Border to false
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
		)

		table.SetColumnColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
		)
		table.AppendBulk(data) // Add Bulk Data
		table.Render()
	}

	filename := filepath.Join(c.config.DataPath, c.config.ClusterId+"-original-workloads-distribution"+".csv")
	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeCsv {
		csvFile, err := os.Create(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		csvW := csv.NewWriter(csvFile)
		csvW.Comma = '\t'
		err = csvW.Write([]string{"Kind", "Namespace", "Name", "CpuReq", "MemReq", "CpuLim", "MemLim", "Replicas", "Serverless", "K8SQoS", "Labels"})
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		err = csvW.WriteAll(data)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
	}
	fmt.Println()
}

func (c *Comparator) ReportRecommendedWorkloadsResourceDistribution(costerCtx *coster.CosterContext) {
	data := [][]string{}
	for kind, kindWorkloads := range costerCtx.WorkloadsRecSpec {
		for nn, workload := range kindWorkloads {
			cpuReqFloat64Cores := float64(workload.RecommendedSpec.Cpu.MilliValue()) / 1000.
			memReqFloat64GB := float64(workload.RecommendedSpec.Mem.Value()) / consts.GB
			cpuLimFloat64Cores := float64(workload.RecommendedSpec.CpuLimit.MilliValue()) / 1000.
			memLimFloat64GB := float64(workload.RecommendedSpec.MemLimit.Value()) / consts.GB
			containerStats, _ := json.Marshal(workload.Containers)
			data = append(data,
				[]string{kind, nn.Namespace, nn.Name, Float642Str(cpuReqFloat64Cores), Float642Str(memReqFloat64GB), Float642Str(cpuLimFloat64Cores), Float642Str(memLimFloat64GB), fmt.Sprintf("%v", workload.RecommendedSpec.GoodsNum), fmt.Sprintf("%v", workload.RecommendedSpec.Serverless), string(workload.RecommendedSpec.QoSClass), string(containerStats)},
			)
		}
	}

	fmt.Println("Reporting, Recommended Workloads Resource Distribution.....................................................................................................")

	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeStdOut {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeaderLine(true)
		table.SetAutoFormatHeaders(false)
		table.SetHeader([]string{"Kind", "Namespace", "Name", "CpuReq", "MemReq", "CpuLim", "MemLim", "Replicas", "Serverless", "K8SQoS", "ContainerStats"})
		table.SetBorder(false) // Set Border to false
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
		)

		table.SetColumnColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
		)

		table.AppendBulk(data) // Add Bulk Data
		table.Render()
	}

	filename := filepath.Join(c.config.DataPath, c.config.ClusterId+"-recommended-workloads-distribution"+".csv")
	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeCsv {
		csvFile, err := os.Create(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		csvW := csv.NewWriter(csvFile)
		csvW.Comma = '\t'
		err = csvW.Write([]string{"Kind", "Namespace", "Name", "CpuReq", "MemReq", "CpuLim", "MemLim", "Replicas", "Serverless", "K8SQoS", "ContainerStats"})
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		err = csvW.WriteAll(data)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
	}

	fmt.Println()
}

func (c *Comparator) ReportOriginalResourceSummary() {

	pods := c.clusterCache.GetPods()
	clusterRequestsTotal, clusterLimitsTotal := util.PodsRequestsAndLimitsTotal(pods, func(pod *v1.Pod) bool {
		return false
	}, false)

	serverfulRequestsTotal, serverfulLimitsTotal := util.PodsRequestsAndLimitsTotal(pods, c.baselineCloud.IsServerlessPod, false)
	serverlessRequestsTotal, serverlessLimitsTotal := util.PodsRequestsAndLimitsTotal(pods, c.baselineCloud.IsServerlessPod, true)

	nodes := c.clusterCache.GetNodes()
	clusterRealNodesCapacityTotal := util.NodesResourceTotal(nodes, c.baselineCloud.IsVirtualNode, false)
	clusterVirtualNodesCapacityTotal := util.NodesResourceTotal(nodes, c.baselineCloud.IsVirtualNode, true)

	data := [][]string{
		{"clusterRequestsTotal", Float642Str(float64(clusterRequestsTotal.Cpu().MilliValue()) / 1000.), Float642Str(float64(clusterRequestsTotal.Memory().Value()) / consts.GB)},
		{"clusterLimitsTotal", Float642Str(float64(clusterLimitsTotal.Cpu().MilliValue()) / 1000.), Float642Str(float64(clusterLimitsTotal.Memory().Value()) / consts.GB)},
		{"serverfulRequestsTotal", Float642Str(float64(serverfulRequestsTotal.Cpu().MilliValue()) / 1000.), Float642Str(float64(serverfulRequestsTotal.Memory().Value()) / consts.GB)},
		{"serverfulLimitsTotal", Float642Str(float64(serverfulLimitsTotal.Cpu().MilliValue()) / 1000.), Float642Str(float64(serverfulLimitsTotal.Memory().Value()) / consts.GB)},
		{"serverlessRequestsTotal", Float642Str(float64(serverlessRequestsTotal.Cpu().MilliValue()) / 1000.), Float642Str(float64(serverlessRequestsTotal.Memory().Value()) / consts.GB)},
		{"serverlessLimitsTotal", Float642Str(float64(serverlessLimitsTotal.Cpu().MilliValue()) / 1000.), Float642Str(float64(serverlessLimitsTotal.Memory().Value()) / consts.GB)},
		{"clusterRealNodesCapacityTotal", Float642Str(float64(clusterRealNodesCapacityTotal.Cpu().MilliValue()) / 1000.), Float642Str(float64(clusterRealNodesCapacityTotal.Memory().Value()) / consts.GB)},
		{"clusterVirtualNodesCapacityTotal", clusterVirtualNodesCapacityTotal.Cpu().String(), clusterVirtualNodesCapacityTotal.Memory().String()},
	}

	fmt.Println("Reporting, Original Resource Summary.....................................................................................................")

	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeStdOut {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeaderLine(true)
		table.SetAutoFormatHeaders(false)
		table.SetHeader([]string{"Type", "Cpu", "Mem"})
		table.SetBorder(false) // Set Border to false
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor})

		table.SetColumnColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor})

		table.AppendBulk(data) // Add Bulk Data
		table.Render()
	}

	filename := filepath.Join(c.config.DataPath, c.config.ClusterId+"-original-resource-summary"+".csv")
	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeCsv {
		csvFile, err := os.Create(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		csvW := csv.NewWriter(csvFile)
		csvW.Comma = '\t'
		err = csvW.Write([]string{"Type", "Cpu", "Mem"})
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		err = csvW.WriteAll(data)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
	}

	fmt.Println()
}

func Float642Str(a float64) string {
	return fmt.Sprintf("%.5f", a)
}

func (c *Comparator) ReportOriginalCostSummary(costerCtx *coster.CosterContext) {
	serverfulCoster := coster.NewServerfulCoster()
	originalFee := serverfulCoster.TotalCost(costerCtx)

	data := [][]string{
		{"tke", Float642Str(originalFee.TotalCost), Float642Str(originalFee.ServerfulCost), Float642Str(originalFee.ServerlessCost), Float642Str(originalFee.ServerfulPlatformCost), Float642Str(originalFee.ServerlessPlatformCost)},
	}

	fmt.Printf("Reporting, Original Cost Summary(TimeSpan: %v, Discount: %v)............................................................................\n", c.config.TimeSpanSeconds, c.config.Discount)

	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeStdOut {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeaderLine(true)
		table.SetAutoFormatHeaders(false)
		table.SetHeader([]string{"Type", "TotalCost", "ServerfulCost", "ServerlessCost", "ServerfulPlatformCost", "ServerlessPlatformCost"})
		table.SetBorder(false) // Set Border to false
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
		)

		table.SetColumnColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
		)

		table.AppendBulk(data) // Add Bulk Data
		table.Render()
	}

	filename := filepath.Join(c.config.DataPath, c.config.ClusterId+"-original-cost-summary"+".csv")
	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeCsv {
		csvFile, err := os.Create(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		csvW := csv.NewWriter(csvFile)
		csvW.Comma = '\t'
		err = csvW.Write([]string{"Type", "TotalCost", "ServerfulCost", "ServerlessCost", "ServerfulPlatformCost", "ServerlessPlatformCost"})
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		err = csvW.WriteAll(data)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
	}

	fmt.Println()
}

func (c *Comparator) ReportRawServerlessCostSummary(costerCtx *coster.CosterContext) {
	serverlessCoster := coster.NewServerlessCoster()
	serverlessFee := serverlessCoster.TotalCost(costerCtx)

	data := [][]string{
		{"eks", Float642Str(serverlessFee.TotalCost), Float642Str(serverlessFee.ServerfulCost), Float642Str(serverlessFee.ServerlessCost), Float642Str(serverlessFee.ServerfulPlatformCost), Float642Str(serverlessFee.ServerlessPlatformCost)},
	}

	fmt.Printf("Reporting, Direct Migrating to Serverless Cost Summary(TimeSpan: %v, Discount: %v)............................................................................\n", c.config.TimeSpanSeconds, c.config.Discount)

	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeStdOut {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeaderLine(true)
		table.SetAutoFormatHeaders(false)
		table.SetHeader([]string{"Type", "TotalCost", "ServerfulCost", "ServerlessCost", "ServerfulPlatformCost", "ServerlessPlatformCost"})
		table.SetBorder(false) // Set Border to false
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
		)

		table.SetColumnColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
		)

		table.AppendBulk(data) // Add Bulk Data
		table.Render()
	}

	filename := filepath.Join(c.config.DataPath, c.config.ClusterId+"-direct-migrate-serverless-cost-summary"+".csv")
	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeCsv {
		csvFile, err := os.Create(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		csvW := csv.NewWriter(csvFile)
		csvW.Comma = '\t'
		err = csvW.Write([]string{"Type", "TotalCost", "ServerfulCost", "ServerlessCost", "ServerfulPlatformCost", "ServerlessPlatformCost"})
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		err = csvW.WriteAll(data)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
	}

	fmt.Println()
}

func (c *Comparator) ReportRecommendedResourceSummary(costerCtx *coster.CosterContext) {
	recomendedResourceTotal := ServerlessWorkloadsResourceTotal(costerCtx.WorkloadsRecSpec)

	data := [][]string{
		{"recomendedServerlessResourceTotal", Float642Str(float64(recomendedResourceTotal.Cpu().MilliValue()) / 1000.), Float642Str(float64(recomendedResourceTotal.Memory().Value()) / consts.GB)},
	}

	fmt.Println("Reporting, Recommended Resource Summary After Migrating to Serverless.....................................................................")

	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeStdOut {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeaderLine(true)
		table.SetAutoFormatHeaders(false)
		table.SetHeader([]string{"Type", "Cpu", "Mem"})
		table.SetBorder(false)
		// Set Border to false
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor})

		table.SetColumnColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor})

		table.AppendBulk(data) // Add Bulk Data
		table.Render()
	}

	filename := filepath.Join(c.config.DataPath, c.config.ClusterId+"-recommended-serverless-resource-summary"+".csv")
	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeCsv {
		csvFile, err := os.Create(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		csvW := csv.NewWriter(csvFile)
		csvW.Comma = '\t'
		err = csvW.Write([]string{"Type", "Cpu", "Mem"})
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		err = csvW.WriteAll(data)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
	}
	fmt.Println()
}

func (c *Comparator) ReportRecommendedCostSummary(costerCtx *coster.CosterContext) {
	recommendedCoster := coster.NewRecommenderCoster()
	RecommendedCost, PercentileCost, MaxCost, MaxMarginCost := recommendedCoster.TotalCost(costerCtx)

	data := [][]string{
		{"eks-recommended-by-percentile-margin", Float642Str(RecommendedCost.TotalCost), Float642Str(RecommendedCost.WorkloadCost), Float642Str(RecommendedCost.PlatformCost)},
		{"eks-recommended-by-percentile", Float642Str(PercentileCost.TotalCost), Float642Str(PercentileCost.WorkloadCost), Float642Str(PercentileCost.PlatformCost)},
		{"eks-recommended-by-max-margin", Float642Str(MaxMarginCost.TotalCost), Float642Str(MaxMarginCost.WorkloadCost), Float642Str(MaxMarginCost.PlatformCost)},
		{"eks-recommended-by-max", Float642Str(MaxCost.TotalCost), Float642Str(MaxCost.WorkloadCost), Float642Str(MaxCost.PlatformCost)},
	}

	fmt.Printf("Reporting, Recommended Cost Summary After Migrating to Serverless(TimeSpan: %v, Discount: %v).............................................\n", c.config.TimeSpanSeconds, c.config.Discount)

	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeStdOut {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeaderLine(true)
		table.SetAutoFormatHeaders(false)
		table.SetHeader([]string{"Type", "TotalCost", "WorkloadCost", "PlatformCost"})
		table.SetBorder(false) // Set Border to false
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
			tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
		)

		table.SetColumnColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
		)

		table.AppendBulk(data) // Add Bulk Data
		table.Render()
	}

	filename := filepath.Join(c.config.DataPath, c.config.ClusterId+"-recommended-cost-summary"+".csv")
	if c.config.OutputMode == "" || c.config.OutputMode == config.OutputModeCsv {
		csvFile, err := os.Create(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		csvW := csv.NewWriter(csvFile)
		csvW.Comma = '\t'
		err = csvW.Write([]string{"Type", "TotalCost", "WorkloadCost", "PlatformCost"})
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
		err = csvW.WriteAll(data)
		if err != nil {
			fmt.Println(err)
			os.Exit(255)
		}
	}

	fmt.Println()
}
