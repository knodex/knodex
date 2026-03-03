package testutil

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// NewFakeDynamicClient creates a fake dynamic K8s client with an empty scheme.
func NewFakeDynamicClient(t *testing.T) *fake.FakeDynamicClient {
	t.Helper()
	scheme := runtime.NewScheme()
	return fake.NewSimpleDynamicClient(scheme)
}

// NewFakeDynamicClientWithListKinds creates a fake dynamic client with
// custom GVR-to-ListKind mappings (required for CRD List operations).
func NewFakeDynamicClientWithListKinds(t *testing.T, gvrToListKind map[schema.GroupVersionResource]string) *fake.FakeDynamicClient {
	t.Helper()
	scheme := runtime.NewScheme()
	return fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
}

// NewFakeClientset creates a fake typed K8s clientset.
func NewFakeClientset(t *testing.T) *k8sfake.Clientset {
	t.Helper()
	return k8sfake.NewSimpleClientset()
}
