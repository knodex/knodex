// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package kagent integrates Knodex with the kagent AI agent operator
// (https://kagent.dev). This package currently provides presence detection
// (Story 49.1); the A2A invocation client lands here in Epic 50.
package kagent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AgentGroupVersion is the kagent Agent CRD group/version.
	AgentGroupVersion = "kagent.dev/v1alpha2"
	// AgentResourceName is the plural resource name of the kagent Agent CRD
	// (agents.kagent.dev).
	AgentResourceName = "agents"

	// healthCheckTimeout bounds the controller /health call so a hung kagent
	// service cannot stall the hub (NFR-A1).
	healthCheckTimeout = 3 * time.Second
	// healthDialTimeout is the TCP dial timeout — kept below healthCheckTimeout
	// to leave headroom for the request itself (mirrors auth/oidc.go).
	healthDialTimeout = 2 * time.Second

	// cacheTTL is how long a presence result is served from memory before the
	// discovery + health checks re-run. Keeps repeated hub loads at memory
	// speed (NFR-A1: <500ms nominal).
	cacheTTL = 15 * time.Second
)

// Status is the tri-state kagent presence outcome.
type Status string

const (
	// StatusReady means the Agent CRD exists AND the controller /health returned 200.
	StatusReady Status = "ready"
	// StatusNotInstalled means a check answered definitively negative:
	// the CRD is absent, or the controller responded non-200.
	StatusNotInstalled Status = "not_installed"
	// StatusDegraded means a check errored without a definitive answer
	// (discovery API error, health transport error/timeout).
	StatusDegraded Status = "degraded"
)

// Result is the outcome of a presence check. Nil pointer fields mean the
// corresponding check was not performed or was indeterminate.
type Result struct {
	Status            Status `json:"status"`
	CRDPresent        *bool  `json:"crdPresent"`
	ControllerHealthy *bool  `json:"controllerHealthy"`
	Message           string `json:"message"`
}

// ResourceDiscovery is the narrow slice of discovery.DiscoveryInterface the
// checker needs. kubernetes.Interface.Discovery() satisfies it.
type ResourceDiscovery interface {
	ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error)
}

// Checker detects kagent presence: (1) the agents.kagent.dev CRD exists in
// discovery, and (2) the kagent controller /health endpoint returns 200.
// Results are cached in memory for cacheTTL.
type Checker struct {
	discovery  ResourceDiscovery
	httpClient *http.Client
	baseURL    string

	mu       sync.Mutex
	cached   *Result
	expires  time.Time
	cacheTTL time.Duration
}

// NewChecker creates a presence checker. discovery must be non-nil (callers
// without a Kubernetes client should not construct a Checker — the handler
// reports degraded instead). baseURL is the kagent controller REST base, e.g.
// http://kagent-controller.kagent.svc.cluster.local:8083.
func NewChecker(discovery ResourceDiscovery, baseURL string) *Checker {
	return &Checker{
		discovery: discovery,
		baseURL:   baseURL,
		httpClient: &http.Client{
			Timeout: healthCheckTimeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: healthDialTimeout,
				}).DialContext,
				TLSHandshakeTimeout: healthDialTimeout,
				MaxIdleConns:        2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		cacheTTL: cacheTTL,
	}
}

// Check runs the presence checks, serving a cached result within the TTL.
// Degraded (indeterminate) results are never cached, so a retry always
// re-runs the checks.
func (c *Checker) Check(ctx context.Context) Result {
	c.mu.Lock()
	if c.cached != nil && time.Now().Before(c.expires) {
		cached := *c.cached
		c.mu.Unlock()
		return cached
	}
	c.mu.Unlock()

	result := c.check(ctx)

	// Only cache definitive answers. A degraded result is indeterminate —
	// caching it would make the UI's Retry action a no-op for the TTL, and a
	// request canceled mid-check (transport error from ctx cancellation) would
	// poison the cache for every user.
	if result.Status != StatusDegraded {
		c.mu.Lock()
		c.cached = &result
		c.expires = time.Now().Add(c.cacheTTL)
		c.mu.Unlock()
	}

	return result
}

// check performs the two presence checks without caching.
// CRD check runs first and short-circuits: when kagent isn't installed at
// all, the health call (which would DNS-fail) never runs.
func (c *Checker) check(ctx context.Context) Result {
	crdPresent, err := c.checkCRD()
	if err != nil {
		// Indeterminate: discovery API errored without a definitive answer.
		return Result{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("kagent CRD discovery failed: %v", err),
		}
	}
	if !crdPresent {
		f := false
		return Result{
			Status:     StatusNotInstalled,
			CRDPresent: &f,
			Message:    "kagent Agent CRD (agents.kagent.dev) not found in cluster",
		}
	}

	tr := true
	healthy, err := c.checkHealth(ctx)
	if err != nil {
		// Indeterminate: transport error (timeout, conn refused, DNS).
		return Result{
			Status:     StatusDegraded,
			CRDPresent: &tr,
			Message:    fmt.Sprintf("kagent controller health check failed: %v", err),
		}
	}
	if !healthy {
		f := false
		return Result{
			Status:            StatusNotInstalled,
			CRDPresent:        &tr,
			ControllerHealthy: &f,
			Message:           "kagent controller responded unhealthy to /health",
		}
	}

	return Result{
		Status:            StatusReady,
		CRDPresent:        &tr,
		ControllerHealthy: &tr,
		Message:           "kagent is installed and healthy",
	}
}

// checkCRD reports whether the kagent Agent CRD is registered in the cluster.
// A discovery NotFound is a definitive absence (false, nil); any other error
// is indeterminate (false, err). Mirrors hasGraphRevisionAPI (app/app.go).
func (c *Checker) checkCRD() (bool, error) {
	resources, err := c.discovery.ServerResourcesForGroupVersion(AgentGroupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	for _, r := range resources.APIResources {
		if r.Name == AgentResourceName {
			return true, nil
		}
	}
	return false, nil
}

// checkHealth reports whether the kagent controller /health returns 200.
// A non-200 response is a definitive unhealthy (false, nil); a transport
// error is indeterminate (false, err).
func (c *Checker) checkHealth(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK, nil
}
