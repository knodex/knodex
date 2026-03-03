package parser

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NewUnstructured creates a new unstructured Kubernetes object with the
// specified apiVersion, kind, namespace, and name.
func NewUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}

	if namespace != "" {
		obj.SetNamespace(namespace)
	}

	return obj
}

// NewClusterScopedUnstructured creates a new unstructured Kubernetes object
// without a namespace (for cluster-scoped resources).
func NewClusterScopedUnstructured(apiVersion, kind, name string) *unstructured.Unstructured {
	return NewUnstructured(apiVersion, kind, "", name)
}

// SetSpec sets the spec field of an unstructured object.
// If the object is nil, this is a no-op.
func SetSpec(obj *unstructured.Unstructured, spec map[string]interface{}) {
	if obj == nil || spec == nil {
		return
	}
	if err := unstructured.SetNestedField(obj.Object, spec, "spec"); err != nil {
		// This should never fail for a valid unstructured object
		return
	}
}

// SetStatus sets the status field of an unstructured object.
// If the object is nil, this is a no-op.
func SetStatus(obj *unstructured.Unstructured, status map[string]interface{}) {
	if obj == nil || status == nil {
		return
	}
	if err := unstructured.SetNestedField(obj.Object, status, "status"); err != nil {
		return
	}
}

// SetLabels sets the labels on an unstructured object, replacing any existing labels.
// If the object is nil, this is a no-op.
func SetLabels(obj *unstructured.Unstructured, labels map[string]string) {
	if obj == nil {
		return
	}
	obj.SetLabels(labels)
}

// SetAnnotations sets the annotations on an unstructured object, replacing any existing annotations.
// If the object is nil, this is a no-op.
func SetAnnotations(obj *unstructured.Unstructured, annotations map[string]string) {
	if obj == nil {
		return
	}
	obj.SetAnnotations(annotations)
}

// MergeLabels merges the provided labels into the object's existing labels.
// Existing labels with the same keys will be overwritten.
// If the object is nil, this is a no-op.
func MergeLabels(obj *unstructured.Unstructured, labels map[string]string) {
	if obj == nil || len(labels) == 0 {
		return
	}

	existing := obj.GetLabels()
	if existing == nil {
		existing = make(map[string]string)
	}

	for k, v := range labels {
		existing[k] = v
	}

	obj.SetLabels(existing)
}

// MergeAnnotations merges the provided annotations into the object's existing annotations.
// Existing annotations with the same keys will be overwritten.
// If the object is nil, this is a no-op.
func MergeAnnotations(obj *unstructured.Unstructured, annotations map[string]string) {
	if obj == nil || len(annotations) == 0 {
		return
	}

	existing := obj.GetAnnotations()
	if existing == nil {
		existing = make(map[string]string)
	}

	for k, v := range annotations {
		existing[k] = v
	}

	obj.SetAnnotations(existing)
}

// SetLabel sets a single label on an unstructured object.
// If the object is nil, this is a no-op.
func SetLabel(obj *unstructured.Unstructured, key, value string) {
	MergeLabels(obj, map[string]string{key: value})
}

// SetAnnotation sets a single annotation on an unstructured object.
// If the object is nil, this is a no-op.
func SetAnnotation(obj *unstructured.Unstructured, key, value string) {
	MergeAnnotations(obj, map[string]string{key: value})
}

// RemoveLabel removes a label from an unstructured object.
// If the object is nil or the label doesn't exist, this is a no-op.
func RemoveLabel(obj *unstructured.Unstructured, key string) {
	if obj == nil {
		return
	}

	labels := obj.GetLabels()
	if labels == nil {
		return
	}

	delete(labels, key)
	obj.SetLabels(labels)
}

// RemoveAnnotation removes an annotation from an unstructured object.
// If the object is nil or the annotation doesn't exist, this is a no-op.
func RemoveAnnotation(obj *unstructured.Unstructured, key string) {
	if obj == nil {
		return
	}

	annotations := obj.GetAnnotations()
	if annotations == nil {
		return
	}

	delete(annotations, key)
	obj.SetAnnotations(annotations)
}

// SetFinalizers sets the finalizers on an unstructured object.
// If the object is nil, this is a no-op.
func SetFinalizers(obj *unstructured.Unstructured, finalizers []string) {
	if obj == nil {
		return
	}
	obj.SetFinalizers(finalizers)
}

// AddFinalizer adds a finalizer to an unstructured object if it doesn't already exist.
// If the object is nil, this is a no-op.
func AddFinalizer(obj *unstructured.Unstructured, finalizer string) {
	if obj == nil || finalizer == "" {
		return
	}

	// Check if finalizer already exists
	if HasFinalizer(obj, finalizer) {
		return
	}

	finalizers := GetFinalizers(obj)
	obj.SetFinalizers(append(finalizers, finalizer))
}

// RemoveFinalizer removes a finalizer from an unstructured object.
// If the object is nil or the finalizer doesn't exist, this is a no-op.
func RemoveFinalizer(obj *unstructured.Unstructured, finalizer string) {
	if obj == nil || finalizer == "" {
		return
	}

	finalizers := GetFinalizers(obj)
	newFinalizers := make([]string, 0, len(finalizers))

	for _, f := range finalizers {
		if f != finalizer {
			newFinalizers = append(newFinalizers, f)
		}
	}

	obj.SetFinalizers(newFinalizers)
}

// SetNestedField sets a field at the specified path in an unstructured object.
// Creates intermediate maps as needed.
// If the object is nil, this is a no-op and returns an error.
func SetNestedField(obj *unstructured.Unstructured, value interface{}, path ...string) error {
	if obj == nil {
		return newPathError("SetNestedField", path, "", "", ErrNilObject)
	}
	if len(path) == 0 {
		return newPathError("SetNestedField", path, "", "", ErrEmptyPath)
	}
	return unstructured.SetNestedField(obj.Object, value, path...)
}

// SetNestedStringMap sets a string map at the specified path in an unstructured object.
// If the object is nil, this is a no-op and returns an error.
func SetNestedStringMap(obj *unstructured.Unstructured, value map[string]string, path ...string) error {
	if obj == nil {
		return newPathError("SetNestedStringMap", path, "", "", ErrNilObject)
	}
	if len(path) == 0 {
		return newPathError("SetNestedStringMap", path, "", "", ErrEmptyPath)
	}
	return unstructured.SetNestedStringMap(obj.Object, value, path...)
}

// SetNestedSlice sets a slice at the specified path in an unstructured object.
// If the object is nil, this is a no-op and returns an error.
func SetNestedSlice(obj *unstructured.Unstructured, value []interface{}, path ...string) error {
	if obj == nil {
		return newPathError("SetNestedSlice", path, "", "", ErrNilObject)
	}
	if len(path) == 0 {
		return newPathError("SetNestedSlice", path, "", "", ErrEmptyPath)
	}
	return unstructured.SetNestedSlice(obj.Object, value, path...)
}

// Clone creates a deep copy of an unstructured object.
// Returns nil if the input is nil.
func Clone(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}
	return obj.DeepCopy()
}

// WithSpec returns a copy of the object with the spec replaced.
// Returns nil if the input is nil.
func WithSpec(obj *unstructured.Unstructured, spec map[string]interface{}) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}
	clone := Clone(obj)
	SetSpec(clone, spec)
	return clone
}

// WithLabels returns a copy of the object with the labels replaced.
// Returns nil if the input is nil.
func WithLabels(obj *unstructured.Unstructured, labels map[string]string) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}
	clone := Clone(obj)
	SetLabels(clone, labels)
	return clone
}

// WithAnnotations returns a copy of the object with the annotations replaced.
// Returns nil if the input is nil.
func WithAnnotations(obj *unstructured.Unstructured, annotations map[string]string) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}
	clone := Clone(obj)
	SetAnnotations(clone, annotations)
	return clone
}
