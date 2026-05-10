// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package app provides a composable application container for the Knodex server.
// It extracts initialization, service wiring, and lifecycle management from main()
// into a reusable package that can be imported by both the OSS and EE entry points.
//
// Usage (OSS):
//
//	cfg, _ := config.Load()
//	a := app.New(cfg)
//	a.Run(context.Background())
//
// Usage (EE overlay):
//
//	cfg, _ := config.Load()
//	a := app.New(cfg)
//	a.SetLicenseService(license.NewService(...))
//	a.SetComplianceService(compliance.NewService(...))
//	a.SetCategoryInitFunc(categories.InitCategoryService)
//	a.Run(context.Background())
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/api"
	"github.com/knodex/knodex/server/internal/api/cookie"
	"github.com/knodex/knodex/server/internal/api/handlers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/bootstrap"
	"github.com/knodex/knodex/server/internal/clients"
	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/drift"
	"github.com/knodex/knodex/server/internal/health"
	"github.com/knodex/knodex/server/internal/history"
	"github.com/knodex/knodex/server/internal/kro"
	krodiff "github.com/knodex/knodex/server/internal/kro/diff"
	kroschema "github.com/knodex/knodex/server/internal/kro/schema"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/repository"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/sso"
	"github.com/knodex/knodex/server/internal/static"
	oldwatcher "github.com/knodex/knodex/server/internal/watcher"
	"github.com/knodex/knodex/server/internal/websocket"
)

// Timeout and duration constants tuned for production Kubernetes environments.
const (
	projectLookupTimeout          = 2 * time.Second
	adminPasswordBootstrapTimeout = 30 * time.Second
	historyRecordTimeout          = 5 * time.Second
	watcherStopTimeout            = 5 * time.Second
	watcherSyncTimeout            = 30 * time.Second
	watcherSyncPollInterval       = 100 * time.Millisecond
	initialPolicySyncTimeout      = 30 * time.Second
	gracefulShutdownTimeout       = 30 * time.Second
	httpServerReadTimeout         = 15 * time.Second
	httpServerReadHeaderTimeout   = 5 * time.Second
	httpServerWriteTimeout        = 15 * time.Second
	httpServerIdleTimeout         = 60 * time.Second
)

// ComplianceInitFunc is the signature for enterprise compliance service initialization.
// Used during the monorepo period to bridge build-tag dispatch files with the app package.
type ComplianceInitFunc func(ctx context.Context, k8sCfg *config.Kubernetes, wsHub *websocket.Hub, redisClient *redis.Client, complianceCfg *config.Compliance) services.ComplianceService

// ViolationHistoryInitFunc is the signature for enterprise violation history service initialization.
type ViolationHistoryInitFunc func() services.ViolationHistoryService

// CategoryInitFunc is the signature for OSS category service initialization.
// Categories are auto-discovered from knodex.io/category RGD annotations — no configPath needed.
type CategoryInitFunc func(rgdWatcher *watcher.RGDWatcher) services.CategoryService

// AuditRecorderInitFunc is the signature for enterprise audit recorder initialization.
// Used during the monorepo period to bridge build-tag dispatch files with the app package.
type AuditRecorderInitFunc func(ctx context.Context, redisClient *redis.Client, k8sClient kubernetes.Interface, namespace string) audit.Recorder

// AuditLoginMiddlewareInitFunc is the signature for enterprise audit login middleware initialization.
// In EE builds, this creates an AuditService + AuditConfigWatcher and returns the login middleware.
// Returns nil in OSS builds (login routes are not wrapped with audit middleware).
type AuditLoginMiddlewareInitFunc func(ctx context.Context, redisClient *redis.Client, k8sClient kubernetes.Interface, namespace string) func(http.Handler) http.Handler

// AuditMiddlewareInitFunc is the signature for enterprise audit middleware initialization.
// This middleware captures 401/403 responses and records audit events for authentication
// failures and authorization denials.
type AuditMiddlewareInitFunc func(ctx context.Context, redisClient *redis.Client, k8sClient kubernetes.Interface, namespace string) func(http.Handler) http.Handler

// AuditAPIServiceInitFunc is the signature for enterprise audit API service initialization.
// Used during the monorepo period to bridge build-tag dispatch files with the app package.
// The ctx parameter controls the config watcher lifecycle — cancel it to stop the watcher.
// The recorder parameter enables audit event recording for config changes (FR-AT6).
type AuditAPIServiceInitFunc func(ctx context.Context, redisClient *redis.Client, k8sClient kubernetes.Interface, namespace string, enforcer rbac.PolicyEnforcer, recorder audit.Recorder) services.AuditAPIService

// App is the composable application container for the Knodex server.
// Create with New(), configure enterprise services via setters, then call Run().
type App struct {
	cfg *config.Config

	// Enterprise services (set via setters before Run)
	licenseService          services.LicenseService
	complianceService       services.ComplianceService
	violationHistoryService services.ViolationHistoryService
	categoryService         services.CategoryService

	// Organization filter for enterprise catalog filtering (empty = no filtering)
	organizationFilter string

	// Init functions (monorepo build-tag dispatch bridge)
	complianceInitFunc           ComplianceInitFunc
	violationHistoryInitFunc     ViolationHistoryInitFunc
	categoryInitFunc             CategoryInitFunc
	auditRecorderInitFunc        AuditRecorderInitFunc
	auditLoginMiddlewareInitFunc AuditLoginMiddlewareInitFunc
	auditMiddlewareInitFunc      AuditMiddlewareInitFunc
	auditAPIServiceInitFunc      AuditAPIServiceInitFunc
}

// New creates a new App with the given configuration.
// Enterprise services default to nil-safe values (NoopLicenseService, nil for others).
// Call setter methods to configure enterprise services before calling Run().
func New(cfg *config.Config) *App {
	return &App{
		cfg:            cfg,
		licenseService: &services.NoopLicenseService{},
	}
}

// SetLicenseService sets the enterprise license validation service.
// Must be called before Run(). Defaults to NoopLicenseService if not set.
func (a *App) SetLicenseService(svc services.LicenseService) {
	a.licenseService = svc
}

// SetComplianceInitFunc registers a factory function for creating the compliance service.
// Used during the monorepo period to bridge build-tag dispatch (ee_compliance.go / oss_compliance.go).
// If SetComplianceService was also called, the direct setter takes precedence.
func (a *App) SetComplianceInitFunc(fn ComplianceInitFunc) {
	a.complianceInitFunc = fn
}

// SetViolationHistoryInitFunc registers a factory function for creating the violation history service.
func (a *App) SetViolationHistoryInitFunc(fn ViolationHistoryInitFunc) {
	a.violationHistoryInitFunc = fn
}

// SetCategoryInitFunc registers a factory function for creating the category service.
func (a *App) SetCategoryInitFunc(fn CategoryInitFunc) {
	a.categoryInitFunc = fn
}

// SetAuditRecorderInitFunc registers a factory function for creating the audit recorder.
// In EE builds, this creates an AuditService + RecorderBridge.
// In OSS builds, the factory returns nil (handlers skip audit recording).
func (a *App) SetAuditRecorderInitFunc(fn AuditRecorderInitFunc) {
	a.auditRecorderInitFunc = fn
}

// SetAuditLoginMiddlewareInitFunc registers a factory function for creating the audit login middleware.
// In EE builds, this wraps login routes to record login/login_failed audit events.
// In OSS builds, the factory returns nil (login routes are not wrapped).
func (a *App) SetAuditLoginMiddlewareInitFunc(fn AuditLoginMiddlewareInitFunc) {
	a.auditLoginMiddlewareInitFunc = fn
}

// SetAuditMiddlewareInitFunc registers a factory function for creating the audit middleware.
// In EE builds, this wraps protected routes to record 401/403 audit events.
// In OSS builds, the factory returns nil (no audit middleware applied).
func (a *App) SetAuditMiddlewareInitFunc(fn AuditMiddlewareInitFunc) {
	a.auditMiddlewareInitFunc = fn
}

// SetAuditAPIServiceInitFunc registers a factory function for creating the audit API service.
// In EE builds, this creates an AuditHandler with query and config endpoints.
// In OSS builds, the factory returns nil (routes not registered, 404 returned).
func (a *App) SetAuditAPIServiceInitFunc(fn AuditAPIServiceInitFunc) {
	a.auditAPIServiceInitFunc = fn
}

// SetOrganizationFilter sets the organization filter for enterprise catalog filtering.
// In EE builds, this is set to cfg.Organization. In OSS builds, this is empty (no filtering).
func (a *App) SetOrganizationFilter(org string) {
	a.organizationFilter = org
}

// Run initializes all services, starts the HTTP server, and blocks until shutdown.
// It handles graceful shutdown on SIGINT/SIGTERM.
func (a *App) Run(ctx context.Context) error { //nolint:gocyclo // orchestration function inherently complex
	cfg := a.cfg

	orgSource := "default"
	if _, ok := os.LookupEnv("KNODEX_ORGANIZATION"); ok {
		orgSource = "KNODEX_ORGANIZATION"
	}
	slog.Info("organization identity configured", "organization", cfg.Organization, "source", orgSource)
	slog.Info("organization catalog filter", "active", a.organizationFilter != "", "organization", a.organizationFilter)

	// Initialize clients
	logger := slog.Default()
	redisClient := clients.NewRedisClient(&cfg.Redis, logger)
	k8sClient := clients.NewKubernetesClient(&cfg.Kubernetes, logger)

	// Get Kubernetes REST config and dynamic client
	var dynamicClient dynamic.Interface
	k8sConfig, k8sConfigErr := clients.GetKubernetesConfig(&cfg.Kubernetes)
	if k8sConfigErr == nil {
		var err error
		dynamicClient, err = dynamic.NewForConfig(k8sConfig)
		if err != nil {
			slog.Warn("failed to create dynamic client", "error", err)
		}
	}

	// Initialize RBAC services
	var projectService *rbac.ProjectService
	var namespaceService *rbac.NamespaceService
	var repositoryService *repository.Service
	var permissionService *rbac.PermissionService
	var redisAuthzCache *rbac.RedisAuthorizationCache

	// SSO store and watcher (ConfigMap is single source of truth — no env var fallback)
	var ssoStore *sso.ProviderStore
	var ssoWatcher *sso.SSOWatcher
	var ssoProviders []auth.OIDCProviderConfig

	if k8sClient != nil && dynamicClient != nil {
		// Create audit logger for RBAC operations
		auditLogger := rbac.NewAuditLogger(slog.Default())

		// Create project service scoped to Knodex namespace
		projectService = rbac.NewProjectService(k8sClient, dynamicClient, cfg.KnodexNamespace)
		slog.Info("project service initialized", "namespace", cfg.KnodexNamespace)

		// Create namespace service for listing cluster namespaces matching project policies
		namespaceService = rbac.NewNamespaceService(k8sClient, projectService)
		slog.Info("namespace service initialized")

		// Get namespace for credential secrets storage
		credentialNamespace := cfg.Log.Namespace
		if credentialNamespace == "" {
			credentialNamespace = "default"
		}

		// Create repository service with audit logging
		var repoErr error
		repositoryService, repoErr = repository.NewService(k8sClient, dynamicClient, auditLogger, credentialNamespace)
		if repoErr != nil {
			slog.Error("failed to create repository service", "error", repoErr)
		} else {
			slog.Info("repository service initialized", "credential_namespace", credentialNamespace)
		}

		// Initialize SSO ProviderStore (for UI CRUD operations)
		ssoStore = sso.NewProviderStore(k8sClient, credentialNamespace)

		// Create SSO watcher — watches ConfigMap for changes (single source of truth)
		ssoWatcher = sso.NewSSOWatcher(k8sClient, credentialNamespace, slog.Default())

		// Load initial providers from ConfigMap via ProviderStore
		initialProviders, err := ssoStore.List(context.Background())
		if err != nil {
			slog.Warn("failed to load initial SSO providers from ConfigMap", "error", err)
		} else if len(initialProviders) > 0 {
			ssoProviders = sso.ToAuthConfigs(initialProviders)
			slog.Info("loaded SSO providers from ConfigMap", "count", len(ssoProviders))
		} else {
			slog.Warn("no SSO ConfigMap found, starting with zero OIDC providers")
		}
	}

	// Initialize Casbin policy enforcer for RBAC
	var policyEnforcer rbac.PolicyEnforcer
	var policyCacheManager *rbac.PolicyCacheManager
	var casbinEnforcer *rbac.CasbinEnforcer
	if projectService != nil {
		var err error
		casbinEnforcer, err = rbac.NewCasbinEnforcer()
		if err != nil {
			slog.Error("failed to create casbin enforcer", "error", err)
		} else {
			// Create Redis role store for persisting user-role assignments
			var roleStoreOpt rbac.PolicyEnforcerOption
			if redisClient != nil {
				redisRoleStore := rbac.NewRedisRoleStore(redisClient, cfg.CasbinRoles.TTL, slog.Default())
				roleStoreOpt = rbac.WithRedisRoleStore(redisRoleStore)
				slog.Info("redis role store created for casbin user-role persistence",
					"ttl", cfg.CasbinRoles.TTL.String(),
				)
			}

			// Build PolicyEnforcer options
			var peOpts []rbac.PolicyEnforcerOption
			if roleStoreOpt != nil {
				peOpts = append(peOpts, roleStoreOpt)
			}

			// Create Redis-backed authorization cache for cross-replica consistency.
			// Always uses Redis when available; gracefully degrades to in-memory fallback if Redis fails at runtime.
			if redisClient != nil {
				cacheTTL := time.Duration(cfg.PolicyCache.TTLSeconds) * time.Second
				redisAuthzCache = rbac.NewRedisAuthorizationCache(redisClient, cacheTTL, slog.Default())
				peOpts = append(peOpts, rbac.WithRedisAuthorizationCache(redisAuthzCache))
				slog.Info("redis authorization cache enabled for cross-replica consistency",
					"ttl", cacheTTL.String(),
				)
			}

			policyEnforcer = rbac.NewPolicyEnforcerWithConfig(casbinEnforcer, &projectServiceAdapter{service: projectService}, rbac.DefaultPolicyEnforcerConfig(), peOpts...)

			// Bootstrap pre-configured admin users (e.g., for E2E tests that inject JWTs directly)
			for _, adminUser := range cfg.CasbinRoles.AdminUsers {
				if _, err := casbinEnforcer.AddUserRole(adminUser, rbac.CasbinRoleServerAdmin); err != nil {
					slog.Warn("failed to bootstrap admin user role", "user", adminUser, "error", err)
				} else {
					slog.Info("bootstrapped admin user role", "user", adminUser, "role", rbac.CasbinRoleServerAdmin)
				}
			}

			// Create ProjectWatcher to watch for Project CRD changes
			var projectWatcher rbac.ProjectWatcher
			if dynamicClient != nil {
				policyHandler := &policyHandlerAdapter{
					enforcer:       policyEnforcer,
					projectService: projectService,
				}
				projectWatcher = rbac.NewProjectWatcher(dynamicClient, policyHandler, cfg.KnodexNamespace, rbac.ProjectWatcherConfig{
					ResyncPeriod: rbac.DefaultProjectWatcherResyncPeriod,
					Logger:       slog.Default(),
				})
				slog.Info("project watcher created for OIDC group policy sync")
			}

			// Create PolicySyncService for periodic background sync
			policySyncService := rbac.NewPolicySyncService(policyEnforcer, rbac.PolicySyncConfig{
				SyncInterval: rbac.DefaultPolicySyncInterval,
				Logger:       slog.Default(),
			})

			// Create PolicyCacheManager with watcher and sync service
			policyCacheManager = rbac.NewPolicyCacheManager(policyEnforcer, projectWatcher, policySyncService, slog.Default())
			slog.Info("policy enforcer and cache manager initialized with watcher and sync service")
		}
	}

	// Create permission service with unified cache
	if projectService != nil && policyEnforcer != nil {
		permissionService = rbac.NewPermissionService(rbac.PermissionServiceConfig{
			ProjectService: projectService,
			PolicyEnforcer: policyEnforcer,
			Logger:         slog.Default(),
		})
		slog.Info("permission service initialized with unified cache")
	}

	// Initialize auth service with AccountStore (ArgoCD pattern)
	var authService *auth.Service
	var accountStore *auth.AccountStore
	if k8sClient != nil && projectService != nil && redisClient != nil {
		namespace := cfg.Log.Namespace
		if namespace == "" {
			namespace = "default"
		}

		accountStore = auth.NewAccountStoreWithRedis(k8sClient, namespace, redisClient)
		slog.Info("account store initialized with Redis rate limiting",
			"namespace", namespace,
			"configmap", "knodex-accounts",
			"secret", "knodex-secret",
		)

		// Auto-generate or retrieve admin password from Kubernetes secret.
		// When LOCAL_LOGIN_ENABLED=false, skip bootstrap entirely so no
		// knodex-initial-admin-password Secret is created.
		//
		// When local login is enabled, a bootstrap failure is FATAL — silently
		// proceeding with an empty password would make the auth service
		// indistinguishable from "operator disabled local login", masking the
		// real problem (e.g., RBAC permission missing on the Secret).
		var adminPassword string
		if cfg.Auth.LocalLoginEnabled {
			bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), adminPasswordBootstrapTimeout)
			pw, wasGenerated, err := bootstrap.GetOrCreateAdminPassword(bootstrapCtx, k8sClient, namespace)
			bootstrapCancel()
			if err != nil {
				slog.Error("failed to get or create admin password — refusing to start with degraded local auth",
					"error", err,
					"namespace", namespace,
					"hint", "either fix the underlying error (often a missing Secret RBAC permission) or set server.auth.localLogin.enabled=false",
				)
				os.Exit(1)
			}
			adminPassword = pw
			if wasGenerated {
				slog.Info("auto-generated admin password stored in Kubernetes secret",
					"secret", bootstrap.SecretName,
					"namespace", namespace,
					"retrieval_command", "kubectl get secret "+bootstrap.SecretName+" -n "+namespace+" -o jsonpath='{.data.password}' | base64 -d",
				)
			} else {
				slog.Info("using existing admin password from Kubernetes secret",
					"secret", bootstrap.SecretName,
					"namespace", namespace,
				)
			}
		} else {
			slog.Info("local login disabled via LOCAL_LOGIN_ENABLED=false; skipping admin password bootstrap")
		}

		// Use ConfigMap-sourced SSO providers (no env var fallback)
		oidcProviders := ssoProviders

		authConfig := &auth.Config{
			JWTExpiry:          cfg.Auth.JWTExpiry,
			LocalAdminUsername: cfg.Auth.AdminUsername,
			LocalAdminPassword: adminPassword,
			LocalLoginEnabled:  cfg.Auth.LocalLoginEnabled,
			OIDCEnabled:        cfg.Auth.OIDCEnabled,
			OIDCProviders:      oidcProviders,
		}

		var authErr error
		authService, authErr = auth.NewService(authConfig, accountStore, projectService, k8sClient, redisClient, casbinEnforcer)
		if authErr != nil {
			slog.Error("failed to create auth service - server cannot start without authentication", "error", authErr)
			os.Exit(1)
		}
		slog.Info("auth service initialized",
			"oidc_enabled", cfg.Auth.OIDCEnabled,
			"oidc_providers", len(oidcProviders),
			"casbin_enabled", casbinEnforcer != nil,
		)
	} else if redisClient == nil {
		slog.Warn("Redis client not available, authentication services disabled")
	}

	// Initialize OIDC service (if OIDC is enabled)
	var oidcService *auth.OIDCService
	if cfg.Auth.OIDCEnabled && authService != nil && redisClient != nil && projectService != nil {
		authConfig := &auth.Config{
			JWTExpiry:     cfg.Auth.JWTExpiry,
			OIDCEnabled:   cfg.Auth.OIDCEnabled,
			OIDCProviders: ssoProviders,
		}

		groupMapper := auth.NewGroupMapper(cfg.Auth.OIDCGroupMappings)
		authService.SetGroupMapper(groupMapper)

		oidcProvisioningService := auth.NewOIDCProvisioningService(projectService, groupMapper, casbinEnforcer, cfg.Auth.DefaultRole)

		var err error
		// Pass policyEnforcer as RolePersister: persists OIDC roles to Redis and
		// invalidates stale cached denials, fixing the cold-start permission delay.
		// When policyEnforcer is nil (e.g., no project service), falls back to
		// in-memory-only role assignment (legacy behavior).
		var rolePersister auth.RolePersister
		if policyEnforcer != nil {
			rolePersister = policyEnforcer
		}
		oidcService, err = auth.NewOIDCService(authConfig, redisClient, authService, oidcProvisioningService, casbinEnforcer, rolePersister)
		if err != nil {
			slog.Warn("failed to create OIDC service, OIDC authentication will be unavailable", "error", err)
		} else {
			providers := oidcService.ListProviders()
			slog.Info("OIDC service initialized",
				"providers_count", len(providers),
				"providers", providers,
				"group_mappings", len(cfg.Auth.OIDCGroupMappings),
			)
		}
	} else if cfg.Auth.OIDCEnabled {
		slog.Warn("OIDC enabled but dependencies missing (auth service, redis, or project service)")
	}

	// Register SSO watcher callback to hot-reload OIDC providers on ConfigMap changes
	if ssoWatcher != nil && oidcService != nil {
		ssoWatcher.OnProvidersChanged(func(providers []sso.SSOProvider) {
			authConfigs := sso.ToAuthConfigs(providers)
			if err := oidcService.ReloadProviders(context.Background(), authConfigs); err != nil {
				slog.Error("failed to hot-reload OIDC providers", "error", err)
			} else {
				slog.Info("OIDC providers hot-reloaded via ConfigMap watcher",
					"configured_count", len(providers),
					"active_count", len(oidcService.ListProviders()),
				)
			}
		})
	}

	// Create shared informer factory for all watchers.
	// The factory deduplicates informers: multiple RGDs producing the same GVR
	// share a single watch stream, reducing API server pressure.
	// Resync period is 10m — watch streams deliver real-time events;
	// resync is periodic cache-consistency reconciliation only.
	var instanceFactory dynamicinformer.DynamicSharedInformerFactory
	if dynamicClient != nil {
		instanceFactory = dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 10*time.Minute)
	}

	// Create RGD watcher using the shared dynamic client (QPS=50/Burst=100) and shared factory.
	// Pass restConfig so the watcher can create a KRO graph builder for richer graph data.
	var rgdWatcher *watcher.RGDWatcher
	if dynamicClient != nil && instanceFactory != nil {
		rgdWatcher = watcher.NewRGDWatcher(dynamicClient, instanceFactory, k8sConfig)
		if len(cfg.CatalogPackageFilter) > 0 {
			rgdWatcher.SetPackageFilter(cfg.CatalogPackageFilter)
			slog.Info("catalog package filter active", "packages", cfg.CatalogPackageFilter)
		}
	}

	// Create instance tracker with shared factory
	var instanceTracker *watcher.InstanceTracker
	if rgdWatcher != nil && k8sClient != nil && instanceFactory != nil {
		instanceTracker = watcher.NewInstanceTracker(dynamicClient, k8sClient.Discovery(), instanceFactory, rgdWatcher)
	}

	// Create GraphRevision watcher (feature-gated: only if internal.kro.run API group is available)
	var graphRevisionWatcher *watcher.GraphRevisionWatcher
	if instanceFactory != nil && k8sClient != nil {
		if hasGraphRevisionAPI(k8sClient) {
			graphRevisionWatcher = watcher.NewGraphRevisionWatcher(instanceFactory)
			slog.Info("GraphRevision watcher created (internal.kro.run/v1alpha1 API available)")
		} else {
			slog.Info("GraphRevision watcher skipped (internal.kro.run/v1alpha1 API not available)")
		}
	}

	// Create remote watcher for child cluster resource visibility (STORY-418)
	var remoteWatcher *watcher.RemoteWatcher
	if dynamicClient != nil && k8sClient != nil {
		remoteWatcher = watcher.NewRemoteWatcher(k8sClient, projectService)
		slog.Info("remote watcher created")
	}

	// Create revision diff service (only if GraphRevision watcher is available)
	var diffService *krodiff.DiffService
	if graphRevisionWatcher != nil {
		var diffErr error
		diffService, diffErr = krodiff.NewDiffService()
		if diffErr != nil {
			slog.Warn("failed to create diff service, revision diff API will be unavailable", "error", diffErr)
			diffService = nil
		} else {
			slog.Info("revision diff service initialized")
		}
	}

	// Create health checker
	healthChecker := health.NewChecker(redisClient, k8sClient, rgdWatcher)
	if policyCacheManager != nil {
		healthChecker.SetRBACHealth(policyCacheManager)
	}

	// Create schema extractor for CRD schema extraction
	var schemaExtractor *kroschema.Extractor
	if schemaExt, err := kroschema.NewExtractor(&cfg.Kubernetes); err != nil {
		slog.Warn("failed to create schema extractor, schema endpoint will be unavailable", "error", err)
	} else {
		schemaExtractor = schemaExt
	}

	// Create history service for deployment history tracking
	historyService := history.NewService(redisClient)
	slog.Info("history service initialized")

	// Create WebSocket hub for real-time updates
	wsHub := websocket.NewHub(nil)
	wsHubCtx, wsHubCancel := context.WithCancel(context.Background())
	go wsHub.Run(wsHubCtx)

	// Create WebSocket handler with lifecycle management (pass Redis for ticket-based WS auth)
	wsHandler := handlers.NewWebSocketHandler(wsHub, authService, redisClient)
	if policyEnforcer != nil {
		wsHandler.SetPolicyEnforcer(policyEnforcer)
	}

	// Initialize enterprise services using init functions (monorepo build-tag bridge)
	auditRecorder := a.initEnterpriseServices(cfg, rgdWatcher, wsHub, redisClient, k8sClient)

	// Create repository secret watcher for declarative audit trail.
	// Uses the same credential namespace as the repository service (cfg.Log.Namespace).
	var repoWatcher *oldwatcher.RepositoryWatcher
	if k8sClient != nil {
		credNS := cfg.Log.Namespace
		if credNS == "" {
			credNS = "default"
		}
		repoWatcher = oldwatcher.NewRepositoryWatcher(k8sClient, credNS, auditRecorder)
		slog.Info("repository secret watcher created", "namespace", credNS)
	}

	// Create context for lifecycle management (used by config watchers and other long-lived goroutines)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Audit login middleware: call init func to create enterprise login audit middleware
	// Uses runCtx so the config watcher stops during graceful shutdown.
	var auditLoginMiddleware func(http.Handler) http.Handler
	if a.auditLoginMiddlewareInitFunc != nil {
		namespace := cfg.Log.Namespace
		if namespace == "" {
			namespace = "default"
		}
		auditLoginMiddleware = a.auditLoginMiddlewareInitFunc(runCtx, redisClient, k8sClient, namespace)
	}
	if auditLoginMiddleware != nil {
		slog.Info("audit login middleware initialized (enterprise feature)")
	}

	// Audit middleware: captures 401/403 responses on protected routes for audit trail.
	// Uses runCtx so the config watcher stops during graceful shutdown.
	var auditMiddleware func(http.Handler) http.Handler
	if a.auditMiddlewareInitFunc != nil {
		namespace := cfg.Log.Namespace
		if namespace == "" {
			namespace = "default"
		}
		auditMiddleware = a.auditMiddlewareInitFunc(runCtx, redisClient, k8sClient, namespace)
	}
	if auditMiddleware != nil {
		slog.Info("audit middleware initialized (enterprise feature)")
	}

	// Audit API service: call init func to create enterprise audit API handler
	// Uses runCtx so the config watcher stops during graceful shutdown.
	var auditAPIService services.AuditAPIService
	if a.auditAPIServiceInitFunc != nil {
		namespace := cfg.Log.Namespace
		if namespace == "" {
			namespace = "default"
		}
		auditAPIService = a.auditAPIServiceInitFunc(runCtx, redisClient, k8sClient, namespace, policyEnforcer, auditRecorder)
	}
	if auditAPIService != nil {
		slog.Info("audit API service initialized (enterprise feature)")
	}

	// Create shared drift detection service (used by both CRUD handler and InstanceTracker callback)
	driftSvc := drift.NewService(redisClient, slog.Default(), cfg.Organization)

	// Create API server
	routerResult := api.NewRouterWithConfig(healthChecker, rgdWatcher, instanceTracker, schemaExtractor, api.RouterConfig{
		SPAHandler:              static.SPAHandler(),
		RateLimitRequestsPerMin: cfg.RateLimit.UserRequestsPerMinute,
		RateLimitBurstSize:      cfg.RateLimit.UserBurstSize,
		WebSocketHub:            wsHub,
		WebSocketHandler:        wsHandler,
		AuthService:             authService,
		OIDCService:             oidcService,
		RepositoryService:       repositoryService,
		PermissionService:       permissionService,
		PolicyEnforcer:          policyEnforcer,
		PolicyCacheManager:      policyCacheManager,
		ProjectService:          projectService,
		HistoryService:          historyService,
		NamespaceService:        namespaceService,
		K8sClient:               k8sClient,
		RedisClient:             redisClient,
		LicenseService:          a.licenseService,
		ComplianceService:       a.complianceService,
		ViolationHistoryService: a.violationHistoryService,
		CategoryService:         a.categoryService,
		SSOStore:                ssoStore,
		AllowedRedirectOrigins:  cfg.Auth.AllowedRedirectOrigins,
		CookieConfig:            cookie.Config{Secure: cfg.Cookie.Secure, Domain: cfg.Cookie.Domain},
		OrganizationFilter:      a.organizationFilter, // EE catalog filtering (empty = no filter)
		Organization:            cfg.Organization,     // Display identity for GET /api/v1/settings
		SwaggerEnabled:          cfg.SwaggerEnabled,   // Serve Swagger UI at /swagger/ (SWAGGER_UI_ENABLED)
		AuditRecorder:           auditRecorder,
		AuditLoginMiddleware:    auditLoginMiddleware,
		AuditMiddleware:         auditMiddleware,
		AuditAPIService:         auditAPIService,
		DriftService:            driftSvc,
		GraphRevisionWatcher:    graphRevisionProvider(graphRevisionWatcher),
		DiffService:             diffService,
		RemoteWatcher:           remoteWatcher,
		DynamicClient:           dynamicClient,
	})

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           routerResult.Handler,
		ReadTimeout:       httpServerReadTimeout,
		ReadHeaderTimeout: httpServerReadHeaderTimeout,
		WriteTimeout:      httpServerWriteTimeout,
		IdleTimeout:       httpServerIdleTimeout,
	}

	// Set up callback to broadcast RGD updates via WebSocket
	if rgdWatcher != nil {
		rgdWatcher.SetOnUpdateCallback(func(action watcher.RGDAction, name string, rgd *models.CatalogRGD) {
			var wsAction websocket.Action
			switch action {
			case watcher.RGDActionAdd:
				wsAction = websocket.ActionAdd
			case watcher.RGDActionUpdate:
				wsAction = websocket.ActionUpdate
			case watcher.RGDActionDelete:
				wsAction = websocket.ActionDelete
			default:
				wsAction = websocket.ActionUpdate
			}

			var projectNamespace string
			if rgd != nil {
				projectNamespace = getProjectIDFromNamespace(k8sClient, rgd.Namespace, projectService)
			}

			wsHub.BroadcastRGDUpdate(wsAction, name, rgd, projectNamespace)
		})
	}

	// Invalidate schema cache when RGDs change so deploy pages get fresh schemas
	if rgdWatcher != nil && schemaExtractor != nil {
		rgdWatcher.SetOnUpdateCallback(func(action watcher.RGDAction, name string, rgd *models.CatalogRGD) {
			if action == watcher.RGDActionUpdate || action == watcher.RGDActionDelete {
				var namespace string
				if rgd != nil {
					namespace = rgd.Namespace
				}
				schemaExtractor.InvalidateCache(namespace, name)
			}
		})
	}

	// Synchronous initial policy sync: load all Project CRD policies before
	// accepting traffic to prevent 403 errors during startup.
	if policyCacheManager != nil {
		syncCtx, syncCancel := context.WithTimeout(runCtx, initialPolicySyncTimeout)
		if err := policyEnforcer.SyncPolicies(syncCtx); err != nil {
			slog.Error("initial policy sync failed", "error", err)
		} else {
			slog.Info("initial policy sync completed")
		}
		syncCancel()

		// Restore persisted user-role assignments AFTER policies are loaded
		// (RestorePersistedRoles depends on policies being present)
		if err := policyEnforcer.RestorePersistedRoles(runCtx); err != nil {
			slog.Error("failed to restore persisted user roles from Redis", "error", err)
		}

		// Mark RBAC as ready — /readyz and authz middleware will now allow traffic
		policyCacheManager.MarkSynced()
		slog.Info("RBAC marked as synced (readiness gate open)")

		// Start background watcher + periodic resync
		if err := policyCacheManager.Start(runCtx); err != nil {
			slog.Error("failed to start policy cache manager", "error", err)
		} else {
			slog.Info("policy cache manager started (watcher + background sync)")
		}
	}

	// Start SSO watcher (watches ConfigMap/Secret for OIDC provider changes)
	if ssoWatcher != nil {
		go func() {
			if err := ssoWatcher.Start(runCtx); err != nil {
				slog.Error("SSO watcher stopped with error", "error", err)
			}
		}()
		slog.Info("SSO ConfigMap/Secret watcher started")
	}

	// Start RGD watcher
	if rgdWatcher != nil {
		if err := rgdWatcher.Start(runCtx); err != nil {
			slog.Error("failed to start RGD watcher", "error", err)
		} else {
			slog.Info("RGD watcher started")
		}
	}

	// Start GraphRevision watcher (register WebSocket callback before Start)
	if graphRevisionWatcher != nil {
		// Register diff pre-compute callback BEFORE Start() so it catches all new revisions.
		if diffService != nil {
			localDiffSvc := diffService
			graphRevisionWatcher.SetOnAddCallback(func(rgdName string, revision int) {
				if revision > 1 {
					localDiffSvc.PreComputeConsecutiveDiff(graphRevisionWatcher, rgdName, revision)
				}
			})
		}

		graphRevisionWatcher.SetOnUpdateCallback(func(action string, rgdName string, revision int) {
			var wsAction websocket.Action
			switch action {
			case "add":
				wsAction = websocket.ActionAdd
			case "update":
				wsAction = websocket.ActionUpdate
			case "delete":
				wsAction = websocket.ActionDelete
			default:
				wsAction = websocket.ActionUpdate
			}

			// Resolve project namespace from the RGD watcher
			var projectNamespace string
			if rgdWatcher != nil {
				if rgd, found := rgdWatcher.GetRGDByName(rgdName); found {
					projectNamespace = getProjectIDFromNamespace(k8sClient, rgd.Namespace, projectService)
				}
			}

			wsHub.BroadcastRevisionUpdate(wsAction, rgdName, revision, projectNamespace)
		})

		if err := graphRevisionWatcher.Start(runCtx); err != nil {
			slog.Error("failed to start GraphRevision watcher", "error", err)
		} else {
			slog.Info("GraphRevision watcher started")
		}
	}

	// Start remote watcher for child cluster resources (STORY-418)
	if remoteWatcher != nil {
		if err := remoteWatcher.Start(runCtx); err != nil {
			slog.Error("failed to start remote watcher", "error", err)
		} else {
			slog.Info("remote watcher started")
		}
	}

	// Start repository secret watcher
	if repoWatcher != nil {
		if err := repoWatcher.Start(runCtx); err != nil {
			slog.Error("failed to start repository secret watcher", "error", err)
		} else {
			slog.Info("repository secret watcher started")
		}
	}

	// Start instance tracker (after RGD watcher is running)
	if instanceTracker != nil {
		instanceTracker.SetOnUpdateCallback(
			newInstanceUpdateCallback(historyService, rgdWatcher, instanceTracker, k8sClient, projectService, wsHub),
		)

		// Register GitOps drift reconciliation callback
		// When an instance is updated and its live spec matches the desired spec in Redis,
		// the drift entry is cleared (ArgoCD/Flux has reconciled the change).
		instanceTracker.SetOnUpdateCallback(func(action watcher.InstanceAction, namespace, kind, name string, instance *models.Instance) {
			if action != watcher.InstanceActionUpdate || instance == nil || instance.Spec == nil {
				return
			}
			deployMode := deployment.ParseDeploymentMode(instance.Labels[models.DeploymentModeLabel])
			if deployMode != deployment.ModeGitOps && deployMode != deployment.ModeHybrid {
				return
			}
			cleared := driftSvc.CheckAndClearIfReconciled(context.Background(), namespace, kind, name, instance.Spec)
			if cleared {
				projectNamespace := ""
				if instance.Labels != nil {
					projectNamespace = instance.Labels["knodex.io/project"]
				}
				if projectNamespace == "" {
					projectNamespace = getProjectIDFromNamespace(k8sClient, namespace, projectService)
				}
				wsHub.BroadcastDriftUpdate(namespace, kind, name, false, projectNamespace)
			}
		})

		if err := instanceTracker.Start(runCtx); err != nil {
			slog.Error("failed to start instance tracker", "error", err)
		} else {
			slog.Info("instance tracker started")
		}

		// Wait for both watchers to sync, then update instance counts.
		// Polls IsSynced() instead of a fixed sleep — accurate on fast and slow clusters.
		go func() {
			ticker := time.NewTicker(watcherSyncPollInterval)
			defer ticker.Stop()
			timeout := time.After(watcherSyncTimeout)
			for {
				select {
				case <-ticker.C:
					if instanceTracker.IsSynced() && rgdWatcher.IsSynced() {
						updateAllRGDInstanceCounts(rgdWatcher, instanceTracker)
						slog.Info("initial RGD instance counts synchronized")
						return
					}
				case <-timeout:
					slog.Warn("timed out waiting for watchers to sync, skipping initial count sync")
					return
				case <-runCtx.Done():
					return
				}
			}
		}()
	}

	// Wire WebSocket count push for sidebar badge updates (AC: #2, #3, #4)
	// Uses in-memory caches only - no Redis/HTTP/K8s API calls
	if rgdWatcher != nil && instanceTracker != nil {
		catalogService := routerResult.CatalogService
		countPushFn := func(ctx context.Context, userID string, projects []string, groups []string) (int, int) {
			// RGD count: delegate to CatalogService.GetCount so the same per-RGD Casbin
			// filter that runs on /api/v1/rgds and /api/v1/rgds/count is applied here.
			// Without this, the WebSocket-pushed sidebar badge could exceed the catalog
			// list count for users who pass the project/org cache filter but lack
			// rgds/{category}/{name} get policies.
			var rgdCount int
			if catalogService != nil {
				authCtx := &services.UserAuthContext{
					UserID:             userID,
					Groups:             groups,
					AccessibleProjects: projects,
				}
				// Hub passes projects=nil for global admins (CachedHasGlobalAccess).
				// Mark AccessibleNamespaces=["*"] so any future tier-resolver path treats
				// this as global admin instead of "no projects → universal-only" tiers.
				if projects == nil {
					authCtx.AccessibleNamespaces = []string{"*"}
				}
				count, err := catalogService.GetCount(ctx, authCtx)
				if err != nil {
					slog.Warn("WebSocket RGD count failed; reporting 0", "userID", userID, "error", err)
				} else {
					rgdCount = count
				}
			}

			// Instance count via tracker cache (in-memory).
			// Resolve project names to their destination namespaces — instances live in
			// destination namespaces, not namespaces named after the project.
			namespaces := resolveProjectDestinationNamespaces(ctx, projectService, projects)
			instanceCount := instanceTracker.CountInstancesByNamespaces(namespaces, rbac.MatchNamespaceInList)
			return rgdCount, instanceCount
		}

		// Register on RGD changes for count push
		rgdWatcher.SetOnChangeCallback(func() {
			wsHub.SendCountsToClients(countPushFn)
		})

		// Register on instance changes for count push
		instanceTracker.SetOnChangeCallback(func() {
			wsHub.SendCountsToClients(countPushFn)
		})

		// Set count function for initial-connect push (AC: #1, #6)
		wsHub.SetCountFunc(countPushFn)

		slog.Info("WebSocket count push wired to RGD and instance watchers")
	}

	// Start server in goroutine
	go func() {
		slog.Info("starting server", "address", cfg.Server.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("received shutdown signal", "signal", sig.String())
	slog.Info("shutting down server...")

	// Cancel context to stop watchers
	cancel()

	// Stop Redis authorization cache goroutines (pub/sub, fallback cleanup, recovery)
	if redisAuthzCache != nil {
		redisAuthzCache.Stop()
	}

	shutdownServices(server, wsHubCancel, wsHandler, policyCacheManager, auditRecorder,
		ssoWatcher, repoWatcher, graphRevisionWatcher, remoteWatcher, instanceTracker, rgdWatcher, routerResult.UserRateLimiters, redisClient, logger)

	slog.Info("server stopped gracefully")
	return nil
}

// newInstanceUpdateCallback creates the callback for instance tracker events.
// It handles WebSocket broadcasts, history recording, and RGD instance count updates.
func newInstanceUpdateCallback(
	historyService *history.Service,
	rgdWatcher *watcher.RGDWatcher,
	instanceTracker *watcher.InstanceTracker,
	k8sClient kubernetes.Interface,
	projectService *rbac.ProjectService,
	wsHub *websocket.Hub,
) func(watcher.InstanceAction, string, string, string, *models.Instance) {
	instanceStatusCache := make(map[string]string)
	var statusCacheMu sync.Mutex

	return func(action watcher.InstanceAction, namespace, kind, name string, instance *models.Instance) {
		var wsAction websocket.Action
		switch action {
		case watcher.InstanceActionAdd:
			wsAction = websocket.ActionAdd
		case watcher.InstanceActionUpdate:
			wsAction = websocket.ActionUpdate
		case watcher.InstanceActionDelete:
			wsAction = websocket.ActionDelete
		default:
			wsAction = websocket.ActionUpdate
		}

		projectNamespace := ""
		if instance != nil && instance.Labels != nil {
			projectNamespace = instance.Labels["knodex.io/project"]
		}
		if projectNamespace == "" {
			projectNamespace = getProjectIDFromNamespace(k8sClient, namespace, projectService)
		}

		wsHub.BroadcastInstanceUpdate(wsAction, namespace, kind, name, instance, projectNamespace)

		// Record history events
		historyCtx, historyCancel := context.WithTimeout(context.Background(), historyRecordTimeout)
		defer historyCancel()

		instanceKey := namespace + "/" + kind + "/" + name
		switch action {
		case watcher.InstanceActionAdd:
			if instance != nil {
				if err := historyService.CreateHistoryFromInstance(historyCtx, instance, "system"); err != nil {
					slog.Warn("failed to record instance creation history",
						"namespace", namespace, "name", name, "error", err)
				}
				statusCacheMu.Lock()
				instanceStatusCache[instanceKey] = string(instance.Health)
				statusCacheMu.Unlock()
			}
		case watcher.InstanceActionUpdate:
			if instance != nil {
				currentStatus := string(instance.Health)
				statusCacheMu.Lock()
				previousStatus, exists := instanceStatusCache[instanceKey]
				if exists && previousStatus != currentStatus {
					if err := historyService.RecordStatusChange(historyCtx, namespace, kind, name, previousStatus, currentStatus); err != nil {
						slog.Warn("failed to record instance status change",
							"namespace", namespace, "kind", kind, "name", name,
							"old_status", previousStatus, "new_status", currentStatus, "error", err)
					}
				}
				instanceStatusCache[instanceKey] = currentStatus
				statusCacheMu.Unlock()
			}
		case watcher.InstanceActionDelete:
			if err := historyService.RecordDeletion(historyCtx, namespace, kind, name, "system"); err != nil {
				slog.Warn("failed to record instance deletion history",
					"namespace", namespace, "kind", kind, "name", name, "error", err)
			}
			statusCacheMu.Lock()
			delete(instanceStatusCache, instanceKey)
			statusCacheMu.Unlock()
		}

		// Update instance count in RGD cache
		if instance != nil && rgdWatcher != nil {
			updateRGDInstanceCount(rgdWatcher, instanceTracker, instance.RGDNamespace, instance.RGDName)
		} else if action == watcher.InstanceActionDelete && rgdWatcher != nil {
			updateAllRGDInstanceCounts(rgdWatcher, instanceTracker)
		}
	}
}

// initEnterpriseServices initializes enterprise-only services using init functions
// registered by EE build-tag overlays. Returns the audit recorder (may be nil).
func (a *App) initEnterpriseServices(
	cfg *config.Config,
	rgdWatcher *watcher.RGDWatcher,
	wsHub *websocket.Hub,
	redisClient *redis.Client,
	k8sClient kubernetes.Interface,
) audit.Recorder {
	initCtx := context.Background()

	// License service: already set via setter (defaults to NoopLicenseService)
	if a.licenseService != nil && a.licenseService.IsLicensed() {
		slog.Info("enterprise license active")
	}

	// Compliance service: use direct setter if set, else call init func
	if a.complianceService == nil && a.complianceInitFunc != nil {
		a.complianceService = a.complianceInitFunc(initCtx, &cfg.Kubernetes, wsHub, redisClient, &cfg.Compliance)
	}
	if a.complianceService != nil {
		slog.Info("compliance service initialized (enterprise feature)")
	}

	// Violation history service: use direct setter if set, else call init func
	if a.violationHistoryService == nil && a.violationHistoryInitFunc != nil {
		a.violationHistoryService = a.violationHistoryInitFunc()
	}

	// Category service: always initialized in OSS builds (auto-discovers from watcher)
	if a.categoryService == nil && a.categoryInitFunc != nil {
		a.categoryService = a.categoryInitFunc(rgdWatcher)
	}
	if a.categoryService != nil {
		slog.Info("category service initialized (OSS feature)")
	}

	// Audit recorder: call init func to create enterprise audit recorder
	var auditRecorder audit.Recorder
	if a.auditRecorderInitFunc != nil {
		namespace := cfg.Log.Namespace
		if namespace == "" {
			namespace = "default"
		}
		auditRecorder = a.auditRecorderInitFunc(initCtx, redisClient, k8sClient, namespace)
	}
	if auditRecorder != nil {
		slog.Info("audit recorder initialized (enterprise feature)")
	}

	return auditRecorder
}

// shutdownServices performs graceful shutdown of all server components in the correct order.
func shutdownServices(
	server *http.Server,
	wsHubCancel context.CancelFunc,
	wsHandler *handlers.WebSocketHandler,
	policyCacheManager *rbac.PolicyCacheManager,
	auditRecorder audit.Recorder,
	ssoWatcher *sso.SSOWatcher,
	repoWatcher *oldwatcher.RepositoryWatcher,
	graphRevisionWatcher *watcher.GraphRevisionWatcher,
	remoteWatcher *watcher.RemoteWatcher,
	instanceTracker *watcher.InstanceTracker,
	rgdWatcher *watcher.RGDWatcher,
	userRateLimiters []*middleware.UserRateLimiter,
	redisClient *redis.Client,
	logger *slog.Logger,
) {
	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer shutdownCancel()

	// Shutdown server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	// Stop WebSocket hub by canceling its context
	if wsHubCancel != nil {
		wsHubCancel()
		slog.Info("WebSocket hub stopped")
	}

	// Stop WebSocket handler's background goroutines
	if wsHandler != nil {
		wsHandler.Shutdown()
		slog.Info("WebSocket handler stopped")
	}

	// Stop policy cache manager
	if policyCacheManager != nil {
		policyCacheManager.Stop()
		slog.Info("policy cache manager stopped")
	}

	// Stop repository secret watcher before flushing audit recorder
	// (watcher may emit audit events until stopped)
	if repoWatcher != nil {
		if !repoWatcher.StopAndWait(watcherStopTimeout) {
			slog.Warn("repository secret watcher did not stop within timeout")
		}
	}

	// Flush audit recorder buffer after watchers that emit audit events are stopped
	if auditRecorder != nil {
		if closer, ok := auditRecorder.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				slog.Warn("failed to close audit recorder", "error", err)
			} else {
				slog.Info("audit recorder flushed and stopped")
			}
		}
	}

	// Stop SSO watcher
	if ssoWatcher != nil {
		ssoWatcher.Stop()
		slog.Info("SSO watcher stopped")
	}

	// Stop GraphRevision watcher
	if graphRevisionWatcher != nil {
		if !graphRevisionWatcher.StopAndWait(watcherStopTimeout) {
			slog.Warn("GraphRevision watcher did not stop within timeout")
		}
	}

	// Stop remote watcher (before instance tracker, since it depends on remote cluster access)
	if remoteWatcher != nil {
		if !remoteWatcher.StopAndWait(watcherStopTimeout) {
			slog.Warn("remote watcher did not stop within timeout")
		}
	}

	// Stop instance tracker first (it depends on RGD watcher)
	if instanceTracker != nil {
		instanceTracker.Stop()
	}

	// Stop RGD watcher
	if rgdWatcher != nil {
		if !rgdWatcher.StopAndWait(watcherStopTimeout) {
			slog.Warn("RGD watcher did not stop within timeout - goroutine may still be running")
		}
	}

	// Stop user rate limiter cleanup goroutines
	for _, rl := range userRateLimiters {
		rl.Stop()
	}
	if len(userRateLimiters) > 0 {
		slog.Info("user rate limiters stopped", "count", len(userRateLimiters))
	}

	// Close clients
	clients.CloseRedisClient(redisClient, logger)
}

// projectServiceAdapter adapts ProjectService to implement ProjectReader interface.
type projectServiceAdapter struct {
	service *rbac.ProjectService
}

func (a *projectServiceAdapter) GetProject(ctx context.Context, name string) (*rbac.Project, error) {
	return a.service.GetProject(ctx, name)
}

func (a *projectServiceAdapter) ListProjects(ctx context.Context) ([]rbac.Project, error) {
	list, err := a.service.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (a *projectServiceAdapter) ProjectExists(ctx context.Context, name string) (bool, error) {
	_, err := a.service.GetProject(ctx, name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *projectServiceAdapter) FindProjectForNamespace(ctx context.Context, namespace string) (*rbac.Project, error) {
	return a.service.GetProjectByDestinationNamespace(ctx, namespace)
}

// policyHandlerAdapter adapts PolicyEnforcer to implement ProjectPolicyHandler interface.
type policyHandlerAdapter struct {
	enforcer       rbac.PolicyEnforcer
	projectService *rbac.ProjectService
}

func (a *policyHandlerAdapter) LoadProjectPolicies(ctx context.Context, projectName string) error {
	project, err := a.projectService.GetProject(ctx, projectName)
	if err != nil {
		return fmt.Errorf("failed to get project %s: %w", projectName, err)
	}
	return a.enforcer.LoadProjectPolicies(ctx, project)
}

func (a *policyHandlerAdapter) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	return a.enforcer.RemoveProjectPolicies(ctx, projectName)
}

func (a *policyHandlerAdapter) InvalidateCache() {
	a.enforcer.InvalidateCache()
}

func (a *policyHandlerAdapter) IncrementWatcherRestarts() {
	a.enforcer.IncrementWatcherRestarts()
}

// getProjectIDFromNamespace looks up the project ID for a given namespace.
func getProjectIDFromNamespace(k8sClient kubernetes.Interface, namespace string, projectService *rbac.ProjectService) string {
	if projectService == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), projectLookupTimeout)
	defer cancel()

	project, err := projectService.GetProjectByDestinationNamespace(ctx, namespace)
	if err != nil {
		return ""
	}

	return project.Name
}

// updateRGDInstanceCount updates the instance count for a specific RGD.
func updateRGDInstanceCount(rgdWatcher *watcher.RGDWatcher, instanceTracker *watcher.InstanceTracker, rgdNamespace, rgdName string) {
	if rgd, found := rgdWatcher.GetRGD(rgdNamespace, rgdName); found {
		count := instanceTracker.CountInstancesByRGD(rgdNamespace, rgdName)
		rgd.InstanceCount = count
		rgdWatcher.Cache().Set(rgd)
	}
}

// graphRevisionProvider converts a *GraphRevisionWatcher to services.GraphRevisionProvider,
// returning nil (not a non-nil interface wrapping a nil pointer) when the watcher is nil.
// This avoids the Go "nil interface" gotcha where a typed nil assigned to an interface is != nil.
func graphRevisionProvider(w *watcher.GraphRevisionWatcher) services.GraphRevisionProvider {
	if w == nil {
		return nil
	}
	return w
}

// hasGraphRevisionAPI checks whether the internal.kro.run/v1alpha1 API group is available in the cluster.
// Returns false if discovery fails or the API group is not found, allowing graceful degradation.
func hasGraphRevisionAPI(k8sClient kubernetes.Interface) bool {
	resources, err := k8sClient.Discovery().ServerResourcesForGroupVersion(kro.GraphRevisionGroup + "/" + kro.GraphRevisionVersion)
	if err != nil {
		return false
	}
	for _, r := range resources.APIResources {
		if r.Name == kro.GraphRevisionResource {
			return true
		}
	}
	return false
}

// resolveProjectDestinationNamespaces collects all destination namespace patterns
// from the given projects. For admin (projects == nil), returns all destinations
// from all projects. This is used for sidebar count filtering — instances only
// appear in counts if they belong to a project's destination namespace.
func resolveProjectDestinationNamespaces(ctx context.Context, ps *rbac.ProjectService, projects []string) []string {
	if ps == nil {
		return []string{"*"}
	}

	allProjects, err := ps.ListProjects(ctx)
	if err != nil {
		slog.Warn("failed to list projects for namespace resolution, falling back to wildcard", "error", err)
		return []string{"*"}
	}

	seen := make(map[string]bool)
	var namespaces []string
	for _, p := range allProjects.Items {
		// If projects list is non-nil, only include matching projects
		if projects != nil {
			found := false
			for _, name := range projects {
				if p.Name == name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		for _, dest := range p.Spec.Destinations {
			if dest.Namespace != "" && !seen[dest.Namespace] {
				seen[dest.Namespace] = true
				namespaces = append(namespaces, dest.Namespace)
			}
		}
	}

	if len(namespaces) == 0 {
		return []string{} // No projects or no destinations — count will be 0
	}
	return namespaces
}

// updateAllRGDInstanceCounts updates instance counts for all RGDs.
func updateAllRGDInstanceCounts(rgdWatcher *watcher.RGDWatcher, instanceTracker *watcher.InstanceTracker) {
	rgds := rgdWatcher.All()
	for _, rgd := range rgds {
		count := instanceTracker.CountInstancesByRGD(rgd.Namespace, rgd.Name)
		rgd.InstanceCount = count
		rgdWatcher.Cache().Set(rgd)
	}
}
