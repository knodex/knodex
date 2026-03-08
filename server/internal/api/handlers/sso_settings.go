package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/netutil"
	"github.com/knodex/knodex/server/internal/sso"
	"github.com/knodex/knodex/server/internal/util/collection"
)

// ssoAccessChecker is the subset of rbac.PolicyEnforcer needed by SSOSettingsHandler.
type ssoAccessChecker interface {
	CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error)
}

// SSOSettingsHandler handles SSO provider CRUD endpoints.
type SSOSettingsHandler struct {
	store         *sso.ProviderStore
	recorder      audit.Recorder
	accessChecker ssoAccessChecker
}

// NewSSOSettingsHandler creates a new SSOSettingsHandler.
// accessChecker should be a rbac.PolicyEnforcer (or nil if not available).
func NewSSOSettingsHandler(store *sso.ProviderStore, recorder audit.Recorder, accessChecker ssoAccessChecker) *SSOSettingsHandler {
	return &SSOSettingsHandler{
		store:         store,
		recorder:      recorder,
		accessChecker: accessChecker,
	}
}

// requireSettingsUpdate checks that the authenticated user has settings:update permission.
// Returns true if authorized, false if a response was already written (caller should return).
// userCtx must already be validated by the caller (via helpers.RequireUserContext).
// auditAction and auditName provide operation context for audit trail specificity.
func (h *SSOSettingsHandler) requireSettingsUpdate(w http.ResponseWriter, r *http.Request, userCtx *middleware.UserContext, auditAction, auditName string) bool {
	requestID := r.Header.Get("X-Request-ID")

	if h.accessChecker == nil {
		slog.Warn("SSO settings: policy enforcer unavailable, denying write operation",
			"userId", userCtx.UserID,
		)
		audit.RecordEvent(h.recorder, r.Context(), audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    auditAction,
			Resource:  "settings",
			Name:      auditName,
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"reason": "policy enforcer unavailable"},
		})
		response.Forbidden(w, "permission denied")
		return false
	}

	allowed, err := h.accessChecker.CanAccessWithGroups(
		r.Context(),
		userCtx.UserID,
		userCtx.Groups,
		"settings/*",
		"update",
	)
	if err != nil {
		slog.Error("failed to check SSO settings permission",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		audit.RecordEvent(h.recorder, r.Context(), audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    auditAction,
			Resource:  "settings",
			Name:      auditName,
			RequestID: requestID,
			Result:    "error",
			Details:   map[string]any{"reason": "policy check failed"},
		})
		response.InternalError(w, "Failed to check authorization")
		return false
	}
	if !allowed {
		slog.Warn("SSO settings write denied",
			"requestId", requestID,
			"userId", userCtx.UserID,
		)
		audit.RecordEvent(h.recorder, r.Context(), audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    auditAction,
			Resource:  "settings",
			Name:      auditName,
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"reason": "insufficient permissions"},
		})
		response.Forbidden(w, "permission denied")
		return false
	}

	return true
}

// requireSettingsRead checks that the authenticated user has settings:get permission.
// Returns true if authorized, false if a response was already written (caller should return).
// Only role:serveradmin has settings/* policies, so this restricts read access to server admins.
func (h *SSOSettingsHandler) requireSettingsRead(w http.ResponseWriter, r *http.Request, userCtx *middleware.UserContext) bool {
	if h.accessChecker == nil {
		slog.Warn("SSO settings: policy enforcer unavailable, denying read operation",
			"userId", userCtx.UserID,
		)
		response.Forbidden(w, "permission denied")
		return false
	}

	allowed, err := h.accessChecker.CanAccessWithGroups(
		r.Context(),
		userCtx.UserID,
		userCtx.Groups,
		"settings/*",
		"get",
	)
	if err != nil {
		slog.Error("failed to check SSO settings read permission",
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to check authorization")
		return false
	}
	if !allowed {
		slog.Warn("SSO settings read denied",
			"userId", userCtx.UserID,
		)
		response.Forbidden(w, "permission denied")
		return false
	}

	return true
}

// SSOProviderResponse is the API representation of an SSO provider.
// ClientSecret is never returned.
type SSOProviderResponse struct {
	Name        string   `json:"name"`
	IssuerURL   string   `json:"issuerURL"`
	ClientID    string   `json:"clientID"`
	RedirectURL string   `json:"redirectURL"`
	Scopes      []string `json:"scopes"`
}

// SSOProviderRequest is the JSON body for create/update.
type SSOProviderRequest struct {
	Name         string   `json:"name"`
	IssuerURL    string   `json:"issuerURL"`
	ClientID     string   `json:"clientID"`
	ClientSecret string   `json:"clientSecret"`
	RedirectURL  string   `json:"redirectURL"`
	Scopes       []string `json:"scopes"`
}

// ListProviders handles GET /api/v1/settings/sso/providers
func (h *SSOSettingsHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	if !h.requireSettingsRead(w, r, userCtx) {
		return
	}

	providers, err := h.store.List(ctx)
	if err != nil {
		slog.Error("failed to list SSO providers",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to list SSO providers")
		return
	}

	resp := make([]SSOProviderResponse, len(providers))
	for i, p := range providers {
		resp[i] = toProviderResponse(p)
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// GetProvider handles GET /api/v1/settings/sso/providers/{name}
func (h *SSOSettingsHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()
	name := r.PathValue("name")

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	if !h.requireSettingsRead(w, r, userCtx) {
		return
	}

	provider, err := h.store.Get(ctx, name)
	if err != nil {
		if _, ok := err.(*sso.NotFoundError); ok {
			response.NotFound(w, "SSO provider", name)
			return
		}
		slog.Error("failed to get SSO provider",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"provider", name,
			"error", err,
		)
		response.InternalError(w, "Failed to get SSO provider")
		return
	}

	response.WriteJSON(w, http.StatusOK, toProviderResponse(*provider))
}

// CreateProvider handles POST /api/v1/settings/sso/providers
func (h *SSOSettingsHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	if !h.requireSettingsUpdate(w, r, userCtx, "create", "sso_provider") {
		return
	}

	req, err := helpers.DecodeJSON[SSOProviderRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	if errs := validateProviderRequest(req, true); errs.HasErrors() {
		errs.WriteResponse(w)
		return
	}

	provider := sso.SSOProvider{
		Name:         req.Name,
		IssuerURL:    req.IssuerURL,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURL:  req.RedirectURL,
		Scopes:       req.Scopes,
	}

	if err := h.store.Create(ctx, provider); err != nil {
		if _, ok := err.(*sso.ConflictError); ok {
			response.WriteError(w, http.StatusConflict, response.ErrCodeBadRequest,
				err.Error(), nil)
			return
		}
		slog.Error("failed to create SSO provider",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"provider", req.Name,
			"error", err,
		)
		response.InternalError(w, "Failed to create SSO provider")
		return
	}

	slog.Warn("SSO provider created",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"email", userCtx.Email,
		"remote_addr", r.RemoteAddr,
		"provider", req.Name,
		"issuer_url", req.IssuerURL,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "create",
		Resource:  "settings",
		Name:      req.Name,
		RequestID: requestID,
		Result:    "success",
		Details:   map[string]any{"settingsType": "sso_provider", "issuerURL": req.IssuerURL},
	})

	response.WriteJSON(w, http.StatusCreated, toProviderResponse(provider))
}

// UpdateProvider handles PUT /api/v1/settings/sso/providers/{name}
func (h *SSOSettingsHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()
	name := r.PathValue("name")

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	if !h.requireSettingsUpdate(w, r, userCtx, "update", name) {
		return
	}

	req, err := helpers.DecodeJSON[SSOProviderRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	if errs := validateProviderRequest(req, false); errs.HasErrors() {
		errs.WriteResponse(w)
		return
	}

	// Fetch old provider state for audit change tracking (safe fields only)
	oldProvider, _ := h.store.Get(ctx, name)

	provider := sso.SSOProvider{
		Name:         name,
		IssuerURL:    req.IssuerURL,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURL:  req.RedirectURL,
		Scopes:       req.Scopes,
	}

	if err := h.store.Update(ctx, name, provider); err != nil {
		if _, ok := err.(*sso.NotFoundError); ok {
			response.NotFound(w, "SSO provider", name)
			return
		}
		slog.Error("failed to update SSO provider",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"provider", name,
			"error", err,
		)
		response.InternalError(w, "Failed to update SSO provider")
		return
	}

	slog.Warn("SSO provider updated",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"email", userCtx.Email,
		"remote_addr", r.RemoteAddr,
		"provider", name,
	)

	// Build audit details with before/after for safe fields (never log clientSecret)
	settingsUpdateDetails := map[string]any{"settingsType": "sso_provider"}
	if oldProvider != nil {
		if oldProvider.IssuerURL != req.IssuerURL {
			settingsUpdateDetails["issuerURL"] = audit.SafeChanges(oldProvider.IssuerURL, req.IssuerURL)
		}
		if oldProvider.ClientID != req.ClientID {
			settingsUpdateDetails["clientID"] = audit.SafeChanges(oldProvider.ClientID, req.ClientID)
		}
		if oldProvider.RedirectURL != req.RedirectURL {
			settingsUpdateDetails["redirectURL"] = audit.SafeChanges(oldProvider.RedirectURL, req.RedirectURL)
		}
		// Track credential change without storing value
		if req.ClientSecret != "" && req.ClientSecret != oldProvider.ClientSecret {
			settingsUpdateDetails["credentialsUpdated"] = true
		}
		// Track scope changes
		if !collection.Equal(oldProvider.Scopes, req.Scopes) {
			settingsUpdateDetails["scopes"] = audit.SafeChanges(oldProvider.Scopes, req.Scopes)
		}
	}

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "update",
		Resource:  "settings",
		Name:      name,
		RequestID: requestID,
		Result:    "success",
		Details:   settingsUpdateDetails,
	})

	// Re-read provider to return current state (secret fields omitted)
	updated, err := h.store.Get(ctx, name)
	if err != nil {
		// Update succeeded but re-read failed — return the non-secret fields we have
		response.WriteJSON(w, http.StatusOK, toProviderResponse(provider))
		return
	}
	response.WriteJSON(w, http.StatusOK, toProviderResponse(*updated))
}

// DeleteProvider handles DELETE /api/v1/settings/sso/providers/{name}
func (h *SSOSettingsHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()
	name := r.PathValue("name")

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	if !h.requireSettingsUpdate(w, r, userCtx, "delete", name) {
		return
	}

	if err := h.store.Delete(ctx, name); err != nil {
		if _, ok := err.(*sso.NotFoundError); ok {
			response.NotFound(w, "SSO provider", name)
			return
		}
		slog.Error("failed to delete SSO provider",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"provider", name,
			"error", err,
		)
		response.InternalError(w, "Failed to delete SSO provider")
		return
	}

	slog.Warn("SSO provider deleted",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"email", userCtx.Email,
		"remote_addr", r.RemoteAddr,
		"provider", name,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "delete",
		Resource:  "settings",
		Name:      name,
		RequestID: requestID,
		Result:    "success",
		Details:   map[string]any{"settingsType": "sso_provider"},
	})

	w.WriteHeader(http.StatusNoContent)
}

// toProviderResponse converts an SSOProvider to its API response (no secrets).
func toProviderResponse(p sso.SSOProvider) SSOProviderResponse {
	scopes := p.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return SSOProviderResponse{
		Name:        p.Name,
		IssuerURL:   p.IssuerURL,
		ClientID:    p.ClientID,
		RedirectURL: p.RedirectURL,
		Scopes:      scopes,
	}
}

// validateProviderRequest validates the SSO provider request fields.
func validateProviderRequest(req *SSOProviderRequest, requireAll bool) helpers.ValidationErrors {
	errs := helpers.NewValidationErrors()

	if requireAll {
		if err := sso.ValidateName(req.Name); err != nil {
			errs.Add("name", err.Error())
		}
	}

	if requireAll || req.IssuerURL != "" {
		if req.IssuerURL == "" {
			errs.Add("issuerURL", "issuer URL is required")
		} else {
			u, err := url.Parse(req.IssuerURL)
			if err != nil || u.Scheme == "" || u.Host == "" {
				errs.Add("issuerURL", "issuer URL must be a valid URL")
			} else if u.Scheme != "https" {
				errs.Add("issuerURL", "issuer URL must use HTTPS")
			} else if netutil.IsPrivateHost(u.Hostname()) {
				errs.Add("issuerURL", "issuer URL must not point to a private or loopback address")
			}
		}
	}

	if requireAll || req.ClientID != "" {
		if req.ClientID == "" {
			errs.Add("clientID", "client ID is required")
		}
	}

	if requireAll {
		if req.ClientSecret == "" {
			errs.Add("clientSecret", "client secret is required")
		}
	}

	if requireAll || req.RedirectURL != "" {
		if req.RedirectURL == "" {
			errs.Add("redirectURL", "redirect URL is required")
		} else {
			u, err := url.Parse(req.RedirectURL)
			if err != nil || u.Scheme == "" || u.Host == "" {
				errs.Add("redirectURL", "redirect URL must be a valid URL")
			}
		}
	}

	return errs
}
