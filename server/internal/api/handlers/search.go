// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

const (
	// maxSearchResultsPerGroup caps each result category to prevent excessive response sizes.
	maxSearchResultsPerGroup = 50
	// maxQueryLength bounds the search query to prevent abuse.
	maxQueryLength = 200
	// instanceFetchMultiplier fetches more instances than needed to account for
	// post-query namespace filtering that may remove results.
	instanceFetchMultiplier = 4
)

// SearchResponse is the unified search response.
type SearchResponse struct {
	Results    SearchResults `json:"results"`
	Query      string        `json:"query"`
	TotalCount int           `json:"totalCount"`
}

// SearchResults groups results by resource type.
type SearchResults struct {
	RGDs      []RGDSearchResult      `json:"rgds"`
	Instances []InstanceSearchResult `json:"instances"`
	Projects  []ProjectSearchResult  `json:"projects"`
}

// RGDSearchResult is a search result for an RGD.
type RGDSearchResult struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// InstanceSearchResult is a search result for an instance.
type InstanceSearchResult struct {
	Name      string `json:"name"`
	Project   string `json:"project"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Kind      string `json:"kind"`
}

// ProjectSearchResult is a search result for a project.
type ProjectSearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SearchHandlerConfig holds dependencies for the search handler.
type SearchHandlerConfig struct {
	AuthService     *services.AuthorizationService
	CatalogService  *services.CatalogService
	InstanceTracker *watcher.InstanceTracker
	ProjectService  rbac.ProjectServiceInterface
	Logger          *slog.Logger
}

// SearchHandler handles GET /api/v1/search.
type SearchHandler struct {
	authService     *services.AuthorizationService
	catalogService  *services.CatalogService
	instanceTracker *watcher.InstanceTracker
	projectService  rbac.ProjectServiceInterface
	logger          *slog.Logger
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(cfg SearchHandlerConfig) *SearchHandler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &SearchHandler{
		authService:     cfg.AuthService,
		catalogService:  cfg.CatalogService,
		instanceTracker: cfg.InstanceTracker,
		projectService:  cfg.ProjectService,
		logger:          logger.With("component", "search-handler"),
	}
}

// Search handles GET /api/v1/search?q={query}.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// Extract, sanitize, and validate query
	query := sanitize.RemoveControlChars(r.URL.Query().Get("q"))
	if len(query) > maxQueryLength {
		query = query[:maxQueryLength]
	}
	query = strings.TrimSpace(query)
	if query == "" {
		response.BadRequest(w, "query parameter 'q' is required and must not be empty", nil)
		return
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	h.logger.Debug("search request",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"query", query,
	)

	// Get authorization context
	var authCtx *services.UserAuthContext
	if h.authService != nil {
		var err error
		authCtx, err = h.authService.GetUserAuthContext(r.Context(), userCtx)
		if err != nil {
			h.logger.Error("failed to get auth context", "requestId", requestID, "userId", userCtx.UserID, "error", err)
			response.InternalError(w, "Failed to get authorization context")
			return
		}
	}

	lowerQuery := strings.ToLower(query)

	// Search each resource type
	rgdResults := h.searchRGDs(r, authCtx, lowerQuery)
	instanceResults := h.searchInstances(authCtx, lowerQuery)
	projectResults := h.searchProjects(r, authCtx, lowerQuery)

	totalCount := len(rgdResults) + len(instanceResults) + len(projectResults)

	resp := SearchResponse{
		Results: SearchResults{
			RGDs:      rgdResults,
			Instances: instanceResults,
			Projects:  projectResults,
		},
		Query:      query,
		TotalCount: totalCount,
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// searchRGDs searches RGDs using the CatalogService with authorization filtering.
func (h *SearchHandler) searchRGDs(r *http.Request, authCtx *services.UserAuthContext, lowerQuery string) []RGDSearchResult {
	if h.catalogService == nil {
		return []RGDSearchResult{}
	}

	// Use CatalogService with search filter — it handles project-scoped access internally
	filters := services.RGDFilters{
		Search:   lowerQuery,
		Page:     1,
		PageSize: maxSearchResultsPerGroup,
	}

	result, err := h.catalogService.ListRGDs(r.Context(), authCtx, filters)
	if err != nil {
		h.logger.Warn("search RGDs failed", "error", err)
		return []RGDSearchResult{}
	}

	results := make([]RGDSearchResult, 0, len(result.Items))
	for _, rgd := range result.Items {
		results = append(results, RGDSearchResult{
			Name:        rgd.Name,
			DisplayName: rgd.Title,
			Category:    rgd.Category,
			Description: rgd.Description,
		})
	}
	return results
}

// searchInstances searches instances with namespace-scoped access filtering.
func (h *SearchHandler) searchInstances(authCtx *services.UserAuthContext, lowerQuery string) []InstanceSearchResult {
	if h.instanceTracker == nil {
		return []InstanceSearchResult{}
	}

	// Fetch more instances than needed to account for post-query namespace filtering
	opts := models.DefaultInstanceListOptions()
	opts.Search = lowerQuery
	opts.PageSize = maxSearchResultsPerGroup * instanceFetchMultiplier
	list := h.instanceTracker.ListInstances(opts)

	// Filter by accessible namespaces.
	// Contract: ["*"] = global admin (matches all), empty = no access, non-empty = filter.
	// Fail closed: no auth context = empty slice (show nothing).
	var accessibleNamespaces []string
	if authCtx != nil {
		accessibleNamespaces = authCtx.AccessibleNamespaces
	} else {
		accessibleNamespaces = []string{}
	}

	results := make([]InstanceSearchResult, 0, len(list.Items))
	for _, inst := range list.Items {
		// Skip instances the user cannot access (["*"] matches everything)
		if !rbac.MatchNamespaceInList(inst.Namespace, accessibleNamespaces) {
			continue
		}

		project := inst.ProjectName
		if project == "" {
			project = inst.Labels[models.ProjectLabel]
		}

		results = append(results, InstanceSearchResult{
			Name:      inst.Name,
			Project:   project,
			Namespace: inst.Namespace,
			Status:    string(inst.Health),
			Kind:      inst.Kind,
		})

		if len(results) >= maxSearchResultsPerGroup {
			break
		}
	}
	return results
}

// searchProjects searches projects with access control.
func (h *SearchHandler) searchProjects(r *http.Request, authCtx *services.UserAuthContext, lowerQuery string) []ProjectSearchResult {
	if h.projectService == nil {
		return []ProjectSearchResult{}
	}

	projectList, err := h.projectService.ListProjects(r.Context())
	if err != nil {
		h.logger.Warn("search projects failed", "error", err)
		return []ProjectSearchResult{}
	}
	if projectList == nil {
		return []ProjectSearchResult{}
	}

	// Build accessible projects set for filtering.
	// Admins have all projects in AccessibleProjects (via Casbin wildcard policy match).
	// Fail closed: no auth context = empty map (show nothing).
	accessibleProjects := make(map[string]bool)
	if authCtx != nil {
		for _, p := range authCtx.AccessibleProjects {
			accessibleProjects[p] = true
		}
	}

	results := make([]ProjectSearchResult, 0)
	for _, proj := range projectList.Items {
		// Filter by access (accessibleProjects is always non-nil)
		if !accessibleProjects[proj.Name] {
			continue
		}

		// Filter by query match (name or description)
		name := strings.ToLower(proj.Name)
		desc := strings.ToLower(proj.Spec.Description)
		if !strings.Contains(name, lowerQuery) && !strings.Contains(desc, lowerQuery) {
			continue
		}

		results = append(results, ProjectSearchResult{
			Name:        proj.Name,
			Description: proj.Spec.Description,
		})

		if len(results) >= maxSearchResultsPerGroup {
			break
		}
	}
	return results
}
