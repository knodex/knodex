// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"log/slog"
	"net/http"

	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/api/cookie"
	"github.com/knodex/knodex/server/internal/api/handlers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	swaggerui "github.com/knodex/knodex/server/internal/api/swagger"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/drift"
	"github.com/knodex/knodex/server/internal/health"
	"github.com/knodex/knodex/server/internal/history"
	kroschema "github.com/knodex/knodex/server/internal/kro/schema"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/repository"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/sso"
	"github.com/knodex/knodex/server/internal/websocket"
)

// RouterConfig holds configuration for the router
type RouterConfig struct {
	RateLimitRequestsPerMin int          // User rate limit requests per minute (default: 100)
	RateLimitBurstSize      int          // User rate limit burst size (default: 100)
	RateLimitTrustedProxies []string     // Trusted proxy IPs/CIDRs for real client IP extraction
	CORSAllowedOrigins      []string     // Allowed origins for CORS (empty = deny all cross-origin)
	SPAHandler              http.Handler // Optional: embedded frontend SPA handler (nil disables static file serving)
	WebSocketHub            *websocket.Hub
	WebSocketHandler        *handlers.WebSocketHandler // Optional: pass from main.go for lifecycle management
	AuthService             *auth.Service
	OIDCService             handlers.OIDCServiceInterface
	RepositoryService       *repository.Service
	PermissionService       *rbac.PermissionService
	PolicyEnforcer          rbac.PolicyEnforcer
	PolicyCacheManager      *rbac.PolicyCacheManager
	ProjectService          rbac.ProjectServiceInterface
	HistoryService          *history.Service
	NamespaceService        *rbac.NamespaceService
	LicenseService          services.LicenseService          // Enterprise feature: License validation
	ComplianceService       services.ComplianceService       // Enterprise feature: OPA Gatekeeper compliance
	ViolationHistoryService services.ViolationHistoryService // Enterprise feature: Violation history & CSV export
	ViewsService            services.ViewsService            // Enterprise feature: Custom category views
	SSOStore                *sso.ProviderStore               // SSO provider management store
	AuditRecorder           audit.Recorder                   // Audit event recorder (nil in OSS builds)
	AuditLoginMiddleware    func(http.Handler) http.Handler  // Enterprise feature: Wraps login routes to record audit events (nil in OSS builds)
	AuditAPIService         services.AuditAPIService         // Enterprise feature: Audit trail API
	DriftService            *drift.Service                   // GitOps drift detection service (nil = created internally from RedisClient)
	K8sClient               kubernetes.Interface
	RedisClient             *redis.Client
	AllowedRedirectOrigins  []string      // Allowed redirect origins for OIDC callbacks
	CookieConfig            cookie.Config // Session cookie configuration (Secure, Domain)
	OrganizationFilter      string        // Enterprise org filter for catalog (empty = no filtering)
	Organization            string        // Organization identity for settings endpoint (from KNODEX_ORGANIZATION, default "default")
	SecretsEnabled          bool          // Enterprise feature: Secrets management (false = 402 Payment Required)
	SwaggerEnabled          bool          // Enable Swagger UI at /swagger/ (default: false, env: SWAGGER_UI_ENABLED)
}

// Note: permissionServiceAdapter removed - all authorization uses CasbinAuthz exclusively.

// NewRouterWithConfig creates the HTTP router with custom configuration
func NewRouterWithConfig(healthChecker *health.Checker, rgdWatcher *watcher.RGDWatcher, instanceTracker *watcher.InstanceTracker, schemaExtractor *kroschema.Extractor, cfg RouterConfig) http.Handler {
	// Create main mux for catch-all routes
	mainMux := http.NewServeMux()

	// Create protected mux for authenticated routes
	protectedMux := http.NewServeMux()

	// Create services for service-based architecture
	logger := slog.Default()

	// Authorization service consolidates authorization logic from handlers
	var authService *services.AuthorizationService
	if cfg.PolicyEnforcer != nil || cfg.PermissionService != nil {
		authService = services.NewAuthorizationService(services.AuthorizationServiceConfig{
			PolicyEnforcer:    cfg.PolicyEnforcer,
			NamespaceProvider: cfg.PermissionService,
			Logger:            logger,
		})
	}

	// Catalog service encapsulates RGD business logic
	// Note: Only set InstanceCounter if not nil to avoid Go interface nil gotcha
	// (a nil *InstanceTracker assigned to an interface is not == nil)
	catalogConfig := services.CatalogServiceConfig{
		RGDProvider:        rgdWatcher,
		RedisClient:        cfg.RedisClient,
		Logger:             logger,
		OrganizationFilter: cfg.OrganizationFilter,
	}
	if instanceTracker != nil {
		catalogConfig.InstanceCounter = instanceTracker
	}
	catalogService := services.NewCatalogService(catalogConfig)

	// Create RGD handler with service-based architecture
	rgdHandler := handlers.NewRGDHandler(handlers.RGDHandlerConfig{
		AuthService:    authService,
		CatalogService: catalogService,
		Logger:         logger,
	})

	resourceHandler := handlers.NewResourceHandler(rgdWatcher)
	schemaHandler := handlers.NewSchemaHandler(rgdWatcher, schemaExtractor)
	if cfg.PolicyEnforcer != nil {
		schemaHandler.SetPolicyEnforcer(cfg.PolicyEnforcer)
	}

	// Get dynamic client for instance handler and k8s resource handler
	var dynamicClient dynamic.Interface
	if rgdWatcher != nil {
		dynamicClient = rgdWatcher.DynamicClient()
	}

	// Get discovery client for Kind → GVR resolution (custom CRDs support)
	var discoveryClient discovery.DiscoveryInterface
	if cfg.K8sClient != nil {
		discoveryClient = cfg.K8sClient.Discovery()
	}

	// Create shared deployment controller for GitOps operations
	var deployCtrl *deployment.Controller
	if dynamicClient != nil && cfg.K8sClient != nil {
		deployCtrl = deployment.NewController(dynamicClient, cfg.K8sClient, logger)
	}

	// Use provided drift service or create one for GitOps spec drift tracking
	driftService := cfg.DriftService
	if driftService == nil {
		driftService = drift.NewService(cfg.RedisClient, logger)
	}

	// Create domain-specific instance handlers
	crudHandler := handlers.NewInstanceCRUDHandler(handlers.InstanceCRUDHandlerConfig{
		InstanceTracker:      instanceTracker,
		DynamicClient:        dynamicClient,
		AuthService:          authService,
		DeploymentController: deployCtrl,
		RepoService:          cfg.RepositoryService,
		DriftService:         driftService,
		AuditRecorder:        cfg.AuditRecorder,
		Logger:               logger,
	})

	deploymentHandler := handlers.NewInstanceDeploymentHandler(handlers.InstanceDeploymentHandlerConfig{
		RGDWatcher:    rgdWatcher,
		DynamicClient: dynamicClient,
		KubeClient:    cfg.K8sClient,
		RepoService:   cfg.RepositoryService,
		AuditRecorder: cfg.AuditRecorder,
		Logger:        logger,
	})

	// Note: GitOpsHandler requires a GitOpsSyncMonitor, not an InstanceTracker
	// GitOps monitoring is not yet wired up in main.go, so gitopsHandler is nil
	var gitopsHandler *handlers.InstanceGitOpsHandler

	k8sResourceHandler := handlers.NewK8sResourceHandler(dynamicClient, discoveryClient)

	// Protected API v1 routes - RGD catalog (require authentication)
	protectedMux.HandleFunc("GET /api/v1/rgds", rgdHandler.ListRGDs)
	protectedMux.HandleFunc("GET /api/v1/rgds/count", rgdHandler.GetCount)
	protectedMux.HandleFunc("GET /api/v1/rgds/filters", rgdHandler.GetFilters)
	protectedMux.HandleFunc("GET /api/v1/rgds/{name}", rgdHandler.GetRGD)
	protectedMux.HandleFunc("GET /api/v1/rgds/{name}/resources", resourceHandler.GetResourceGraph)
	protectedMux.HandleFunc("GET /api/v1/rgds/{name}/schema", schemaHandler.GetSchema)
	protectedMux.HandleFunc("POST /api/v1/rgds/{name}/schema/invalidate", schemaHandler.InvalidateSchemaCache)

	// Protected API v1 routes - Instance management (require authentication)
	protectedMux.HandleFunc("GET /api/v1/instances", crudHandler.ListInstances)
	protectedMux.HandleFunc("GET /api/v1/instances/count", crudHandler.GetCount)
	protectedMux.HandleFunc("GET /api/v1/instances/{namespace}/{kind}/{name}", crudHandler.GetInstance)

	// GitOps monitoring routes (only registered if GitOpsHandler is available)
	if gitopsHandler != nil {
		protectedMux.HandleFunc("GET /api/v1/instances/pending", gitopsHandler.GetPendingInstances)
		protectedMux.HandleFunc("GET /api/v1/instances/stuck", gitopsHandler.GetStuckInstances)
		protectedMux.HandleFunc("GET /api/v1/instances/timeline/{instanceId}", gitopsHandler.GetStatusTimeline)
	}

	// Instance creation with deployment validation middleware
	// Validates: project exists, user has deploy permission, source repo allowed, destination allowed
	if cfg.ProjectService != nil && cfg.PolicyEnforcer != nil {
		deploymentValidator := middleware.DeploymentValidator(middleware.DeploymentValidatorConfig{
			ProjectService: cfg.ProjectService,
			PolicyEnforcer: cfg.PolicyEnforcer,
			Logger:         slog.Default().With("component", "deployment-validator"),
		})
		protectedMux.Handle("POST /api/v1/instances", deploymentValidator(http.HandlerFunc(deploymentHandler.CreateInstance)))
	} else {
		// Fail-closed: return 503 when authorization services are not initialized
		protectedMux.HandleFunc("POST /api/v1/instances", func(w http.ResponseWriter, r *http.Request) {
			response.ServiceUnavailable(w, "instance creation temporarily unavailable")
		})
	}

	protectedMux.HandleFunc("PATCH /api/v1/instances/{namespace}/{kind}/{name}", crudHandler.UpdateInstance)
	protectedMux.HandleFunc("DELETE /api/v1/instances/{namespace}/{kind}/{name}", crudHandler.DeleteInstance)

	// Protected API v1 routes - Deployment history (require authentication)
	if cfg.HistoryService != nil {
		historyHandler := handlers.NewHistoryHandler(cfg.HistoryService)
		protectedMux.HandleFunc("GET /api/v1/instances/{namespace}/{kind}/{name}/history", historyHandler.GetHistory)
		protectedMux.HandleFunc("GET /api/v1/instances/{namespace}/{kind}/{name}/history/export", historyHandler.ExportHistory)
		protectedMux.HandleFunc("GET /api/v1/instances/{namespace}/{kind}/{name}/timeline", historyHandler.GetTimeline)
	}

	// Protected API v1 routes - K8s resource listing (for ExternalRef selectors)
	protectedMux.HandleFunc("GET /api/v1/resources", k8sResourceHandler.ListResources)

	// Protected API v1 routes - Kubernetes API resources discovery (for constraint match rules)
	if cfg.K8sClient != nil {
		apiResourcesHandler := handlers.NewAPIResourcesHandler(cfg.K8sClient.Discovery())
		protectedMux.HandleFunc("GET /api/v1/kubernetes/api-resources", apiResourcesHandler.ListAPIResources)
	}

	// Protected API v1 routes - Repository configuration management (PolicyEnforcer for project-scoped authorization)
	if cfg.RepositoryService != nil {
		repoHandler := handlers.NewRepositoryHandler(cfg.RepositoryService, cfg.PermissionService, cfg.PolicyEnforcer, cfg.AuditRecorder)
		protectedMux.HandleFunc("POST /api/v1/repositories", repoHandler.CreateRepositoryConfig)
		protectedMux.HandleFunc("GET /api/v1/repositories", repoHandler.ListRepositoryConfigs)
		protectedMux.HandleFunc("GET /api/v1/repositories/{repoId}", repoHandler.GetRepositoryConfig)
		protectedMux.HandleFunc("PATCH /api/v1/repositories/{repoId}", repoHandler.UpdateRepositoryConfig)
		protectedMux.HandleFunc("DELETE /api/v1/repositories/{repoId}", repoHandler.DeleteRepositoryConfig)
		protectedMux.HandleFunc("POST /api/v1/repositories/test-connection", repoHandler.TestConnection)
		protectedMux.HandleFunc("POST /api/v1/repositories/{repoId}/test", repoHandler.TestRepositoryConnection)
	}

	// WebSocket ticket endpoint (authenticated) - generates single-use tickets for WS connections
	if cfg.RedisClient != nil {
		wsTicketHandler := handlers.NewWSTicketHandler(cfg.RedisClient)
		protectedMux.HandleFunc("POST /api/v1/ws/ticket", wsTicketHandler.CreateTicket)
	}

	// WebSocket metrics endpoint (authenticated)
	// Use passed-in handler (for lifecycle management) or create one
	if cfg.WebSocketHandler != nil {
		protectedMux.HandleFunc("GET /api/v1/ws/metrics", cfg.WebSocketHandler.GetMetrics)
	} else if cfg.WebSocketHub != nil {
		wsHandler := handlers.NewWebSocketHandler(cfg.WebSocketHub, cfg.AuthService, cfg.RedisClient)
		if cfg.PolicyEnforcer != nil {
			wsHandler.SetPolicyEnforcer(cfg.PolicyEnforcer)
		}
		protectedMux.HandleFunc("GET /api/v1/ws/metrics", wsHandler.GetMetrics)
	}

	// RBAC metrics endpoint (authenticated) - exposes policy cache and enforcement metrics
	if cfg.PolicyEnforcer != nil {
		rbacMetricsHandler := handlers.NewRBACMetricsHandler(cfg.PolicyEnforcer, cfg.PolicyCacheManager)
		protectedMux.HandleFunc("GET /api/v1/rbac/metrics", rbacMetricsHandler.GetMetrics)
	}

	// Protected API v1 routes - Project management (require authentication)
	if cfg.ProjectService != nil && cfg.PolicyEnforcer != nil {
		projectHandler := handlers.NewProjectHandler(cfg.ProjectService, cfg.PolicyEnforcer, cfg.AuditRecorder)
		protectedMux.HandleFunc("GET /api/v1/projects", projectHandler.ListProjects)
		protectedMux.HandleFunc("GET /api/v1/projects/{name}", projectHandler.GetProject)
		protectedMux.HandleFunc("POST /api/v1/projects", projectHandler.CreateProject)
		protectedMux.HandleFunc("PUT /api/v1/projects/{name}", projectHandler.UpdateProject)
		protectedMux.HandleFunc("DELETE /api/v1/projects/{name}", projectHandler.DeleteProject)
	}

	// Protected API v1 routes - Secrets management (enterprise feature, require authentication + K8s client)
	if !cfg.SecretsEnabled {
		// OSS builds: secrets endpoints return 402 Payment Required
		secretsNotLicensed := func(w http.ResponseWriter, r *http.Request) {
			response.WriteError(w, http.StatusPaymentRequired, "ENTERPRISE_REQUIRED",
				"secrets management requires an Enterprise license", nil)
		}
		protectedMux.HandleFunc("POST /api/v1/secrets", secretsNotLicensed)
		protectedMux.HandleFunc("GET /api/v1/secrets", secretsNotLicensed)
		protectedMux.HandleFunc("HEAD /api/v1/secrets/{name}", secretsNotLicensed)
		protectedMux.HandleFunc("GET /api/v1/secrets/{name}", secretsNotLicensed)
		protectedMux.HandleFunc("PUT /api/v1/secrets/{name}", secretsNotLicensed)
		protectedMux.HandleFunc("DELETE /api/v1/secrets/{name}", secretsNotLicensed)
	} else if cfg.K8sClient != nil && cfg.PolicyEnforcer != nil {
		secretsHandler := handlers.NewSecretsHandler(handlers.SecretsHandlerConfig{
			K8sClient:     cfg.K8sClient,
			DynamicClient: dynamicClient,
			Enforcer:      cfg.PolicyEnforcer,
			Recorder:      cfg.AuditRecorder,
		})
		if cfg.LicenseService != nil {
			secretsHandler.SetLicenseService(cfg.LicenseService)
		}
		protectedMux.HandleFunc("POST /api/v1/secrets", secretsHandler.CreateSecret)
		protectedMux.HandleFunc("GET /api/v1/secrets", secretsHandler.ListSecrets)
		protectedMux.HandleFunc("HEAD /api/v1/secrets/{name}", secretsHandler.CheckSecretExists)
		protectedMux.HandleFunc("GET /api/v1/secrets/{name}", secretsHandler.GetSecret)
		protectedMux.HandleFunc("PUT /api/v1/secrets/{name}", secretsHandler.UpdateSecret)
		protectedMux.HandleFunc("DELETE /api/v1/secrets/{name}", secretsHandler.DeleteSecret)
	} else {
		// Fail-closed: return 503 when K8s client or authorization services are not initialized
		secretsUnavailable := func(w http.ResponseWriter, r *http.Request) {
			response.ServiceUnavailable(w, "secrets management temporarily unavailable")
		}
		protectedMux.HandleFunc("POST /api/v1/secrets", secretsUnavailable)
		protectedMux.HandleFunc("GET /api/v1/secrets", secretsUnavailable)
		protectedMux.HandleFunc("HEAD /api/v1/secrets/{name}", secretsUnavailable)
		protectedMux.HandleFunc("GET /api/v1/secrets/{name}", secretsUnavailable)
		protectedMux.HandleFunc("PUT /api/v1/secrets/{name}", secretsUnavailable)
		protectedMux.HandleFunc("DELETE /api/v1/secrets/{name}", secretsUnavailable)
	}

	// Protected API v1 routes - Role binding management (require authentication)
	if cfg.ProjectService != nil && cfg.PolicyEnforcer != nil {
		roleBindingHandler := handlers.NewRoleBindingHandler(cfg.ProjectService, cfg.PolicyEnforcer, cfg.AuditRecorder)
		protectedMux.HandleFunc("POST /api/v1/projects/{name}/roles/{role}/users/{user}", roleBindingHandler.AssignUserRole)
		protectedMux.HandleFunc("POST /api/v1/projects/{name}/roles/{role}/groups/{group}", roleBindingHandler.AssignGroupRole)
		protectedMux.HandleFunc("GET /api/v1/projects/{name}/role-bindings", roleBindingHandler.ListRoleBindings)
		protectedMux.HandleFunc("DELETE /api/v1/projects/{name}/roles/{role}/users/{user}", roleBindingHandler.RemoveUserRole)
		protectedMux.HandleFunc("DELETE /api/v1/projects/{name}/roles/{role}/groups/{group}", roleBindingHandler.RemoveGroupRole)
	}

	// Protected API v1 routes - Policy validation (require authentication)
	if cfg.ProjectService != nil && cfg.PolicyEnforcer != nil {
		validationHandler := handlers.NewValidationHandler(cfg.ProjectService, cfg.PolicyEnforcer)
		protectedMux.HandleFunc("POST /api/v1/projects/validate", validationHandler.ValidateProjectCreation)
		protectedMux.HandleFunc("POST /api/v1/projects/{name}/validate", validationHandler.ValidateProjectUpdate)
	}

	// Protected API v1 routes - Account/permission checking (ArgoCD-style can-i endpoint)
	if cfg.AuthService != nil {
		accountHandler := handlers.NewAccountHandler(cfg.AuthService)
		if cfg.ProjectService != nil {
			accountHandler.SetProjectService(cfg.ProjectService)
		}
		// Register EE-only resources so OSS builds return 400 for unknown resources
		if cfg.SecretsEnabled {
			accountHandler.RegisterEnterpriseResource("secrets")
		}
		if cfg.ComplianceService != nil {
			accountHandler.RegisterEnterpriseResource("compliance")
		}
		protectedMux.HandleFunc("GET /api/v1/account/can-i/{resource}/{action}/{subresource}", accountHandler.CanI)
		protectedMux.HandleFunc("GET /api/v1/account/info", accountHandler.Info)
	}

	// Protected API v1 routes - Namespace listing (require authentication)
	// Namespace dropdown shows real cluster namespaces matching project policies
	if cfg.NamespaceService != nil && cfg.PolicyEnforcer != nil {
		namespaceHandler := handlers.NewNamespaceHandler(cfg.NamespaceService, cfg.PolicyEnforcer)
		protectedMux.HandleFunc("GET /api/v1/namespaces", namespaceHandler.ListNamespaces)

		// SECURITY (M-2): Endpoint-specific rate limiting for project namespace endpoint
		// Stricter rate limit (20 req/min vs 100 req/min) to prevent namespace enumeration attacks
		// An attacker could probe project names via /projects/{name}/namespaces responses (200 vs 403 vs 404)
		projectNsRateLimiter := middleware.RateLimit(middleware.RateLimitConfig{
			RequestsPerMinute: 20,
			BurstSize:         5,
			TrustedProxies:    cfg.RateLimitTrustedProxies,
		})
		protectedMux.Handle("GET /api/v1/projects/{name}/namespaces",
			projectNsRateLimiter(http.HandlerFunc(namespaceHandler.ListProjectNamespaces)))
	}

	// Protected API v1 routes - General settings (organization identity, etc.)
	settingsHandler := handlers.NewSettingsHandler(cfg.Organization)
	protectedMux.HandleFunc("GET /api/v1/settings", settingsHandler.GetSettings)

	// Protected API v1 routes - License status (enterprise feature)
	// GET is available to all authenticated users, POST requires settings:update
	if cfg.LicenseService != nil {
		licenseHandler := handlers.NewLicenseHandler(cfg.LicenseService, cfg.PolicyEnforcer, logger)
		protectedMux.HandleFunc("GET /api/v1/license", licenseHandler.GetStatus)
		protectedMux.HandleFunc("POST /api/v1/license", licenseHandler.UpdateLicense)
	}

	// Protected API v1 routes - Compliance (enterprise feature: OPA Gatekeeper)
	// Handler checks IsEnabled() and returns 402 Payment Required for OSS builds
	// or 503 Service Unavailable when Gatekeeper is not installed
	if cfg.ComplianceService != nil && cfg.PolicyEnforcer != nil {
		complianceHandler := handlers.NewComplianceHandler(cfg.ComplianceService, cfg.PolicyEnforcer, logger)
		if cfg.LicenseService != nil {
			complianceHandler.SetLicenseService(cfg.LicenseService)
		}
		if cfg.ViolationHistoryService != nil {
			complianceHandler.SetViolationHistoryService(cfg.ViolationHistoryService)
		}
		if cfg.AuditRecorder != nil {
			complianceHandler.SetAuditRecorder(cfg.AuditRecorder)
		}
		if cfg.ProjectService != nil {
			complianceHandler.SetProjectService(cfg.ProjectService)
		}
		if cfg.RedisClient != nil {
			complianceHandler.SetRedisClient(cfg.RedisClient)
		}
		protectedMux.HandleFunc("GET /api/v1/compliance/status", complianceHandler.GetStatus)
		protectedMux.HandleFunc("GET /api/v1/compliance/summary", complianceHandler.GetSummary)
		protectedMux.HandleFunc("GET /api/v1/compliance/templates", complianceHandler.ListTemplates)
		protectedMux.HandleFunc("GET /api/v1/compliance/templates/{name}", complianceHandler.GetTemplate)
		protectedMux.HandleFunc("GET /api/v1/compliance/constraints", complianceHandler.ListConstraints)
		protectedMux.HandleFunc("POST /api/v1/compliance/constraints", complianceHandler.CreateConstraint)
		protectedMux.HandleFunc("GET /api/v1/compliance/constraints/{kind}/{name}", complianceHandler.GetConstraint)
		protectedMux.HandleFunc("PATCH /api/v1/compliance/constraints/{kind}/{name}/enforcement", complianceHandler.UpdateConstraintEnforcement)
		protectedMux.HandleFunc("GET /api/v1/compliance/violations", complianceHandler.ListViolations)
		protectedMux.HandleFunc("GET /api/v1/compliance/violations/history", complianceHandler.ListViolationHistory)
		protectedMux.HandleFunc("GET /api/v1/compliance/violations/history/count", complianceHandler.CountViolationHistory)
		protectedMux.HandleFunc("GET /api/v1/compliance/violations/history/export", complianceHandler.ExportViolationHistory)
	}

	// Protected API v1 routes - Custom Views (enterprise feature)
	// Returns 404 in OSS builds (service is nil), list of views in EE builds
	if cfg.ViewsService != nil && cfg.PolicyEnforcer != nil {
		viewsHandler := handlers.NewViewsHandler(cfg.ViewsService, cfg.PolicyEnforcer, logger)
		if cfg.LicenseService != nil {
			viewsHandler.SetLicenseService(cfg.LicenseService)
		}
		protectedMux.HandleFunc("GET /api/v1/ee/views", viewsHandler.ListViews)
		protectedMux.HandleFunc("GET /api/v1/ee/views/{slug}", viewsHandler.GetView)
	}

	// Protected API v1 routes - SSO provider management (require authentication + settings access)
	if cfg.SSOStore != nil {
		ssoHandler := handlers.NewSSOSettingsHandler(cfg.SSOStore, cfg.AuditRecorder, cfg.PolicyEnforcer)

		// SSO mutation rate limiter: burst of 5, then 1 per minute sustained (SSRF probe / DoS prevention)
		ssoMutationLimiter := middleware.UserRateLimit(middleware.UserRateLimitConfig{
			RequestsPerMinute: 1, // 1/min sustained rate, burst of 5 initial requests
			BurstSize:         5,
			FallbackToIP:      false,
			RetryAfterSeconds: 60,
		})

		// GET endpoints: no extra rate limiting (read-only, safe)
		protectedMux.HandleFunc("GET /api/v1/settings/sso/providers", ssoHandler.ListProviders)
		protectedMux.HandleFunc("GET /api/v1/settings/sso/providers/{name}", ssoHandler.GetProvider)

		// Mutation endpoints: apply SSO-specific rate limit
		protectedMux.Handle("POST /api/v1/settings/sso/providers",
			ssoMutationLimiter(http.HandlerFunc(ssoHandler.CreateProvider)))
		protectedMux.Handle("PUT /api/v1/settings/sso/providers/{name}",
			ssoMutationLimiter(http.HandlerFunc(ssoHandler.UpdateProvider)))
		protectedMux.Handle("DELETE /api/v1/settings/sso/providers/{name}",
			ssoMutationLimiter(http.HandlerFunc(ssoHandler.DeleteProvider)))
	}

	// Protected API v1 routes - Audit trail management (enterprise feature, require authentication + settings access)
	if cfg.AuditAPIService != nil {
		cfg.AuditAPIService.RegisterRoutes(protectedMux)
	}

	// Apply middleware to protected routes (order matters: last applied = first executed)
	// Middleware chain: RequestID -> Security Headers -> Logging -> Auth -> Authz -> Rate Limit -> Request Size Limit -> Handler
	var protectedHandler http.Handler = protectedMux

	// Apply request body size limiting (1MB max) to prevent DoS attacks
	protectedHandler = middleware.RequestSizeLimit(protectedHandler)

	// Apply per-user rate limiting (configurable via environment variables)
	if cfg.AuthService != nil {
		rateLimitReqPerMin := cfg.RateLimitRequestsPerMin
		rateLimitBurst := cfg.RateLimitBurstSize
		// Use defaults if not configured
		if rateLimitReqPerMin == 0 {
			rateLimitReqPerMin = 100
		}
		if rateLimitBurst == 0 {
			rateLimitBurst = 100
		}
		protectedHandler = middleware.UserRateLimit(middleware.UserRateLimitConfig{
			RequestsPerMinute: rateLimitReqPerMin,
			BurstSize:         rateLimitBurst,
			FallbackToIP:      false, // Only rate limit authenticated users
			TrustedProxies:    cfg.RateLimitTrustedProxies,
			RetryAfterSeconds: 1, // General API: 100 req/min ≈ 0.6s between tokens
		})(protectedHandler)
	}

	// Apply authorization middleware (permission checks)
	// Use CasbinAuthz exclusively - supports OIDC groups via CanAccessWithGroups
	if cfg.AuthService != nil && cfg.PolicyEnforcer != nil {
		authzConfig := middleware.CasbinAuthzConfig{
			Enforcer: cfg.PolicyEnforcer,
			Logger:   slog.Default(),
		}
		if cfg.PolicyCacheManager != nil {
			authzConfig.ReadyChecker = cfg.PolicyCacheManager
		}
		protectedHandler = middleware.CasbinAuthz(authzConfig)(protectedHandler)
	}
	// Note: Legacy Authz fallback removed - PolicyEnforcer is now required for authorization

	// Apply authentication middleware
	if cfg.AuthService != nil {
		protectedHandler = middleware.Auth(middleware.AuthConfig{
			AuthService: cfg.AuthService,
		})(protectedHandler)
	}

	// Apply logging (logs user context if available)
	protectedHandler = middleware.Logging(protectedHandler)

	// Note: SecurityHeaders and RequestID are applied once in the combined handler chain (below),
	// which wraps all routes including protected ones. No need to duplicate them here.

	// Combine main and protected muxes
	// All /api/v1/* routes go through protected handler except login endpoint
	// Other routes go through main mux with minimal middleware
	combinedMux := http.NewServeMux()

	// Register unauthenticated endpoints first (more specific patterns take precedence)
	if cfg.AuthService != nil {
		authHandler := handlers.NewAuthHandler(cfg.AuthService, cfg.OIDCService)
		authHandler.SetAuditRecorder(cfg.AuditRecorder)
		authHandler.SetCookieConfig(cfg.CookieConfig)
		if cfg.RedisClient != nil {
			authHandler.SetRedisClient(cfg.RedisClient)
		}
		if cfg.AllowedRedirectOrigins != nil {
			authHandler.SetAllowedRedirectOrigins(cfg.AllowedRedirectOrigins)
		}

		// Local admin login (rate limited)
		loginRateLimiter := middleware.RateLimit(middleware.RateLimitConfig{
			RequestsPerMinute: 5,
			BurstSize:         5,
			TrustedProxies:    cfg.RateLimitTrustedProxies,
		})

		// Local admin login (optionally wrapped with audit login middleware)
		var loginHandler http.Handler = http.HandlerFunc(authHandler.LocalLogin)
		if cfg.AuditLoginMiddleware != nil {
			loginHandler = cfg.AuditLoginMiddleware(loginHandler)
		}
		loginHandler = loginRateLimiter(loginHandler)
		combinedMux.Handle("POST /api/v1/auth/local/login", loginHandler)

		// Logout endpoint (authenticated, no authz — any authenticated user can log out)
		logoutHandler := middleware.Auth(middleware.AuthConfig{
			AuthService: cfg.AuthService,
		})(http.HandlerFunc(authHandler.Logout))
		combinedMux.Handle("POST /api/v1/auth/logout", logoutHandler)

		// Auth code exchange endpoint (unauthenticated - used by frontend after OIDC callback)
		if cfg.RedisClient != nil {
			authCodeHandler := handlers.NewAuthCodeHandler(cfg.RedisClient, cfg.AuthService, cfg.CookieConfig)
			tokenExchangeHandler := loginRateLimiter(http.HandlerFunc(authCodeHandler.TokenExchange))
			combinedMux.Handle("POST /api/v1/auth/token-exchange", tokenExchangeHandler)
		}

		// OIDC endpoints (if OIDC service is configured)
		if cfg.OIDCService != nil {
			// OIDC login rate limiter (lighter than callback to prevent state exhaustion)
			oidcLoginRateLimiter := middleware.RateLimit(middleware.RateLimitConfig{
				RequestsPerMinute: 20,
				BurstSize:         5,
				TrustedProxies:    cfg.RateLimitTrustedProxies,
			})

			// OIDC login initiation (rate limited to prevent state token exhaustion and provider enumeration)
			oidcLoginHandler := oidcLoginRateLimiter(http.HandlerFunc(authHandler.OIDCLogin))
			combinedMux.Handle("GET /api/v1/auth/oidc/login", oidcLoginHandler)

			// OIDC callback (optionally wrapped with audit login middleware for login event recording)
			var oidcCallbackHandler http.Handler = http.HandlerFunc(authHandler.OIDCCallback)
			if cfg.AuditLoginMiddleware != nil {
				oidcCallbackHandler = cfg.AuditLoginMiddleware(oidcCallbackHandler)
			}
			oidcCallbackHandler = loginRateLimiter(oidcCallbackHandler)
			combinedMux.Handle("GET /api/v1/auth/oidc/callback", oidcCallbackHandler)

			// List OIDC providers (unauthenticated - used by login page, rate limited)
			oidcProvidersRateLimiter := middleware.RateLimit(middleware.RateLimitConfig{
				RequestsPerMinute: 30,
				BurstSize:         10,
				TrustedProxies:    cfg.RateLimitTrustedProxies,
			})
			combinedMux.Handle("GET /api/v1/auth/oidc/providers",
				oidcProvidersRateLimiter(http.HandlerFunc(authHandler.ListOIDCProviders)))
		}
	}

	// Register health endpoints
	healthHandler := handlers.NewHealthHandler(healthChecker)
	combinedMux.HandleFunc("GET /healthz", healthHandler.Healthz)
	combinedMux.HandleFunc("GET /readyz", healthHandler.Readyz)

	// Register Swagger UI (unauthenticated, configurable)
	if cfg.SwaggerEnabled {
		combinedMux.Handle("/swagger/", swaggerui.Handler("/swagger/"))
	}

	// All other /api/v1/* routes go through protected handler
	combinedMux.Handle("/api/v1/", protectedHandler)

	// WebSocket endpoint (if configured)
	// Use passed-in handler (for lifecycle management) or create one
	if cfg.WebSocketHandler != nil {
		combinedMux.Handle("/ws", cfg.WebSocketHandler)
	} else if cfg.WebSocketHub != nil {
		wsHandler := handlers.NewWebSocketHandler(cfg.WebSocketHub, cfg.AuthService, cfg.RedisClient)
		if cfg.PolicyEnforcer != nil {
			wsHandler.SetPolicyEnforcer(cfg.PolicyEnforcer)
		}
		combinedMux.Handle("/ws", wsHandler)
	}

	// Catch-all: serve embedded frontend (SPA) or empty response
	if cfg.SPAHandler != nil {
		combinedMux.Handle("/", cfg.SPAHandler)
	} else {
		combinedMux.Handle("/", mainMux)
	}

	// Apply minimal middleware to combined mux (RequestID, Security Headers, CORS for all routes)
	var handler http.Handler = combinedMux
	handler = middleware.SecurityHeaders(handler)
	if len(cfg.CORSAllowedOrigins) > 0 {
		handler = middleware.CORS(middleware.CORSConfig{
			AllowedOrigins: cfg.CORSAllowedOrigins,
		})(handler)
	}
	handler = middleware.RequestID(handler)

	return handler
}
