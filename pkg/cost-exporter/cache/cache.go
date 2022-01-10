package cache

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	lister "k8s.io/client-go/listers/core/v1"
	clientcache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type Cache interface {
	GetPods() []*v1.Pod
	GetNodes() []*v1.Node
	WaitForCacheSync(stopCh <-chan struct{})
}

type cache struct {
	sharedInformer informers.SharedInformerFactory

	podInformer  clientcache.SharedIndexInformer
	nodeInformer clientcache.SharedIndexInformer

	podLister  lister.PodLister
	nodeLister lister.NodeLister
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
	c.sharedInformer.Start(stopCh)
	c.sharedInformer.WaitForCacheSync(stopCh)
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
