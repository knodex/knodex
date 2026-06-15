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
	"errors"
	"fmt"
	"io"
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
	db "github.com/knodex/knodex/server/internal/database"
	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/drift"
	"github.com/knodex/knodex/server/internal/groups"
	"github.com/knodex/knodex/server/internal/health"
	"github.com/knodex/knodex/server/internal/history"
	identitypg "github.com/knodex/knodex/server/internal/identity/postgres"
	"github.com/knodex/knodex/server/internal/kagent"
	"github.com/knodex/knodex/server/internal/kagent/runs"
	"github.com/knodex/knodex/server/internal/kro"
	krodiff "github.com/knodex/knodex/server/internal/kro/diff"
	kroschema "github.com/knodex/knodex/server/internal/kro/schema"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/repository"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/services/wrapper"
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
// defaultOrg scopes persisted violation history to a single tenant in Phase 1.
type ComplianceInitFunc func(ctx context.Context, k8sCfg *config.Kubernetes, wsHub *websocket.Hub, defaultOrg string, complianceCfg *config.Compliance) services.ComplianceService

// ViolationHistoryInitFunc is the signature for enterprise violation history service initialization.
type ViolationHistoryInitFunc func() services.ViolationHistoryService

// CategoryInitFunc is the signature for OSS category service initialization.
// Categories are auto-discovered from knodex.io/category RGD annotations — no configPath needed.
type CategoryInitFunc func(rgdWatcher *watcher.RGDWatcher) services.CategoryService

// AuditRecorderInitFunc is the signature for enterprise audit recorder initialization.
// defaultOrg is the tenant scope applied to events that arrive without an explicit
// Organization field (Phase 1: every event uses cfg.Organization).
type AuditRecorderInitFunc func(ctx context.Context, k8sClient kubernetes.Interface, namespace string, defaultOrg string) audit.Recorder

// AgentRunStoreWrapFunc is the signature for the EE agent-run-store audit
// decorator (Story 49.5). In EE builds it wraps the OSS run store so every
// successful Create/Update additionally emits an audit event through the
// shared recorder; in OSS builds it is the identity function (zero Postgres
// interaction in the run path). Applied in Run() at the one point where both
// the run store and the audit recorder exist.
type AgentRunStoreWrapFunc func(inner runs.Store, recorder audit.Recorder) runs.Store

// AgentSpecValidatorInitFunc is the signature for the EE agent spec
// validator (Story 50.3): in EE builds it constructs the Gatekeeper
// AdmissionReview validator for the RGD Builder completion path; in OSS
// builds it returns nil (no policy validation). Invoked in
// initEnterpriseServices AFTER the compliance init so gatekeeper's service
// registration has run.
type AgentSpecValidatorInitFunc func(k8sCfg *config.Kubernetes, license services.LicenseService) handlers.AgentSpecValidator

// AuditLoginMiddlewareInitFunc is the signature for enterprise audit login middleware initialization.
// In EE builds, this creates an AuditService + AuditConfigWatcher and returns the login middleware.
// Returns nil in OSS builds (login routes are not wrapped with audit middleware).
type AuditLoginMiddlewareInitFunc func(ctx context.Context, k8sClient kubernetes.Interface, namespace string, defaultOrg string) func(http.Handler) http.Handler

// AuditMiddlewareInitFunc is the signature for enterprise audit middleware initialization.
// This middleware captures 401/403 responses and records audit events for authentication
// failures and authorization denials.
type AuditMiddlewareInitFunc func(ctx context.Context, k8sClient kubernetes.Interface, namespace string, defaultOrg string) func(http.Handler) http.Handler

// AuditAPIServiceInitFunc is the signature for enterprise audit API service initialization.
// The ctx parameter controls the config watcher lifecycle — cancel it to stop the watcher.
// The recorder parameter enables audit event recording for config changes (FR-AT6).
type AuditAPIServiceInitFunc func(ctx context.Context, k8sClient kubernetes.Interface, namespace string, defaultOrg string, enforcer rbac.PolicyEnforcer, recorder audit.Recorder) services.AuditAPIService

// DatabaseManagerInitFunc is the signature for enterprise database manager initialization.
// In EE builds, creates pgx connection pools and runs schema migrations.
// In OSS builds, returns nil, nil (no-op — zero database dependency).
type DatabaseManagerInitFunc func(ctx context.Context, cfg *config.Config) (io.Closer, error)

// SeatReconcilerInitFunc is the signature for the EE license seat reconciler
// initialization. Story 15.2 (R5-2): the reconciler reads the canonical identity
// roster via IdentityService.BilledSeatCount (entitlement-based, uniform), runs
// its first poll synchronously so GetSeatUsage is populated before the first
// HTTP request, wires itself into the LicenseService via SetUsageProvider, and
// returns the reconciler's Run loop for app.Run to launch on runCtx. In OSS /
// EE-builds-without-Postgres (nil identitySvc), it returns nil and the license
// service keeps returning the cold-start sentinel.
//
// orgID is the static single-org scope (cfg.Organization) used for metrics labels.
type SeatReconcilerInitFunc func(licSvc services.LicenseService, identitySvc services.IdentityService, orgID string, logger *slog.Logger) func(context.Context)

// IdentityHooksInitFunc builds the post-commit identity hooks (Story 15.2 AC16).
// On EE it returns audit-emitting hooks bound to the recorder; on OSS it returns
// the zero-value (no emission). The composition root is the only edition-aware
// place (AC24).
type IdentityHooksInitFunc func(recorder audit.Recorder, logger *slog.Logger) services.IdentityHooks

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
	agentRunStoreWrapFunc        AgentRunStoreWrapFunc
	agentSpecValidatorInitFunc   AgentSpecValidatorInitFunc
	auditLoginMiddlewareInitFunc AuditLoginMiddlewareInitFunc
	auditMiddlewareInitFunc      AuditMiddlewareInitFunc
	auditAPIServiceInitFunc      AuditAPIServiceInitFunc
	databaseManagerInitFunc      DatabaseManagerInitFunc
	seatReconcilerInitFunc       SeatReconcilerInitFunc
	identityHooksInitFunc        IdentityHooksInitFunc

	// identitySourceKind is the federated_identities.source_kind stamped on JIT
	// rows ("oidc_jit"). Set from the composition root (AC24).
	identitySourceKind string

	// identityService is the composed canonical user-persistence port (Story 15.2).
	// Built in Run() from the database manager's identity pool and stored here for
	// Story 15.2a's router wiring (the Users API). Nil until the pool is up.
	identityService services.IdentityService

	// teamStore holds the in-memory Team CRD lookup table (team name → OIDC
	// groups) populated by the cluster-scoped TeamWatcher. Story 10.2 injects
	// it into the policy generator to resolve roles[].teams[]. It is a passive
	// lookup table and is NOT wired into Casbin in this story.
	teamStore *rbac.TeamStore

	// teamService provides cluster-scoped Team CRUD for the operator-gated
	// /api/v1/teams API (Story 10.4). Writes mutate the cluster object; the
	// teamStore-backed TeamWatcher above re-resolves policies automatically.
	teamService rbac.TeamServiceInterface

	// agentRunStore persists agent invocation run records (Story 49.4). OSS
	// defaults to the Redis store in Run(); EE (49.5) overlays a Postgres
	// implementation via SetAgentRunStore — the same seam as SetLicenseService.
	agentRunStore runs.Store

	// agentSpecValidator validates RGD Builder output against Gatekeeper
	// policy (Story 50.3). Built in initEnterpriseServices from the EE init
	// func; nil in OSS builds and on EE construction failure — the invoke
	// handler is nil-safe.
	agentSpecValidator handlers.AgentSpecValidator
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

// SetAgentRunStore sets the agent run record store (Story 49.4 / 49.5 EE
// seam). Must be called before Run(). When unset, Run() defaults to the
// Redis-backed OSS store (nil when Redis is unavailable — handlers fail
// soft/closed accordingly).
func (a *App) SetAgentRunStore(s runs.Store) {
	a.agentRunStore = s
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

// SetAgentRunStoreWrapFunc registers the build-tag-dispatched agent-run-store
// audit decorator (Story 49.5). In EE builds the wrap layers audit emission
// over the run store; in OSS builds it is the identity function. The direct
// SetAgentRunStore setter stays untouched — the wrap composes over whatever
// store is present when Run() applies it.
func (a *App) SetAgentRunStoreWrapFunc(fn AgentRunStoreWrapFunc) {
	a.agentRunStoreWrapFunc = fn
}

// SetAgentSpecValidatorInitFunc registers the build-tag-dispatched agent
// spec validator factory (Story 50.3). In EE builds it builds the Gatekeeper
// AdmissionReview validator; in OSS builds it returns nil. Invoked inside
// initEnterpriseServices AFTER the compliance init (gatekeeper.RegisterService
// must have run for the validator to see constraint metadata).
func (a *App) SetAgentSpecValidatorInitFunc(fn AgentSpecValidatorInitFunc) {
	a.agentSpecValidatorInitFunc = fn
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

// SetDatabaseManagerInitFunc registers a factory function for initializing the EE database manager.
// In EE builds, creates pgx connection pools and runs schema migrations.
// In OSS builds, the factory returns nil, nil (no-op).
func (a *App) SetDatabaseManagerInitFunc(fn DatabaseManagerInitFunc) {
	a.databaseManagerInitFunc = fn
}

// SetSeatReconcilerInitFunc registers a factory for the EE license seat
// reconciler (STORY-465 AC #9). In EE builds the factory wires the reconciler
// into the license service (SetUsageProvider) and returns its Run loop; in OSS
// builds the factory returns nil (the license service keeps returning the
// cold-start sentinel). Invoked during Run() AFTER the audit recorder init —
// which is the call that constructs the shared seat store on top of the audit
// pool (AC #2 same-transaction wiring).
func (a *App) SetSeatReconcilerInitFunc(fn SeatReconcilerInitFunc) {
	a.seatReconcilerInitFunc = fn
}

// SetIdentityHooksInitFunc registers the factory for the post-commit identity
// hooks (Story 15.2). EE wires audit-emitting hooks; OSS wires the zero-value.
func (a *App) SetIdentityHooksInitFunc(fn IdentityHooksInitFunc) {
	a.identityHooksInitFunc = fn
}

// SetIdentitySourceKind sets the federated_identities.source_kind for JIT rows
// (build-tag constant from the composition root — AC24).
func (a *App) SetIdentitySourceKind(kind string) {
	a.identitySourceKind = kind
}

// IdentityService returns the composed canonical user-persistence port, or nil
// before Run() builds it. Exposed for Story 15.2a's Users API router wiring.
func (a *App) IdentityService() services.IdentityService {
	return a.identityService
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

	// NOTE (story 12.1): the story-8.11 claims-parity boot probe
	// (auth.VerifyClaimsParityAtBoot) was retired here. Per ADR
	// adr-cloud-team-membership-keycloak-groups (Accepted), cloud tokens carry
	// team groups only — sourced natively by Keycloak's group-membership mapper —
	// so Knodex no longer fabricates kx-org-*/kx-proj-* claims and the three-place
	// formatGroups invariant (and its boot probe) no longer exists.

	orgSource := "default"
	if _, ok := os.LookupEnv("KNODEX_ORGANIZATION"); ok {
		orgSource = "KNODEX_ORGANIZATION"
	}
	slog.Info("organization identity configured", "organization", cfg.Organization, "source", orgSource)
	slog.Info("organization catalog filter", "active", a.organizationFilter != "", "organization", a.organizationFilter)

	// Database manager init (EE: pgx pool + migrations; OSS: no-op).
	// Must run before any service that needs the DB (STORY-444, STORY-445).
	// The defer fires last among Run()'s deferred statements, so all writers
	// (audit recorder, watchers) drain before the pools close.
	if a.databaseManagerInitFunc != nil {
		dbCloser, err := a.databaseManagerInitFunc(ctx, cfg)
		if err != nil {
			return fmt.Errorf("database initialization failed: %w", err)
		}
		if dbCloser != nil {
			defer func() {
				if closeErr := dbCloser.Close(); closeErr != nil {
					slog.Warn("failed to close database manager", "error", closeErr)
				}
			}()
		}
	}

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

	// Wrapper registry: ConfigMap-backed Kind→RGD mapping that the project handler
	// consults to route Project creation through an operator-defined wrapper RGD.
	var wrapperStore *wrapper.Store
	var wrapperWatcher *wrapper.Watcher
	var wrapperHelpers *wrapper.Helpers

	if k8sClient != nil && dynamicClient != nil {
		// Create audit logger for RBAC operations
		auditLogger := rbac.NewAuditLogger(slog.Default())

		// Create project service scoped to Knodex namespace
		projectService = rbac.NewProjectService(k8sClient, dynamicClient, cfg.KnodexNamespace)
		slog.Info("project service initialized", "namespace", cfg.KnodexNamespace)

		// Create the in-memory Team CRD lookup table (team name → OIDC groups)
		// BEFORE the policy enforcer / project service consume it, so it can be
		// injected as the TeamResolver (Story 10.2 roles[].teams[] → groups). The
		// cluster-scoped TeamWatcher (created below) populates it. Independent of
		// Casbin: a Team yields no policy on its own — it only resolves to groups.
		a.teamStore = rbac.NewTeamStore()
		projectService.SetTeamResolver(a.teamStore)
		slog.Info("team store initialized and wired into project service as team resolver")

		// Namespace-scoped Team CRUD service backing the operator-gated
		// /api/v1/teams API (Story 10.4). Symmetric with ProjectService —
		// writes mutate the Team object in the install namespace; the
		// TeamWatcher (created below) observes the change and the debounced
		// SyncPolicies re-resolves roles[].teams[] — no second re-sync path
		// is added (NFR-T1).
		a.teamService = rbac.NewTeamService(dynamicClient, cfg.KnodexNamespace)
		slog.Info("team service initialized", "namespace", cfg.KnodexNamespace)

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

		// Wrapper registry: same namespace as Projects (KnodexNamespace). Watcher
		// is started later alongside the other watchers; helpers are wired after
		// the InstanceTracker is constructed so the GVRResolver dependency is satisfied.
		wrapperStore = wrapper.NewStore(k8sClient, cfg.KnodexNamespace)
		wrapperWatcher = wrapper.NewWatcher(k8sClient, cfg.KnodexNamespace, slog.Default())
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

			// Inject the team resolver so roles[].teams[] are expanded into OIDC
			// groups when generating Casbin grouping policies (Story 10.2). Teams
			// resolve to groups only — no separate team enforcement (NFR-T1).
			if a.teamStore != nil {
				peOpts = append(peOpts, rbac.WithTeamResolver(a.teamStore))
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

	// Create the cluster-scoped Team CRD watcher. Its in-memory store was created
	// above (a.teamStore) so it could be injected as the TeamResolver into the
	// policy enforcer + project service. Independent of Casbin: a Team produces
	// no policy on its own (Story 10.2 resolves roles[].teams[] → groups →
	// policies). When a team's groups change or it is deleted, OnChange triggers
	// a coalesced Casbin policy re-sync so bound projects reflect the new groups.
	var teamWatcher rbac.TeamWatcher
	// teamChangeCh coalesces team-change signals; a single drainer goroutine
	// (started in the watcher-start section, where runCtx exists) debounces them
	// into at most one SyncPolicies per window. Buffered size 1 = coalescing.
	var teamChangeCh chan struct{}
	if dynamicClient != nil && a.teamStore != nil {
		teamWatcherConfig := rbac.TeamWatcherConfig{
			ResyncPeriod: rbac.DefaultTeamWatcherResyncPeriod,
			Logger:       slog.Default(),
		}
		// Only wire re-sync when there is an enforcer to re-sync; otherwise the
		// store still populates but no policy regeneration is needed.
		if policyEnforcer != nil {
			teamChangeCh = make(chan struct{}, 1)
			teamWatcherConfig.OnChange = func(teamName string) {
				// Non-blocking enqueue: must not block the informer goroutine. If a
				// signal is already pending, coalesce (the drainer re-syncs ALL
				// projects, so one pending signal covers any number of changes).
				select {
				case teamChangeCh <- struct{}{}:
				default:
				}
			}
		}
		teamWatcher = rbac.NewTeamWatcher(dynamicClient, a.teamStore, cfg.KnodexNamespace, teamWatcherConfig)
		slog.Info("team watcher created", "namespace", cfg.KnodexNamespace)
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
	var observedGroupsStore *groups.RedisStore // Story 10.3: nil when Redis unavailable
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
				return fmt.Errorf("admin password bootstrap failed: %w", err)
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
			JWTExpiry:                 cfg.Auth.JWTExpiry,
			LocalAdminUsername:        cfg.Auth.AdminUsername,
			LocalAdminPassword:        adminPassword,
			LocalLoginEnabled:         cfg.Auth.LocalLoginEnabled,
			OIDCEnabled:               cfg.Auth.OIDCEnabled,
			OIDCProviders:             oidcProviders,
			BootstrapProjectName:      cfg.BootstrapProjectName,
			BootstrapProjectNamespace: cfg.BootstrapProjectNamespace,
		}

		var authErr error
		authService, authErr = auth.NewService(authConfig, accountStore, projectService, k8sClient, redisClient, casbinEnforcer)
		if authErr != nil {
			slog.Error("failed to create auth service - server cannot start without authentication", "error", authErr)
			return fmt.Errorf("auth service initialization failed: %w", authErr)
		}
		slog.Info("auth service initialized",
			"oidc_enabled", cfg.Auth.OIDCEnabled,
			"oidc_providers", len(oidcProviders),
			"casbin_enabled", casbinEnforcer != nil,
		)

		// Story 10.3: passive observed-groups discovery. Record the distinct OIDC
		// group strings seen at login (in auth.Service.GenerateTokenWithGroups) so
		// the team/role editor can offer a typeahead of real groups. Backed by the
		// same Redis client; guarded on redisClient != nil like auth construction.
		if redisClient != nil {
			observedGroupsStore = groups.NewRedisStore(redisClient)
			authService.SetObservedGroupsStore(observedGroupsStore)
			slog.Info("observed-groups discovery store initialized")
		}
	} else if redisClient == nil {
		slog.Warn("Redis client not available, authentication services disabled")
	}

	// Initialize OIDC service (if OIDC is enabled)
	var oidcService *auth.OIDCService
	// Hoisted so the identity service (built after the audit recorder, below) can
	// inject the ObserveLogin adapter via SetIdentityObserver (Story 15.2).
	var oidcProvisioningService *auth.OIDCProvisioningService
	if cfg.Auth.OIDCEnabled && authService != nil && redisClient != nil && projectService != nil {
		authConfig := &auth.Config{
			JWTExpiry:     cfg.Auth.JWTExpiry,
			OIDCEnabled:   cfg.Auth.OIDCEnabled,
			OIDCProviders: ssoProviders,
		}

		groupMapper := auth.NewGroupMapper(cfg.Auth.OIDCGroupMappings)
		authService.SetGroupMapper(groupMapper)

		oidcProvisioningService = auth.NewOIDCProvisioningService(projectService, groupMapper, casbinEnforcer, cfg.Auth.DefaultRole)

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

	// Wire wrapper helpers now that the RGD watcher and instance tracker (GVR resolver)
	// are available. Wrapper instances live in the same namespace as Projects.
	if wrapperWatcher != nil && rgdWatcher != nil && instanceTracker != nil && dynamicClient != nil {
		wrapperHelpers = wrapper.NewHelpers(wrapperWatcher, rgdWatcher, instanceTracker, dynamicClient, cfg.KnodexNamespace)
		slog.Info("wrapper helpers initialized", "namespace", cfg.KnodexNamespace)
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

	// Build the canonical identity store (Story 15.2). Edition awareness enters
	// ONLY here (AC24): SourceKind is a build-tag constant; Hooks are EE
	// audit-emitting or OSS zero-value. The store is the single IdentityService
	// impl on every edition, backed by the database manager's identity pool. When
	// the pool is unavailable the service stays nil and ObserveLogin/seat counting
	// degrade gracefully (the startup fail-fast in InitDatabaseManager — AC22 —
	// means a nil pool here only happens in tests / OSS-without-DB unit paths).
	if mgr := db.GetManager(); mgr != nil && mgr.IdentityPool() != nil {
		hooks := services.IdentityHooks{}
		if a.identityHooksInitFunc != nil {
			hooks = a.identityHooksInitFunc(auditRecorder, logger)
		}
		sourceKind := a.identitySourceKind
		if sourceKind == "" {
			sourceKind = services.SourceKindOIDCJIT
		}
		a.identityService = identitypg.New(mgr.IdentityPool(), identitypg.Config{
			SourceKind: sourceKind,
			Hooks:      hooks,
			Logger:     logger,
			OrgID:      cfg.Organization,
		})
		slog.Info("identity store initialized", "source_kind", sourceKind, "org_id", cfg.Organization)

		// Inject the ObserveLogin adapter into the OIDC provisioning service so a
		// successful login materializes the canonical user (best-effort; never
		// fails the login). The flat adapter avoids an auth→services import cycle.
		if oidcProvisioningService != nil {
			identitySvc := a.identityService
			org := cfg.Organization
			oidcProvisioningService.SetIdentityObserver(func(ctx context.Context, issuer, sub, email, displayName string, emailVerified bool) error {
				_, err := identitySvc.ObserveLogin(ctx, services.ObserveLoginParams{
					OrgID:         org,
					Issuer:        issuer,
					Sub:           sub,
					Email:         email,
					DisplayName:   displayName,
					EmailVerified: emailVerified,
					ProviderKind:  "oidc",
				})
				return err
			})
		}
	} else {
		slog.Warn("identity store not initialized: database manager/identity pool unavailable")
	}

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
		auditLoginMiddleware = a.auditLoginMiddlewareInitFunc(runCtx, k8sClient, namespace, a.cfg.Organization)
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
		auditMiddleware = a.auditMiddlewareInitFunc(runCtx, k8sClient, namespace, a.cfg.Organization)
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
		auditAPIService = a.auditAPIServiceInitFunc(runCtx, k8sClient, namespace, a.cfg.Organization, policyEnforcer, auditRecorder)
	}
	if auditAPIService != nil {
		slog.Info("audit API service initialized (enterprise feature)")
	}

	// Create shared drift detection service (used by both CRUD handler and InstanceTracker callback)
	driftSvc := drift.NewService(redisClient, slog.Default(), cfg.Organization)

	// Story 10.3: expose the observed-groups store to the router as a List-er
	// interface. Declared as the interface (not the concrete *RedisStore) so a nil
	// store yields a true nil interface — the handler then returns an empty list.
	var observedGroupsLister handlers.ObservedGroupsLister
	if observedGroupsStore != nil {
		observedGroupsLister = observedGroupsStore
	}

	// Story 49.1: kagent presence checker for the /agents hub. Declared as the
	// handler interface so a missing K8s client yields a true nil — the status
	// endpoint then reports a degraded payload instead of failing.
	var kagentChecker handlers.KagentPresenceChecker
	if k8sClient != nil {
		kagentChecker = kagent.NewChecker(k8sClient.Discovery(), cfg.KagentControllerBaseURL)
	}

	// Story 49.4: agent run store + A2A invoker. The store defaults to the
	// Redis-backed OSS implementation unless an overlay injected one via
	// SetAgentRunStore (the 49.5 EE seam). The A2A client reuses the same
	// kagent controller base URL as the presence checker — no new config.
	if a.agentRunStore == nil && redisClient != nil {
		a.agentRunStore = runs.NewRedisStore(redisClient)
	}
	// Story 49.5: apply the EE audit decorator (identity in OSS builds) at the
	// one point where both the run store and the audit recorder exist.
	if a.agentRunStoreWrapFunc != nil {
		a.agentRunStore = a.agentRunStoreWrapFunc(a.agentRunStore, auditRecorder)
	}
	agentInvoker := kagent.NewA2AClient(cfg.KagentControllerBaseURL)

	// Story 50.1: full-result store for the built-in conversational lane.
	// Deliberately NOT behind the 49.5 EE seam — results are ephemeral
	// conversation payloads outside audit scope. Declared as the interface so
	// a missing Redis client yields a true nil (503/404 fail paths).
	var agentRunResultStore runs.ResultStore
	if redisClient != nil {
		agentRunResultStore = runs.NewRedisResultStore(redisClient)
	}

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
		ObservedGroupsStore:     observedGroupsLister, // Story 10.3 (nil interface when Redis unavailable)
		ProjectService:          projectService,
		TeamService:             a.teamService,       // Story 10.4 (nil when dynamic client unavailable)
		IdentityService:         a.IdentityService(), // Story 15.8: operator Users API over the canonical roster
		HistoryService:          historyService,
		NamespaceService:        namespaceService,
		K8sClient:               k8sClient,
		RedisClient:             redisClient,
		LicenseService:          a.licenseService,
		ComplianceService:       a.complianceService,
		ViolationHistoryService: a.violationHistoryService,
		CategoryService:         a.categoryService,
		SSOStore:                ssoStore,
		WrapperStore:            wrapperStore,
		WrapperHelpers:          wrapperHelpers,
		AllowedRedirectOrigins:  cfg.Auth.AllowedRedirectOrigins,
		CookieConfig:            cookie.Config{Secure: cfg.Cookie.Secure, Domain: cfg.Cookie.Domain},
		OrganizationFilter:      a.organizationFilter,     // EE catalog filtering (empty = no filter)
		Organization:            cfg.Organization,         // Display identity for GET /api/v1/settings
		SwaggerEnabled:          cfg.SwaggerEnabled,       // Serve Swagger UI at /swagger/ (SWAGGER_UI_ENABLED)
		CatalogPackageFilter:    cfg.CatalogPackageFilter, // Per-package category config filtering
		AuditRecorder:           auditRecorder,
		AuditLoginMiddleware:    auditLoginMiddleware,
		AuditMiddleware:         auditMiddleware,
		AuditAPIService:         auditAPIService,
		DriftService:            driftSvc,
		GraphRevisionWatcher:    graphRevisionProvider(graphRevisionWatcher),
		DiffService:             diffService,
		RemoteWatcher:           remoteWatcher,
		DynamicClient:           dynamicClient,
		KagentChecker:           kagentChecker,        // Story 49.1: kagent presence for the /agents hub (nil without K8s client)
		AgentRunStore:           a.agentRunStore,      // Story 49.4: run history store (nil without Redis → fail-soft/closed)
		AgentInvoker:            agentInvoker,         // Story 49.4: kagent A2A invocation client
		AgentRunResultStore:     agentRunResultStore,  // Story 50.1: full-result store for built-in conversations (nil without Redis)
		AgentSpecValidator:      a.agentSpecValidator, // Story 50.3: EE Gatekeeper policy validation of generated specs (nil in OSS)
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

	// Start the cluster-scoped Team CRD watcher (populates the TeamStore).
	if teamWatcher != nil {
		go func() {
			if err := teamWatcher.Start(runCtx); err != nil {
				slog.Error("team watcher stopped with error", "error", err)
			}
		}()
		slog.Info("team watcher started")
	}

	// Drainer goroutine: when a Team changes (groups updated or team deleted),
	// the watcher's OnChange enqueues a signal; we debounce a burst and re-sync
	// all project policies so Casbin grouping policies reflect the new team→group
	// resolution (Story 10.2 AC #3/#6). A coarse "re-sync all on any team change"
	// is fine at pre-release scale. Runs off the informer goroutine so OnChange
	// never blocks the watcher.
	if teamChangeCh != nil && policyEnforcer != nil {
		go func() {
			const teamChangeDebounce = 2 * time.Second
			for {
				select {
				case <-runCtx.Done():
					return
				case <-teamChangeCh:
					// Debounce: coalesce a burst (e.g. the informer's initial list)
					// into a single re-sync.
					timer := time.NewTimer(teamChangeDebounce)
					select {
					case <-runCtx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
					if err := policyEnforcer.SyncPolicies(runCtx); err != nil {
						slog.Warn("team-change policy re-sync failed", "error", err)
					} else {
						slog.Info("team-change policy re-sync completed")
					}
				}
			}
		}()
		slog.Info("team-change policy re-sync drainer started")
	}

	// Start the EE license seat reconciler (STORY-465 AC #9, build-tag dispatch).
	// Returns nil in OSS / EE-without-Postgres builds. In EE builds the factory
	// constructs the SeatReconciler on the shared seat store (created by the
	// audit recorder init above), wires it into the LicenseService via
	// SetUsageProvider, and runs the first poll synchronously so GetSeatUsage
	// is populated before the first HTTP request. The returned Run loop drives
	// the 5-minute ticker on runCtx.
	if a.seatReconcilerInitFunc != nil {
		if run := a.seatReconcilerInitFunc(a.licenseService, a.identityService, cfg.Organization, logger); run != nil {
			go run(runCtx)
			slog.Info("license seat reconciler started")
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

	// Start wrapper watcher (watches ConfigMap for resource-wrapper registry changes)
	if wrapperWatcher != nil {
		go func() {
			if err := wrapperWatcher.Start(runCtx); err != nil {
				slog.Error("wrapper watcher stopped with error", "error", err)
			}
		}()
		slog.Info("wrapper ConfigMap watcher started")
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
		instanceTracker.SetOnUpdateCallback(func(action watcher.InstanceAction, group, namespace, kind, name string, instance *models.Instance) {
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
				wsHub.BroadcastDriftUpdate(group, namespace, kind, name, false, projectNamespace)
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

	// Start server in goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		slog.Info("starting server", "address", cfg.Server.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	// Wait for interrupt signal or server error for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case sig := <-quit:
		slog.Info("received shutdown signal", "signal", sig.String())
	case serverErr := <-serverErrCh:
		slog.Error("server error, initiating shutdown", "error", serverErr)
	}
	slog.Info("shutting down server...")

	// Cancel context to stop watchers
	cancel()

	// Stop Redis authorization cache goroutines (pub/sub, fallback cleanup, recovery)
	if redisAuthzCache != nil {
		redisAuthzCache.Stop()
	}

	shutdownServices(server, wsHubCancel, wsHandler, policyCacheManager, auditRecorder,
		ssoWatcher, wrapperWatcher, repoWatcher, graphRevisionWatcher, remoteWatcher, instanceTracker, rgdWatcher, routerResult.UserRateLimiters, redisClient, logger)

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
) func(watcher.InstanceAction, string, string, string, string, *models.Instance) {
	instanceStatusCache := make(map[string]string)
	var statusCacheMu sync.Mutex

	return func(action watcher.InstanceAction, group, namespace, kind, name string, instance *models.Instance) {
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

		wsHub.BroadcastInstanceUpdate(wsAction, group, namespace, kind, name, instance, projectNamespace)

		// Record history events
		historyCtx, historyCancel := context.WithTimeout(context.Background(), historyRecordTimeout)
		defer historyCancel()

		instanceKey := group + "/" + namespace + "/" + kind + "/" + name
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
					if err := historyService.RecordStatusChange(historyCtx, group, namespace, kind, name, previousStatus, currentStatus); err != nil {
						slog.Warn("failed to record instance status change",
							"group", group, "namespace", namespace, "kind", kind, "name", name,
							"old_status", previousStatus, "new_status", currentStatus, "error", err)
					}
				}
				instanceStatusCache[instanceKey] = currentStatus
				statusCacheMu.Unlock()
			}
		case watcher.InstanceActionDelete:
			if err := historyService.RecordDeletion(historyCtx, group, namespace, kind, name, "system"); err != nil {
				slog.Warn("failed to record instance deletion history",
					"group", group, "namespace", namespace, "kind", kind, "name", name, "error", err)
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
		a.complianceService = a.complianceInitFunc(initCtx, &cfg.Kubernetes, wsHub, cfg.Organization, &cfg.Compliance)
	}
	if a.complianceService != nil {
		slog.Info("compliance service initialized (enterprise feature)")
	}

	// Agent spec validator (Story 50.3): MUST run AFTER the compliance init —
	// it reads the gatekeeper service the compliance init registered. Nil in
	// OSS builds and on EE construction failure (the handler is nil-safe).
	if a.agentSpecValidatorInitFunc != nil {
		a.agentSpecValidator = a.agentSpecValidatorInitFunc(&cfg.Kubernetes, a.licenseService)
	}
	if a.agentSpecValidator != nil {
		slog.Info("agent spec validator initialized (enterprise feature)")
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
		auditRecorder = a.auditRecorderInitFunc(initCtx, k8sClient, namespace, a.cfg.Organization)
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
	wrapperWatcher *wrapper.Watcher,
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

	// Stop wrapper watcher
	if wrapperWatcher != nil {
		wrapperWatcher.Stop()
		slog.Info("wrapper watcher stopped")
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

// updateAllRGDInstanceCounts updates instance counts for all RGDs.
func updateAllRGDInstanceCounts(rgdWatcher *watcher.RGDWatcher, instanceTracker *watcher.InstanceTracker) {
	rgds := rgdWatcher.All()
	for _, rgd := range rgds {
		count := instanceTracker.CountInstancesByRGD(rgd.Namespace, rgd.Name)
		rgd.InstanceCount = count
		rgdWatcher.Cache().Set(rgd)
	}
}
