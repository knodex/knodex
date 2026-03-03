package parser

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GetName returns the name of a Kubernetes object.
// Returns an empty string if the object is nil.
func GetName(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	return obj.GetName()
}

// GetNamespace returns the namespace of a Kubernetes object.
// Returns an empty string if the object is nil or cluster-scoped.
func GetNamespace(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	return obj.GetNamespace()
}

// GetLabels returns the labels of a Kubernetes object.
// Returns an empty map (not nil) if the object is nil or has no labels.
func GetLabels(obj *unstructured.Unstructured) map[string]string {
	if obj == nil {
		return make(map[string]string)
	}
	labels := obj.GetLabels()
	if labels == nil {
		return make(map[string]string)
	}
	return labels
}

// GetAnnotations returns the annotations of a Kubernetes object.
// Returns an empty map (not nil) if the object is nil or has no annotations.
func GetAnnotations(obj *unstructured.Unstructured) map[string]string {
	if obj == nil {
		return make(map[string]string)
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return make(map[string]string)
	}
	return annotations
}

// GetAnnotation returns the value of a specific annotation and whether it exists.
// Returns ("", false) if the object is nil or the annotation doesn't exist.
func GetAnnotation(obj *unstructured.Unstructured, key string) (string, bool) {
	if obj == nil {
		return "", false
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return "", false
	}
	value, ok := annotations[key]
	return value, ok
}

// HasAnnotation returns true if the object has the specified annotation.
func HasAnnotation(obj *unstructured.Unstructured, key string) bool {
	_, ok := GetAnnotation(obj, key)
	return ok
}

// GetAnnotationOrDefault returns the value of a specific annotation, or a default
// value if the annotation doesn't exist.
func GetAnnotationOrDefault(obj *unstructured.Unstructured, key, defaultVal string) string {
	value, ok := GetAnnotation(obj, key)
	if !ok {
		return defaultVal
	}
	return value
}

// GetLabel returns the value of a specific label and whether it exists.
// Returns ("", false) if the object is nil or the label doesn't exist.
func GetLabel(obj *unstructured.Unstructured, key string) (string, bool) {
	if obj == nil {
		return "", false
	}
	labels := obj.GetLabels()
	if labels == nil {
		return "", false
	}
	value, ok := labels[key]
	return value, ok
}

// HasLabel returns true if the object has the specified label.
func HasLabel(obj *unstructured.Unstructured, key string) bool {
	_, ok := GetLabel(obj, key)
	return ok
}

// GetLabelOrDefault returns the value of a specific label, or a default value
// if the label doesn't exist.
func GetLabelOrDefault(obj *unstructured.Unstructured, key, defaultVal string) string {
	value, ok := GetLabel(obj, key)
	if !ok {
		return defaultVal
	}
	return value
}

// GetCreationTimestamp returns the creation timestamp of a Kubernetes object.
// Returns a zero time if the object is nil.
func GetCreationTimestamp(obj *unstructured.Unstructured) time.Time {
	if obj == nil {
		return time.Time{}
	}
	return obj.GetCreationTimestamp().Time
}

// GetUID returns the UID of a Kubernetes object.
// Returns an empty string if the object is nil.
func GetUID(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	return string(obj.GetUID())
}

// GetResourceVersion returns the resource version of a Kubernetes object.
// Returns an empty string if the object is nil.
func GetResourceVersion(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	return obj.GetResourceVersion()
}

// GetAPIVersion returns the API version of a Kubernetes object.
// Returns an empty string if the object is nil.
func GetAPIVersion(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	return obj.GetAPIVersion()
}

// GetKind returns the kind of a Kubernetes object.
// Returns an empty string if the object is nil.
func GetKind(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	return obj.GetKind()
}

// GetGeneration returns the generation of a Kubernetes object.
// Returns 0 if the object is nil.
func GetGeneration(obj *unstructured.Unstructured) int64 {
	if obj == nil {
		return 0
	}
	return obj.GetGeneration()
}

// GetDeletionTimestamp returns the deletion timestamp of a Kubernetes object.
// Returns nil if the object is nil or not being deleted.
func GetDeletionTimestamp(obj *unstructured.Unstructured) *time.Time {
	if obj == nil {
		return nil
	}
	ts := obj.GetDeletionTimestamp()
	if ts == nil {
		return nil
	}
	t := ts.Time
	return &t
}

// IsBeingDeleted returns true if the object has a deletion timestamp set.
func IsBeingDeleted(obj *unstructured.Unstructured) bool {
	return GetDeletionTimestamp(obj) != nil
}

// GetFinalizers returns the finalizers of a Kubernetes object.
// Returns an empty slice (not nil) if the object is nil or has no finalizers.
func GetFinalizers(obj *unstructured.Unstructured) []string {
	if obj == nil {
		return []string{}
	}
	finalizers := obj.GetFinalizers()
	if finalizers == nil {
		return []string{}
	}
	return finalizers
}

// HasFinalizer returns true if the object has the specified finalizer.
func HasFinalizer(obj *unstructured.Unstructured, finalizer string) bool {
	for _, f := range GetFinalizers(obj) {
		if f == finalizer {
			return true
		}
	}
	return false
}

// GetOwnerReferencesCount returns the number of owner references on an object.
// Returns 0 if the object is nil.
func GetOwnerReferencesCount(obj *unstructured.Unstructured) int {
	if obj == nil {
		return 0
	}
	return len(obj.GetOwnerReferences())
}

// HasOwnerReferences returns true if the object has any owner references.
func HasOwnerReferences(obj *unstructured.Unstructured) bool {
	return GetOwnerReferencesCount(obj) > 0
}

// NamespacedName returns the namespaced name of a Kubernetes object in
// "namespace/name" format, or just "name" for cluster-scoped resources.
func NamespacedName(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	ns := obj.GetNamespace()
	name := obj.GetName()
	if ns == "" {
		return name
	}
	return ns + "/" + name
}
