package target

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/scale"
)

type TargetInfoFetcher interface {
	// FetchSelector returns labelSelector, used to gather Pods controlled by the given targetRef.
	FetchSelector(targetRef *corev1.ObjectReference) (labels.Selector, error)
	// FetchReplicas return desired replicas, current replicas
	FetchReplicas(targetRef *corev1.ObjectReference) (int32, int32, error)
}

const (
	DaemonSet             string = "DaemonSet"
	Deployment            string = "Deployment"
	StatefulSet           string = "StatefulSet"
	ReplicaSet            string = "ReplicaSet"
	ReplicationController string = "ReplicationController"
	CronJob               string = "CronJob"
	Job                   string = "Job"
)

// NewTargetInfoFetcher returns new instance of TargetInfoFetcher
func NewTargetInfoFetcher(restMapper meta.RESTMapper, scaleClient scale.ScalesGetter, kubeClient clientset.Interface) TargetInfoFetcher {
	return &targetInfoFetcher{
		RestMapper:  restMapper,
		ScaleClient: scaleClient,
		KubeClient:  kubeClient,
	}
}

type targetInfoFetcher struct {
	Scheme      *runtime.Scheme
	RestMapper  meta.RESTMapper
	ScaleClient scale.ScalesGetter
	KubeClient  clientset.Interface
}

func (f *targetInfoFetcher) FetchReplicas(target *corev1.ObjectReference) (int32, int32, error) {
	groupVersion, err := schema.ParseGroupVersion(target.APIVersion)
	if err != nil {
		return 0, 0, err
	}
	groupKind := schema.GroupKind{
		Group: groupVersion.Group,
		Kind:  target.Kind,
	}

	if strings.ToLower(target.Kind) == strings.ToLower(DaemonSet) {
		daemonset, err := f.KubeClient.AppsV1().DaemonSets(target.Namespace).Get(context.TODO(), target.Name, metav1.GetOptions{})
		if err != nil {
			return 0, 0, err
		}
		return daemonset.Status.DesiredNumberScheduled, daemonset.Status.CurrentNumberScheduled, nil
	} else {
		return f.getReplicasFromScale(groupKind, target.Namespace, target.Name)
	}
}

func (f *targetInfoFetcher) FetchSelector(target *corev1.ObjectReference) (labels.Selector, error) {
	groupVersion, err := schema.ParseGroupVersion(target.APIVersion)
	if err != nil {
		return nil, err
	}
	groupKind := schema.GroupKind{
		Group: groupVersion.Group,
		Kind:  target.Kind,
	}

	if strings.ToLower(target.Kind) == strings.ToLower(DaemonSet) {
		daemonset, err := f.KubeClient.AppsV1().DaemonSets(target.Namespace).Get(context.TODO(), target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		selector, err := metav1.LabelSelectorAsSelector(daemonset.Spec.Selector)
		if err != nil {
			return nil, err
		}
		return selector, nil
	} else {
		return f.getLabelSelectorFromScale(groupKind, target.Namespace, target.Name)
	}
}

func (f *targetInfoFetcher) getLabelSelectorFromScale(groupKind schema.GroupKind, namespace, name string) (labels.Selector, error) {
	mappings, err := f.RestMapper.RESTMappings(groupKind)
	if err != nil {
		return nil, err
	}

	var errs []error
	for _, mapping := range mappings {
		groupResource := mapping.Resource.GroupResource()
		scale, err := f.ScaleClient.Scales(namespace).Get(context.TODO(), groupResource, name, metav1.GetOptions{})
		if err == nil {
			if scale.Status.Selector == "" {
				return nil, fmt.Errorf("Resource %s/%s has an empty selector for scale sub-resource", namespace, name)
			}
			selector, err := labels.Parse(scale.Status.Selector)
			if err != nil {
				return nil, err
			}
			return selector, nil
		}
		errs = append(errs, err)
	}
	return nil, fmt.Errorf("%+v", errs)
}

func (f *targetInfoFetcher) getReplicasFromScale(groupKind schema.GroupKind, namespace, name string) (int32, int32, error) {
	mappings, err := f.RestMapper.RESTMappings(groupKind)
	if err != nil {
		return 0, 0, err
	}

	var errs []error
	for _, mapping := range mappings {
		groupResource := mapping.Resource.GroupResource()
		scale, err := f.ScaleClient.Scales(namespace).Get(context.TODO(), groupResource, name, metav1.GetOptions{})
		if err == nil {
			return scale.Spec.Replicas, scale.Status.Replicas, nil
		}
		errs = append(errs, err)
	}
	return 0, 0, fmt.Errorf("%+v", errs)
}
