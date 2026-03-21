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
//	a.SetViewsService(views.NewService(...))
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
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/api"
	"github.com/knodex/knodex/server/internal/api/cookie"
	"github.com/knodex/knodex/server/internal/api/handlers"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/bootstrap"
	"github.com/knodex/knodex/server/internal/clients"
	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/drift"
	"github.com/knodex/knodex/server/internal/health"
	"github.com/knodex/knodex/server/internal/history"
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
	watcherInitialSyncDelay       = 3 * time.Second
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

// ViewsInitFunc is the signature for enterprise views service initialization.
type ViewsInitFunc func(rgdWatcher *watcher.RGDWatcher, configPath string) services.ViewsService

// AuditRecorderInitFunc is the signature for enterprise audit recorder initialization.
// Used during the monorepo period to bridge build-tag dispatch files with the app package.
type AuditRecorderInitFunc func(ctx context.Context, redisClient *redis.Client, k8sClient kubernetes.Interface, namespace string) audit.Recorder

// AuditLoginMiddlewareInitFunc is the signature for enterprise audit login middleware initialization.
// In EE builds, this creates an AuditService + AuditConfigWatcher and returns the login middleware.
// Returns nil in OSS builds (login routes are not wrapped with audit middleware).
type AuditLoginMiddlewareInitFunc func(ctx context.Context, redisClient *redis.Client, k8sClient kubernetes.Interface, namespace string) func(http.Handler) http.Handler

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
	viewsService            services.ViewsService

	// Organization filter for enterprise catalog filtering (empty = no filtering)
	organizationFilter string

	// Secrets enabled flag for enterprise secrets management (false = OSS, true = EE)
	secretsEnabled bool

	// Enterprise init functions (monorepo build-tag dispatch bridge)
	complianceInitFunc           ComplianceInitFunc
	violationHistoryInitFunc     ViolationHistoryInitFunc
	viewsInitFunc                ViewsInitFunc
	auditRecorderInitFunc        AuditRecorderInitFunc
	auditLoginMiddlewareInitFunc AuditLoginMiddlewareInitFunc
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

// SetViewsInitFunc registers a factory function for creating the views service.
func (a *App) SetViewsInitFunc(fn ViewsInitFunc) {
	a.viewsInitFunc = fn
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

// SetSecretsEnabled enables secrets management routes.
// In EE builds, this is true. In OSS builds, this is false (secrets endpoints return 402).
func (a *App) SetSecretsEnabled(enabled bool) {
	a.secretsEnabled = enabled
}

// Run initializes all services, starts the HTTP server, and blocks until shutdown.
// It handles graceful shutdown on SIGINT/SIGTERM.
func (a *App) Run(ctx context.Context) error {
	cfg := a.cfg

	orgSource := "default"
	if _, ok := os.LookupEnv("KNODEX_ORGANIZATION"); ok {
		orgSource = "KNODEX_ORGANIZATION"
	}
	slog.Info("organization identity configured", "organization", cfg.Organization, "source", orgSource)
	slog.Info("organization catalog filter", "active", a.organizationFilter != "", "organization", a.organizationFilter)

	// Initialize clients
	redisClient := clients.NewRedisClient(&cfg.Redis)
	k8sClient := clients.NewKubernetesClient(&cfg.Kubernetes)

	// Get dynamic client for RBAC services
	var dynamicClient dynamic.Interface
	if k8sConfig, err := clients.GetKubernetesConfig(&cfg.Kubernetes); err == nil {
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

		// Create project service
		projectService = rbac.NewProjectService(k8sClient, dynamicClient)
		slog.Info("project service initialized")

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
				projectWatcher = rbac.NewProjectWatcher(dynamicClient, policyHandler, rbac.ProjectWatcherConfig{
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

		// Auto-generate or retrieve admin password from Kubernetes secret
		bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), adminPasswordBootstrapTimeout)
		adminPassword, wasGenerated, err := bootstrap.GetOrCreateAdminPassword(bootstrapCtx, k8sClient, namespace)
		bootstrapCancel()
		if err != nil {
			slog.Error("failed to get or create admin password - local authentication may not work",
				"error", err,
				"namespace", namespace,
			)
			adminPassword = ""
		} else {
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
		}

		// Use ConfigMap-sourced SSO providers (no env var fallback)
		oidcProviders := ssoProviders

		authConfig := &auth.Config{
			JWTExpiry:          cfg.Auth.JWTExpiry,
			LocalAdminUsername: cfg.Auth.AdminUsername,
			LocalAdminPassword: adminPassword,
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

	// Create RGD watcher
	var rgdWatcher *watcher.RGDWatcher
	rgdWatcher, err := watcher.NewRGDWatcher(&cfg.Kubernetes)
	if err != nil {
		slog.Warn("failed to create RGD watcher, continuing without watcher", "error", err)
	}

	// Create instance tracker
	var instanceTracker *watcher.InstanceTracker
	if rgdWatcher != nil && k8sClient != nil {
		instanceTracker = watcher.NewInstanceTracker(rgdWatcher.DynamicClient(), k8sClient.Discovery(), rgdWatcher)
	}

	// Create health checker
	healthChecker := health.NewChecker(redisClient, k8sClient, rgdWatcher)
	if policyCacheManager != nil {
		healthChecker.SetRBACHealth(policyCacheManager)
	}

	// Create schema extractor for CRD schema extraction
	var schemaExtractor *kroschema.Extractor
	schemaExtractor, err = kroschema.NewExtractor(&cfg.Kubernetes)
	if err != nil {
		slog.Warn("failed to create schema extractor, schema endpoint will be unavailable", "error", err)
	}

	// Create history service for deployment history tracking
	historyService := history.NewService(redisClient)
	slog.Info("history service initialized")

	// Create WebSocket hub for real-time updates
	wsHub := websocket.NewHub()
	go wsHub.Run()
	slog.Info("WebSocket hub started")

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
	driftSvc := drift.NewService(redisClient, slog.Default())

	// Create API server
	router := api.NewRouterWithConfig(healthChecker, rgdWatcher, instanceTracker, schemaExtractor, api.RouterConfig{
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
		ViewsService:            a.viewsService,
		SSOStore:                ssoStore,
		AllowedRedirectOrigins:  cfg.Auth.AllowedRedirectOrigins,
		CookieConfig:            cookie.Config{Secure: cfg.Cookie.Secure, Domain: cfg.Cookie.Domain},
		SecretsEnabled:          a.secretsEnabled,     // EE secrets management (false = 402)
		OrganizationFilter:      a.organizationFilter, // EE catalog filtering (empty = no filter)
		Organization:            cfg.Organization,     // Display identity for GET /api/v1/settings
		SwaggerEnabled:          cfg.SwaggerEnabled,   // Serve Swagger UI at /swagger/ (SWAGGER_UI_ENABLED)
		AuditRecorder:           auditRecorder,
		AuditLoginMiddleware:    auditLoginMiddleware,
		AuditAPIService:         auditAPIService,
		DriftService:            driftSvc,
	})

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           router,
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
			driftSvc.CheckAndClearIfReconciled(context.Background(), namespace, kind, name, instance.Spec)
		})

		if err := instanceTracker.Start(runCtx); err != nil {
			slog.Error("failed to start instance tracker", "error", err)
		} else {
			slog.Info("instance tracker started")
		}

		// Initial sync of instance counts
		go func() {
			time.Sleep(watcherInitialSyncDelay)
			if instanceTracker.IsSynced() && rgdWatcher.IsSynced() {
				updateAllRGDInstanceCounts(rgdWatcher, instanceTracker)
				slog.Info("initial RGD instance counts synchronized")
			}
		}()
	}

	// Wire WebSocket count push for sidebar badge updates (AC: #2, #3, #4)
	// Uses in-memory caches only - no Redis/HTTP/K8s API calls
	if rgdWatcher != nil && instanceTracker != nil {
		countPushFn := func(_ context.Context, _ string, projects []string, _ []string) (int, int) {
			// RGD count via watcher cache (in-memory)
			rgdOpts := models.ListOptions{Page: 1, PageSize: 1, IncludePublic: true}
			rgdOpts.Organization = a.organizationFilter // Enterprise org filter for consistent counts
			if projects != nil {
				rgdOpts.Projects = projects
			}
			rgdResult := rgdWatcher.ListRGDs(rgdOpts)

			// Instance count via tracker cache (in-memory)
			var instanceCount int
			if projects == nil { // global admin sees all
				instanceCount = instanceTracker.CountInstancesByNamespaces(nil, rbac.MatchNamespaceInList)
			} else {
				// In Knodex, project names = namespace names
				instanceCount = instanceTracker.CountInstancesByNamespaces(projects, rbac.MatchNamespaceInList)
			}
			return rgdResult.TotalCount, instanceCount
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

	shutdownServices(server, wsHub, wsHandler, policyCacheManager, auditRecorder,
		ssoWatcher, repoWatcher, instanceTracker, rgdWatcher, redisClient)

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

		wsHub.BroadcastInstanceUpdate(wsAction, namespace, name, instance, projectNamespace)

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

	// Views service: use direct setter if set, else call init func
	if a.viewsService == nil && a.viewsInitFunc != nil {
		a.viewsService = a.viewsInitFunc(rgdWatcher, cfg.Views.ConfigPath)
	}
	if a.viewsService != nil {
		slog.Info("views service initialized (enterprise feature)")
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
	wsHub *websocket.Hub,
	wsHandler *handlers.WebSocketHandler,
	policyCacheManager *rbac.PolicyCacheManager,
	auditRecorder audit.Recorder,
	ssoWatcher *sso.SSOWatcher,
	repoWatcher *oldwatcher.RepositoryWatcher,
	instanceTracker *watcher.InstanceTracker,
	rgdWatcher *watcher.RGDWatcher,
	redisClient *redis.Client,
) {
	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer shutdownCancel()

	// Shutdown server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	// Stop WebSocket hub
	if wsHub != nil {
		wsHub.Stop()
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

	// Close clients
	clients.CloseRedisClient(redisClient)
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

// updateAllRGDInstanceCounts updates instance counts for all RGDs.
func updateAllRGDInstanceCounts(rgdWatcher *watcher.RGDWatcher, instanceTracker *watcher.InstanceTracker) {
	rgds := rgdWatcher.All()
	for _, rgd := range rgds {
		count := instanceTracker.CountInstancesByRGD(rgd.Namespace, rgd.Name)
		rgd.InstanceCount = count
		rgdWatcher.Cache().Set(rgd)
	}
}
