// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// secretsOperationTimeout is the maximum duration for any K8s operation in secrets handlers.
const secretsOperationTimeout = 15 * time.Second

// auditProjectHeader is the request header that carries the caller's "current
// project" lens for audit recording. Web sets it from useUserStore.currentProject;
// CLI/API clients may omit it (the audit Project field is then empty). The lens
// is informational only — it never participates in authorization decisions.
const auditProjectHeader = "X-Knodex-Project"

// SecretsHandlerConfig holds configuration for creating a SecretsHandler.
//
// Authorization is enforced at the Casbin middleware layer (see
// CasbinAuthz + inferCasbinObjectAndAction in internal/api/middleware/authz.go).
// The handler uses NamespaceAccessProvider purely as a defense-in-depth
// namespace-membership check — it does NOT call Casbin directly.
type SecretsHandlerConfig struct {
	K8sClient     kubernetes.Interface
	DynamicClient dynamic.Interface
	Recorder      audit.Recorder
	NSAccess      NamespaceAccessProvider // Filters results to user's accessible namespaces
}

// SecretsHandler handles secret-related HTTP requests
type SecretsHandler struct {
	k8sClient     kubernetes.Interface
	dynamicClient dynamic.Interface
	recorder      audit.Recorder
	nsAccess      NamespaceAccessProvider
}

// NewSecretsHandler creates a new secrets handler
func NewSecretsHandler(cfg SecretsHandlerConfig) *SecretsHandler {
	return &SecretsHandler{
		k8sClient:     cfg.K8sClient,
		dynamicClient: cfg.DynamicClient,
		recorder:      cfg.Recorder,
		nsAccess:      cfg.NSAccess,
	}
}

// auditLensMaxLen caps the X-Knodex-Project audit-lens header length. 253
// matches the DNS-1123 subdomain max (the same upper bound used for project
// name validation throughout the codebase). The lens is informational and
// caller-controlled; truncation defends the audit row + downstream consumers
// against oversized values without changing the URL contract.
const auditLensMaxLen = 253

// auditLens returns the caller's current-project lens for audit recording.
// Empty when the X-Knodex-Project header is absent (e.g., "All Projects" view,
// CLI/API clients) — the audit UI handles empty values as unscoped.
//
// The header value is caller-controlled and never participates in any
// authorization decision (CLAUDE.md "Authorization Resources and Actions"
// note on secrets). To prevent audit-row log injection / oversize attacks,
// we strip ASCII control characters and cap at auditLensMaxLen.
func auditLens(r *http.Request) string {
	raw := r.Header.Get(auditProjectHeader)
	if raw == "" {
		return ""
	}
	cleaned := sanitize.RemoveControlChars(raw)
	if len(cleaned) > auditLensMaxLen {
		cleaned = cleaned[:auditLensMaxLen]
	}
	return cleaned
}

// getAccessibleNamespaces returns the user's accessible namespace patterns.
// Returns:
//   - []string{"*"} when the user is a global admin (matches any namespace).
//   - empty slice when the user has no namespace access (secure default).
//   - concrete patterns drawn from roles[].destinations for everyone else.
//
// Mirrors InstanceCRUDHandler.getAccessibleNamespaces so the two handlers
// share identical namespace-access semantics, including the (slice, error)
// signature that lets the caller surface a 500 when the provider fails.
func (h *SecretsHandler) getAccessibleNamespaces(ctx context.Context, userCtx *middleware.UserContext) ([]string, error) {
	if userCtx == nil {
		return []string{}, nil
	}
	if h.nsAccess == nil {
		return []string{}, nil
	}
	return h.nsAccess.GetAccessibleNamespaces(ctx, userCtx)
}

// authorizeSecretAccess reports whether the user is permitted to act on a
// secret in the given namespace. Pattern-aware via rbac.MatchNamespaceInList
// so global admins ([]string{"*"}) and wildcard destinations like "staging-*"
// behave correctly. Mirrors InstanceCRUDHandler.authorizeInstanceAccess's
// namespaced branch — secrets are always namespaced, so there is no
// cluster-scoped fallback. An NSAccess provider error surfaces to the caller,
// which is expected to convert it into a 500 response (matching the instance
// handler's behavior).
func (h *SecretsHandler) authorizeSecretAccess(ctx context.Context, userCtx *middleware.UserContext, namespace string) (bool, error) {
	userNamespaces, err := h.getAccessibleNamespaces(ctx, userCtx)
	if err != nil {
		return false, err
	}
	return rbac.MatchNamespaceInList(namespace, userNamespaces), nil
}

// CreateSecret handles POST /api/v1/namespaces/{namespace}/secrets
func (h *SecretsHandler) CreateSecret(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	namespace := r.PathValue("namespace")
	if namespace == "" {
		response.BadRequest(w, "namespace path parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(namespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	req, err := helpers.DecodeJSON[CreateSecretRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	// Defense-in-depth: middleware already enforces Casbin at route level
	// (secrets/{ns}/{...}, action). Repeat the namespace-membership check
	// in the handler for the same reason instance_crud.go does:
	// authorizeInstanceAccess at instance_crud.go:222-232.
	allowed, err := h.authorizeSecretAccess(ctx, userCtx, namespace)
	if err != nil {
		slog.Error("failed to determine namespace access",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "failed to determine namespace access")
		return
	}
	if !allowed {
		// Match the instance handler's "not leaking existence" behavior:
		// users without namespace access get NotFound on namespaces they
		// cannot see.
		response.NotFound(w, "namespace", namespace)
		audit.RecordEvent(h.recorder, ctx, audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    "create",
			Resource:  "secrets",
			Project:   auditLens(r),
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"namespace": namespace},
		})
		return
	}

	// Validate the secret payload (name, data, metadata).
	validationErrors := validateCreateSecretRequest(req)
	if len(validationErrors) > 0 {
		response.BadRequest(w, "Validation failed", validationErrors)
		return
	}

	slog.Info("creating secret",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"secretName", req.Name,
		"namespace", namespace,
	)

	// Create K8s secret. We stamp ManagedByLabel only — the ProjectLabel
	// is intentionally NOT set. Namespace alone is the access boundary
	// for secrets; the project lens is captured in audit, not on the K8s
	// object (see TD-2 / TD-3 in the spec).
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: namespace,
			Labels: map[string]string{
				models.ManagedByLabel: models.ManagedByValue,
			},
		},
		StringData: req.Data,
		Type:       corev1.SecretTypeOpaque,
	}
	// Stamp typed metadata (rotation label + docs-url / expires-at
	// annotations) when supplied. System labels above remain intact —
	// applyMetadataToSecret only touches the three metadata keys.
	applyMetadataToSecret(secret, req.Metadata)

	created, err := h.k8sClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			response.Conflict(w, "secret", req.Name)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			response.Forbidden(w, "service account lacks permission to manage secrets in namespace "+namespace)
			return
		}
		if k8serrors.IsNotFound(err) {
			response.BadRequest(w, "namespace does not exist: "+namespace, nil)
			return
		}
		if k8serrors.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			response.ServiceUnavailable(w, "secrets API timed out: K8s API server is slow or unreachable")
			return
		}
		slog.Error("failed to create secret",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"secretName", req.Name,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "Failed to create secret")
		return
	}

	// Extract keys only — NEVER return values
	keys := make([]string, 0, len(req.Data))
	for k := range req.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	createdMeta := extractSecretMetadata(created)
	resp := SecretResponse{
		Name:      created.Name,
		Namespace: created.Namespace,
		Keys:      keys,
		CreatedAt: created.CreationTimestamp.Time,
		Labels:    created.Labels,
		Metadata:  createdMeta,
		Status:    computeSecretStatus(createdMeta, time.Now()),
	}

	slog.Info("secret created successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"secretName", req.Name,
		"namespace", namespace,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "create",
		Resource:  "secrets",
		Name:      req.Name,
		Project:   auditLens(r),
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"namespace": namespace,
			"keyCount":  len(keys),
		},
	})

	response.WriteJSON(w, http.StatusCreated, resp)
}

// ListSecrets handles GET /api/v1/secrets
// Multi-namespace list filtered to the user's accessible namespaces. Optional
// ?namespace= narrows to a single namespace (still subject to membership).
func (h *SecretsHandler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Optional namespace filter — when set, list is narrowed to that single
	// namespace (still gated by membership). Mirrors the optional namespace
	// filter on /api/v1/instances.
	filterNamespace := r.URL.Query().Get("namespace")
	if filterNamespace != "" && !sanitize.IsValidDNS1123Label(filterNamespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	// Parse pagination parameters
	limit := defaultSecretPageSize
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxSecretPageSize {
		limit = maxSecretPageSize
	}
	continueToken := r.URL.Query().Get("continue")

	slog.Info("listing secrets",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"namespace", filterNamespace,
		"limit", limit,
	)

	// Filter to ManagedBy=knodex regardless of how the list is bounded — the
	// handler only surfaces secrets it created.
	labelSelector := models.ManagedByLabel + "=" + models.ManagedByValue

	userNamespaces, err := h.getAccessibleNamespaces(ctx, userCtx)
	if err != nil {
		slog.Error("failed to determine accessible namespaces",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "failed to determine namespace access")
		return
	}
	isAdmin := len(userNamespaces) == 1 && userNamespaces[0] == "*"

	// When ?namespace= is provided, narrow to that one — but the membership
	// check still applies. AC13 (empty list for unauthorized namespace)
	// mirrors instance_crud.go:280-289.
	if filterNamespace != "" {
		if !isAdmin && !rbac.MatchNamespaceInList(filterNamespace, userNamespaces) {
			writeEmptySecretList(w, requestID, userCtx.UserID, r, h.recorder, ctx)
			return
		}
		userNamespaces = []string{filterNamespace}
		isAdmin = false
	}

	var allSecrets []corev1.Secret

	switch {
	case isAdmin:
		// Admin: single cluster-wide query with K8s pagination
		secretList, err := h.k8sClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         int64(limit),
			Continue:      continueToken,
		})
		if err != nil {
			if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
				response.Forbidden(w, "service account lacks permission to manage secrets")
				return
			}
			if k8serrors.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				response.ServiceUnavailable(w, "secrets API timed out: K8s API server is slow or unreachable")
				return
			}
			slog.Error("failed to list secrets",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"error", err,
			)
			response.InternalError(w, "Failed to list secrets")
			return
		}
		allSecrets = secretList.Items
		continueToken = secretList.Continue

	default:
		// Non-admin: query each accessible namespace individually. Per-namespace
		// queries avoid the pagination + post-filter anti-pattern where K8s
		// Continue tokens become semantically incorrect after removing items
		// from a page.
		//
		// KNOWN LIMITATION (TD-9 follow-up): Pattern destinations (e.g.,
		// "staging-*") cannot be issued as a K8s LIST directly — they would
		// require listing every namespace in the cluster and filtering by
		// pattern. We skip pattern entries silently here. The user's GET /
		// PUT / DELETE on a concrete matching namespace (e.g., staging-team-a)
		// STILL succeed because authorizeSecretAccess uses
		// rbac.MatchNamespaceInList (pattern-aware). The asymmetry is:
		//   - Single-resource verbs: pattern destinations work normally.
		//   - List: pattern destinations contribute zero results.
		// This is acceptable for the initial rollout — pattern destinations
		// are uncommon, and a future namespace-lister pass can close the
		// gap without changing the URL contract. The skip is silent (no
		// log per-request) to avoid spamming logs for the common case where
		// destinations are all concrete.
		for _, ns := range userNamespaces {
			if strings.Contains(ns, "*") {
				// Skip patterns — see KNOWN LIMITATION above.
				continue
			}
			nsList, listErr := h.k8sClient.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if listErr != nil {
				if k8serrors.IsForbidden(listErr) {
					continue // Skip namespaces where SA lacks permissions
				}
				if k8serrors.IsTimeout(listErr) || errors.Is(listErr, context.DeadlineExceeded) || errors.Is(listErr, context.Canceled) {
					continue
				}
				slog.Warn("failed to list secrets in namespace",
					"requestId", requestID, "namespace", ns, "error", listErr)
				continue
			}
			allSecrets = append(allSecrets, nsList.Items...)
		}
	}

	// Apply limit for non-admin queries (admin uses K8s-level pagination)
	hasMore := false
	if !isAdmin {
		sort.Slice(allSecrets, func(i, j int) bool {
			if allSecrets[i].Namespace != allSecrets[j].Namespace {
				return allSecrets[i].Namespace < allSecrets[j].Namespace
			}
			return allSecrets[i].Name < allSecrets[j].Name
		})
		if len(allSecrets) > limit {
			allSecrets = allSecrets[:limit]
			hasMore = true
		}
		continueToken = "" // No K8s continue token for per-namespace queries
	}

	items := make([]SecretResponse, 0, len(allSecrets))
	now := time.Now()
	for i := range allSecrets {
		s := &allSecrets[i]
		keys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		meta := extractSecretMetadata(s)
		items = append(items, SecretResponse{
			Name:      s.Name,
			Namespace: s.Namespace,
			Keys:      keys,
			CreatedAt: s.CreationTimestamp.Time,
			UpdatedAt: parseUpdatedAt(s.Annotations),
			Labels:    s.Labels,
			Metadata:  meta,
			Status:    computeSecretStatus(meta, now),
		})
	}

	resp := SecretListResponse{
		Items:     items,
		PageCount: len(items),
		Continue:  continueToken,
		HasMore:   continueToken != "" || hasMore,
	}

	slog.Info("secrets listed successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"count", resp.PageCount,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "list",
		Resource:  "secrets",
		Project:   auditLens(r),
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"count":     resp.PageCount,
			"namespace": filterNamespace,
		},
	})

	response.WriteJSON(w, http.StatusOK, resp)
}

// writeEmptySecretList returns a 200 OK with an empty list, matching the
// "no leak" pattern for unauthorized namespace filters on the instance list
// (instance_crud.go:280-289). Audits as a success with count=0 so the
// caller's request is still recorded.
func writeEmptySecretList(w http.ResponseWriter, requestID, userID string, r *http.Request, recorder audit.Recorder, ctx context.Context) {
	resp := SecretListResponse{Items: []SecretResponse{}, PageCount: 0}
	audit.RecordEvent(recorder, ctx, audit.Event{
		UserID:    userID,
		SourceIP:  audit.SourceIP(r),
		Action:    "list",
		Resource:  "secrets",
		Project:   auditLens(r),
		RequestID: requestID,
		Result:    "success",
		Details:   map[string]any{"count": 0},
	})
	response.WriteJSON(w, http.StatusOK, resp)
}

// GetSecret handles GET /api/v1/namespaces/{namespace}/secrets/{name}
func (h *SecretsHandler) GetSecret(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	namespace := r.PathValue("namespace")
	if namespace == "" {
		response.BadRequest(w, "namespace path parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(namespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	name := r.PathValue("name")
	if name == "" {
		response.BadRequest(w, "secret name is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Subdomain(name) {
		response.BadRequest(w, "name must be a valid DNS-1123 subdomain (lowercase alphanumeric, hyphens, and dots)", nil)
		return
	}

	// Defense-in-depth namespace membership check (middleware already
	// performed the Casbin enforcement against secrets/{ns}/{name}).
	allowed, err := h.authorizeSecretAccess(ctx, userCtx, namespace)
	if err != nil {
		slog.Error("failed to determine namespace access",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "failed to determine namespace access")
		return
	}
	if !allowed {
		response.NotFound(w, "secret", name)
		audit.RecordEvent(h.recorder, ctx, audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    "get",
			Resource:  "secrets",
			Name:      name,
			Project:   auditLens(r),
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"namespace": namespace},
		})
		return
	}

	slog.Info("getting secret",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"secretName", name,
		"namespace", namespace,
	)

	secret, err := h.k8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			response.NotFound(w, "secret", name)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			response.Forbidden(w, "service account lacks permission to manage secrets in namespace "+namespace)
			return
		}
		if k8serrors.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			response.ServiceUnavailable(w, "secrets API timed out: K8s API server is slow or unreachable")
			return
		}
		slog.Error("failed to get secret",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"secretName", name,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "Failed to get secret")
		return
	}

	// Decode secret data (K8s client already base64-decodes Data)
	data := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		data[k] = string(v)
	}

	getMeta := extractSecretMetadata(secret)
	resp := SecretDetailResponse{
		Name:      secret.Name,
		Namespace: secret.Namespace,
		Data:      data,
		CreatedAt: secret.CreationTimestamp.Time,
		UpdatedAt: parseUpdatedAt(secret.Annotations),
		Labels:    secret.Labels,
		Metadata:  getMeta,
		Status:    computeSecretStatus(getMeta, time.Now()),
	}

	slog.Info("secret retrieved successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"secretName", name,
		"namespace", namespace,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "get",
		Resource:  "secrets",
		Name:      name,
		Project:   auditLens(r),
		RequestID: requestID,
		Result:    "success",
		Details:   map[string]any{"namespace": namespace},
	})

	response.WriteJSON(w, http.StatusOK, resp)
}

// CheckSecretExists handles HEAD /api/v1/namespaces/{namespace}/secrets/{name}
// Returns 200 if the secret exists in the user's accessible namespace, 404 otherwise.
// No response body is written — this is a lightweight existence check for the frontend.
func (h *SecretsHandler) CheckSecretExists(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	namespace := r.PathValue("namespace")
	if namespace == "" || !sanitize.IsValidDNS1123Label(namespace) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	name := r.PathValue("name")
	if name == "" || !sanitize.IsValidDNS1123Subdomain(name) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	allowed, err := h.authorizeSecretAccess(ctx, userCtx, namespace)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !allowed {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	_, err = h.k8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if k8serrors.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		slog.Error("failed to check secret existence",
			"userId", userCtx.UserID,
			"secretName", name,
			"namespace", namespace,
			"error", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// UpdateSecret handles PUT /api/v1/namespaces/{namespace}/secrets/{name}
func (h *SecretsHandler) UpdateSecret(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	namespace := r.PathValue("namespace")
	if namespace == "" {
		response.BadRequest(w, "namespace path parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(namespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	name := r.PathValue("name")
	if name == "" {
		response.BadRequest(w, "secret name is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Subdomain(name) {
		response.BadRequest(w, "name must be a valid DNS-1123 subdomain (lowercase alphanumeric, hyphens, and dots)", nil)
		return
	}

	req, err := helpers.DecodeJSON[UpdateSecretRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	// Validate payload before security check so callers always get the most
	// informative error.
	validationErrors := make(map[string]string)
	if len(req.Data) == 0 {
		validationErrors["data"] = "data must contain at least one key-value pair"
	} else {
		validationErrors = validateSecretData(req.Data, validationErrors)
	}
	validationErrors = validateSecretMetadata(req.Metadata, validationErrors)
	if len(validationErrors) > 0 {
		response.BadRequest(w, "Validation failed", validationErrors)
		return
	}

	allowed, err := h.authorizeSecretAccess(ctx, userCtx, namespace)
	if err != nil {
		slog.Error("failed to determine namespace access",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "failed to determine namespace access")
		return
	}
	if !allowed {
		response.NotFound(w, "secret", name)
		audit.RecordEvent(h.recorder, ctx, audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    "update",
			Resource:  "secrets",
			Name:      name,
			Project:   auditLens(r),
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"namespace": namespace},
		})
		return
	}

	slog.Info("updating secret",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"secretName", name,
		"namespace", namespace,
	)

	existing, err := h.k8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			response.NotFound(w, "secret", name)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			response.Forbidden(w, "service account lacks permission to manage secrets in namespace "+namespace)
			return
		}
		if k8serrors.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			response.ServiceUnavailable(w, "secrets API timed out: K8s API server is slow or unreachable")
			return
		}
		slog.Error("failed to get secret for update",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"secretName", name,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "Failed to update secret")
		return
	}

	// Update secret with new values via StringData
	existing.StringData = req.Data

	// Apply typed metadata changes (nil = leave existing untouched).
	// Runs before updatedAt stamping so metadata changes are part of the
	// same "modification" the timestamp records.
	applyMetadataToSecret(existing, req.Metadata)

	// Stamp updatedAt annotation for tracking last rotation time
	if existing.Annotations == nil {
		existing.Annotations = make(map[string]string)
	}
	existing.Annotations[updatedAtAnnotation] = time.Now().UTC().Format(time.RFC3339)

	updated, err := h.k8sClient.CoreV1().Secrets(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		if k8serrors.IsConflict(err) {
			response.WriteError(w, http.StatusConflict, response.ErrCodeConflict, "secret was modified concurrently, please retry", nil)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			response.Forbidden(w, "service account lacks permission to manage secrets in namespace "+namespace)
			return
		}
		if k8serrors.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			response.ServiceUnavailable(w, "secrets API timed out: K8s API server is slow or unreachable")
			return
		}
		slog.Error("failed to update secret",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"secretName", name,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "Failed to update secret")
		return
	}

	// Return ALL keys from the updated secret — union Data and StringData because real K8s
	// converts StringData into Data (and clears StringData), while fake clients may not.
	// This produces the correct key list in both production and test environments.
	keySet := make(map[string]struct{}, len(updated.Data)+len(updated.StringData))
	for k := range updated.Data {
		keySet[k] = struct{}{}
	}
	for k := range updated.StringData {
		keySet[k] = struct{}{}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	updatedMeta := extractSecretMetadata(updated)
	resp := SecretResponse{
		Name:      updated.Name,
		Namespace: updated.Namespace,
		Keys:      keys,
		CreatedAt: updated.CreationTimestamp.Time,
		UpdatedAt: parseUpdatedAt(updated.Annotations),
		Labels:    updated.Labels,
		Metadata:  updatedMeta,
		Status:    computeSecretStatus(updatedMeta, time.Now()),
	}

	slog.Info("secret updated successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"secretName", name,
		"namespace", namespace,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "update",
		Resource:  "secrets",
		Name:      name,
		Project:   auditLens(r),
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"namespace": namespace,
			"keyCount":  len(keys),
		},
	})

	response.WriteJSON(w, http.StatusOK, resp)
}

// DeleteSecret handles DELETE /api/v1/namespaces/{namespace}/secrets/{name}
func (h *SecretsHandler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	namespace := r.PathValue("namespace")
	if namespace == "" {
		response.BadRequest(w, "namespace path parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(namespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	name := r.PathValue("name")
	if name == "" {
		response.BadRequest(w, "secret name is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Subdomain(name) {
		response.BadRequest(w, "name must be a valid DNS-1123 subdomain (lowercase alphanumeric, hyphens, and dots)", nil)
		return
	}

	allowed, err := h.authorizeSecretAccess(ctx, userCtx, namespace)
	if err != nil {
		slog.Error("failed to determine namespace access",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "failed to determine namespace access")
		return
	}
	if !allowed {
		response.NotFound(w, "secret", name)
		audit.RecordEvent(h.recorder, ctx, audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    "delete",
			Resource:  "secrets",
			Name:      name,
			Project:   auditLens(r),
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"namespace": namespace},
		})
		return
	}

	slog.Info("deleting secret",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"secretName", name,
		"namespace", namespace,
	)

	// Verify the secret exists before scanning for references / deleting.
	if _, err := h.k8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			response.NotFound(w, "secret", name)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			response.Forbidden(w, "service account lacks permission to manage secrets in namespace "+namespace)
			return
		}
		if k8serrors.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			response.ServiceUnavailable(w, "secrets API timed out: K8s API server is slow or unreachable")
			return
		}
		slog.Error("failed to check secret existence",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"secretName", name,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "Failed to delete secret")
		return
	}

	// Scan for Instance references (best-effort, non-blocking) with dedicated timeout
	var warnings []string
	if h.dynamicClient != nil {
		scanCtx, scanCancel := context.WithTimeout(ctx, referenceScanTimeout)
		defer scanCancel()
		warnings = h.findSecretReferences(scanCtx, name, namespace)
	}

	// Delete the K8s Secret regardless of references
	err = h.k8sClient.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Race condition: secret was deleted between our Get and Delete calls
			response.NotFound(w, "secret", name)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			response.Forbidden(w, "service account lacks permission to manage secrets in namespace "+namespace)
			return
		}
		if k8serrors.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			response.ServiceUnavailable(w, "secrets API timed out: K8s API server is slow or unreachable")
			return
		}
		slog.Error("failed to delete secret",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"secretName", name,
			"namespace", namespace,
			"error", err,
		)
		response.InternalError(w, "Failed to delete secret")
		return
	}

	resp := DeleteSecretResponse{
		Deleted:  true,
		Warnings: warnings,
	}

	slog.Info("secret deleted successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"secretName", name,
		"namespace", namespace,
		"warnings", len(warnings),
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "delete",
		Resource:  "secrets",
		Name:      name,
		Project:   auditLens(r),
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"namespace": namespace,
			"warnings":  warnings,
		},
	})

	response.WriteJSON(w, http.StatusOK, resp)
}

// findSecretReferences scans kro.run Instances in the namespace for references to the given secret.
// This is a best-effort scan — errors are logged but don't block deletion.
//
// Note: This uses heuristic key-name matching on live Instance specs (runtime, unstructured).
// It differs from kro/parser.extractSecretRefs which does structural extraction from RGD
// definitions at parse-time. The parser approach cannot be reused here because Instances
// have resolved specs (no externalRef metadata), so we must scan field values instead.
func (h *SecretsHandler) findSecretReferences(ctx context.Context, secretName, namespace string) []string {
	var warnings []string

	// List kro.run instances in the namespace (capped to avoid unbounded scans)
	instanceList, err := h.dynamicClient.Resource(kroInstanceGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		Limit: instanceListLimit,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			warnings = append(warnings, "could not complete reference scan (timeout)")
			return warnings
		}
		slog.Warn("failed to scan for secret references (best-effort)",
			"secretName", secretName,
			"namespace", namespace,
			"error", err,
		)
		return warnings
	}

	for _, instance := range instanceList.Items {
		// Bail early if scan timeout exceeded
		if ctx.Err() != nil {
			warnings = append(warnings, "could not complete reference scan (timeout)")
			return warnings
		}
		spec := parser.GetSpecOrEmpty(&instance)
		if len(spec) == 0 {
			continue
		}
		// Check if any field in spec references the secret name
		if containsSecretReference(spec, secretName) {
			warnings = append(warnings, "Referenced by Instance "+instance.GetName())
		}
	}

	return warnings
}

// containsSecretReference recursively checks if a value contains a reference to the given secret name.
// Only matches string values when descended from a map key whose name contains "ref" or "secret"
// (case-insensitive). This avoids false positives from coincidental string matches in unrelated
// spec fields such as display names or descriptions.
func containsSecretReference(val interface{}, secretName string) bool {
	return searchSecretRef(val, secretName, false, 0)
}

// searchSecretRef traverses val looking for secretName, using inRefContext to track whether
// we are inside a key that suggests a secret reference (key name contains "ref" or "secret").
// depth limits recursion to maxSearchDepth to prevent stack overflow on deeply nested specs.
func searchSecretRef(val interface{}, secretName string, inRefContext bool, depth int) bool {
	if depth > maxSearchDepth {
		return false
	}
	switch v := val.(type) {
	case string:
		return inRefContext && v == secretName
	case map[string]interface{}:
		for key, child := range v {
			keyLower := strings.ToLower(key)
			childInRef := inRefContext || strings.Contains(keyLower, "ref") || strings.Contains(keyLower, "secret")
			if searchSecretRef(child, secretName, childInRef, depth+1) {
				return true
			}
		}
	case []interface{}:
		for _, child := range v {
			if searchSecretRef(child, secretName, inRefContext, depth+1) {
				return true
			}
		}
	}
	return false
}

// kroInstanceGVR is the GroupVersionResource for kro.run Instance CRDs.
// Centralized so a KRO API version bump only requires a single change.
var kroInstanceGVR = schema.GroupVersionResource{
	Group:    "kro.run",
	Version:  "v1alpha1",
	Resource: "instances",
}

// instanceListLimit caps the number of Instances fetched during best-effort
// reference scanning to avoid unbounded memory/latency on large namespaces.
const instanceListLimit = 500

// maxSearchDepth limits recursion depth in searchSecretRef to prevent stack
// overflow on deeply nested specs. 50 levels is far beyond any realistic K8s spec.
const maxSearchDepth = 50

// referenceScanTimeout limits how long the delete reference scan can run.
const referenceScanTimeout = 5 * time.Second

// secretKeyRegexp validates K8s secret data keys: must contain only [-._a-zA-Z0-9].
var secretKeyRegexp = regexp.MustCompile(`^[-._a-zA-Z0-9]+$`)

// updatedAtAnnotation is the annotation key used to track when a secret was last updated via Knodex.
const updatedAtAnnotation = "knodex.io/updated-at"

// parseUpdatedAt extracts the updatedAt timestamp from secret annotations.
// Returns nil for secrets that were never updated via Knodex.
func parseUpdatedAt(annotations map[string]string) *time.Time {
	if raw, ok := annotations[updatedAtAnnotation]; ok {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			slog.Warn("malformed updatedAt annotation, ignoring",
				"annotation", updatedAtAnnotation,
				"value", raw,
				"error", err,
			)
			return nil
		}
		return &t
	}
	return nil
}

// validateCreateSecretRequest validates the create secret request.
// Namespace is validated separately at the path-param level — not here.
func validateCreateSecretRequest(req *CreateSecretRequest) map[string]string {
	errors := make(map[string]string)

	if req.Name == "" {
		errors["name"] = "name is required"
	} else if !sanitize.IsValidDNS1123Subdomain(req.Name) {
		errors["name"] = "name must be a valid DNS-1123 subdomain (lowercase alphanumeric, hyphens, and dots)"
	}

	if len(req.Data) == 0 {
		errors["data"] = "data must contain at least one key-value pair"
	} else {
		errors = validateSecretData(req.Data, errors)
	}

	errors = validateSecretMetadata(req.Metadata, errors)

	return errors
}

// validateSecretData checks for empty keys, key character sets, per-value size limits, and total size limits.
// All errors are collected before returning so callers see every issue in a single round trip.
func validateSecretData(data map[string]string, errors map[string]string) map[string]string {
	var totalSize int
	for key, value := range data {
		if key == "" {
			errors["data:emptyKey"] = "secret keys must not be empty"
		} else if !secretKeyRegexp.MatchString(key) {
			errors["data:"+key] = fmt.Sprintf("secret key %q contains invalid characters (must match [-._a-zA-Z0-9]+)", key)
		}
		valueSize := len(value)
		if valueSize > MaxSecretValueSize {
			errors["data:"+key+":size"] = "secret value exceeds maximum size of 256KB for key: " + key
		}
		totalSize += valueSize
	}
	if totalSize > MaxSecretTotalSize {
		errors["data:totalSize"] = "total secret data exceeds maximum size of 512KB"
	}
	return errors
}
