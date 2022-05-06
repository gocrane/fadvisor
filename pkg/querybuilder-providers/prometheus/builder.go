package prometheus

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/gocrane/fadvisor/pkg/consts"
	"github.com/gocrane/fadvisor/pkg/metricquery"
	"github.com/gocrane/fadvisor/pkg/querybuilder"
)

// todo: later we change these templates to configurable like prometheus-adapter
const (
	// WorkloadCpuUsageExprTemplate is used to query workload cpu usage by promql,  param is namespace,workload-name,common condition,duration str
	WorkloadCpuUsageExprTemplate = `sum(irate(container_cpu_usage_seconds_total{container!="",image!="",container!="POD",namespace="%s",pod=~"^%s-.*$",%s}[%s]))`
	// WorkloadMemUsageExprTemplate is used to query workload mem usage by promql, param is namespace, workload-name,common condition
	WorkloadMemUsageExprTemplate = `sum(container_memory_working_set_bytes{container!="",image!="",container!="POD",namespace="%s",pod=~"^%s-.*$",%s})`

	// following is node exporter metric for node cpu/memory usage
	// NodeCpuUsageExprTemplate is used to query node cpu usage by promql,  param is node name,common condition,node name, common condition, which prometheus scrape, duration str
	NodeCpuUsageExprTemplate = `sum(count(node_cpu_seconds_total{mode="idle",instance=~"(%s)(:\\d+)?",%s}) by (mode, cpu)) - sum(irate(node_cpu_seconds_total{mode="idle",instance=~"(%s)(:\\d+)?",%s}[%s]))`
	// NodeMemUsageExprTemplate is used to query node cpu memory by promql,  param is node name, node name which prometheus scrape
	NodeMemUsageExprTemplate = `sum(node_memory_MemTotal_bytes{instance=~"(%s)(:\\d+)?",%s} - node_memory_MemAvailable_bytes{instance=~"(%s)(:\\d+)?",%s})`

	// PodCpuUsageExprTemplate is used to query pod cpu usage by promql,  param is namespace,pod, common condition, duration str
	PodCpuUsageExprTemplate = `sum(irate(container_cpu_usage_seconds_total{container!="POD",namespace="%s",pod="%s",%s}[%s]))`
	// PodMemUsageExprTemplate is used to query pod cpu usage by promql,  param is namespace,pod
	PodMemUsageExprTemplate = `sum(container_memory_working_set_bytes{container!="POD",namespace="%s",pod="%s",%s})`

	// ContainerCpuUsageExprTemplate is used to query container cpu usage by promql,  param is namespace,pod,container, common condition, duration str
	ContainerCpuUsageExprTemplate = `irate(container_cpu_usage_seconds_total{container!="POD",namespace="%s",pod=~"^%s.*$",container="%s",%s}[%s])`
	// ContainerMemUsageExprTemplate is used to query container cpu usage by promql,  param is namespace,pod,container
	ContainerMemUsageExprTemplate = `container_memory_working_set_bytes{container!="POD",namespace="%s",pod=~"^%s.*$",container="%s",%s}`

	/**
	NOTE: following metrics require kube-state-metrics on your monitoring system
	*/
	// container, resource, workloadname,namespace,common condition, workloadkind,workloadname,namespace,common condition
	ContainerResourceRequestExprTemplate = `avg(
    kube_pod_container_resource_requests{container!="", container!="POD", container="%s", resource="%s",pod=~"%s-.*",namespace="%s",%s} 
    * on (pod, namespace) group_left(owner_kind, owner_name) 
    max(kube_pod_owner{owner_is_controller="true",owner_kind=~"%s",owner_name=~"%s",namespace="%s",%s}) without (instance)
) by (namespace, owner_kind,owner_name,container)`

	// container, resource, workloadname,namespace,common condition, workloadkind,workloadname,namespace, common condition
	ContainerResourceLimitExprTemplate = `avg(
    kube_pod_container_resource_limits{container!="", container!="POD", container="%s", resource="%s",pod=~"%s-.*",namespace="%s",%s} 
    * on (pod, namespace) group_left(owner_kind, owner_name) 
    max(kube_pod_owner{owner_is_controller="true",owner_kind=~"%s",owner_name=~"%s",namespace="%s",%s}) without (instance)
) by (namespace, owner_kind,owner_name,container)`

	// container, resource, deploymentname,namespace,common condition, deploymentname,namespace,common condition
	DeploymentContainerResourceRequestExprTemplate = `avg(
    kube_pod_container_resource_requests{container!="", container!="POD", container="%s",resource="%s",pod=~"%s-.*",namespace="%s",%s} 
    * on (pod, namespace) group_left(owner_kind, owner_name) 
    max(label_replace(kube_pod_owner{owner_is_controller="true",owner_kind="ReplicaSet",owner_name=~"%s-.*",namespace="%s",%s}, "owner_name", "$1", "owner_name", "(.*)-.(.*)")) without (instance)
) by (namespace, owner_kind,owner_name,container)`

	// container, resource, deploymentname,namespace,common condition, deploymentname,namespace,common condition
	DeploymentContainerResourceLimitExprTemplate = `avg(
    kube_pod_container_resource_limits{container!="", container!="POD", container="%s",resource="%s",pod=~"%s-.*",namespace="%s",%s} 
    * on (pod, namespace) group_left(owner_kind, owner_name) 
    max(label_replace(kube_pod_owner{owner_is_controller="true",owner_kind="ReplicaSet",owner_name=~"%s-.*",namespace="%s",%s}, "owner_name", "$1", "owner_name", "(.*)-.(.*)")) without (instance)
) by (namespace, owner_kind,owner_name,container)`

	// resource,workloadname,namespace,common condition, workloadname,namespace, common condition
	DeploymentResourceRequestExprTemplate = `max(
    sum(
        kube_pod_container_resource_requests{container!="", container!="POD", resource="%s",pod=~"%s-.*",namespace="%s",%s} 
        * on (pod, namespace) group_left(owner_kind, owner_name) 
        max(label_replace(kube_pod_owner{owner_is_controller="true",owner_kind="ReplicaSet",owner_name=~"%s-.*",namespace="%s",%s}, "owner_name", "$1", "owner_name", "(.*)-.(.*)")) without (instance)
    ) by (namespace, owner_kind,owner_name)
) without (instance)`

	// resource,workloadname,namespace,common condition,workloadname,namespace, common condition
	DeploymentResourceLimitExprTemplate = `max(
    sum(
        kube_pod_container_resource_limits{container!="", container!="POD", resource="%s",pod=~"%s-.*",namespace="%s",%s} 
        * on (pod, namespace) group_left(owner_kind, owner_name) 
        max(label_replace(kube_pod_owner{owner_is_controller="true",owner_kind="ReplicaSet",owner_name=~"%s-.*",namespace="%s",%s}, "owner_name", "$1", "owner_name", "(.*)-.(.*)")) without (instance)
    ) by (namespace, owner_kind,owner_name)
) without (instance)`

	// WorkloadResourceRequestExprTemplate used to query workload request,
	// param is resource,workloadname,namespace,common condition, workloadkind,workloadname,namespace, common condition
	WorkloadResourceRequestExprTemplate = `max(
    sum(
        kube_pod_container_resource_requests{container!="", container!="POD", resource="%s",pod=~"%s-.*",namespace="%s",%s} 
        * on (pod, namespace) group_left(owner_kind, owner_name) 
        max(kube_pod_owner{owner_is_controller="true",owner_kind=~"%s",owner_name=~"%s",namespace="%s",%s}) without (instance)
    ) by (namespace, owner_kind,owner_name)
) without (instance)`

	// WorkloadResourceLimitExprTemplate used to query workload limit, param is resource,workloadname,namespace,common condition,workloadkind,workloadname,namespace,common condition
	WorkloadResourceLimitExprTemplate = `max(
    sum(
        kube_pod_container_resource_limits{container!="", container!="POD", resource="%s",pod=~"%s-.*",namespace="%s",%s} 
        * on (pod, namespace) group_left(owner_kind, owner_name) 
        max(kube_pod_owner{owner_is_controller="true",owner_kind=~"%s",owner_name=~"%s",namespace="%s",%s}) without (instance)
    ) by (namespace, owner_kind,owner_name)
) without (instance)`

	// StatefulSetPlusReplicasExprTemplate replicas, param is namespace, name, common condition
	StatefulSetPlusReplicasExprTemplate = `label_replace(label_replace(max(kube_statefulsetplus_status_replicas_ready{namespace="%s",statefulset="%s",%s}) without (instance, job), "owner_name", "$1", "statefulset", "(.*)"), "owner_kind", "StatefulSetPlus", "", "")`

	// StatefulSetReplicasExprTemplate replicas, param is namespace, name,common condition
	StatefulSetReplicasExprTemplate = `label_replace(label_replace(max(kube_statefulset_status_replicas_ready{namespace="%s",statefulset="%s",%s}) without (instance, job), "owner_name", "$1", "statefulset", "(.*)"), "owner_kind", "StatefulSet", "", "")`

	// DeploymentReplicasExprTemplate replicas, param is namespace, name,common condition
	DeploymentReplicasExprTemplate = `label_replace(label_replace(max(kube_deployment_status_replicas_ready{namespace="%s",deployment="%s",%s}) without (instance, job), "owner_name", "$1", "deployment", "(.*)"), "owner_kind", "Deployment", "", "")`

	// DaemonSetReplicasExprTemplate replicas, param is namespace, name,common condition
	DaemonSetReplicasExprTemplate = `label_replace(label_replace(max(kube_daemonset_status_number_ready{namespace="%s",daemonset="%s",%s}) without (instance, job),  "owner_name", "$1", "daemonset", "(.*)"),"owner_kind", "DaemonSet", "", "")`
)

var supportedResources = sets.NewString(v1.ResourceCPU.String(), v1.ResourceMemory.String())

var _ querybuilder.Builder = &builder{}

type builder struct {
	metric *metricquery.Metric
}

func NewPromQueryBuilder(metric *metricquery.Metric) querybuilder.Builder {
	return &builder{
		metric: metric,
	}
}

func (b *builder) BuildQuery(behavior querybuilder.BuildQueryBehavior) (*metricquery.Query, error) {
	switch b.metric.Type {
	case metricquery.WorkloadMetricType:
		return b.workloadQuery(b.metric, behavior)
	case metricquery.PodMetricType:
		return b.podQuery(b.metric, behavior)
	case metricquery.ContainerMetricType:
		return b.containerQuery(b.metric, behavior)
	case metricquery.NodeMetricType:
		return b.nodeQuery(b.metric, behavior)
	case metricquery.PromQLMetricType:
		return b.promQuery(b.metric, behavior)
	default:
		return nil, fmt.Errorf("metric type %v not supported", b.metric.Type)
	}
}

func (b *builder) workloadQuery(metric *metricquery.Metric, behavior querybuilder.BuildQueryBehavior) (*metricquery.Query, error) {
	if metric.Workload == nil {
		return nil, fmt.Errorf("metric type %v, but no WorkloadNamerInfo provided", metric.Type)
	}
	selector := metric.Workload.Selector
	clusterCond := ""
	if selector != nil && behavior.FederatedClusterScope {
		if id, exists := selector.RequiresExactMatch(consts.LabelClusterId); exists {
			clusterCond = fmt.Sprintf(`cluster="%v"`, id)
		}
	}
	switch strings.ToLower(metric.MetricName) {
	case v1.ResourceCPU.String():
		return promQuery(&metricquery.PrometheusQuery{
			Query: fmt.Sprintf(WorkloadCpuUsageExprTemplate, metric.Workload.Namespace, metric.Workload.Name, clusterCond, "3m"),
		}), nil
	case v1.ResourceMemory.String():
		return promQuery(&metricquery.PrometheusQuery{
			Query: fmt.Sprintf(WorkloadMemUsageExprTemplate, metric.Workload.Namespace, metric.Workload.Name, clusterCond),
		}), nil
	case consts.MetricCpuRequest:
		if strings.ToLower(metric.Workload.Kind) == "deployment" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DeploymentResourceRequestExprTemplate, v1.ResourceCPU.String(), metric.Workload.Name, metric.Workload.Namespace, clusterCond, metric.Workload.Name, metric.Workload.Namespace, clusterCond),
			}), nil
		} else {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(WorkloadResourceRequestExprTemplate, v1.ResourceCPU.String(), metric.Workload.Name, metric.Workload.Namespace, clusterCond, metric.Workload.Kind, metric.Workload.Name, metric.Workload.Namespace, clusterCond),
			}), nil
		}
	case consts.MetricCpuLimit:
		if strings.ToLower(metric.Workload.Kind) == "deployment" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DeploymentResourceLimitExprTemplate, v1.ResourceCPU.String(), metric.Workload.Name, metric.Workload.Namespace, clusterCond, metric.Workload.Name, metric.Workload.Namespace, clusterCond),
			}), nil
		} else {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(WorkloadResourceLimitExprTemplate, v1.ResourceCPU.String(), metric.Workload.Name, metric.Workload.Namespace, clusterCond, metric.Workload.Kind, metric.Workload.Name, metric.Workload.Namespace, clusterCond),
			}), nil
		}
	case consts.MetricMemRequest:
		if strings.ToLower(metric.Workload.Kind) == "deployment" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DeploymentResourceRequestExprTemplate, v1.ResourceMemory.String(), metric.Workload.Name, metric.Workload.Namespace, clusterCond, metric.Workload.Name, metric.Workload.Namespace, clusterCond),
			}), nil
		} else {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(WorkloadResourceRequestExprTemplate, v1.ResourceMemory.String(), metric.Workload.Name, metric.Workload.Namespace, clusterCond, metric.Workload.Kind, metric.Workload.Name, metric.Workload.Namespace, clusterCond),
			}), nil
		}
	case consts.MetricMemLimit:
		if strings.ToLower(metric.Workload.Kind) == "deployment" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DeploymentResourceLimitExprTemplate, v1.ResourceMemory.String(), metric.Workload.Name, metric.Workload.Namespace, clusterCond, metric.Workload.Name, metric.Workload.Namespace, clusterCond),
			}), nil
		} else {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(WorkloadResourceLimitExprTemplate, v1.ResourceMemory.String(), metric.Workload.Name, metric.Workload.Namespace, clusterCond, metric.Workload.Kind, metric.Workload.Name, metric.Workload.Namespace, clusterCond),
			}), nil
		}
	case consts.MetricWorkloadReplicas:
		workloadkind := strings.ToLower(metric.Workload.Kind)
		if workloadkind == "deployment" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DeploymentReplicasExprTemplate, metric.Workload.Namespace, metric.Workload.Name, clusterCond),
			}), nil
		} else if workloadkind == "statefulset" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(StatefulSetReplicasExprTemplate, metric.Workload.Namespace, metric.Workload.Name, clusterCond),
			}), nil
		} else if workloadkind == "statefulsetplus" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(StatefulSetPlusReplicasExprTemplate, metric.Workload.Namespace, metric.Workload.Name, clusterCond),
			}), nil
		} else if workloadkind == "daemonset" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DaemonSetReplicasExprTemplate, metric.Workload.Namespace, metric.Workload.Name, clusterCond),
			}), nil
		} else {
			return nil, fmt.Errorf("metric type %v do not support workload kind %v", metric.Type, metric.Workload.Kind)
		}
	default:
		return nil, fmt.Errorf("metric type %v do not support resource metric %v. only support %v now", metric.Type, metric.MetricName, supportedResources.List())
	}
}

func (b *builder) containerQuery(metric *metricquery.Metric, behavior querybuilder.BuildQueryBehavior) (*metricquery.Query, error) {
	if metric.Container == nil {
		return nil, fmt.Errorf("metric type %v, but no ContainerNamerInfo provided", metric.Type)
	}
	selector := metric.Container.Selector
	clusterCond := ""
	if selector != nil && behavior.FederatedClusterScope {
		if id, exists := selector.RequiresExactMatch(consts.LabelClusterId); exists {
			clusterCond = fmt.Sprintf(`cluster="%v"`, id)
		}
	}
	switch strings.ToLower(metric.MetricName) {
	case v1.ResourceCPU.String():
		return promQuery(&metricquery.PrometheusQuery{
			Query: fmt.Sprintf(ContainerCpuUsageExprTemplate, metric.Container.Namespace, metric.Container.WorkloadName, metric.Container.ContainerName, clusterCond, "3m"),
		}), nil
	case v1.ResourceMemory.String():
		return promQuery(&metricquery.PrometheusQuery{
			Query: fmt.Sprintf(ContainerMemUsageExprTemplate, metric.Container.Namespace, metric.Container.WorkloadName, metric.Container.ContainerName, clusterCond),
		}), nil
	case consts.MetricCpuRequest:
		if strings.ToLower(metric.Container.Kind) == "deployment" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DeploymentContainerResourceRequestExprTemplate, metric.Container.ContainerName, v1.ResourceCPU.String(), metric.Container.WorkloadName, metric.Container.Namespace, clusterCond, metric.Container.WorkloadName, metric.Container.Namespace, clusterCond),
			}), nil
		} else {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(ContainerResourceRequestExprTemplate, metric.Container.ContainerName, v1.ResourceCPU.String(), metric.Container.WorkloadName, metric.Container.Namespace, clusterCond, metric.Container.Kind, metric.Container.WorkloadName, metric.Container.Namespace, clusterCond),
			}), nil
		}
	case consts.MetricCpuLimit:
		if strings.ToLower(metric.Container.Kind) == "deployment" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DeploymentContainerResourceLimitExprTemplate, metric.Container.ContainerName, v1.ResourceCPU.String(), metric.Container.WorkloadName, metric.Container.Namespace, clusterCond, metric.Container.WorkloadName, metric.Container.Namespace, clusterCond),
			}), nil
		} else {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(ContainerResourceLimitExprTemplate, metric.Container.ContainerName, v1.ResourceCPU.String(), metric.Container.WorkloadName, metric.Container.Namespace, clusterCond, metric.Container.Kind, metric.Container.WorkloadName, metric.Container.Namespace, clusterCond),
			}), nil
		}
	case consts.MetricMemRequest:
		if strings.ToLower(metric.Container.Kind) == "deployment" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DeploymentContainerResourceRequestExprTemplate, metric.Container.ContainerName, v1.ResourceMemory.String(), metric.Container.WorkloadName, metric.Container.Namespace, clusterCond, metric.Container.WorkloadName, metric.Container.Namespace, clusterCond),
			}), nil
		} else {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(ContainerResourceRequestExprTemplate, metric.Container.ContainerName, v1.ResourceMemory.String(), metric.Container.WorkloadName, metric.Container.Namespace, clusterCond, metric.Container.Kind, metric.Container.WorkloadName, metric.Container.Namespace, clusterCond),
			}), nil
		}
	case consts.MetricMemLimit:
		if strings.ToLower(metric.Container.Kind) == "deployment" {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(DeploymentContainerResourceLimitExprTemplate, metric.Container.ContainerName, v1.ResourceMemory.String(), metric.Container.WorkloadName, metric.Container.Namespace, clusterCond, metric.Container.WorkloadName, metric.Container.Namespace, clusterCond),
			}), nil
		} else {
			return promQuery(&metricquery.PrometheusQuery{
				Query: fmt.Sprintf(ContainerResourceLimitExprTemplate, metric.Container.ContainerName, v1.ResourceMemory.String(), metric.Container.WorkloadName, metric.Container.Namespace, clusterCond, metric.Container.Kind, metric.Container.WorkloadName, metric.Container.Namespace, clusterCond),
			}), nil
		}
	default:
		return nil, fmt.Errorf("metric type %v do not support resource metric %v. only support %v now", metric.Type, metric.MetricName, supportedResources.List())
	}
}

func (b *builder) podQuery(metric *metricquery.Metric, behavior querybuilder.BuildQueryBehavior) (*metricquery.Query, error) {
	if metric.Pod == nil {
		return nil, fmt.Errorf("metric type %v, but no PodNamerInfo provided", metric.Type)
	}
	selector := metric.Pod.Selector
	clusterCond := ""
	if selector != nil && behavior.FederatedClusterScope {
		if id, exists := selector.RequiresExactMatch(consts.LabelClusterId); exists {
			clusterCond = fmt.Sprintf(`cluster="%v"`, id)
		}
	}
	switch strings.ToLower(metric.MetricName) {
	case v1.ResourceCPU.String():
		return promQuery(&metricquery.PrometheusQuery{
			Query: fmt.Sprintf(PodCpuUsageExprTemplate, metric.Pod.Namespace, metric.Pod.Name, clusterCond, "3m"),
		}), nil
	case v1.ResourceMemory.String():
		return promQuery(&metricquery.PrometheusQuery{
			Query: fmt.Sprintf(PodMemUsageExprTemplate, metric.Pod.Namespace, metric.Pod.Name, clusterCond),
		}), nil
	default:
		return nil, fmt.Errorf("metric type %v do not support resource metric %v. only support %v now", metric.Type, metric.MetricName, supportedResources.List())
	}
}

func (b *builder) nodeQuery(metric *metricquery.Metric, behavior querybuilder.BuildQueryBehavior) (*metricquery.Query, error) {
	if metric.Node == nil {
		return nil, fmt.Errorf("metric type %v, but no NodeNamerInfo provided", metric.Type)
	}
	selector := metric.Node.Selector
	clusterCond := ""
	if selector != nil && behavior.FederatedClusterScope {
		if id, exists := selector.RequiresExactMatch(consts.LabelClusterId); exists {
			clusterCond = fmt.Sprintf(`cluster="%v"`, id)
		}
	}
	switch strings.ToLower(metric.MetricName) {
	case v1.ResourceCPU.String():
		return promQuery(&metricquery.PrometheusQuery{
			Query: fmt.Sprintf(NodeCpuUsageExprTemplate, metric.Node.Name, clusterCond, metric.Node.Name, clusterCond, "3m"),
		}), nil
	case v1.ResourceMemory.String():
		return promQuery(&metricquery.PrometheusQuery{
			Query: fmt.Sprintf(NodeMemUsageExprTemplate, metric.Node.Name, clusterCond, metric.Node.Name, clusterCond),
		}), nil
	default:
		return nil, fmt.Errorf("metric type %v do not support resource metric %v. only support %v now", metric.Type, metric.MetricName, supportedResources.List())
	}
}

//nolint:unparam
func (b *builder) promQuery(metric *metricquery.Metric, behavior querybuilder.BuildQueryBehavior) (*metricquery.Query, error) {
	if metric.Prom == nil {
		return nil, fmt.Errorf("metric type %v, but no PromNamerInfo provided", metric.Type)
	}
	return promQuery(&metricquery.PrometheusQuery{
		Query: metric.Prom.QueryExpr,
	}), nil
}

func promQuery(prom *metricquery.PrometheusQuery) *metricquery.Query {
	return &metricquery.Query{
		Type:       metricquery.PrometheusMetricSource,
		Prometheus: prom,
	}
}

func init() {
	querybuilder.RegisterBuilderFactory(metricquery.PrometheusMetricSource, NewPromQueryBuilder)
}
