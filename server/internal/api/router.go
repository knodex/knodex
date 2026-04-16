// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sort"

	"github.com/redis/go-redis/v9"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/knodex/knodex/server/internal/api/cookie"
	"github.com/knodex/knodex/server/internal/api/handlers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	swaggerui "github.com/knodex/knodex/server/internal/api/swagger"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/compliance"
	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/drift"
	"github.com/knodex/knodex/server/internal/health"
	"github.com/knodex/knodex/server/internal/history"
	"github.com/knodex/knodex/server/internal/icons"
	"github.com/knodex/knodex/server/internal/kro/children"
	"github.com/knodex/knodex/server/internal/kro/diff"
	kroparser "github.com/knodex/knodex/server/internal/kro/parser"
	kroschema "github.com/knodex/knodex/server/internal/kro/schema"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/repository"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/sso"
	"github.com/knodex/knodex/server/internal/userprefs"
	"github.com/knodex/knodex/server/internal/websocket"
)

// serverNamespace returns the Kubernetes namespace the server is running in.
// Reads POD_NAMESPACE env var, defaulting to "knodex".
func serverNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}
	return "knodex"
}

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
	CategoryService         services.CategoryService         // OSS feature: Auto-discovered category sidebar
	SSOStore                *sso.ProviderStore               // SSO provider management store
	AuditRecorder           audit.Recorder                   // Audit event recorder (nil in OSS builds)
	AuditLoginMiddleware    func(http.Handler) http.Handler  // Enterprise feature: Wraps login routes to record audit events (nil in OSS builds)
	AuditMiddleware         func(http.Handler) http.Handler  // Enterprise feature: Records 401/403 audit events on protected routes (nil in OSS builds)
	AuditAPIService         services.AuditAPIService         // Enterprise feature: Audit trail API
	DriftService            *drift.Service                   // GitOps drift detection service (nil = created internally from RedisClient)
	GraphRevisionWatcher    services.GraphRevisionProvider   // GraphRevision watcher (nil = revisions API disabled)
	DiffService             *diff.DiffService                // Revision diff service (nil = diff API disabled)
	RemoteWatcher           *watcher.RemoteWatcher           // Remote cluster resource watcher (nil = no remote clusters)
	DynamicClient           dynamic.Interface                // Shared dynamic K8s client (QPS=50/Burst=100)
	K8sClient               kubernetes.Interface
	RedisClient             *redis.Client
	AllowedRedirectOrigins  []string      // Allowed redirect origins for OIDC callbacks
	CookieConfig            cookie.Config // Session cookie configuration (Secure, Domain)
	OrganizationFilter      string        // Enterprise org filter for catalog (empty = no filtering)
	Organization            string        // Organization identity for settings endpoint (from KNODEX_ORGANIZATION, default "default")
	SwaggerEnabled          bool          // Enable Swagger UI at /swagger/ (default: false, env: SWAGGER_UI_ENABLED)
}

// RouterResult holds the HTTP handler and resources that need lifecycle management.
type RouterResult struct {
	Handler          http.Handler
	UserRateLimiters []*middleware.UserRateLimiter
}

// Note: permissionServiceAdapter removed - all authorization uses CasbinAuthz exclusively.

// NewRouterWithConfig creates the HTTP router with custom configuration
func NewRouterWithConfig(healthChecker *health.Checker, rgdWatcher *watcher.RGDWatcher, instanceTracker *watcher.InstanceTracker, schemaExtractor *kroschema.Extractor, cfg RouterConfig) RouterResult {
	// Track user rate limiters for shutdown
	var userRateLimiters []*middleware.UserRateLimiter

	// Create main mux for catch-all routes
	mainMux := http.NewServeMux()

	// Create protected mux for authenticated routes
	protectedMux := http.NewServeMux()

	// Create services for service-based architecture
	logger := slog.Default()

	// Authorization service consolidates authorization logic from handlers
	var authService *services.AuthorizationService
	if cfg.PolicyEnforcer != nil || cfg.PermissionService != nil {
		authCfg := services.AuthorizationServiceConfig{
			PolicyEnforcer: cfg.PolicyEnforcer,
			Logger:         logger,
		}
		// Avoid Go interface nil gotcha: only set NamespaceProvider if not nil
		// (a nil *PermissionService assigned to an interface is not == nil)
		if cfg.PermissionService != nil {
			authCfg.NamespaceProvider = cfg.PermissionService
		}
		authService = services.NewAuthorizationService(authCfg)
	}

	// Catalog service encapsulates RGD business logic
	// Note: Only set InstanceCounter if not nil to avoid Go interface nil gotcha
	// (a nil *InstanceTracker assigned to an interface is not == nil)
	catalogConfig := services.CatalogServiceConfig{
		RGDProvider:        rgdWatcher,
		PolicyEnforcer:     cfg.PolicyEnforcer,
		RedisClient:        cfg.RedisClient,
		Logger:             logger,
		OrganizationFilter: cfg.OrganizationFilter,
	}
	if instanceTracker != nil {
		catalogConfig.InstanceCounter = instanceTracker
	}
	catalogService := services.NewCatalogService(catalogConfig)

	// Create icons registry from embedded built-in set
	iconsRegistry, err := icons.NewRegistry()
	if err != nil {
		logger.Error("failed to load built-in icons registry", "error", err)
		// Fallback to empty registry so icons endpoint still serves (will 404 for all slugs)
		iconsRegistry = icons.NewEmptyRegistry()
	}

	// Load custom icons from labeled ConfigMaps if K8s client is available
	if cfg.K8sClient != nil {
		cmList, cmErr := cfg.K8sClient.CoreV1().ConfigMaps(serverNamespace()).List(
			context.Background(), metav1.ListOptions{
				LabelSelector: "knodex.io/icon-registry=true",
			})
		if cmErr != nil {
			logger.Warn("failed to list icon registry ConfigMaps", "error", cmErr)
		} else if len(cmList.Items) > 0 {
			// Sort alphabetically by name for deterministic first-wins collision resolution
			sort.Slice(cmList.Items, func(i, j int) bool {
				return cmList.Items[i].Name < cmList.Items[j].Name
			})
			entries := make([]icons.ConfigMapEntry, len(cmList.Items))
			for i, cm := range cmList.Items {
				entries[i] = icons.ConfigMapEntry{Name: cm.Name, Data: cm.Data}
			}
			iconsRegistry.LoadFromConfigMaps(entries, logger)
		}
		// No matching ConfigMaps is not an error
	}

	// Load category ordering config from knodex-category-config ConfigMap.
	// Absent ConfigMap or missing/empty 'categories' key → no category sub-nav (nil slice).
	var categoryConfig []services.CategoryEntry
	if cfg.K8sClient != nil {
		cm, cmErr := cfg.K8sClient.CoreV1().ConfigMaps(serverNamespace()).Get(
			context.Background(), "knodex-category-config", metav1.GetOptions{})
		if cmErr != nil {
			if !apierrors.IsNotFound(cmErr) {
				logger.Warn("failed to load category config ConfigMap", "error", cmErr)
			}
			// Not found or error (including 403) → categoryConfig remains nil
		} else {
			yamlData, hasKey := cm.Data["categories"]
			if !hasKey || yamlData == "" {
				logger.Warn("knodex-category-config ConfigMap exists but 'categories' key is missing or empty")
			} else if parseErr := yaml.Unmarshal([]byte(yamlData), &categoryConfig); parseErr != nil {
				logger.Warn("failed to parse category config YAML", "error", parseErr)
				categoryConfig = nil
			} else if len(categoryConfig) == 0 {
				logger.Warn("knodex-category-config ConfigMap parsed successfully but contains no entries")
			} else {
				logger.Info("category config loaded from ConfigMap; server restart required to apply changes",
					"entries", len(categoryConfig))
			}
		}
	}

	iconsHandler := handlers.NewIconsHandler(iconsRegistry)

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

	// Use shared dynamic client from app-level configuration (QPS=50/Burst=100)
	dynamicClient := cfg.DynamicClient

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
		driftService = drift.NewService(cfg.RedisClient, logger, cfg.Organization)
	}

	// Create child resource discovery service (STORY-408)
	childService := children.NewService(dynamicClient, rgdWatcher, instanceTracker, kroparser.NewResourceParser(), rgdWatcher, logger)

	// Wire remote client provider for multi-cluster child resource queries (STORY-421)
	if cfg.RemoteWatcher != nil {
		childService.SetRemoteClientProvider(cfg.RemoteWatcher)

		// On cluster recovery, broadcast instance updates for affected instances (STORY-421)
		if cfg.WebSocketHub != nil {
			wsHub := cfg.WebSocketHub
			cfg.RemoteWatcher.SetOnRecoveryCallback(func(clusterRef string) {
				go func() {
					instances := instanceTracker.Cache().GetByTargetCluster(clusterRef)
					for _, inst := range instances {
						wsHub.BroadcastInstanceUpdate("update", inst.Namespace, inst.Kind, inst.Name, inst, inst.ProjectName)
					}
				}()
			})
		}
	}

	// Create domain-specific instance handlers
	crudHandler := handlers.NewInstanceCRUDHandler(handlers.InstanceCRUDHandlerConfig{
		InstanceTracker:      instanceTracker,
		RGDWatcher:           rgdWatcher,
		DynamicClient:        dynamicClient,
		K8sClient:            cfg.K8sClient,
		AuthService:          authService,
		DeploymentController: deployCtrl,
		RepoService:          cfg.RepositoryService,
		DriftService:         driftService,
		ChildService:         childService,
		AuditRecorder:        cfg.AuditRecorder,
		Logger:               logger,
	})

	deploymentHandler := handlers.NewInstanceDeploymentHandler(handlers.InstanceDeploymentHandlerConfig{
		RGDWatcher:      rgdWatcher,
		InstanceTracker: instanceTracker,
		DynamicClient:   dynamicClient,
		KubeClient:      cfg.K8sClient,
		RepoService:     cfg.RepositoryService,
		HistoryService:  cfg.HistoryService,
		AuditRecorder:   cfg.AuditRecorder,
		Logger:          logger,
	})

	k8sResourceHandler := handlers.NewK8sResourceHandler(dynamicClient, discoveryClient)
	if authService != nil {
		k8sResourceHandler.SetNamespaceAccessProvider(authService)
	}

	// Search handler for unified search across RGDs, instances, and projects
	searchHandler := handlers.NewSearchHandler(handlers.SearchHandlerConfig{
		AuthService:     authService,
		CatalogService:  catalogService,
		InstanceTracker: instanceTracker,
		ProjectService:  cfg.ProjectService,
		Logger:          logger,
	})

	// Protected API v1 routes - Unified search (require authentication)
	protectedMux.HandleFunc("GET /api/v1/search", searchHandler.Search)

	// Protected API v1 routes - RGD catalog (require authentication)
	protectedMux.HandleFunc("GET /api/v1/rgds", rgdHandler.ListRGDs)
	protectedMux.HandleFunc("GET /api/v1/rgds/count", rgdHandler.GetCount)
	protectedMux.HandleFunc("GET /api/v1/rgds/filters", rgdHandler.GetFilters)
	protectedMux.HandleFunc("GET /api/v1/rgds/{name}", rgdHandler.GetRGD)
	relatedHandler := handlers.NewRelatedRGDsHandler(authService, catalogService, logger)
	protectedMux.HandleFunc("GET /api/v1/rgds/{name}/related", relatedHandler.GetRelated)
	protectedMux.HandleFunc("GET /api/v1/rgds/{name}/resources", resourceHandler.GetResourceGraph)
	protectedMux.HandleFunc("GET /api/v1/rgds/{name}/graph", resourceHandler.GetDefinitionGraph)
	protectedMux.HandleFunc("GET /api/v1/rgds/{name}/schema", schemaHandler.GetSchema)
	protectedMux.HandleFunc("POST /api/v1/rgds/{name}/schema/invalidate", schemaHandler.InvalidateSchemaCache)

	// GraphRevision routes (only if watcher is available)
	if cfg.GraphRevisionWatcher != nil {
		revisionHandler := handlers.NewRevisionHandler(handlers.RevisionHandlerConfig{
			Provider:       cfg.GraphRevisionWatcher,
			PolicyEnforcer: cfg.PolicyEnforcer,
			DiffService:    cfg.DiffService,
			Logger:         logger,
		})
		protectedMux.HandleFunc("GET /api/v1/rgds/{name}/revisions", revisionHandler.ListRevisions)
		protectedMux.HandleFunc("GET /api/v1/rgds/{name}/revisions/{revision}", revisionHandler.GetRevision)
		protectedMux.HandleFunc("GET /api/v1/rgds/{name}/revisions/{rev1}/diff/{rev2}", revisionHandler.DiffRevisions)
	}

	// Protected API v1 routes - Instance management (require authentication)
	// Collection endpoints (scope-agnostic, no change)
	protectedMux.HandleFunc("GET /api/v1/instances", crudHandler.ListInstances)
	protectedMux.HandleFunc("GET /api/v1/instances/count", crudHandler.GetCount)

	// K8s-aligned instance routes (STORY-327):
	// Namespaced: /api/v1/namespaces/{namespace}/instances/{kind}/{name}
	// Cluster-scoped: /api/v1/instances/{kind}/{name} (no namespace segment)
	protectedMux.HandleFunc("GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}", crudHandler.GetInstance)
	protectedMux.HandleFunc("GET /api/v1/instances/{kind}/{name}", crudHandler.GetInstance)

	// GitOps monitoring routes (only registered if GitOpsHandler is available)
	// Instance creation with deployment validation middleware
	// K8s-aligned: POST /api/v1/namespaces/{ns}/instances/{kind} (namespaced)
	//              POST /api/v1/instances/{kind} (cluster-scoped)
	if cfg.ProjectService != nil && cfg.PolicyEnforcer != nil {
		deploymentValidator := middleware.DeploymentValidator(middleware.DeploymentValidatorConfig{
			ProjectService: cfg.ProjectService,
			PolicyEnforcer: cfg.PolicyEnforcer,
			Logger:         slog.Default().With("component", "deployment-validator"),
		})
		createHandler := deploymentValidator(http.HandlerFunc(deploymentHandler.CreateInstance))
		protectedMux.Handle("POST /api/v1/namespaces/{namespace}/instances/{kind}", createHandler)
		protectedMux.Handle("POST /api/v1/instances/{kind}", createHandler)
	} else {
		// Fail-closed: return 503 when authorization services are not initialized
		unavailableHandler := func(w http.ResponseWriter, r *http.Request) {
			response.ServiceUnavailable(w, "instance creation temporarily unavailable")
		}
		protectedMux.HandleFunc("POST /api/v1/namespaces/{namespace}/instances/{kind}", unavailableHandler)
		protectedMux.HandleFunc("POST /api/v1/instances/{kind}", unavailableHandler)
	}

	// Preflight dry-run — validates via Kubernetes server-side dry-run, no resources created
	protectedMux.HandleFunc("POST /api/v1/namespaces/{namespace}/instances/{kind}/preflight", deploymentHandler.PreflightInstance)
	protectedMux.HandleFunc("POST /api/v1/instances/{kind}/preflight", deploymentHandler.PreflightInstance)

	protectedMux.HandleFunc("PATCH /api/v1/namespaces/{namespace}/instances/{kind}/{name}", crudHandler.UpdateInstance)
	protectedMux.HandleFunc("DELETE /api/v1/namespaces/{namespace}/instances/{kind}/{name}", crudHandler.DeleteInstance)
	protectedMux.HandleFunc("PATCH /api/v1/instances/{kind}/{name}", crudHandler.UpdateInstance)
	protectedMux.HandleFunc("DELETE /api/v1/instances/{kind}/{name}", crudHandler.DeleteInstance)

	// Protected API v1 routes - Deployment history (require authentication)
	if cfg.HistoryService != nil {
		historyHandler := handlers.NewHistoryHandler(cfg.HistoryService, cfg.GraphRevisionWatcher, logger)
		// Namespaced history routes
		protectedMux.HandleFunc("GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}/history", historyHandler.GetHistory)
		protectedMux.HandleFunc("GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}/history/export", historyHandler.ExportHistory)
		protectedMux.HandleFunc("GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}/timeline", historyHandler.GetTimeline)
		// Cluster-scoped history routes
		protectedMux.HandleFunc("GET /api/v1/instances/{kind}/{name}/history", historyHandler.GetHistory)
		protectedMux.HandleFunc("GET /api/v1/instances/{kind}/{name}/history/export", historyHandler.ExportHistory)
		protectedMux.HandleFunc("GET /api/v1/instances/{kind}/{name}/timeline", historyHandler.GetTimeline)
	}

	// Instance graph sub-resource routes (STORY-331)
	protectedMux.HandleFunc("GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}/graph", crudHandler.GetInstanceGraph)
	protectedMux.HandleFunc("GET /api/v1/instances/{kind}/{name}/graph", crudHandler.GetInstanceGraph)

	// Instance child resources sub-resource routes (STORY-408)
	protectedMux.HandleFunc("GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}/children", crudHandler.GetInstanceChildren)
	protectedMux.HandleFunc("GET /api/v1/instances/{kind}/{name}/children", crudHandler.GetInstanceChildren)

	// Instance K8s events — on-demand from K8s API (instance + child resources)
	protectedMux.HandleFunc("GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}/events", crudHandler.GetInstanceEvents)
	protectedMux.HandleFunc("GET /api/v1/instances/{kind}/{name}/events", crudHandler.GetInstanceEvents)

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

		// Resource aggregation across clusters (requires RemoteWatcher)
		if cfg.RemoteWatcher != nil {
			resourceAggHandler := handlers.NewResourceAggregationHandler(cfg.ProjectService, cfg.PolicyEnforcer, cfg.RemoteWatcher)
			protectedMux.HandleFunc("GET /api/v1/projects/{name}/resources", resourceAggHandler.ListProjectResources)
		}
	}

	// Protected API v1 routes - Secrets management (OSS feature)
	if cfg.K8sClient != nil && cfg.PolicyEnforcer != nil {
		secretsHandler := handlers.NewSecretsHandler(handlers.SecretsHandlerConfig{
			K8sClient:     cfg.K8sClient,
			DynamicClient: dynamicClient,
			Enforcer:      cfg.PolicyEnforcer,
			Recorder:      cfg.AuditRecorder,
			NSAccess:      authService,
		})
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
		if cfg.ComplianceService != nil {
			accountHandler.RegisterEnterpriseResource("compliance")
		}
		protectedMux.HandleFunc("GET /api/v1/account/can-i/{resource}/{action}/{subresource}", accountHandler.CanI)
		protectedMux.HandleFunc("GET /api/v1/account/info", accountHandler.Info)
	}

	// Protected API v1 routes - User preferences (require authentication)
	if cfg.RedisClient != nil {
		prefsStore := userprefs.NewRedisStore(cfg.RedisClient)
		prefsHandler := userprefs.NewHandler(prefsStore, logger)
		protectedMux.HandleFunc("GET /api/v1/users/preferences", prefsHandler.GetPreferences)
		protectedMux.HandleFunc("PUT /api/v1/users/preferences", prefsHandler.PutPreferences)
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
	// Protected API v1 routes - Compliance validate (works in both OSS and EE)
	// OSS builds return pass unconditionally; EE builds perform actual compliance checks
	complianceValidateHandler := handlers.NewComplianceValidateHandler(compliance.GetChecker(), logger)
	protectedMux.HandleFunc("POST /api/v1/compliance/validate", complianceValidateHandler.Validate)

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

	// Protected API v1 routes - Categories (OSS feature)
	// Auto-discovers categories from knodex.io/category annotations on live RGDs.
	// Per-category Casbin filter (rgds/{category}/*, get) — always registered.
	categoriesHandler := handlers.NewCategoriesHandler(cfg.CategoryService, cfg.PolicyEnforcer, iconsRegistry, categoryConfig, logger)
	protectedMux.HandleFunc("GET /api/v1/categories", categoriesHandler.ListCategories)
	protectedMux.HandleFunc("GET /api/v1/categories/{slug}", categoriesHandler.GetCategory)

	// Protected API v1 routes - SSO provider management (require authentication + settings access)
	if cfg.SSOStore != nil {
		ssoHandler := handlers.NewSSOSettingsHandler(cfg.SSOStore, cfg.AuditRecorder, cfg.PolicyEnforcer)

		// SSO mutation rate limiter: burst of 5, then 1 per minute sustained (SSRF probe / DoS prevention)
		ssoMutationLimiter, ssoRateLimiter := middleware.UserRateLimit(middleware.UserRateLimitConfig{
			RequestsPerMinute: 1, // 1/min sustained rate, burst of 5 initial requests
			BurstSize:         5,
			FallbackToIP:      false,
			RetryAfterSeconds: 60,
		})
		userRateLimiters = append(userRateLimiters, ssoRateLimiter)

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
		apiRateLimitMw, apiRateLimiter := middleware.UserRateLimit(middleware.UserRateLimitConfig{
			RequestsPerMinute: rateLimitReqPerMin,
			BurstSize:         rateLimitBurst,
			FallbackToIP:      false, // Only rate limit authenticated users
			TrustedProxies:    cfg.RateLimitTrustedProxies,
			RetryAfterSeconds: 1, // General API: 100 req/min ≈ 0.6s between tokens
		})
		userRateLimiters = append(userRateLimiters, apiRateLimiter)
		protectedHandler = apiRateLimitMw(protectedHandler)
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

	// Apply audit middleware for 401/403 event recording (enterprise feature).
	// Placed outside Auth so it observes auth/authz rejection responses.
	if cfg.AuditMiddleware != nil {
		protectedHandler = cfg.AuditMiddleware(protectedHandler)
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

	// Icons endpoint (unauthenticated — SVGs are public assets, no auth required)
	combinedMux.HandleFunc("GET /api/v1/icons/{slug}", iconsHandler.GetIcon)

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
	handler = middleware.Recovery(handler)

	return RouterResult{
		Handler:          handler,
		UserRateLimiters: userRateLimiters,
	}
}
