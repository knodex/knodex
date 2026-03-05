package handlers

import (
	"net/http"

	"github.com/provops-org/knodex/server/internal/api/helpers"
	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/models"
	"github.com/provops-org/knodex/server/internal/parser"
	"github.com/provops-org/knodex/server/internal/rbac"
	"github.com/provops-org/knodex/server/internal/schema"
	"github.com/provops-org/knodex/server/internal/watcher"
)

// SchemaHandler handles schema-related HTTP requests
type SchemaHandler struct {
	watcher        *watcher.RGDWatcher
	extractor      *schema.Extractor
	resourceParser *parser.ResourceParser
	policyEnforcer rbac.Authorizer
}

// NewSchemaHandler creates a new schema handler
func NewSchemaHandler(w *watcher.RGDWatcher, e *schema.Extractor) *SchemaHandler {
	return &SchemaHandler{
		watcher:        w,
		extractor:      e,
		resourceParser: parser.NewResourceParser(),
	}
}

// SetPolicyEnforcer sets the policy enforcer for authorization checks
func (h *SchemaHandler) SetPolicyEnforcer(enforcer rbac.Authorizer) {
	h.policyEnforcer = enforcer
}

// GetSchema handles GET /api/v1/rgds/{name}/schema
// @Summary Get RGD schema
// @Description Returns the form schema for a specific RGD, extracted from its CRD
// @Tags rgds
// @Accept json
// @Produce json
// @Param name path string true "RGD name"
// @Param namespace query string false "Namespace (optional)"
// @Success 200 {object} models.SchemaResponse
// @Failure 404 {object} api.ErrorResponse
// @Failure 503 {object} api.ErrorResponse
// @Router /api/v1/rgds/{name}/schema [get]
func (h *SchemaHandler) GetSchema(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Validate name parameter
	if name == "" {
		response.BadRequest(w, "name is required", map[string]string{"name": "path parameter is required"})
		return
	}

	// Check if watcher is available
	if h.watcher == nil {
		response.ServiceUnavailable(w, "RGD watcher not available")
		return
	}

	// Check if extractor is available
	if h.extractor == nil {
		response.ServiceUnavailable(w, "schema extractor not available")
		return
	}

	// Get optional namespace from query param
	namespace := r.URL.Query().Get("namespace")

	var rgd *models.CatalogRGD
	var found bool

	if namespace != "" {
		rgd, found = h.watcher.GetRGD(namespace, name)
	} else {
		// Search by name across all namespaces
		rgd, found = h.watcher.GetRGDByName(name)
	}

	if !found {
		response.NotFound(w, "RGD", name)
		return
	}

	// Extract schema from CRD
	formSchema, err := h.extractor.ExtractSchema(r.Context(), rgd)

	resp := models.SchemaResponse{
		RGD:      name,
		CRDFound: err == nil,
	}

	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Schema = formSchema

		// Enrich schema with metadata from RGD resources (conditional sections, externalRefSelectors)
		if rgd.RawSpec != nil && h.resourceParser != nil {
			resourceGraph, parseErr := h.resourceParser.ParseRGDResources(rgd.Name, rgd.Namespace, rgd.RawSpec)
			if parseErr == nil && resourceGraph != nil {
				if enrichErr := schema.EnrichSchemaFromResources(formSchema, resourceGraph, h.watcher); enrichErr != nil {
					// Log enrichment error but don't fail the request - return schema without enrichment
					resp.Error = "failed to enrich schema: " + enrichErr.Error()
				}
			}
		}
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// InvalidateSchemaCache handles POST /api/v1/rgds/{name}/schema/invalidate
// @Summary Invalidate schema cache
// @Description Invalidates the cached schema for a specific RGD
// @Tags rgds
// @Accept json
// @Produce json
// @Param name path string true "RGD name"
// @Param namespace query string false "Namespace (optional)"
// @Success 200 {object} map[string]string
// @Failure 404 {object} api.ErrorResponse
// @Failure 503 {object} api.ErrorResponse
// @Router /api/v1/rgds/{name}/schema/invalidate [post]
func (h *SchemaHandler) InvalidateSchemaCache(w http.ResponseWriter, r *http.Request) {
	// Require admin access (settings:update) for cache invalidation
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}
	if h.policyEnforcer == nil {
		// Fail-closed: deny access when authorization is not configured
		response.Forbidden(w, "authorization not configured")
		return
	}
	if !helpers.RequireAccess(w, r.Context(), h.policyEnforcer, userCtx, "settings/*", "update", r.Header.Get("X-Request-ID")) {
		return
	}

	name := r.PathValue("name")

	// Validate name parameter
	if name == "" {
		response.BadRequest(w, "name is required", map[string]string{"name": "path parameter is required"})
		return
	}

	// Check if watcher is available
	if h.watcher == nil {
		response.ServiceUnavailable(w, "RGD watcher not available")
		return
	}

	// Check if extractor is available
	if h.extractor == nil {
		response.ServiceUnavailable(w, "schema extractor not available")
		return
	}

	// Get optional namespace from query param
	namespace := r.URL.Query().Get("namespace")

	var rgd *models.CatalogRGD
	var found bool

	if namespace != "" {
		rgd, found = h.watcher.GetRGD(namespace, name)
	} else {
		// Search by name across all namespaces
		rgd, found = h.watcher.GetRGDByName(name)
	}

	if !found {
		response.NotFound(w, "RGD", name)
		return
	}

	// Invalidate the cache
	h.extractor.InvalidateCache(rgd.Namespace, rgd.Name)

	response.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "schema cache invalidated",
		"rgd":     name,
	})
}
