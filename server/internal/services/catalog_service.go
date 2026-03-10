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
	// namespaces == nil means count all instances (global admin)
	// namespaces == [] means count is 0 (no access)
	CountInstancesByRGDAndNamespaces(rgdNamespace, rgdName string, namespaces []string, matchFn func(instanceNS string, namespaces []string) bool) int
}

// CatalogService encapsulates business logic for RGD catalog operations.
// It consolidates the visibility filtering, caching, and response formatting
// that was previously scattered across the RGDHandler.
type CatalogService struct {
	rgdProvider        RGDProvider
	instanceCounter    InstanceCounter
	redisClient        *redis.Client
	logger             *slog.Logger
	organizationFilter string // Enterprise org filter (empty = no filtering)
}

// CatalogServiceConfig holds configuration for creating a CatalogService.
type CatalogServiceConfig struct {
	RGDProvider        RGDProvider
	InstanceCounter    InstanceCounter
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

	// Apply organization filter (enterprise feature)
	opts.Organization = s.organizationFilter

	// Apply visibility filtering
	if authCtx != nil {
		opts.Projects = authCtx.AccessibleProjects
		opts.IncludePublic = true
	}

	// Try cache first
	result := s.getRGDsWithCaching(ctx, opts)

	// Convert to response format with instance counts
	items := make([]RGDResponse, len(result.Items))
	for i, rgd := range result.Items {
		instanceCount := s.getFilteredInstanceCount(&rgd, authCtx)
		items[i] = ToRGDResponse(&rgd, instanceCount)
	}

	return &ListRGDsResult{
		Items:      items,
		TotalCount: result.TotalCount,
		Page:       result.Page,
		PageSize:   result.PageSize,
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
		if !s.canAccessRGD(rgd, authCtx) {
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
func (s *CatalogService) GetCount(ctx context.Context, authCtx *UserAuthContext) (int, error) {
	if s.rgdProvider == nil {
		return 0, ErrServiceUnavailable
	}

	opts := models.ListOptions{
		Page:     1,
		PageSize: 1, // Minimal - we only need count
	}

	// Apply organization filter (enterprise feature)
	opts.Organization = s.organizationFilter

	if authCtx != nil {
		opts.Projects = authCtx.AccessibleProjects
		opts.IncludePublic = true
	}

	result := s.rgdProvider.ListRGDs(opts)
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

	// Try cache first
	if s.redisClient != nil {
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

	// Extract unique values
	projectSet := make(map[string]bool)
	tagSet := make(map[string]bool)
	categorySet := make(map[string]bool)

	for _, rgd := range result.Items {
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

	// Cache filter options - they change infrequently (only when RGDs are added/removed)
	if s.redisClient != nil {
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

// canAccessRGD checks if the user can access a specific RGD based on project visibility.
// An RGD is accessible if:
// - It's public (no project label)
// - The user's accessible projects include the RGD's project label
func (s *CatalogService) canAccessRGD(rgd *models.CatalogRGD, authCtx *UserAuthContext) bool {
	if authCtx == nil {
		return false
	}

	// Note: Organization filtering is handled in GetRGD() (returns 404 to hide existence).
	// canAccessRGD only checks project-level access (returns 403 for denied).

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
	if len(filters.Tags) > 0 {
		opts.Tags = filters.Tags
	}
	if filters.Search != "" {
		opts.Search = filters.Search
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
	if s.redisClient != nil {
		cacheKey := s.listCacheKey(opts)
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
		cacheKey := s.listCacheKey(opts)
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

	return fmt.Sprintf("rgd:list:org=%s:ns=%s:cat=%s:tags=%s:search=%s:projects=%s:public=%t:page=%d:size=%d:sort=%s:%s",
		opts.Organization,
		opts.Namespace,
		opts.Category,
		strings.Join(tags, ","),
		opts.Search,
		strings.Join(projects, ","),
		opts.IncludePublic,
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
	return fmt.Sprintf("rgd:filters:org=%s:projects=%s:public=%t", s.organizationFilter, strings.Join(sortedProjects, ","), includePublic)
}
