package target

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	jsonpatch "github.com/evanphx/json-patch"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	fakerest "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"
	coretesting "k8s.io/client-go/testing"
)

var scaleConverter = scale.NewScaleConverter()
var codecs = serializer.NewCodecFactory(scaleConverter.Scheme())

func bytesBody(bodyBytes []byte) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(bodyBytes))
}

func defaultHeaders() http.Header {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	return header
}

func fakeKubeClient() kubernetes.Interface {
	fakeKubeClient := fake.NewSimpleClientset(
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "foo"}, Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 10, CurrentNumberScheduled: 8}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "foo"}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "foo"}},
		&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "foo"}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "foo"}},
	)
	return fakeKubeClient
}

func fakeScaleClient(t *testing.T) (scale.ScalesGetter, []schema.GroupResource, meta.RESTMapper) {
	fakeDiscoveryClient := &fakedisco.FakeDiscovery{Fake: &coretesting.Fake{}}
	//  make sure all APIResource in the same GroupVersion, not split the same GroupVersion APIResources to different element of APIResourceList,
	// or it will override previous same GroupVersion, because it is a map's key in restmapper.GetAPIGroupResources
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: corev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
				{Name: "replicationcontrollers/scale", Namespaced: true, Kind: "Scale"},
				{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet"},
				{Name: "replicasets/scale", Namespaced: true, Kind: "Scale"},
			},
		},

		{
			GroupVersion: extv1beta1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet"},
				{Name: "replicasets/scale", Namespaced: true, Kind: "Scale"},
			},
		},
		{
			GroupVersion: appsv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
				{Name: "deployments/scale", Namespaced: true, Kind: "Scale", Group: "apps", Version: "v1"},
				{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
				{Name: "statefulsets/scale", Namespaced: true, Kind: "Scale", Group: "apps", Version: "v1"},
			},
		},
		{
			GroupVersion: appsv1beta2.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
				{Name: "deployments/scale", Namespaced: true, Kind: "Scale", Group: "apps", Version: "v1beta2"},
			},
		},
		{
			GroupVersion: appsv1beta1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
				{Name: "statefulsets/scale", Namespaced: true, Kind: "Scale", Group: "apps", Version: "v1beta1"},
			},
		},
		// test a resource that doesn't exist anywhere to make sure we're not accidentally depending
		// on a static RESTMapper anywhere.
		{
			GroupVersion: "cheese.testing.k8s.io/v27alpha15",
			APIResources: []metav1.APIResource{
				{Name: "cheddars", Namespaced: true, Kind: "Cheddar"},
				{Name: "cheddars/scale", Namespaced: true, Kind: "Scale", Group: "extensions", Version: "v1beta1"},
			},
		},
	}

	restMapperRes, err := restmapper.GetAPIGroupResources(fakeDiscoveryClient)
	if err != nil {
		t.Fatalf("unexpected error while constructing resource list from fake discovery client: %v", err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(restMapperRes)

	autoscalingScale := &autoscalingv1.Scale{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scale",
			APIVersion: autoscalingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: autoscalingv1.ScaleSpec{Replicas: 10},
		Status: autoscalingv1.ScaleStatus{
			Replicas: 10,
			Selector: "foo=bar",
		},
	}
	extScale := &extv1beta1.Scale{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scale",
			APIVersion: extv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: extv1beta1.ScaleSpec{Replicas: 10},
		Status: extv1beta1.ScaleStatus{
			Replicas:       10,
			TargetSelector: "foo=bar",
		},
	}
	appsV1beta2Scale := &appsv1beta2.Scale{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scale",
			APIVersion: appsv1beta2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: appsv1beta2.ScaleSpec{Replicas: 10},
		Status: appsv1beta2.ScaleStatus{
			Replicas:       10,
			TargetSelector: "foo=bar",
		},
	}
	appsV1beta1Scale := &appsv1beta1.Scale{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scale",
			APIVersion: appsv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: appsv1beta1.ScaleSpec{Replicas: 10},
		Status: appsv1beta1.ScaleStatus{
			Replicas:       10,
			TargetSelector: "foo=bar",
		},
	}

	resourcePaths := map[string]runtime.Object{
		"/api/v1/namespaces/default/replicationcontrollers/foo/scale":                  autoscalingScale,
		"/apis/extensions/v1beta1/namespaces/default/replicasets/foo/scale":            extScale,
		"/apis/apps/v1beta1/namespaces/default/statefulsets/foo/scale":                 appsV1beta1Scale,
		"/apis/apps/v1/namespaces/default/statefulsets/foo/scale":                      autoscalingScale,
		"/apis/apps/v1/namespaces/default/deployments/foo/scale":                       autoscalingScale,
		"/apis/apps/v1beta2/namespaces/default/deployments/foo/scale":                  appsV1beta2Scale,
		"/apis/cheese.testing.k8s.io/v27alpha15/namespaces/default/cheddars/foo/scale": extScale,
	}

	fakeReqHandler := func(req *http.Request) (*http.Response, error) {
		scale, isScalePath := resourcePaths[req.URL.Path]
		if !isScalePath {
			return nil, fmt.Errorf("unexpected request for URL %q with method %q", req.URL.String(), req.Method)
		}

		switch req.Method {
		case "GET":
			res, err := json.Marshal(scale)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: http.StatusOK, Header: defaultHeaders(), Body: bytesBody(res)}, nil
		case "PUT":
			decoder := codecs.UniversalDeserializer()
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			newScale, newScaleGVK, err := decoder.Decode(body, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("unexpected request body: %v", err)
			}
			if *newScaleGVK != scale.GetObjectKind().GroupVersionKind() {
				return nil, fmt.Errorf("unexpected scale API version %s (expected %s)", newScaleGVK.String(), scale.GetObjectKind().GroupVersionKind().String())
			}
			res, err := json.Marshal(newScale)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: http.StatusOK, Header: defaultHeaders(), Body: bytesBody(res)}, nil
		case "PATCH":
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			originScale, err := json.Marshal(scale)
			if err != nil {
				return nil, err
			}
			var res []byte
			contentType := req.Header.Get("Content-Type")
			pt := types.PatchType(contentType)
			switch pt {
			case types.MergePatchType:
				res, err = jsonpatch.MergePatch(originScale, body)
				if err != nil {
					return nil, err
				}
			case types.JSONPatchType:
				patch, err := jsonpatch.DecodePatch(body)
				if err != nil {
					return nil, err
				}
				res, err = patch.Apply(originScale)
				if err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("invalid patch type")
			}
			return &http.Response{StatusCode: http.StatusOK, Header: defaultHeaders(), Body: bytesBody(res)}, nil
		default:
			return nil, fmt.Errorf("unexpected request for URL %q with method %q", req.URL.String(), req.Method)
		}
	}

	fakeClient := &fakerest.RESTClient{
		Client:               fakerest.CreateHTTPClient(fakeReqHandler),
		NegotiatedSerializer: codecs.WithoutConversion(),
		GroupVersion:         schema.GroupVersion{},
		VersionedAPIPath:     "/not/a/real/path",
	}

	resolver := scale.NewDiscoveryScaleKindResolver(fakeDiscoveryClient)
	client := scale.New(fakeClient, restMapper, dynamic.LegacyAPIPathResolverFunc, resolver)

	groupResources := []schema.GroupResource{
		{Group: corev1.GroupName, Resource: "replicationcontrollers"},
		{Group: extv1beta1.GroupName, Resource: "replicasets"},
		{Group: appsv1beta2.GroupName, Resource: "deployments"},
		{Group: "cheese.testing.k8s.io", Resource: "cheddars"},
	}

	return client, groupResources, restMapper
}

func TestFetcher(t *testing.T) {
	kubeClient := fakeKubeClient()
	scaleClient, _, restMapper := fakeScaleClient(t)
	targetFetcher := NewTargetInfoFetcher(restMapper, scaleClient, kubeClient)

	testCases := []struct {
		desc                string
		input               *corev1.ObjectReference
		wantDesiredReplicas int32
		wantCurrentReplicas int32
		wantSelector        labels.Selector
		wantReplicaErr      error
		wantSelectorErr     error
	}{
		{
			desc: "tc1",
			input: &corev1.ObjectReference{
				Kind:       DaemonSet,
				Namespace:  "default",
				Name:       "foo",
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
			wantDesiredReplicas: 10,
			wantCurrentReplicas: 8,
			wantSelector:        labels.Nothing(),
		},
		{
			desc: "tc2",
			input: &corev1.ObjectReference{
				Kind:       Deployment,
				Namespace:  "default",
				Name:       "foo",
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
			wantDesiredReplicas: 10,
			wantCurrentReplicas: 10,
			wantSelector: labels.SelectorFromSet(labels.Set{
				"foo": "bar",
			}),
		},
		{
			desc: "tc3",
			input: &corev1.ObjectReference{
				Kind:       StatefulSet,
				Namespace:  "default",
				Name:       "foo",
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
			wantDesiredReplicas: 10,
			wantCurrentReplicas: 10,
			wantSelector: labels.SelectorFromSet(labels.Set{
				"foo": "bar",
			}),
		},
		{
			desc: "tc4",
			input: &corev1.ObjectReference{
				Kind:       ReplicaSet,
				Namespace:  "default",
				Name:       "foo",
				APIVersion: extv1beta1.SchemeGroupVersion.String(),
			},
			wantDesiredReplicas: 10,
			wantCurrentReplicas: 10,
			wantSelector: labels.SelectorFromSet(labels.Set{
				"foo": "bar",
			}),
		},
		{
			desc: "tc5",
			input: &corev1.ObjectReference{
				Kind:       ReplicationController,
				Namespace:  "default",
				Name:       "foo",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			wantDesiredReplicas: 10,
			wantCurrentReplicas: 10,
			wantSelector: labels.SelectorFromSet(labels.Set{
				"foo": "bar",
			}),
		},
	}

	for _, tc := range testCases {
		gotDesiredReplicas, gotCurrentReplicas, gotReplicaErr := targetFetcher.FetchReplicas(tc.input)
		gotSelector, gotSelectorErr := targetFetcher.FetchSelector(tc.input)
		if !reflect.DeepEqual(gotReplicaErr, tc.wantReplicaErr) {
			t.Fatalf("tc %v failed, gotReplicaErr: %v, wantReplicaErr: %v", tc.desc, gotReplicaErr, tc.wantReplicaErr)
		}
		if !reflect.DeepEqual(gotSelectorErr, tc.wantSelectorErr) {
			t.Fatalf("tc %v failed, gotSelectorErr: %v, wantSelectorErr: %v", tc.desc, gotSelectorErr, tc.wantSelectorErr)
		}

		if !reflect.DeepEqual(gotDesiredReplicas, tc.wantDesiredReplicas) {
			t.Fatalf("tc %v failed, gotDesiredReplicas: %v, wantDesiredReplicas: %v", tc.desc, gotDesiredReplicas, tc.wantDesiredReplicas)
		}
		if !reflect.DeepEqual(gotCurrentReplicas, tc.wantCurrentReplicas) {
			t.Fatalf("tc %v failed, gotCurrentReplicas: %v, wantCurrentReplicas: %v", tc.desc, gotCurrentReplicas, tc.wantCurrentReplicas)
		}

		if !reflect.DeepEqual(gotSelector, tc.wantSelector) {
			t.Fatalf("tc %v failed, gotSelector: %v, wantSelector: %v", tc.desc, gotSelector, tc.wantSelector)
		}

	}
}
