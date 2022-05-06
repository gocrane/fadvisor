package metricnaming

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/gocrane/fadvisor/pkg/consts"
	"github.com/gocrane/fadvisor/pkg/metricquery"
	"github.com/gocrane/fadvisor/pkg/querybuilder"
)

// MetricNamer is an interface. it is the bridge between predictor and different data sources and other component.
type MetricNamer interface {
	// Used for datasource provider, data source provider call QueryBuilder
	QueryBuilder() querybuilder.QueryBuilder
	// Used for predictor now
	BuildUniqueKey() string

	Validate() error

	AddSelectorRequirement(requirement labels.Requirement)
}

type GeneralMetricNamer struct {
	Metric *metricquery.Metric
}

func (gmn *GeneralMetricNamer) QueryBuilder() querybuilder.QueryBuilder {
	return NewQueryBuilder(gmn.Metric)
}

func (gmn *GeneralMetricNamer) BuildUniqueKey() string {
	return gmn.Metric.BuildUniqueKey()
}

func (gmn *GeneralMetricNamer) Validate() error {
	return gmn.Metric.ValidateMetric()
}

func (gmn *GeneralMetricNamer) AddSelectorRequirement(requirement labels.Requirement) {
	if gmn.Metric.Node != nil && gmn.Metric.Node.Selector != nil {
		gmn.Metric.Node.Selector.Add(requirement)
	}
	if gmn.Metric.Pod != nil && gmn.Metric.Pod.Selector != nil {
		gmn.Metric.Pod.Selector.Add(requirement)
	}
	if gmn.Metric.Container != nil && gmn.Metric.Container.Selector != nil {
		gmn.Metric.Container.Selector.Add(requirement)
	}
	if gmn.Metric.Workload != nil && gmn.Metric.Workload.Selector != nil {
		gmn.Metric.Workload.Selector.Add(requirement)
	}
	if gmn.Metric.Prom != nil && gmn.Metric.Prom.Selector != nil {
		gmn.Metric.Prom.Selector.Add(requirement)
	}
}

type queryBuilderFactory struct {
	metric *metricquery.Metric
}

func (q queryBuilderFactory) Builder(source metricquery.MetricSource) querybuilder.Builder {
	initFunc := querybuilder.GetBuilderFactory(source)
	return initFunc(q.metric)
}

func NewQueryBuilder(metric *metricquery.Metric) querybuilder.QueryBuilder {
	return &queryBuilderFactory{
		metric: metric,
	}
}

func WorkloadMetricNamer(clusterid string, target *corev1.ObjectReference, metricName string, workloadLabelSelector labels.Selector) MetricNamer {
	// workload
	set := labels.Set{}
	if clusterid != "" {
		set[consts.LabelClusterId] = clusterid
	}

	selector := labels.SelectorFromSet(set)
	if reqs, ok := selector.Requirements(); ok {
		if workloadLabelSelector == nil {
			workloadLabelSelector = selector
		} else {
			workloadLabelSelector = workloadLabelSelector.Add(reqs...)
		}
	}

	return &GeneralMetricNamer{
		Metric: &metricquery.Metric{
			Type:       metricquery.WorkloadMetricType,
			MetricName: metricName,
			Workload: &metricquery.WorkloadNamerInfo{
				Namespace:  target.Namespace,
				Kind:       target.Kind,
				APIVersion: target.APIVersion,
				Name:       target.Name,
				Selector:   workloadLabelSelector,
			},
		},
	}
}

func ContainerMetricNamer(clusterid, kind, namespace, workloadName, containername, metricName string, containerLabelSelector labels.Selector) MetricNamer {
	// container
	set := labels.Set{}
	if clusterid != "" {
		set[consts.LabelClusterId] = clusterid
	}
	if namespace != "" {
		set[consts.LabelNamespace] = namespace
	}

	if workloadName != "" {
		set[consts.LabelWorkloadName] = workloadName
	}

	if containername != "" {
		set[consts.LabelContainerName] = containername
	}

	selector := labels.SelectorFromSet(set)
	if reqs, ok := selector.Requirements(); ok {
		if containerLabelSelector == nil {
			containerLabelSelector = selector
		} else {
			containerLabelSelector = containerLabelSelector.Add(reqs...)
		}
	}

	return &GeneralMetricNamer{
		Metric: &metricquery.Metric{
			Type:       metricquery.ContainerMetricType,
			MetricName: metricName,
			Container: &metricquery.ContainerNamerInfo{
				Namespace:     namespace,
				Kind:          kind,
				WorkloadName:  workloadName,
				ContainerName: containername,
				Selector:      containerLabelSelector,
			},
		},
	}
}

func ResourceToContainerMetricNamer(clusterid, namespace, workloadName, containername string, resourceName corev1.ResourceName) MetricNamer {
	// container
	set := labels.Set{}
	if clusterid != "" {
		set[consts.LabelClusterId] = clusterid
	}
	if namespace != "" {
		set[consts.LabelNamespace] = namespace
	}

	if workloadName != "" {
		set[consts.LabelWorkloadName] = workloadName
	}

	if containername != "" {
		set[consts.LabelContainerName] = containername
	}

	return &GeneralMetricNamer{
		Metric: &metricquery.Metric{
			Type:       metricquery.ContainerMetricType,
			MetricName: resourceName.String(),
			Container: &metricquery.ContainerNamerInfo{
				Namespace:     namespace,
				WorkloadName:  workloadName,
				ContainerName: containername,
				Selector:      labels.SelectorFromSet(set),
			},
		},
	}
}

func ResourceToWorkloadMetricNamer(clusterid string, target *corev1.ObjectReference, resourceName corev1.ResourceName, workloadLabelSelector labels.Selector) MetricNamer {
	// workload
	set := labels.Set{}
	if clusterid != "" {
		set[consts.LabelClusterId] = clusterid
	}

	selector := labels.SelectorFromSet(set)
	if reqs, ok := selector.Requirements(); ok {
		workloadLabelSelector = workloadLabelSelector.Add(reqs...)
	}

	return &GeneralMetricNamer{
		Metric: &metricquery.Metric{
			Type:       metricquery.WorkloadMetricType,
			MetricName: resourceName.String(),
			Workload: &metricquery.WorkloadNamerInfo{
				Namespace:  target.Namespace,
				Kind:       target.Kind,
				APIVersion: target.APIVersion,
				Name:       target.Name,
				Selector:   workloadLabelSelector,
			},
		},
	}
}
