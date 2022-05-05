package util

import (
	"fmt"
	"hash/fnv"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ref "k8s.io/client-go/tools/reference"
	componentconfig "k8s.io/component-base/config"
	"k8s.io/klog/v2"
)

type ClientCreator func(name string) kubernetes.Interface

// CreateK8sClient creates the k8s build in core client and custom client.
func CreateK8sClient(c componentconfig.ClientConnectionConfiguration, maxIdleConnsPerHost int) (ClientCreator, error) {
	cfg, err := NewK8sConfig(c, maxIdleConnsPerHost)
	if err != nil {
		return nil, err
	}
	defaultClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Errorf("Create k8s client failed: %v", err)
		return nil, err
	}
	creator := func(name string) kubernetes.Interface {
		cfg, err := NewK8sConfig(c, maxIdleConnsPerHost)
		if err != nil {
			return defaultClient
		}
		client, err := kubernetes.NewForConfig(rest.AddUserAgent(cfg, name))
		if err != nil {
			klog.Errorf("Create k8s client failed: %v", err)
			return defaultClient
		}
		return client
	}

	return creator, nil
}

// NewK8sConfig creates a cluster config of kubernetes.
func NewK8sConfig(c componentconfig.ClientConnectionConfiguration, maxIdleConnsPerHost int) (*rest.Config, error) {
	var (
		err    error
		k8sCfg *rest.Config
	)
	if c.Kubeconfig != "" {
		k8sCfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: c.Kubeconfig},
			&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}}).ClientConfig()
	} else {
		k8sCfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}

	k8sCfg.DisableCompression = true
	k8sCfg.AcceptContentTypes = c.AcceptContentTypes
	k8sCfg.ContentType = c.ContentType
	k8sCfg.QPS = c.QPS
	k8sCfg.Burst = int(c.Burst)

	//if err := setTransport(k8sCfg, maxIdleConnsPerHost); err != nil {
	//	return nil, err
	//}

	return k8sCfg, err
}

// nolint:unused
//func setTransport(k8sCfg *rest.Config, maxIdleConnsPerHost int) error {
//	transportConfig, err := k8sCfg.TransportConfig()
//	if err != nil {
//		return err
//	}
//	tlsConfig, err := transport.TLSConfigFor(transportConfig)
//	if err != nil {
//		return err
//	}
//	k8sCfg.Transport = utilnet.SetTransportDefaults(&http.Transport{
//		Proxy:               http.ProxyFromEnvironment,
//		TLSHandshakeTimeout: 10 * time.Second,
//		TLSClientConfig:     tlsConfig,
//		MaxIdleConnsPerHost: maxIdleConnsPerHost,
//		DialContext: (&net.Dialer{
//			Timeout:   30 * time.Second,
//			KeepAlive: 30 * time.Second,
//		}).DialContext,
//	})
//	// Overwrite TLS-related fields from config to avoid collision with
//	// Transport field.
//	k8sCfg.TLSClientConfig = rest.TLSClientConfig{}
//
//	return nil
//}

// CreateLeaderElectionConfig creates a LeaderElectionConfig.
func CreateLeaderElectionConfig(
	name string,
	k8sClient kubernetes.Interface,
	eventRecorder events.EventRecorder,
	LeaderElection componentconfig.LeaderElectionConfiguration) (*leaderelection.LeaderElectionConfig, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %v", err)
	}
	// add a unique suffix so that two processes on the same host don't accidentally both become active
	id := hostname + "_" + string(uuid.NewUUID())

	rl, err := resourcelock.New(LeaderElection.ResourceLock,
		LeaderElection.ResourceNamespace,
		LeaderElection.ResourceName,
		k8sClient.CoreV1(),
		k8sClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: eventRecorderAdaptor{eventRecorder},
		})
	if err != nil {
		return nil, fmt.Errorf("couldn't create resource lock: %v", err)
	}

	return &leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: LeaderElection.LeaseDuration.Duration,
		RenewDeadline: LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   LeaderElection.RetryPeriod.Duration,
		WatchDog:      leaderelection.NewLeaderHealthzAdaptor(time.Second * 20),
		Name:          name,
	}, nil
}

type eventRecorderAdaptor struct {
	recorder events.EventRecorder
}

func (a eventRecorderAdaptor) Eventf(obj runtime.Object, eventType, reason, message string, args ...interface{}) {
	a.recorder.Eventf(obj, nil, eventType, reason, "LeaderElection", message, args)
}

// NewEventRecorderSet creates a EventRecorderSet.
func NewEventRecorderSet(name string, num int, creator events.EventBroadcasterAdapter) *EventRecorderSet {
	cs := &EventRecorderSet{recorders: make([]events.EventRecorder, num)}
	for i := 0; i < num; i++ {
		cs.recorders[i] = creator.NewRecorder(name)
	}
	return cs
}

// ClientSet is a set of k8s EventRecorder.
type EventRecorderSet struct {
	recorders []events.EventRecorder
}

func (ers *EventRecorderSet) Eventf(regarding runtime.Object, related runtime.Object, eventType, reason, action, note string, args ...interface{}) {
	recorder := ers.recorders[0]
	if len(ers.recorders) > 0 {
		reference, err := ref.GetReference(scheme.Scheme, regarding)
		if err != nil {
			klog.Errorf("Could not construct reference to: '%#v' due to: '%v'. "+
				"Will not report event: '%v' '%v'", regarding, err, eventType, reason)
			return
		}
		h := fnv.New32()
		if _, err := h.Write([]byte(reference.UID)); err == nil {
			recorder = ers.recorders[h.Sum32()%uint32(len(ers.recorders))]
		}
	}
	recorder.Eventf(regarding, related, eventType, reason, action, note, args...)
}
