// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package kagent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// fakeDiscoveryWithKagent returns a fake discovery seeded with the
// kagent.dev/v1alpha2 APIResourceList containing the agents resource.
func fakeDiscoveryWithKagent(t *testing.T) ResourceDiscovery {
	t.Helper()
	clientset := k8sfake.NewSimpleClientset()
	disc, ok := clientset.Discovery().(*fakediscovery.FakeDiscovery)
	require.True(t, ok, "fake clientset discovery should be *fakediscovery.FakeDiscovery")
	disc.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: AgentGroupVersion,
			APIResources: []metav1.APIResource{
				{Name: AgentResourceName, Kind: "Agent", Namespaced: true},
			},
		},
	}
	return disc
}

// fakeDiscoveryEmpty returns a fake discovery with NO kagent group — the
// fake returns a NotFound StatusError for unknown group/versions, which the
// checker must classify as definitive absence.
func fakeDiscoveryEmpty(t *testing.T) ResourceDiscovery {
	t.Helper()
	clientset := k8sfake.NewSimpleClientset()
	disc, ok := clientset.Discovery().(*fakediscovery.FakeDiscovery)
	require.True(t, ok, "fake clientset discovery should be *fakediscovery.FakeDiscovery")
	return disc
}

// stubDiscovery lets tests inject arbitrary discovery results and count calls.
type stubDiscovery struct {
	calls atomic.Int64
	list  *metav1.APIResourceList
	err   error
}

func (s *stubDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	s.calls.Add(1)
	return s.list, s.err
}

// healthServer spins up an httptest server answering GET /health with the
// given status code and counts hits.
func healthServer(t *testing.T, statusCode int) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			hits.Add(1)
			w.WriteHeader(statusCode)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestChecker_Check_Ready(t *testing.T) {
	t.Parallel()
	srv, hits := healthServer(t, http.StatusOK)
	c := NewChecker(fakeDiscoveryWithKagent(t), srv.URL)

	result := c.Check(context.Background())

	assert.Equal(t, StatusReady, result.Status)
	require.NotNil(t, result.CRDPresent)
	assert.True(t, *result.CRDPresent)
	require.NotNil(t, result.ControllerHealthy)
	assert.True(t, *result.ControllerHealthy)
	assert.Equal(t, int64(1), hits.Load())
}

func TestChecker_Check_CRDAbsent_NotInstalled_ShortCircuits(t *testing.T) {
	t.Parallel()
	srv, hits := healthServer(t, http.StatusOK)
	c := NewChecker(fakeDiscoveryEmpty(t), srv.URL)

	result := c.Check(context.Background())

	assert.Equal(t, StatusNotInstalled, result.Status)
	require.NotNil(t, result.CRDPresent)
	assert.False(t, *result.CRDPresent)
	assert.Nil(t, result.ControllerHealthy, "health check must not be performed when CRD is absent")
	assert.Equal(t, int64(0), hits.Load(), "health endpoint must never be called when CRD is absent")
	assert.NotEmpty(t, result.Message)
}

func TestChecker_Check_GroupPresentButNoAgentsResource_NotInstalled(t *testing.T) {
	t.Parallel()
	srv, hits := healthServer(t, http.StatusOK)
	disc := &stubDiscovery{list: &metav1.APIResourceList{
		GroupVersion: AgentGroupVersion,
		APIResources: []metav1.APIResource{{Name: "other-resource"}},
	}}
	c := NewChecker(disc, srv.URL)

	result := c.Check(context.Background())

	assert.Equal(t, StatusNotInstalled, result.Status)
	require.NotNil(t, result.CRDPresent)
	assert.False(t, *result.CRDPresent)
	assert.Equal(t, int64(0), hits.Load())
}

func TestChecker_Check_HealthNon200_NotInstalled(t *testing.T) {
	t.Parallel()
	srv, hits := healthServer(t, http.StatusServiceUnavailable)
	c := NewChecker(fakeDiscoveryWithKagent(t), srv.URL)

	result := c.Check(context.Background())

	assert.Equal(t, StatusNotInstalled, result.Status)
	require.NotNil(t, result.CRDPresent)
	assert.True(t, *result.CRDPresent)
	require.NotNil(t, result.ControllerHealthy)
	assert.False(t, *result.ControllerHealthy)
	assert.Equal(t, int64(1), hits.Load())
}

func TestChecker_Check_DiscoveryTransientError_Degraded(t *testing.T) {
	t.Parallel()
	srv, hits := healthServer(t, http.StatusOK)
	disc := &stubDiscovery{err: errors.New("connection reset by peer")}
	c := NewChecker(disc, srv.URL)

	result := c.Check(context.Background())

	assert.Equal(t, StatusDegraded, result.Status)
	assert.Nil(t, result.CRDPresent, "indeterminate CRD check must surface as nil")
	assert.Nil(t, result.ControllerHealthy)
	assert.Equal(t, int64(0), hits.Load())
	assert.Contains(t, result.Message, "discovery")
}

func TestChecker_Check_HealthTransportError_Degraded(t *testing.T) {
	t.Parallel()
	// Closed server → connection refused (transport error, indeterminate).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	c := NewChecker(fakeDiscoveryWithKagent(t), url)

	result := c.Check(context.Background())

	assert.Equal(t, StatusDegraded, result.Status)
	require.NotNil(t, result.CRDPresent)
	assert.True(t, *result.CRDPresent)
	assert.Nil(t, result.ControllerHealthy, "indeterminate health check must surface as nil")
	assert.Contains(t, result.Message, "health")
}

func TestChecker_Check_HealthTimeout_Degraded(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	c := NewChecker(fakeDiscoveryWithKagent(t), srv.URL)
	// Shrink the client timeout so the test stays fast (in-package access).
	c.httpClient.Timeout = 20 * time.Millisecond

	result := c.Check(context.Background())

	assert.Equal(t, StatusDegraded, result.Status)
	assert.Nil(t, result.ControllerHealthy)
}

func TestChecker_Check_CacheServesWithinTTL(t *testing.T) {
	t.Parallel()
	srv, hits := healthServer(t, http.StatusOK)
	disc := &stubDiscovery{list: &metav1.APIResourceList{
		GroupVersion: AgentGroupVersion,
		APIResources: []metav1.APIResource{{Name: AgentResourceName}},
	}}
	c := NewChecker(disc, srv.URL)

	first := c.Check(context.Background())
	second := c.Check(context.Background())

	assert.Equal(t, StatusReady, first.Status)
	assert.Equal(t, first, second)
	assert.Equal(t, int64(1), disc.calls.Load(), "discovery must be called once within TTL")
	assert.Equal(t, int64(1), hits.Load(), "health endpoint must be called once within TTL")
}

func TestChecker_Check_DegradedNotCached_RetryRerunsChecks(t *testing.T) {
	t.Parallel()
	srv, hits := healthServer(t, http.StatusOK)
	disc := &stubDiscovery{err: errors.New("transient discovery error")}
	c := NewChecker(disc, srv.URL)

	first := c.Check(context.Background())
	require.Equal(t, StatusDegraded, first.Status)

	// Discovery recovers; a retry within the TTL must re-run the checks
	// instead of serving the degraded result from cache (AC #3 retry).
	disc.list = &metav1.APIResourceList{
		GroupVersion: AgentGroupVersion,
		APIResources: []metav1.APIResource{{Name: AgentResourceName}},
	}
	disc.err = nil
	second := c.Check(context.Background())

	assert.Equal(t, StatusReady, second.Status)
	assert.Equal(t, int64(2), disc.calls.Load(), "degraded result must not be cached")
	assert.Equal(t, int64(1), hits.Load())
}

func TestChecker_Check_CacheExpires(t *testing.T) {
	t.Parallel()
	srv, hits := healthServer(t, http.StatusOK)
	disc := &stubDiscovery{list: &metav1.APIResourceList{
		GroupVersion: AgentGroupVersion,
		APIResources: []metav1.APIResource{{Name: AgentResourceName}},
	}}
	c := NewChecker(disc, srv.URL)
	c.cacheTTL = 1 * time.Millisecond // in-package access for fast expiry

	_ = c.Check(context.Background())
	time.Sleep(5 * time.Millisecond)
	_ = c.Check(context.Background())

	assert.Equal(t, int64(2), disc.calls.Load(), "discovery must re-run after TTL expiry")
	assert.Equal(t, int64(2), hits.Load())
}
