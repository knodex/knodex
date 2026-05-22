// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package wrapper

import (
	"context"
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// Sentinel errors returned by the wrapper helpers. Callers in the HTTP layer
// translate these into 422 WRAPPER_MISCONFIGURED responses.
var (
	ErrWrapperRGDNotFound = errors.New("wrapper RGD not found in cluster")
	ErrWrapperRGDNotReady = errors.New("wrapper RGD not ready")
)

// Helpers exposes the request-time wrapper-routing operations consumed by the
// project handler (and, in the future, secret/repository handlers).
type Helpers struct {
	watcher       *Watcher          // for LookupWrapper (in-memory cache, hot path)
	rgdResolver   RGDResolver       // for GVK resolution at request time
	gvrResolver   GVRResolver       // for GVR resolution (discovery + naive fallback)
	dynamicClient dynamic.Interface // for instance CRUD
	namespace     string            // Knodex install namespace
}

// NewHelpers constructs a *Helpers.
//
// All four collaborators are required for the hot path. The constructor does
// not panic on nil — the caller is expected to gate registration on availability
// (see app/app.go pattern for SSO and other optional features).
func NewHelpers(w *Watcher, r RGDResolver, gvr GVRResolver, dc dynamic.Interface, namespace string) *Helpers {
	return &Helpers{
		watcher:       w,
		rgdResolver:   r,
		gvrResolver:   gvr,
		dynamicClient: dc,
		namespace:     namespace,
	}
}

// LookupWrapper returns the registered RGD name for the given Kind, or ("", false).
// Implemented against the watcher's in-memory cache.
func (h *Helpers) LookupWrapper(kind string) (rgdName string, ok bool) {
	if h == nil || h.watcher == nil {
		return "", false
	}
	return h.watcher.Lookup(kind)
}

// ResolveInstanceGVK looks up the wrapper RGD by name and returns the
// instance GVK declared by the RGD's spec.schema.
func (h *Helpers) ResolveInstanceGVK(rgdName string) (apiVersion, kind string, err error) {
	if h == nil || h.rgdResolver == nil {
		return "", "", ErrWrapperRGDNotFound
	}
	rgd, ok := h.rgdResolver.GetRGDByName(rgdName)
	if !ok || rgd == nil {
		return "", "", fmt.Errorf("%w: %s", ErrWrapperRGDNotFound, rgdName)
	}
	if rgd.APIVersion == "" || rgd.Kind == "" {
		return "", "", fmt.Errorf("%w: %s (missing apiVersion or kind in RGD schema)", ErrWrapperRGDNotReady, rgdName)
	}
	return rgd.APIVersion, rgd.Kind, nil
}

// CreateViaWrapper builds an unstructured wrapper instance carrying `spec` and
// creates it via the dynamic client in the install namespace. The wrapper RGD
// MUST be registered for `kind = KindProject` in v1; callers gate on that.
func (h *Helpers) CreateViaWrapper(ctx context.Context, rgdName, instanceName string, spec map[string]any) (*unstructured.Unstructured, error) {
	if h == nil || h.dynamicClient == nil {
		return nil, ErrWrapperRGDNotFound
	}
	apiVersion, kind, err := h.ResolveInstanceGVK(rgdName)
	if err != nil {
		return nil, err
	}
	gvr, err := h.resolveGVR(apiVersion, kind)
	if err != nil {
		return nil, fmt.Errorf("resolve GVR for %s/%s: %w", apiVersion, kind, err)
	}

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(instanceName)
	obj.SetNamespace(h.namespace)
	obj.SetLabels(map[string]string{
		LabelManagedBy:   LabelManagedByVal,
		WrapperKindLabel: KindProject,
	})
	if spec != nil {
		if err := unstructured.SetNestedField(obj.Object, spec, "spec"); err != nil {
			// SetNestedField only fails on type mismatch — the spec map is well-formed.
			return nil, fmt.Errorf("set spec on wrapper instance: %w", err)
		}
	}

	created, err := h.dynamicClient.Resource(gvr).Namespace(h.namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateViaWrapper PATCHes the wrapper instance's spec with a merge patch.
// Returns the updated object.
func (h *Helpers) UpdateViaWrapper(ctx context.Context, rgdName, instanceName string, spec map[string]any) (*unstructured.Unstructured, error) {
	if h == nil || h.dynamicClient == nil {
		return nil, ErrWrapperRGDNotFound
	}
	apiVersion, kind, err := h.ResolveInstanceGVK(rgdName)
	if err != nil {
		return nil, err
	}
	gvr, err := h.resolveGVR(apiVersion, kind)
	if err != nil {
		return nil, fmt.Errorf("resolve GVR for %s/%s: %w", apiVersion, kind, err)
	}

	patch := map[string]any{"spec": spec}
	patchBytes, err := jsonMarshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshal patch: %w", err)
	}
	updated, err := h.dynamicClient.Resource(gvr).Namespace(h.namespace).Patch(
		ctx, instanceName, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// DeleteViaWrapper deletes the wrapper instance. kro garbage-collects the
// bundle (Project + bootstrap extras) via its owner references.
func (h *Helpers) DeleteViaWrapper(ctx context.Context, rgdName, instanceName string) error {
	if h == nil || h.dynamicClient == nil {
		return ErrWrapperRGDNotFound
	}
	apiVersion, kind, err := h.ResolveInstanceGVK(rgdName)
	if err != nil {
		return err
	}
	gvr, err := h.resolveGVR(apiVersion, kind)
	if err != nil {
		return fmt.Errorf("resolve GVR for %s/%s: %w", apiVersion, kind, err)
	}
	return h.dynamicClient.Resource(gvr).Namespace(h.namespace).Delete(ctx, instanceName, metav1.DeleteOptions{})
}

// resolveGVR delegates to the injected GVRResolver when available (discovery-backed),
// falling back to naive pluralization. This mirrors instance_deployment.directDeploy.
func (h *Helpers) resolveGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	if h.gvrResolver != nil {
		return h.gvrResolver.ResolveGVR(apiVersion, kind)
	}
	group, version := parseAPIVersion(apiVersion)
	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(kind) + "s",
	}, nil
}

// IsWrapped reports whether the given annotation set marks the resource as
// wrapper-managed. Pure function — no I/O.
func IsWrapped(annotations map[string]string) bool {
	return OwningRGDInstance(annotations) != ""
}

// OwningRGDInstance returns the value of the marker annotation, or "" if absent.
func OwningRGDInstance(annotations map[string]string) string {
	if annotations == nil {
		return ""
	}
	return annotations[MarkerAnnotation]
}

// parseAPIVersion splits "group/version" into (group, version). For core-API
// strings like "v1" returns ("", "v1").
func parseAPIVersion(apiVersion string) (group, version string) {
	if i := strings.Index(apiVersion, "/"); i >= 0 {
		return apiVersion[:i], apiVersion[i+1:]
	}
	return "", apiVersion
}

// jsonMarshal is package-local so we can swap it during tests if needed.
var jsonMarshal = func(v any) ([]byte, error) {
	return jsonStdMarshal(v)
}
