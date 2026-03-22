// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
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
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// secretsOperationTimeout is the maximum duration for any K8s operation in secrets handlers.
const secretsOperationTimeout = 15 * time.Second

// SecretsHandlerEnforcer is a type alias for rbac.Authorizer, required because
// helpers.RequireAccess accepts rbac.Authorizer. The full interface is needed to
// satisfy the dependency; only CanAccessWithGroups is exercised at runtime.
type SecretsHandlerEnforcer = rbac.Authorizer

// SecretsHandlerConfig holds configuration for creating a SecretsHandler
type SecretsHandlerConfig struct {
	K8sClient     kubernetes.Interface
	DynamicClient dynamic.Interface
	Enforcer      SecretsHandlerEnforcer
	Recorder      audit.Recorder
}

// SecretsHandler handles secret-related HTTP requests
type SecretsHandler struct {
	k8sClient     kubernetes.Interface
	dynamicClient dynamic.Interface
	enforcer      SecretsHandlerEnforcer
	recorder      audit.Recorder
}

// NewSecretsHandler creates a new secrets handler
func NewSecretsHandler(cfg SecretsHandlerConfig) *SecretsHandler {
	return &SecretsHandler{
		k8sClient:     cfg.K8sClient,
		dynamicClient: cfg.DynamicClient,
		enforcer:      cfg.Enforcer,
		recorder:      cfg.Recorder,
	}
}

// CreateSecret handles POST /api/v1/secrets
func (h *SecretsHandler) CreateSecret(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	project := r.URL.Query().Get("project")
	if project == "" {
		response.BadRequest(w, "project query parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(project) {
		response.BadRequest(w, "project must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	// Check Casbin permission
	if !helpers.RequireAccess(w, ctx, h.enforcer, userCtx, "secrets/"+project, "create", requestID) {
		return
	}

	req, err := helpers.DecodeJSON[CreateSecretRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	// Validate request
	validationErrors := validateCreateSecretRequest(req)
	if len(validationErrors) > 0 {
		response.BadRequest(w, "Validation failed", validationErrors)
		return
	}

	slog.Info("creating secret",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", project,
		"secretName", req.Name,
		"namespace", req.Namespace,
	)

	// Create K8s secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				"knodex.io/project":    project,
				"knodex.io/managed-by": "knodex",
			},
		},
		StringData: req.Data,
		Type:       corev1.SecretTypeOpaque,
	}

	created, err := h.k8sClient.CoreV1().Secrets(req.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			response.BadRequest(w, "Secret already exists: "+req.Name, nil)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			response.Forbidden(w, "service account lacks permission to manage secrets in namespace "+req.Namespace)
			return
		}
		if k8serrors.IsNotFound(err) {
			response.BadRequest(w, "namespace does not exist: "+req.Namespace, nil)
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
			"namespace", req.Namespace,
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

	resp := SecretResponse{
		Name:      created.Name,
		Namespace: created.Namespace,
		Keys:      keys,
		CreatedAt: created.CreationTimestamp.Time,
		Labels:    created.Labels,
	}

	slog.Info("secret created successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", project,
		"secretName", req.Name,
		"namespace", req.Namespace,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "create",
		Resource:  "secrets",
		Name:      req.Name,
		Project:   project,
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"namespace": req.Namespace,
			"keyCount":  len(keys),
		},
	})

	response.WriteJSON(w, http.StatusCreated, resp)
}

// ListSecrets handles GET /api/v1/secrets
func (h *SecretsHandler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	project := r.URL.Query().Get("project")
	if project == "" {
		response.BadRequest(w, "project query parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(project) {
		response.BadRequest(w, "project must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	// Check Casbin permission
	if !helpers.RequireAccess(w, ctx, h.enforcer, userCtx, "secrets/"+project, "get", requestID) {
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
		"project", project,
		"limit", limit,
	)

	// List secrets with project label selector and pagination
	secretList, err := h.k8sClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		LabelSelector: "knodex.io/project=" + project + ",knodex.io/managed-by=knodex",
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
			"project", project,
			"error", err,
		)
		response.InternalError(w, "Failed to list secrets")
		return
	}

	items := make([]SecretResponse, 0, len(secretList.Items))
	for _, s := range secretList.Items {
		keys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		items = append(items, SecretResponse{
			Name:      s.Name,
			Namespace: s.Namespace,
			Keys:      keys,
			CreatedAt: s.CreationTimestamp.Time,
			Labels:    s.Labels,
		})
	}

	resp := SecretListResponse{
		Items:      items,
		TotalCount: len(items),
		Continue:   secretList.Continue,
		HasMore:    secretList.Continue != "",
	}

	slog.Info("secrets listed successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", project,
		"count", resp.TotalCount,
	)

	response.WriteJSON(w, http.StatusOK, resp)
}

// GetSecret handles GET /api/v1/secrets/{name}
func (h *SecretsHandler) GetSecret(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
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

	project := r.URL.Query().Get("project")
	if project == "" {
		response.BadRequest(w, "project query parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(project) {
		response.BadRequest(w, "project must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		response.BadRequest(w, "namespace query parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(namespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	// Check Casbin permission
	if !helpers.RequireAccess(w, ctx, h.enforcer, userCtx, "secrets/"+project, "get", requestID) {
		return
	}

	slog.Info("getting secret",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", project,
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

	// Verify the secret belongs to the requested project (prevent cross-project access)
	if secret.Labels["knodex.io/project"] != project {
		response.NotFound(w, "secret", name)
		return
	}

	// Decode secret data (K8s client already base64-decodes Data)
	data := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		data[k] = string(v)
	}

	resp := SecretDetailResponse{
		Name:      secret.Name,
		Namespace: secret.Namespace,
		Data:      data,
		CreatedAt: secret.CreationTimestamp.Time,
		Labels:    secret.Labels,
	}

	slog.Info("secret retrieved successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", project,
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
		Project:   project,
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"namespace": namespace,
		},
	})

	response.WriteJSON(w, http.StatusOK, resp)
}

// CheckSecretExists handles HEAD /api/v1/secrets/{name}
// Returns 200 if the secret exists and belongs to the project, 404 otherwise.
// No response body is written — this is a lightweight existence check for the frontend.
func (h *SecretsHandler) CheckSecretExists(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	name := r.PathValue("name")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !sanitize.IsValidDNS1123Subdomain(name) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	project := r.URL.Query().Get("project")
	if project == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !sanitize.IsValidDNS1123Label(project) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !sanitize.IsValidDNS1123Label(namespace) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Reuse the same Casbin permission as GetSecret
	if !helpers.RequireAccess(w, ctx, h.enforcer, userCtx, "secrets/"+project, "get", requestID) {
		return
	}

	secret, err := h.k8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
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
			"requestId", requestID,
			"userId", userCtx.UserID,
			"secretName", name,
			"namespace", namespace,
			"error", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Treat cross-project access as 404 (same as GetSecret) to avoid leaking secret names
	if secret.Labels["knodex.io/project"] != project {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// UpdateSecret handles PUT /api/v1/secrets/{name}
func (h *SecretsHandler) UpdateSecret(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
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

	project := r.URL.Query().Get("project")
	if project == "" {
		response.BadRequest(w, "project query parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(project) {
		response.BadRequest(w, "project must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	// Check Casbin permission
	if !helpers.RequireAccess(w, ctx, h.enforcer, userCtx, "secrets/"+project, "update", requestID) {
		return
	}

	req, err := helpers.DecodeJSON[UpdateSecretRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	if req.Namespace == "" {
		response.BadRequest(w, "namespace is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(req.Namespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}
	if len(req.Data) == 0 {
		response.BadRequest(w, "data must contain at least one key-value pair", nil)
		return
	}
	if dataErrors := validateSecretData(req.Data, make(map[string]string)); len(dataErrors) > 0 {
		for _, msg := range dataErrors {
			response.BadRequest(w, msg, nil)
			return
		}
	}

	slog.Info("updating secret",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", project,
		"secretName", name,
		"namespace", req.Namespace,
	)

	// Get existing secret
	existing, err := h.k8sClient.CoreV1().Secrets(req.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			response.NotFound(w, "secret", name)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			response.Forbidden(w, "service account lacks permission to manage secrets in namespace "+req.Namespace)
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
			"namespace", req.Namespace,
			"error", err,
		)
		response.InternalError(w, "Failed to update secret")
		return
	}

	// Verify the secret belongs to the requested project (prevent cross-project access)
	if existing.Labels["knodex.io/project"] != project {
		response.NotFound(w, "secret", name)
		return
	}

	// Update secret with new values via StringData
	existing.StringData = req.Data

	updated, err := h.k8sClient.CoreV1().Secrets(req.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		if k8serrors.IsConflict(err) {
			response.WriteError(w, http.StatusConflict, response.ErrCodeConflict, "secret was modified concurrently, please retry", nil)
			return
		}
		if k8serrors.IsForbidden(err) || k8serrors.IsUnauthorized(err) {
			response.Forbidden(w, "service account lacks permission to manage secrets in namespace "+req.Namespace)
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
			"namespace", req.Namespace,
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

	resp := SecretResponse{
		Name:      updated.Name,
		Namespace: updated.Namespace,
		Keys:      keys,
		CreatedAt: updated.CreationTimestamp.Time,
		Labels:    updated.Labels,
	}

	slog.Info("secret updated successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", project,
		"secretName", name,
		"namespace", req.Namespace,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "update",
		Resource:  "secrets",
		Name:      name,
		Project:   project,
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"namespace": req.Namespace,
			"keyCount":  len(keys),
		},
	})

	response.WriteJSON(w, http.StatusOK, resp)
}

// DeleteSecret handles DELETE /api/v1/secrets/{name}
func (h *SecretsHandler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx, cancel := context.WithTimeout(r.Context(), secretsOperationTimeout)
	defer cancel()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
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

	project := r.URL.Query().Get("project")
	if project == "" {
		response.BadRequest(w, "project query parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(project) {
		response.BadRequest(w, "project must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		response.BadRequest(w, "namespace query parameter is required", nil)
		return
	}
	if !sanitize.IsValidDNS1123Label(namespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	// Check Casbin permission
	if !helpers.RequireAccess(w, ctx, h.enforcer, userCtx, "secrets/"+project, "delete", requestID) {
		return
	}

	slog.Info("deleting secret",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", project,
		"secretName", name,
		"namespace", namespace,
	)

	// Check if the secret exists and belongs to the requested project
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

	// Verify the secret belongs to the requested project (prevent cross-project access)
	if existing.Labels["knodex.io/project"] != project {
		response.NotFound(w, "secret", name)
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
		"project", project,
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
		Project:   project,
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
		specRaw := instance.Object["spec"]
		if specRaw == nil {
			continue
		}
		// Check if any field in spec references the secret name
		if containsSecretReference(specRaw, secretName) {
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

// validateCreateSecretRequest validates the create secret request
func validateCreateSecretRequest(req *CreateSecretRequest) map[string]string {
	errors := make(map[string]string)

	if req.Name == "" {
		errors["name"] = "name is required"
	} else if !sanitize.IsValidDNS1123Subdomain(req.Name) {
		errors["name"] = "name must be a valid DNS-1123 subdomain (lowercase alphanumeric, hyphens, and dots)"
	}

	if req.Namespace == "" {
		errors["namespace"] = "namespace is required"
	} else if !sanitize.IsValidDNS1123Label(req.Namespace) {
		errors["namespace"] = "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)"
	}

	if len(req.Data) == 0 {
		errors["data"] = "data must contain at least one key-value pair"
	} else {
		errors = validateSecretData(req.Data, errors)
	}

	return errors
}

// validateSecretData checks for empty keys, per-value size limits, and total size limits.
func validateSecretData(data map[string]string, errors map[string]string) map[string]string {
	var totalSize int
	for key, value := range data {
		if key == "" {
			errors["data"] = "secret keys must not be empty"
			return errors
		}
		valueSize := len(value)
		if valueSize > MaxSecretValueSize {
			errors["data"] = "secret value exceeds maximum size of 256KB for key: " + key
			return errors
		}
		totalSize += valueSize
	}
	if totalSize > MaxSecretTotalSize {
		errors["data"] = "total secret data exceeds maximum size of 512KB"
	}
	return errors
}
