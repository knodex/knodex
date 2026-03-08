package handlers

import (
	"log/slog"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/kro/parser"
	kroschema "github.com/knodex/knodex/server/internal/kro/schema"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
)

// SchemaHandler handles schema-related HTTP requests
type SchemaHandler struct {
	watcher        *watcher.RGDWatcher
	extractor      *kroschema.Extractor
	resourceParser *parser.ResourceParser
	policyEnforcer rbac.Authorizer
}

// NewSchemaHandler creates a new schema handler
func NewSchemaHandler(w *watcher.RGDWatcher, e *kroschema.Extractor) *SchemaHandler {
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
	formSchema, crdErr := h.extractor.ExtractSchema(r.Context(), rgd)

	resp := models.SchemaResponse{
		RGD: name,
	}

	if crdErr != nil {
		// Check if this is a CRD not-found error — fall back to RGD-only schema
		if kroschema.IsNotFoundError(crdErr) {
			// Degraded path: build schema from RGD spec.schema only
			degradedSchema, buildErr := kroschema.BuildFormSchemaFromRGD(rgd)
			if buildErr != nil {
				slog.Warn("failed to build degraded schema from RGD", "rgd", name, "error", buildErr)
				resp.CRDFound = false
				resp.Error = crdErr.Error()
			} else {
				resp.Schema = degradedSchema
				resp.CRDFound = false
				resp.Source = "rgd-only"
				resp.Warnings = append(resp.Warnings, "CRD not yet available — form shows fields and defaults but validation constraints (minLength, pattern, enum) are missing")
			}
		} else {
			// Non-404 error (timeout, auth, etc.) — propagate
			resp.CRDFound = false
			resp.Error = crdErr.Error()
		}
	} else {
		resp.Schema = formSchema
		resp.CRDFound = true
		resp.Source = "crd+rgd"
	}

	// Dual-source enrichment: parse RGD intent + resource graph
	if resp.Schema != nil && rgd.RawSpec != nil {
		// For CRD path, parse RGD schema intent for defaults/descriptions
		if resp.CRDFound {
			rgdIntent, parseErr := kroschema.ParseRGDSchema(rgd.RawSpec)
			if parseErr != nil {
				slog.Warn("RGD schema intent parsing failed", "rgd", name, "error", parseErr)
				resp.Warnings = append(resp.Warnings, "RGD schema intent parsing failed: defaults and descriptions may be missing")
			}

			// Parse resource graph for conditional/externalRef enrichment
			var resourceGraph *parser.ResourceGraph
			if h.resourceParser != nil {
				rg, rgErr := h.resourceParser.ParseRGDResources(rgd.Name, rgd.Namespace, rgd.RawSpec)
				if rgErr == nil {
					resourceGraph = rg
				}
			}

			if enrichErr := kroschema.EnrichSchema(resp.Schema, rgdIntent, resourceGraph, h.watcher); enrichErr != nil {
				slog.Warn("schema enrichment failed", "rgd", name, "error", enrichErr)
				resp.Error = "failed to enrich schema with resource metadata"
			}
		} else {
			// Degraded path: still enrich with resource graph (conditional sections, externalRef, advanced)
			var resourceGraph *parser.ResourceGraph
			if h.resourceParser != nil {
				rg, rgErr := h.resourceParser.ParseRGDResources(rgd.Name, rgd.Namespace, rgd.RawSpec)
				if rgErr == nil {
					resourceGraph = rg
				}
			}

			if resourceGraph != nil {
				if enrichErr := kroschema.EnrichSchemaFromResources(resp.Schema, resourceGraph, h.watcher); enrichErr != nil {
					slog.Warn("degraded schema resource enrichment failed", "rgd", name, "error", enrichErr)
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
