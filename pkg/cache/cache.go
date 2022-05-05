package cache

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appslister "k8s.io/client-go/listers/apps/v1"
	autoscalinglister "k8s.io/client-go/listers/autoscaling/v1"
	lister "k8s.io/client-go/listers/core/v1"
	clientcache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type Cache interface {
	GetAllHPAs() []*autoscalingv1.HorizontalPodAutoscaler
	GetStatefulSets() []*appsv1.StatefulSet
	GetDaemonSets() []*appsv1.DaemonSet
	GetDeployments() []*appsv1.Deployment
	GetPods() []*v1.Pod
	GetNodes() []*v1.Node
	WaitForCacheSync(stopCh <-chan struct{})
}

type cache struct {
	sharedInformer informers.SharedInformerFactory

	podInformer  clientcache.SharedIndexInformer
	nodeInformer clientcache.SharedIndexInformer

	podLister        lister.PodLister
	nodeLister       lister.NodeLister
	deploymentLister appslister.DeploymentLister
	daemonsetLister  appslister.DaemonSetLister
	stsLister        appslister.StatefulSetLister
	hpaLister        autoscalinglister.HorizontalPodAutoscalerLister
}

func (c *cache) GetStatefulSets() []*appsv1.StatefulSet {
	stsList, err := c.stsLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to GetStatefulSets in cache: %v", err)
		return stsList
	}
	return stsList
}

func (c *cache) GetDaemonSets() []*appsv1.DaemonSet {
	dsList, err := c.daemonsetLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to GetDaemonSets in cache: %v", err)
		return dsList
	}
	return dsList
}

func (c *cache) GetDeployments() []*appsv1.Deployment {
	dpList, err := c.deploymentLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to GetDeployments in cache: %v", err)
		return dpList
	}
	return dpList
}

func NewCache(client kubernetes.Interface) Cache {
	return &cache{
		sharedInformer: informers.NewSharedInformerFactory(client, 30*time.Minute),
	}
}

func (c *cache) WaitForCacheSync(stopCh <-chan struct{}) {
	c.podInformer = c.sharedInformer.Core().V1().Pods().Informer()
	c.nodeInformer = c.sharedInformer.Core().V1().Nodes().Informer()

	c.podLister = c.sharedInformer.Core().V1().Pods().Lister()
	c.nodeLister = c.sharedInformer.Core().V1().Nodes().Lister()
	c.deploymentLister = c.sharedInformer.Apps().V1().Deployments().Lister()
	c.daemonsetLister = c.sharedInformer.Apps().V1().DaemonSets().Lister()
	c.stsLister = c.sharedInformer.Apps().V1().StatefulSets().Lister()
	c.hpaLister = c.sharedInformer.Autoscaling().V1().HorizontalPodAutoscalers().Lister()

	c.sharedInformer.Start(stopCh)
	c.sharedInformer.WaitForCacheSync(stopCh)
}

func (c *cache) GetAllHPAs() []*autoscalingv1.HorizontalPodAutoscaler {
	hpaList, err := c.hpaLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to GetHPAs in cache: %v", err)
		return hpaList
	}
	return hpaList
}

func (c *cache) GetPods() []*v1.Pod {
	podList, err := c.podLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to GetPods in cache: %v", err)
		return podList
	}
	return podList
}

func (c *cache) GetNodes() []*v1.Node {
	nodeList, err := c.nodeLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to GetNodes in cache: %v", err)
		return nodeList
	}
	return nodeList
}
