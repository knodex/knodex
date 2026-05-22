// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package wrapper implements operator-configured wrapper-RGD routing for
// resource creation. When a kro ResourceGraphDefinition is registered for a
// Kind (currently Project only), the corresponding HTTP handler creates a
// wrapper-RGD instance instead of the resource directly. The kro controller
// then materializes the bundle (built-in resource plus any operator bootstrap
// extras like namespaces, NetworkPolicies, secret stores).
//
// The wrapper registry is stored in a Kubernetes ConfigMap and exposed via a
// settings-style HTTP CRUD surface. The watcher refreshes an in-memory cache
// on change; the helpers package consumes the cache on the request hot path.
package wrapper

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knodex/knodex/server/internal/models"
)

const (
	// ConfigMapName is the name of the ConfigMap storing wrapper registry entries.
	ConfigMapName = "knodex-resource-wrappers"
	// ConfigMapKey is the key in the ConfigMap that holds the wrapper JSON array.
	ConfigMapKey = "wrappers.json"

	// LabelManagedBy / LabelManagedByVal mark the ConfigMap as managed by Knodex.
	LabelManagedBy    = "app.kubernetes.io/managed-by"
	LabelManagedByVal = "knodex"
	// LabelConfigType / LabelConfigTypeVal scope the ConfigMap to the wrapper feature.
	LabelConfigType    = "knodex.io/config-type"
	LabelConfigTypeVal = "resource-wrappers"

	// MarkerAnnotation is set on resources created via a wrapper RGD. Its value
	// is the name of the owning wrapper-RGD instance. Lifecycle CRUD on the
	// resource is routed via this annotation: present → route through the
	// instance; absent → operate on the resource directly.
	MarkerAnnotation = "knodex.io/wrapper-rgd-instance"

	// WrapperKindLabel is stamped on the wrapper-RGD instance so operators can
	// list "all instances created by Knodex wrappers" with a single selector.
	WrapperKindLabel = "knodex.io/wrapper-kind"

	// KindProject is the only Kind allowed in v1. Future Kinds widen this allowlist
	// without re-architecture (per tech-spec scope).
	KindProject = "Project"

	// MaxRGDNameLength is the maximum allowed length for an RGD name (DNS-1123 subdomain).
	MaxRGDNameLength = 253
)

// Entry is one wrapper registry entry: Kind → RGD name.
type Entry struct {
	Kind    string `json:"kind"`
	RGDName string `json:"rgdName"`
}

// RGDResolver is the minimal interface the wrapper helpers need to look up a
// wrapper RGD's instance GVK at request time. Implemented by *watcher.RGDWatcher.
type RGDResolver interface {
	GetRGDByName(name string) (*models.CatalogRGD, bool)
}

// GVRResolver resolves an apiVersion+kind pair to a GroupVersionResource via
// API discovery (with a naive-pluralization fallback). Implemented by
// *watcher.InstanceTracker.
type GVRResolver interface {
	ResolveGVR(apiVersion, kind string) (schema.GroupVersionResource, error)
}

// SupportedKinds returns the allowlist of Kinds for which wrappers may be
// registered. v1 supports only Project.
func SupportedKinds() []string {
	return []string{KindProject}
}

// IsSupportedKind reports whether the given Kind is in the wrapper allowlist.
func IsSupportedKind(kind string) bool {
	for _, k := range SupportedKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
