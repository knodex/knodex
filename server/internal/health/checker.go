package health

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker/v2"
	"k8s.io/client-go/kubernetes"

	"github.com/provops-org/knodex/server/internal/resilience"
)

// RGDWatcherHealth is an interface for checking RGD watcher health
// This avoids circular imports with the watcher package
type RGDWatcherHealth interface {
	IsRunning() bool
	IsSynced() bool
}

// RBACHealth is an interface for checking RBAC policy sync status.
// Returns false until the initial policy sync completes, causing /readyz
// to return 503 so Kubernetes doesn't route traffic before policies are loaded.
type RBACHealth interface {
	IsPolicySynced() bool
}

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"

	// healthCheckTimeout is the max time for individual component health checks.
	// 2s is aggressive to keep readiness probes fast; failures trigger circuit breakers.
	healthCheckTimeout = 2 * time.Second
)

// ComponentHealth represents the health status of a single component
type ComponentHealth struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

// HealthStatus represents the overall health status
type HealthStatus struct {
	Status     Status            `json:"status"`
	Components []ComponentHealth `json:"components,omitempty"`
}

// Checker performs health checks on system components
type Checker struct {
	redisClient     *redis.Client
	k8sClient       kubernetes.Interface
	rgdWatcher      RGDWatcherHealth
	rbacHealth      RBACHealth
	circuitBreakers *resilience.CircuitBreakers
	mu              sync.RWMutex

	// Cached status for liveness (updated in background)
	lastLivenessCheck time.Time
	livenessStatus    Status
}

// NewChecker creates a new health checker
func NewChecker(redisClient *redis.Client, k8sClient kubernetes.Interface, rgdWatcher RGDWatcherHealth) *Checker {
	return &Checker{
		redisClient:    redisClient,
		k8sClient:      k8sClient,
		rgdWatcher:     rgdWatcher,
		livenessStatus: StatusHealthy,
	}
}

// SetCircuitBreakers attaches circuit breakers for dependency health reporting
func (c *Checker) SetCircuitBreakers(cb *resilience.CircuitBreakers) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.circuitBreakers = cb
}

// SetRBACHealth attaches an RBAC health checker for readiness reporting.
// When set, /readyz returns 503 until the initial policy sync completes.
func (c *Checker) SetRBACHealth(rh RBACHealth) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rbacHealth = rh
}

// CircuitBreakers returns the attached circuit breakers (may be nil)
func (c *Checker) CircuitBreakers() *resilience.CircuitBreakers {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.circuitBreakers
}

// CheckLiveness performs a quick liveness check
// Returns true if the application is alive and should not be restarted
func (c *Checker) CheckLiveness(ctx context.Context) *HealthStatus {
	// Liveness is always healthy unless there's a critical internal failure
	// We don't check external dependencies for liveness
	return &HealthStatus{
		Status: StatusHealthy,
	}
}

// CheckReadiness performs a readiness check
// Returns true if the application is ready to receive traffic
func (c *Checker) CheckReadiness(ctx context.Context) *HealthStatus {
	var components []ComponentHealth
	overallStatus := StatusHealthy

	// Check Redis connectivity (if client is configured)
	if c.redisClient != nil {
		redisHealth := c.checkRedis(ctx)
		components = append(components, redisHealth)
		if redisHealth.Status != StatusHealthy {
			overallStatus = StatusUnhealthy
		}
	}

	// Check Kubernetes API connectivity (if client is configured)
	if c.k8sClient != nil {
		k8sHealth := c.checkKubernetes(ctx)
		components = append(components, k8sHealth)
		if k8sHealth.Status != StatusHealthy {
			overallStatus = StatusUnhealthy
		}
	}

	// Check RGD watcher status (if configured)
	if c.rgdWatcher != nil {
		watcherHealth := c.checkRGDWatcher()
		components = append(components, watcherHealth)
		// Watcher not being synced is degraded, not unhealthy
		if watcherHealth.Status == StatusUnhealthy {
			if overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		}
	}

	// Check RBAC policy sync status (if configured)
	c.mu.RLock()
	rh := c.rbacHealth
	c.mu.RUnlock()

	if rh != nil {
		rbacComponent := c.checkRBAC(rh)
		components = append(components, rbacComponent)
		if rbacComponent.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		}
	}

	// Include circuit breaker states if available
	c.mu.RLock()
	cb := c.circuitBreakers
	c.mu.RUnlock()

	if cb != nil {
		cbComponents := c.checkCircuitBreakers(cb)
		components = append(components, cbComponents...)
		for _, comp := range cbComponents {
			if comp.Status == StatusUnhealthy && overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		}
	}

	return &HealthStatus{
		Status:     overallStatus,
		Components: components,
	}
}

// checkCircuitBreakers reports the state of circuit breakers as health components
func (c *Checker) checkCircuitBreakers(cb *resilience.CircuitBreakers) []ComponentHealth {
	var components []ComponentHealth

	names := []resilience.CircuitBreakerName{resilience.CBKubernetesAPI, resilience.CBRedis}
	for _, name := range names {
		status := StatusHealthy
		msg := "circuit closed"

		switch cb.State(name) {
		case gobreaker.StateOpen:
			status = StatusUnhealthy
			counts := cb.Counts(name)
			msg = fmt.Sprintf("circuit open (consecutive failures: %d)", counts.ConsecutiveFailures)
		case gobreaker.StateHalfOpen:
			status = StatusDegraded
			msg = "circuit half-open (testing recovery)"
		}

		components = append(components, ComponentHealth{
			Name:    fmt.Sprintf("cb-%s", name),
			Status:  status,
			Message: msg,
		})
	}

	return components
}

// checkRBAC checks RBAC policy sync status
func (c *Checker) checkRBAC(rh RBACHealth) ComponentHealth {
	if !rh.IsPolicySynced() {
		return ComponentHealth{
			Name:    "rbac",
			Status:  StatusUnhealthy,
			Message: "initial policy sync in progress",
		}
	}

	return ComponentHealth{
		Name:    "rbac",
		Status:  StatusHealthy,
		Message: "policies synced",
	}
}

// checkRedis checks Redis connectivity
func (c *Checker) checkRedis(ctx context.Context) ComponentHealth {
	checkCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	if err := c.redisClient.Ping(checkCtx).Err(); err != nil {
		slog.Warn("redis health check failed", "error", err)
		return ComponentHealth{
			Name:    "redis",
			Status:  StatusUnhealthy,
			Message: err.Error(),
		}
	}

	return ComponentHealth{
		Name:   "redis",
		Status: StatusHealthy,
	}
}

// checkKubernetes checks Kubernetes API connectivity
func (c *Checker) checkKubernetes(ctx context.Context) ComponentHealth {
	checkCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	// Use server version as a lightweight health check
	_, err := c.k8sClient.Discovery().ServerVersion()
	if err != nil {
		slog.Warn("kubernetes health check failed", "error", err)
		return ComponentHealth{
			Name:    "kubernetes",
			Status:  StatusUnhealthy,
			Message: err.Error(),
		}
	}

	// Use the context to check for cancellation
	select {
	case <-checkCtx.Done():
		return ComponentHealth{
			Name:    "kubernetes",
			Status:  StatusUnhealthy,
			Message: "health check timeout",
		}
	default:
	}

	return ComponentHealth{
		Name:   "kubernetes",
		Status: StatusHealthy,
	}
}

// checkRGDWatcher checks RGD watcher status
func (c *Checker) checkRGDWatcher() ComponentHealth {
	running := c.rgdWatcher.IsRunning()
	synced := c.rgdWatcher.IsSynced()

	if !running {
		return ComponentHealth{
			Name:    "rgd-watcher",
			Status:  StatusUnhealthy,
			Message: "watcher not running",
		}
	}

	if !synced {
		return ComponentHealth{
			Name:    "rgd-watcher",
			Status:  StatusDegraded,
			Message: "initial sync in progress",
		}
	}

	return ComponentHealth{
		Name:    "rgd-watcher",
		Status:  StatusHealthy,
		Message: fmt.Sprintf("running and synced"),
	}
}
