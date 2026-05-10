// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/collection"
)

// Redis cache TTL constants for catalog data.
const (
	// filtersCacheTTL is the cache duration for RGD filter options (categories, tags, etc).
	// 5min is appropriate since filter options change infrequently (only when RGDs are added/removed).
	filtersCacheTTL = 5 * time.Minute

	// listCacheTTL is the cache duration for RGD list queries.
	// 30s keeps data fresh while reducing API load; WebSocket provides real-time updates.
	listCacheTTL = 30 * time.Second
)

// RGDProvider defines the interface for retrieving RGD data.
// This matches the watcher's public API for dependency injection.
type RGDProvider interface {
	// ListRGDs returns a paginated list of RGDs matching the options
	ListRGDs(opts models.ListOptions) models.CatalogRGDList
	// GetRGD returns a single RGD by namespace and name
	GetRGD(namespace, name string) (*models.CatalogRGD, bool)
	// GetRGDByName returns the first RGD matching the name across all namespaces
	GetRGDByName(name string) (*models.CatalogRGD, bool)
}

// InstanceCounter defines the interface for counting instances by RGD.
// This matches the InstanceTracker's public API for dependency injection.
type InstanceCounter interface {
	// CountInstancesByRGDAndNamespaces returns the instance count for an RGD,
	// filtered by the user's accessible namespaces.
	// namespaces == ["*"] means count all instances (global admin)
	// namespaces == [] means count is 0 (no access)
	CountInstancesByRGDAndNamespaces(rgdNamespace, rgdName string, namespaces []string, matchFn func(instanceNS string, namespaces []string) bool) int
}

// CatalogService encapsulates business logic for RGD catalog operations.
// It consolidates the visibility filtering, caching, and response formatting
// that was previously scattered across the RGDHandler.
type CatalogService struct {
	rgdProvider        RGDProvider
	instanceCounter    InstanceCounter
	policyEnforcer     PolicyEnforcer
	redisClient        *redis.Client
	logger             *slog.Logger
	organizationFilter string // Enterprise org filter (empty = no filtering)
}

// CatalogServiceConfig holds configuration for creating a CatalogService.
type CatalogServiceConfig struct {
	RGDProvider        RGDProvider
	InstanceCounter    InstanceCounter
	PolicyEnforcer     PolicyEnforcer
	RedisClient        *redis.Client
	Logger             *slog.Logger
	OrganizationFilter string // Enterprise org filter (empty = no filtering)
}

// NewCatalogService creates a new CatalogService.
func NewCatalogService(config CatalogServiceConfig) *CatalogService {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &CatalogService{
		rgdProvider:        config.RGDProvider,
		instanceCounter:    config.InstanceCounter,
		policyEnforcer:     config.PolicyEnforcer,
		redisClient:        config.RedisClient,
		logger:             logger.With("component", "catalog-service"),
		organizationFilter: config.OrganizationFilter,
	}
}

// ListRGDs returns a paginated list of RGDs filtered by the user's access.
// This consolidates the visibility filtering and response formatting from RGDHandler.
//
// The authCtx determines visibility:
// - nil authCtx: Returns empty list (secure default for unauthenticated)
// - authCtx.AccessibleProjects: Filters to RGDs in these projects + public RGDs
// - authCtx.AccessibleNamespaces: Filters instance counts to accessible namespaces
func (s *CatalogService) ListRGDs(ctx context.Context, authCtx *UserAuthContext, filters RGDFilters) (*ListRGDsResult, error) {
	if s.rgdProvider == nil {
		return nil, ErrServiceUnavailable
	}

	// Convert filters to list options
	opts := s.filtersToListOptions(filters)

	// Save original pagination for post-filter application when PolicyEnforcer is active.
	// Casbin filtering is per-user/per-RGD, so we must fetch all items first, filter,
	// then paginate the filtered set to produce accurate TotalCount and page contents.
	requestedPage := opts.Page
	requestedPageSize := opts.PageSize

	// Apply organization filter (enterprise feature)
	opts.Organization = s.organizationFilter

	// Apply visibility filtering
	if authCtx != nil {
		opts.Projects = authCtx.AccessibleProjects
		opts.IncludePublic = true
	}

	// When PolicyEnforcer is active, fetch all items so Casbin filtering
	// produces accurate total counts and correct pagination.
	if s.policyEnforcer != nil && authCtx != nil {
		opts.Page = 1
		opts.PageSize = 10000
	}

	// Try cache first
	result := s.getRGDsWithCaching(ctx, opts)

	// Warn when the "fetch all" cap is reached: items beyond 10,000 are silently excluded
	// from Casbin filtering, producing incomplete counts and missing RGDs for large catalogs.
	if s.policyEnforcer != nil && authCtx != nil && len(result.Items) >= 10000 {
		s.logger.Warn("RGD catalog fetch limit reached — Casbin filtering may be incomplete; consider caching category-level decisions",
			"fetched", len(result.Items), "limit", 10000)
	}

	// Filter through Casbin authorization per-RGD when policy enforcer is configured.
	// This provides category-scoped access control: rgds/{category}/{name}.
	// Note: The cache layer (getRGDsWithCaching) stores project-filtered results shared
	// across users with the same project set. Casbin filtering is applied post-cache and
	// is per-user, so cache results are never user-specific — this is by design.
	var filteredItems []models.CatalogRGD
	if s.policyEnforcer != nil && authCtx != nil {
		filteredItems = make([]models.CatalogRGD, 0, len(result.Items))
		for _, rgd := range result.Items {
			if s.canAccessRGD(ctx, &rgd, authCtx) {
				filteredItems = append(filteredItems, rgd)
			}
		}
	} else {
		filteredItems = result.Items
	}

	// Apply post-filter pagination when PolicyEnforcer is active.
	// The full set was fetched above; now slice to the requested page.
	// When policyEnforcer filters, totalCount is len(filtered); otherwise use provider's TotalCount
	// which accounts for items beyond the current page.
	totalCount := result.TotalCount
	if s.policyEnforcer != nil && authCtx != nil {
		totalCount = len(filteredItems)
	}
	page := result.Page
	pageSize := result.PageSize

	if s.policyEnforcer != nil && authCtx != nil {
		page = requestedPage
		pageSize = requestedPageSize
		start := (requestedPage - 1) * requestedPageSize
		if start >= len(filteredItems) {
			filteredItems = nil
		} else {
			end := start + requestedPageSize
			if end > len(filteredItems) {
				end = len(filteredItems)
			}
			filteredItems = filteredItems[start:end]
		}
	}

	// Convert to response format with instance counts
	items := make([]RGDResponse, len(filteredItems))
	for i, rgd := range filteredItems {
		instanceCount := s.getFilteredInstanceCount(&rgd, authCtx)
		items[i] = ToRGDResponse(&rgd, instanceCount)
	}

	return &ListRGDsResult{
		Items:      items,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// GetRGD retrieves a single RGD by name and optional namespace.
// Returns ErrNotFound if the RGD doesn't exist.
// Returns ErrForbidden if the user doesn't have access to the RGD.
func (s *CatalogService) GetRGD(ctx context.Context, authCtx *UserAuthContext, name, namespace string) (*RGDResponse, error) {
	if s.rgdProvider == nil {
		return nil, ErrServiceUnavailable
	}

	var rgd *models.CatalogRGD
	var found bool

	if namespace != "" {
		rgd, found = s.rgdProvider.GetRGD(namespace, name)
	} else {
		rgd, found = s.rgdProvider.GetRGDByName(name)
	}

	if !found {
		return nil, ErrNotFound
	}

	// Organization filter: hide RGDs from other orgs (returns 404 to hide existence per AC#7)
	// This is separate from project access checks because org mismatch should not reveal
	// the RGD exists — returning 403 would confirm its existence to other-org users.
	if s.organizationFilter != "" && rgd.Organization != "" && rgd.Organization != s.organizationFilter {
		return nil, ErrNotFound
	}

	// Check project visibility if authenticated
	if authCtx != nil {
		if !s.canAccessRGD(ctx, rgd, authCtx) {
			s.logger.Debug("RGD access denied",
				"user_id", authCtx.UserID,
				"rgd_name", name,
				"accessible_projects", authCtx.AccessibleProjects)
			return nil, ErrForbidden
		}
	}

	// Get filtered instance count
	instanceCount := s.getFilteredInstanceCount(rgd, authCtx)
	resp := ToRGDResponse(rgd, instanceCount)

	return &resp, nil
}

// GetCount returns the total number of RGDs accessible to the user.
// Note: This bypasses getRGDsWithCaching() intentionally (count doesn't need the same
// response caching as list).
func (s *CatalogService) GetCount(ctx context.Context, authCtx *UserAuthContext) (int, error) {
	if s.rgdProvider == nil {
		return 0, ErrServiceUnavailable
	}

	pageSize := 1 // Minimal - we only need count
	if s.policyEnforcer != nil {
		// Need all items for per-RGD Casbin filtering — O(n) enforcement calls.
		// TODO(perf): Cache category-level authorization decisions per-user to avoid
		// O(N) Casbin calls on every count request. Pre-compute visible categories
		// once and filter by category set instead of per-RGD enforcement.
		pageSize = 10000
	}
	opts := models.ListOptions{
		Page:     1,
		PageSize: pageSize,
	}

	// Apply organization filter (enterprise feature)
	opts.Organization = s.organizationFilter

	if authCtx != nil {
		opts.Projects = authCtx.AccessibleProjects
		opts.IncludePublic = true
	}

	result := s.rgdProvider.ListRGDs(opts)

	// Warn when the fetch cap is reached for the same reason as ListRGDs.
	if s.policyEnforcer != nil && authCtx != nil && len(result.Items) >= 10000 {
		s.logger.Warn("RGD count fetch limit reached — count may be incomplete for catalogs larger than 10,000 RGDs",
			"fetched", len(result.Items), "limit", 10000)
	}

	// Apply per-RGD Casbin filtering for accurate count
	if s.policyEnforcer != nil && authCtx != nil {
		count := 0
		for _, rgd := range result.Items {
			if s.canAccessRGD(ctx, &rgd, authCtx) {
				count++
			}
		}
		return count, nil
	}

	return result.TotalCount, nil
}

// GetFilters returns available filter values (projects, tags, categories)
// based on the RGDs visible to the user.
func (s *CatalogService) GetFilters(ctx context.Context, authCtx *UserAuthContext) (*RGDFilterOptions, error) {
	if s.rgdProvider == nil {
		return nil, ErrServiceUnavailable
	}

	var accessibleProjects []string
	includePublic := true

	if authCtx != nil {
		accessibleProjects = authCtx.AccessibleProjects
	}

	// Skip cache when PolicyEnforcer is active: filter results are per-user (Casbin-scoped)
	// and cannot be safely shared across users with identical project membership but different roles.
	if s.redisClient != nil && s.policyEnforcer == nil {
		cacheKey := s.filtersCacheKey(accessibleProjects, includePublic)
		data, err := s.redisClient.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var opts RGDFilterOptions
			if err := json.Unmarshal(data, &opts); err == nil {
				s.logger.Debug("cache hit for RGD filters", "cache_key", cacheKey)
				return &opts, nil
			}
		} else if err != redis.Nil {
			s.logger.Debug("failed to get cached RGD filters from redis", "error", err)
		}
	}

	// Cache miss - compute from all accessible RGDs
	opts := models.ListOptions{
		Organization:  s.organizationFilter,
		Projects:      accessibleProjects,
		IncludePublic: includePublic,
		Page:          1,
		PageSize:      10000, // Get all for filter extraction
	}
	result := s.rgdProvider.ListRGDs(opts)

	// Extract unique values from authorized RGDs only
	projectSet := make(map[string]bool)
	tagSet := make(map[string]bool)
	categorySet := make(map[string]bool)

	for _, rgd := range result.Items {
		// Apply per-RGD Casbin filtering for consistent filter options
		if s.policyEnforcer != nil && authCtx != nil {
			if !s.canAccessRGD(ctx, &rgd, authCtx) {
				continue
			}
		}

		if rgd.Labels != nil {
			if project := rgd.Labels[models.RGDProjectLabel]; project != "" {
				projectSet[project] = true
			}
		}
		for _, tag := range rgd.Tags {
			if tag != "" {
				tagSet[tag] = true
			}
		}
		if rgd.Category != "" {
			categorySet[rgd.Category] = true
		}
	}

	// Convert to sorted slices
	filterOpts := &RGDFilterOptions{
		Projects:   collection.SortedKeys(projectSet),
		Tags:       collection.SortedKeys(tagSet),
		Categories: collection.SortedKeys(categorySet),
	}

	// Cache filter options only when no PolicyEnforcer is active (results are shared across users).
	// With PolicyEnforcer, results are per-user Casbin-filtered and must not be shared.
	if s.redisClient != nil && s.policyEnforcer == nil {
		cacheKey := s.filtersCacheKey(accessibleProjects, includePublic)
		data, err := json.Marshal(filterOpts)
		if err == nil {
			if err := s.redisClient.Set(ctx, cacheKey, data, filtersCacheTTL).Err(); err != nil {
				s.logger.Warn("failed to cache RGD filters in redis", "error", err)
			} else {
				s.logger.Debug("cached RGD filters", "cache_key", cacheKey, "ttl", filtersCacheTTL)
			}
		}
	}

	return filterOpts, nil
}

// canAccessRGD checks if the user can access a specific RGD.
// Authorization is now category-scoped via Casbin: rgds/{category}/{name}.
// Falls back to project-based visibility when no PolicyEnforcer is configured.
func (s *CatalogService) canAccessRGD(ctx context.Context, rgd *models.CatalogRGD, authCtx *UserAuthContext) bool {
	if authCtx == nil {
		return false
	}

	// Note: Organization filtering is handled in GetRGD() (returns 404 to hide existence).
	// canAccessRGD checks category-scoped Casbin authorization (primary) and
	// project-level access (fallback).

	// Primary: Category-scoped Casbin authorization check
	if s.policyEnforcer != nil {
		obj, err := rbac.FormatRGDObject(rgd.Category, rgd.Name)
		if err != nil {
			s.logger.Warn("failed to format RGD object for authorization",
				"rgd_name", rgd.Name, "category", rgd.Category, "error", err)
			return false
		}

		allowed, err := s.policyEnforcer.CanAccessWithGroups(
			ctx,
			authCtx.UserID,
			authCtx.Groups,
			obj,
			rbac.ActionGet,
		)
		if err != nil {
			s.logger.Warn("RGD authorization check failed",
				"user_id", authCtx.UserID, "rgd_name", rgd.Name, "error", err)
			return false
		}
		return allowed
	}

	// Fallback: project-based visibility (when no PolicyEnforcer configured)
	projectLabel := ""
	if rgd.Labels != nil {
		projectLabel = rgd.Labels[models.RGDProjectLabel]
	}

	// Public RGD (no project label) - accessible to all authenticated users
	if projectLabel == "" {
		return true
	}

	// Check if user has access to the RGD's project
	for _, project := range authCtx.AccessibleProjects {
		if projectLabel == project {
			return true
		}
	}

	return false
}

// getFilteredInstanceCount returns the instance count for an RGD,
// filtered by the user's accessible namespaces.
func (s *CatalogService) getFilteredInstanceCount(rgd *models.CatalogRGD, authCtx *UserAuthContext) int {
	if s.instanceCounter == nil {
		return rgd.InstanceCount
	}

	var namespaces []string
	if authCtx != nil {
		namespaces = authCtx.AccessibleNamespaces
	}

	return s.instanceCounter.CountInstancesByRGDAndNamespaces(
		rgd.Namespace,
		rgd.Name,
		namespaces,
		rbac.MatchNamespaceInList,
	)
}

// filtersToListOptions converts service filters to model list options.
func (s *CatalogService) filtersToListOptions(filters RGDFilters) models.ListOptions {
	opts := models.DefaultListOptions()

	if filters.Namespace != "" {
		opts.Namespace = filters.Namespace
	}
	if filters.Category != "" {
		opts.Category = filters.Category
	}
	if filters.ExtendsKind != "" {
		opts.ExtendsKind = filters.ExtendsKind
	}
	if len(filters.Tags) > 0 {
		opts.Tags = filters.Tags
	}
	if filters.Search != "" {
		opts.Search = filters.Search
	}
	if filters.DependsOnKind != "" {
		opts.DependsOnKind = filters.DependsOnKind
	}
	if filters.ProducesKind != "" {
		opts.ProducesKind = filters.ProducesKind
	}
	if filters.ProducesGroup != "" {
		opts.ProducesGroup = filters.ProducesGroup
	}
	if filters.Status != "" {
		opts.Status = filters.Status
	}
	if filters.Page > 0 {
		opts.Page = filters.Page
	}
	if filters.PageSize > 0 {
		opts.PageSize = filters.PageSize
	}
	if filters.SortBy != "" {
		opts.SortBy = filters.SortBy
	}
	if filters.SortOrder != "" {
		opts.SortOrder = filters.SortOrder
	}

	return opts
}

// getRGDsWithCaching retrieves RGDs with Redis caching support.
func (s *CatalogService) getRGDsWithCaching(ctx context.Context, opts models.ListOptions) models.CatalogRGDList {
	var cacheKey string
	if s.redisClient != nil {
		cacheKey = s.listCacheKey(opts)
		data, err := s.redisClient.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var result models.CatalogRGDList
			if err := json.Unmarshal(data, &result); err == nil {
				s.logger.Debug("cache hit for RGD list", "cache_key", cacheKey, "count", len(result.Items))
				return result
			}
			s.logger.Warn("failed to unmarshal cached RGD list", "error", err)
		} else if err != redis.Nil {
			s.logger.Debug("failed to get cached RGD list from redis", "error", err)
		}
	}

	// Cache miss - get from provider
	result := s.rgdProvider.ListRGDs(opts)

	// Cache list results briefly - WebSocket provides real-time updates
	if s.redisClient != nil {
		data, err := json.Marshal(result)
		if err == nil {
			if err := s.redisClient.Set(ctx, cacheKey, data, listCacheTTL).Err(); err != nil {
				s.logger.Warn("failed to cache RGD list in redis", "error", err)
			} else {
				s.logger.Debug("cached RGD list", "cache_key", cacheKey, "count", len(result.Items), "ttl", listCacheTTL)
			}
		} else {
			s.logger.Warn("failed to marshal RGD list for caching", "error", err)
		}
	}

	return result
}

// listCacheKey generates a Redis cache key for RGD list based on list options.
func (s *CatalogService) listCacheKey(opts models.ListOptions) string {
	projects := make([]string, len(opts.Projects))
	copy(projects, opts.Projects)
	sort.Strings(projects)

	tags := make([]string, len(opts.Tags))
	copy(tags, opts.Tags)
	sort.Strings(tags)

	return fmt.Sprintf("rgd:list:org=%s:ns=%s:cat=%s:tags=%s:ek=%s:search=%s:dok=%s:pk=%s:pg=%s:projects=%s:public=%t:status=%s:page=%d:size=%d:sort=%s:%s",
		opts.Organization,
		opts.Namespace,
		opts.Category,
		strings.Join(tags, ","),
		opts.ExtendsKind,
		opts.Search,
		opts.DependsOnKind,
		opts.ProducesKind,
		opts.ProducesGroup,
		strings.Join(projects, ","),
		opts.IncludePublic,
		opts.Status,
		opts.Page,
		opts.PageSize,
		opts.SortBy,
		opts.SortOrder,
	)
}

// filtersCacheKey generates a Redis cache key for RGD filters.
func (s *CatalogService) filtersCacheKey(projects []string, includePublic bool) string {
	sortedProjects := make([]string, len(projects))
	copy(sortedProjects, projects)
	sort.Strings(sortedProjects)

	return fmt.Sprintf("rgd:filters:org=%s:projects=%s:public=%t",
		s.organizationFilter,
		strings.Join(sortedProjects, ","),
		includePublic,
	)
}
