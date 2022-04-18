package owner

import (
	"context"
	"reflect"
	"testing"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "k8s.io/apiserver/pkg/apis/example/v1"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/restmapper"
	coretesting "k8s.io/client-go/testing"
)

var (
	scheme = runtime.NewScheme()
)

func fakeKubeDynamicClient(t *testing.T) dynamic.Interface {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	pod1, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pod1",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "pod1-dep"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	pod2, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pod2",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "CronJob", Name: "pod2-cronjob"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	pod3, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pod3",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Name: "pod3-rs"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	rs, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&extv1beta1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "ReplicaSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pod3-rs",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "pod3-rs-dep"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	dep1, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1-dep",
			Namespace: "default",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	dep2, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod3-rs-dep",
			Namespace: "default",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cjob, err := runtime.DefaultUnstructuredConverter.ToUnstructured(
		&batchv1.CronJob{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "batch/v1",
				Kind:       "CronJob",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod2-cronjob",
				Namespace: "default",
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	fakeKubeClient := fakedynamic.NewSimpleDynamicClient(scheme,
		&unstructured.Unstructured{Object: pod1},
		&unstructured.Unstructured{Object: pod2},
		&unstructured.Unstructured{Object: pod3},
		&unstructured.Unstructured{Object: rs},
		&unstructured.Unstructured{Object: dep1},
		&unstructured.Unstructured{Object: dep2},
		&unstructured.Unstructured{Object: cjob},
	)
	return fakeKubeClient
}

func fakeRestMapper(t *testing.T) ([]schema.GroupResource, meta.RESTMapper) {
	scheme.AllKnownTypes()

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
		{
			GroupVersion: batchv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "jobs", Namespaced: true, Kind: "Job"},
				{Name: "cronjobs", Namespaced: true, Kind: "CronJob"},
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

	groupResources := []schema.GroupResource{
		{Group: corev1.GroupName, Resource: "replicationcontrollers"},
		{Group: extv1beta1.GroupName, Resource: "replicasets"},
		{Group: appsv1beta2.GroupName, Resource: "deployments"},
		{Group: "cheese.testing.k8s.io", Resource: "cheddars"},
	}

	return groupResources, restMapper
}

func TestFindRootOwner(t *testing.T) {
	dclient := fakeKubeDynamicClient(t)
	_, restMapper := fakeRestMapper(t)
	converter := runtime.DefaultUnstructuredConverter
	type workload struct {
		name       string
		namespace  string
		kind       string
		apiversion string
	}
	tcs := []struct {
		desc  string
		input runtime.Object
		want  *workload
		err   error
	}{
		{
			desc: "tc1",
			input: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "pod1",
					Namespace:       "default",
					OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "pod1-dep"}},
				},
			},
			want: &workload{
				name:       "pod1-dep",
				namespace:  "default",
				kind:       "Deployment",
				apiversion: "apps/v1",
			},
		},
		{
			desc: "tc2",
			input: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "pod2",
					Namespace:       "default",
					OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "CronJob", Name: "pod2-cronjob"}},
				},
			},
			want: &workload{
				name:       "pod2-cronjob",
				namespace:  "default",
				kind:       "CronJob",
				apiversion: "batch/v1",
			},
		},
		{
			desc: "tc3",
			input: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "pod3",
					Namespace:       "default",
					OwnerReferences: []metav1.OwnerReference{{APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Name: "pod3-rs"}},
				},
			},
			want: &workload{
				name:       "pod3-rs-dep",
				namespace:  "default",
				kind:       "Deployment",
				apiversion: "apps/v1",
			},
		},
		{
			desc: "tc4",
			input: &extv1beta1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "pod3-rs",
					Namespace:       "default",
					OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "pod3-rs-dep"}},
				},
			},
			want: &workload{
				name:       "pod3-rs-dep",
				namespace:  "default",
				kind:       "Deployment",
				apiversion: "apps/v1",
			},
		},
	}

	for _, tc := range tcs {
		unstruct, err := converter.ToUnstructured(tc.input)
		if err != nil {
			t.Fatal(err)
		}
		gotRootUnstruct, gotErr := FindRootOwner(context.TODO(), restMapper, dclient, &unstructured.Unstructured{Object: unstruct})
		if !reflect.DeepEqual(gotErr, tc.err) {
			t.Fatalf("tc %v failed, gotErr: %v, wantErr: %v", tc.desc, gotErr, tc.want)
		}
		gotWorkload := &workload{
			namespace:  gotRootUnstruct.GetNamespace(),
			name:       gotRootUnstruct.GetName(),
			kind:       gotRootUnstruct.GetKind(),
			apiversion: gotRootUnstruct.GetAPIVersion(),
		}
		if !reflect.DeepEqual(gotWorkload, tc.want) {
			t.Fatalf("tc %v failed, gotWorkload: %v, want: %v", tc.desc, gotWorkload, tc.want)
		}
	}

}
