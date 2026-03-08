package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/util/collection"
)

// RGDHandler handles RGD-related HTTP requests.
// This is a thin HTTP layer that delegates business logic to services.
type RGDHandler struct {
	authService    *services.AuthorizationService
	catalogService *services.CatalogService
	logger         *slog.Logger
}

// RGDHandlerConfig holds configuration for creating an RGDHandler.
type RGDHandlerConfig struct {
	AuthService    *services.AuthorizationService
	CatalogService *services.CatalogService
	Logger         *slog.Logger
}

// NewRGDHandler creates a new RGD handler with the service-based architecture.
func NewRGDHandler(config RGDHandlerConfig) *RGDHandler {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &RGDHandler{
		authService:    config.AuthService,
		catalogService: config.CatalogService,
		logger:         logger.With("component", "rgd-handler"),
	}
}

// ListRGDsResponse represents the response for listing RGDs
type ListRGDsResponse struct {
	Items      []services.RGDResponse `json:"items"`
	TotalCount int                    `json:"totalCount"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"pageSize"`
}

// CountResponse represents the response for count endpoints
type CountResponse struct {
	Count int `json:"count"`
}

// FiltersResponse represents the response for the filters endpoint
type FiltersResponse struct {
	Projects   []string `json:"projects"`
	Tags       []string `json:"tags"`
	Categories []string `json:"categories"`
}

// Valid sort fields for RGD listing
var validSortFields = map[string]bool{
	"name":      true,
	"namespace": true,
	"createdAt": true,
	"updatedAt": true,
	"category":  true,
}

// Valid sort orders
var validSortOrders = map[string]bool{
	"asc":  true,
	"desc": true,
}

// ListRGDs handles GET /api/v1/rgds
func (h *RGDHandler) ListRGDs(w http.ResponseWriter, r *http.Request) {
	// Check if catalog service is available
	if h.catalogService == nil {
		response.ServiceUnavailable(w, "RGD service not available")
		return
	}

	// Parse and validate query parameters
	filters, validationErrors := h.parseAndValidateFilters(r)
	if len(validationErrors) > 0 {
		response.BadRequest(w, "invalid query parameters", validationErrors)
		return
	}

	// Get auth context
	authCtx, err := h.getAuthContext(r)
	if err != nil {
		h.logger.Error("failed to get auth context", "error", err)
		response.InternalError(w, "authorization error")
		return
	}

	// Delegate to service
	result, err := h.catalogService.ListRGDs(r.Context(), authCtx, filters)
	if err != nil {
		h.handleServiceError(w, err, "list RGDs")
		return
	}

	resp := ListRGDsResponse{
		Items:      result.Items,
		TotalCount: result.TotalCount,
		Page:       result.Page,
		PageSize:   result.PageSize,
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// GetRGD handles GET /api/v1/rgds/{name}
func (h *RGDHandler) GetRGD(w http.ResponseWriter, r *http.Request) {
	// Check if catalog service is available
	if h.catalogService == nil {
		response.ServiceUnavailable(w, "RGD service not available")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		response.BadRequest(w, "name is required", map[string]string{"name": "path parameter is required"})
		return
	}

	namespace := r.URL.Query().Get("namespace")

	// Get auth context
	authCtx, err := h.getAuthContext(r)
	if err != nil {
		h.logger.Error("failed to get auth context", "error", err)
		response.InternalError(w, "authorization error")
		return
	}

	// Delegate to service
	result, err := h.catalogService.GetRGD(r.Context(), authCtx, name, namespace)
	if err != nil {
		h.handleServiceError(w, err, "get RGD")
		return
	}

	response.WriteJSON(w, http.StatusOK, result)
}

// GetCount handles GET /api/v1/rgds/count
func (h *RGDHandler) GetCount(w http.ResponseWriter, r *http.Request) {
	// Check if catalog service is available
	if h.catalogService == nil {
		response.ServiceUnavailable(w, "RGD service not available")
		return
	}

	// Get auth context
	authCtx, err := h.getAuthContext(r)
	if err != nil {
		h.logger.Error("failed to get auth context", "error", err)
		response.InternalError(w, "authorization error")
		return
	}

	// Delegate to service
	count, err := h.catalogService.GetCount(r.Context(), authCtx)
	if err != nil {
		h.handleServiceError(w, err, "get RGD count")
		return
	}

	response.WriteJSON(w, http.StatusOK, CountResponse{Count: count})
}

// GetFilters handles GET /api/v1/rgds/filters
func (h *RGDHandler) GetFilters(w http.ResponseWriter, r *http.Request) {
	// Check if catalog service is available
	if h.catalogService == nil {
		response.ServiceUnavailable(w, "RGD service not available")
		return
	}

	// Get auth context
	authCtx, err := h.getAuthContext(r)
	if err != nil {
		h.logger.Error("failed to get auth context", "error", err)
		response.InternalError(w, "authorization error")
		return
	}

	// Delegate to service
	result, err := h.catalogService.GetFilters(r.Context(), authCtx)
	if err != nil {
		h.handleServiceError(w, err, "get RGD filters")
		return
	}

	resp := FiltersResponse{
		Projects:   result.Projects,
		Tags:       result.Tags,
		Categories: result.Categories,
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// getAuthContext extracts user context and computes authorization context.
func (h *RGDHandler) getAuthContext(r *http.Request) (*services.UserAuthContext, error) {
	userCtx, _ := middleware.GetUserContext(r)
	if userCtx == nil {
		return nil, nil
	}

	if h.authService != nil {
		return h.authService.GetUserAuthContext(r.Context(), userCtx)
	}

	// Fallback: create basic auth context from user context
	return services.NewUserAuthContextFromMiddleware(userCtx), nil
}

// handleServiceError maps service errors to HTTP responses.
func (h *RGDHandler) handleServiceError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		response.NotFound(w, "RGD", "")
	case errors.Is(err, services.ErrForbidden):
		response.Forbidden(w, "you do not have access to this RGD")
	case errors.Is(err, services.ErrServiceUnavailable):
		response.ServiceUnavailable(w, "RGD service not available")
	default:
		h.logger.Error("service error", "operation", operation, "error", err)
		response.InternalError(w, "internal error")
	}
}

// parseAndValidateFilters extracts and validates filter parameters from the request.
func (h *RGDHandler) parseAndValidateFilters(r *http.Request) (services.RGDFilters, map[string]string) {
	q := r.URL.Query()
	filters := services.DefaultRGDFilters()
	errs := make(map[string]string)

	// Namespace filter
	if ns := q.Get("namespace"); ns != "" {
		filters.Namespace = ns
	}

	// Category filter
	if cat := q.Get("category"); cat != "" {
		filters.Category = cat
	}

	// Tags filter (comma-separated)
	if tags := q.Get("tags"); tags != "" {
		filters.Tags = collection.Filter(
			collection.Map(strings.Split(tags, ","), strings.TrimSpace),
			func(s string) bool { return s != "" },
		)
	}

	// Search filter
	if search := q.Get("search"); search != "" {
		filters.Search = search
	}

	// Pagination - page
	if page := q.Get("page"); page != "" {
		p, err := strconv.Atoi(page)
		if err != nil {
			errs["page"] = "must be a valid integer"
		} else if p < 1 {
			errs["page"] = "must be at least 1"
		} else {
			filters.Page = p
		}
	}

	// Pagination - pageSize (also accept 'limit' as alias)
	pageSizeStr := q.Get("pageSize")
	if pageSizeStr == "" {
		pageSizeStr = q.Get("limit")
	}
	if pageSizeStr != "" {
		ps, err := strconv.Atoi(pageSizeStr)
		if err != nil {
			errs["pageSize"] = "must be a valid integer"
		} else if ps < 1 {
			errs["pageSize"] = "must be at least 1"
		} else if ps > 100 {
			errs["pageSize"] = "must not exceed 100"
		} else {
			filters.PageSize = ps
		}
	}

	// Sorting - sortBy (also accept 'sort' as alias)
	sortBy := q.Get("sortBy")
	if sortBy == "" {
		sortBy = q.Get("sort")
	}
	if sortBy != "" {
		if !validSortFields[sortBy] {
			errs["sortBy"] = "must be one of: name, namespace, createdAt, updatedAt, category"
		} else {
			filters.SortBy = sortBy
		}
	}

	// Sorting - sortOrder (also accept 'order' as alias)
	sortOrder := q.Get("sortOrder")
	if sortOrder == "" {
		sortOrder = q.Get("order")
	}
	if sortOrder != "" {
		sortOrder = strings.ToLower(sortOrder)
		if !validSortOrders[sortOrder] {
			errs["sortOrder"] = "must be 'asc' or 'desc'"
		} else {
			filters.SortOrder = sortOrder
		}
	}

	return filters, errs
}
