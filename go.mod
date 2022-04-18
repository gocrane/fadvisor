module github.com/gocrane/fadvisor

go 1.16

require (
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/gocrane/crane v0.3.0
	github.com/json-iterator/go v1.1.12
	github.com/montanaflynn/stats v0.6.6
	github.com/olekukonko/tablewriter v0.0.4
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.26.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common v1.0.383
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm v1.0.309
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/monitor v1.0.371
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke v1.0.383
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.22.3
	k8s.io/apimachinery v0.22.4
	k8s.io/apiserver v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/component-base v0.22.4
	k8s.io/klog v0.3.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kubernetes v1.22.3
	k8s.io/metrics v0.22.3
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/controller-runtime v0.10.2
)

replace (
	k8s.io/api => k8s.io/api v0.22.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.3
	k8s.io/apiserver => k8s.io/apiserver v0.22.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.3
	k8s.io/client-go => k8s.io/client-go v0.22.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.3
	k8s.io/code-generator => k8s.io/code-generator v0.22.3
	k8s.io/component-base => k8s.io/component-base v0.22.3
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.3
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.3
	k8s.io/cri-api => k8s.io/cri-api v0.22.3
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.3
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.3
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.3
	k8s.io/kubectl => k8s.io/kubectl v0.22.3
	k8s.io/kubelet => k8s.io/kubelet v0.22.3
	k8s.io/kubernetes => k8s.io/kubernetes v1.22.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.3
	k8s.io/metrics => k8s.io/metrics v0.22.3
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.3
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.3
)

replace github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke => ./staging/src/github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke
