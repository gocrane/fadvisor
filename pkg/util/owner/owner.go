package owner

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func FindRootOwner(ctx context.Context, restMapper meta.RESTMapper, dynamicClient dynamic.Interface, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	refs := obj.GetOwnerReferences()
	ns := obj.GetNamespace()

	if len(refs) > 0 {
		owner := refs[0]
		ownerGV, err := schema.ParseGroupVersion(owner.APIVersion)
		if err != nil {
			return nil, err
		}
		// assume owner group
		mappings, err := restMapper.RESTMappings(schema.GroupKind{Group: ownerGV.Group, Kind: owner.Kind}, ownerGV.Version)
		if err != nil {
			return nil, err
		}
		var errs []error
		for _, mapping := range mappings {
			ret, err := dynamicClient.Resource(mapping.Resource).Namespace(ns).Get(ctx, owner.Name, metav1.GetOptions{})
			if err == nil {
				return FindRootOwner(ctx, restMapper, dynamicClient, ret)
			}
			errs = append(errs, err)
		}
		return nil, fmt.Errorf("%v", errs)
	} else {
		unstruct, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, err
		}
		return &unstructured.Unstructured{Object: unstruct}, nil
	}
}
