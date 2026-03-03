package watcher

import (
	"sort"
	"strings"
	"sync"

	"github.com/provops-org/knodex/server/internal/models"
)

// RGDCache provides thread-safe storage for discovered RGDs
type RGDCache struct {
	mu           sync.RWMutex
	rgds         map[string]*models.CatalogRGD // key: namespace/name
	tagIndex     map[string]map[string]bool    // tag -> set of cache keys (namespace/name)
	projectIndex map[string]map[string]bool    // project -> set of cache keys (namespace/name)
}

// NewRGDCache creates a new RGD cache
func NewRGDCache() *RGDCache {
	return &RGDCache{
		rgds:         make(map[string]*models.CatalogRGD),
		tagIndex:     make(map[string]map[string]bool),
		projectIndex: make(map[string]map[string]bool),
	}
}

// cacheKey generates a unique key for an RGD
func cacheKey(namespace, name string) string {
	return namespace + "/" + name
}

// Set adds or updates an RGD in the cache
func (c *RGDCache) Set(rgd *models.CatalogRGD) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(rgd.Namespace, rgd.Name)

	// If updating an existing RGD, remove old index entries
	if oldRGD, exists := c.rgds[key]; exists {
		// Remove old tag index entries (using normalized tags)
		for _, tag := range oldRGD.Tags {
			normalizedTag := strings.ToLower(tag)
			if keys, ok := c.tagIndex[normalizedTag]; ok {
				delete(keys, key)
				// Clean up empty sets
				if len(keys) == 0 {
					delete(c.tagIndex, normalizedTag)
				}
			}
		}

		// Remove old project index entry
		if oldRGD.Labels != nil {
			if projectLabel, ok := oldRGD.Labels[models.RGDProjectLabel]; ok && projectLabel != "" {
				if keys, ok := c.projectIndex[projectLabel]; ok {
					delete(keys, key)
					// Clean up empty sets
					if len(keys) == 0 {
						delete(c.projectIndex, projectLabel)
					}
				}
			}
		}
	}

	// Add/update the RGD in the main cache
	c.rgds[key] = rgd

	// Add new tag index entries (normalized to lowercase for case-insensitive matching)
	for _, tag := range rgd.Tags {
		normalizedTag := strings.ToLower(tag)
		if c.tagIndex[normalizedTag] == nil {
			c.tagIndex[normalizedTag] = make(map[string]bool)
		}
		c.tagIndex[normalizedTag][key] = true
	}

	// Add new project index entry
	if rgd.Labels != nil {
		if projectLabel, ok := rgd.Labels[models.RGDProjectLabel]; ok && projectLabel != "" {
			if c.projectIndex[projectLabel] == nil {
				c.projectIndex[projectLabel] = make(map[string]bool)
			}
			c.projectIndex[projectLabel][key] = true
		}
	}
}

// Get retrieves an RGD from the cache
func (c *RGDCache) Get(namespace, name string) (*models.CatalogRGD, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := cacheKey(namespace, name)
	rgd, ok := c.rgds[key]
	return rgd, ok
}

// Delete removes an RGD from the cache
func (c *RGDCache) Delete(namespace, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(namespace, name)

	// Get the existing RGD to clean up index entries
	if oldRGD, exists := c.rgds[key]; exists {
		// Remove tag index entries (using normalized tags)
		for _, tag := range oldRGD.Tags {
			normalizedTag := strings.ToLower(tag)
			if keys, ok := c.tagIndex[normalizedTag]; ok {
				delete(keys, key)
				// Clean up empty sets
				if len(keys) == 0 {
					delete(c.tagIndex, normalizedTag)
				}
			}
		}

		// Remove project index entry
		if oldRGD.Labels != nil {
			if projectLabel, ok := oldRGD.Labels[models.RGDProjectLabel]; ok && projectLabel != "" {
				if keys, ok := c.projectIndex[projectLabel]; ok {
					delete(keys, key)
					// Clean up empty sets
					if len(keys) == 0 {
						delete(c.projectIndex, projectLabel)
					}
				}
			}
		}
	}

	// Delete from main cache
	delete(c.rgds, key)
}

// List returns all RGDs matching the given options
func (c *RGDCache) List(opts models.ListOptions) models.CatalogRGDList {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Use indexes to build candidate set when possible
	candidateKeys := c.getCandidateKeys(opts)

	// Collect all matching RGDs from candidates
	var matches []models.CatalogRGD
	if candidateKeys != nil {
		// Use filtered candidate set (from indexes)
		for key := range candidateKeys {
			if rgd, ok := c.rgds[key]; ok {
				if c.matchesFilter(rgd, opts) {
					matches = append(matches, *rgd)
				}
			}
		}
	} else {
		// No index optimization possible, iterate all RGDs
		for _, rgd := range c.rgds {
			if c.matchesFilter(rgd, opts) {
				matches = append(matches, *rgd)
			}
		}
	}

	// Sort results
	c.sortRGDs(matches, opts.SortBy, opts.SortOrder)

	// Apply pagination
	totalCount := len(matches)
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize

	if start >= totalCount {
		return models.CatalogRGDList{
			Items:      []models.CatalogRGD{},
			TotalCount: totalCount,
			Page:       opts.Page,
			PageSize:   opts.PageSize,
		}
	}

	if end > totalCount {
		end = totalCount
	}

	return models.CatalogRGDList{
		Items:      matches[start:end],
		TotalCount: totalCount,
		Page:       opts.Page,
		PageSize:   opts.PageSize,
	}
}

// getCandidateKeys returns a filtered set of RGD keys using indexes
// Returns nil if no index optimization is applicable (fall back to full scan)
func (c *RGDCache) getCandidateKeys(opts models.ListOptions) map[string]bool {
	var tagCandidates map[string]bool
	var projectCandidates map[string]bool

	// Use tag index if tags filter is present
	if len(opts.Tags) > 0 {
		// For AND logic (all tags must match), intersect all tag sets
		for i, tag := range opts.Tags {
			// Normalize tag for case-insensitive lookup
			normalizedTag := strings.ToLower(tag)
			tagKeys := c.tagIndex[normalizedTag]
			if tagKeys == nil {
				// Tag doesn't exist in index, no RGDs have this tag
				return make(map[string]bool) // Return empty set
			}
			if i == 0 {
				// First tag: initialize candidates
				tagCandidates = make(map[string]bool)
				for key := range tagKeys {
					tagCandidates[key] = true
				}
			} else {
				// Subsequent tags: intersect with existing candidates
				for key := range tagCandidates {
					if !tagKeys[key] {
						delete(tagCandidates, key)
					}
				}
			}
		}
	}

	// Use project index if projects filter is present
	// BUT: Cannot use index when IncludePublic is true because public RGDs
	// (no project label) are not in the projectIndex
	if len(opts.Projects) > 0 && !opts.IncludePublic {
		projectCandidates = make(map[string]bool)
		// For OR logic (any project matches), union all project sets
		for _, project := range opts.Projects {
			if projectKeys := c.projectIndex[project]; projectKeys != nil {
				for key := range projectKeys {
					projectCandidates[key] = true
				}
			}
		}
	}

	// Combine tag and project candidates
	if tagCandidates != nil && projectCandidates != nil {
		// Both filters: intersect the sets
		result := make(map[string]bool)
		for key := range tagCandidates {
			if projectCandidates[key] {
				result[key] = true
			}
		}
		return result
	} else if tagCandidates != nil {
		// Only tag filter
		return tagCandidates
	} else if projectCandidates != nil {
		// Only project filter
		return projectCandidates
	}

	// No index optimization applicable
	return nil
}

// Count returns the total number of RGDs in the cache
func (c *RGDCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.rgds)
}

// All returns all RGDs in the cache (for debugging/testing)
func (c *RGDCache) All() []*models.CatalogRGD {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*models.CatalogRGD, 0, len(c.rgds))
	for _, rgd := range c.rgds {
		result = append(result, rgd)
	}
	return result
}

// Clear removes all RGDs from the cache
func (c *RGDCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rgds = make(map[string]*models.CatalogRGD)
	c.tagIndex = make(map[string]map[string]bool)
	c.projectIndex = make(map[string]map[string]bool)
}

// matchesFilter checks if an RGD matches the filter options
func (c *RGDCache) matchesFilter(rgd *models.CatalogRGD, opts models.ListOptions) bool {
	// Simplified visibility model:
	// - knodex.io/catalog: "true" is the GATEWAY to the catalog
	// - RGDs without catalog annotation are NOT part of the catalog system
	// - catalog: true alone = visible to ALL authenticated users (public)
	// - catalog: true + project label = visible to project members only
	//
	// Visibility rules:
	// | catalog annotation | project label | Behavior                          |
	// |-------------------|---------------|-----------------------------------|
	// | (none)            | (any)         | NOT in catalog (filtered out)     |
	// | "true"            | (none)        | All authenticated users           |
	// | "true"            | proj-xxx      | Project members only              |
	//
	// Admin view (no IncludePublic and no Projects): Shows all catalog RGDs
	// User view (IncludePublic=true): Shows public RGDs + user's project RGDs

	// Get catalog annotation - this is the gateway to the catalog
	catalogValue := ""
	if rgd.Annotations != nil {
		catalogValue = rgd.Annotations[models.CatalogAnnotation]
	}
	isInCatalog := catalogValue == "true"

	// RGDs without catalog annotation are not part of the catalog
	// They are invisible to everyone, including admins (not ingested)
	if !isInCatalog {
		return false
	}

	// Organization filter (enterprise feature)
	// When active: shared RGDs (empty org) pass, matching org passes, mismatching org filtered
	if opts.Organization != "" {
		if rgd.Organization != "" && rgd.Organization != opts.Organization {
			return false
		}
	}

	// For user views (IncludePublic or specific Projects), apply visibility rules
	if opts.IncludePublic || len(opts.Projects) > 0 {
		// Get project label from the RGD
		projectLabel := ""
		if rgd.Labels != nil {
			projectLabel = rgd.Labels[models.RGDProjectLabel]
		}

		// Determine visibility:
		// - catalog: true with no project label = public (visible to all)
		// - catalog: true with project label = visible to project members only
		isPublic := projectLabel == ""
		isInUserProject := false

		if len(opts.Projects) > 0 && projectLabel != "" {
			for _, project := range opts.Projects {
				if projectLabel == project {
					isInUserProject = true
					break
				}
			}
		}

		// Apply visibility rules:
		// - Public RGDs (no project label) are visible when IncludePublic is set
		// - Project RGDs are visible only to users in that project
		if opts.IncludePublic && isPublic {
			// Allow public RGDs through (catalog: true, no project label)
		} else if isInUserProject {
			// Allow project RGDs through
		} else {
			// RGD doesn't match visibility rules - filter it out
			return false
		}
	}

	// Namespace filter - support both single and multi-namespace filtering
	// Namespaces takes precedence over Namespace for backward compatibility
	// Note: For RGDs (cluster-scoped), namespace is usually empty
	if len(opts.Namespaces) > 0 {
		// Multi-namespace filter: RGD must be in one of the specified namespaces
		found := false
		for _, ns := range opts.Namespaces {
			if rgd.Namespace == ns {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	} else if opts.Namespace != "" {
		// Single namespace filter (backward compatibility)
		if rgd.Namespace != opts.Namespace {
			return false
		}
	}

	// Category filter
	if opts.Category != "" && rgd.Category != opts.Category {
		return false
	}

	// Tags filter (AND logic - all tags must match)
	if len(opts.Tags) > 0 {
		rgdTags := make(map[string]bool)
		for _, tag := range rgd.Tags {
			rgdTags[strings.ToLower(tag)] = true
		}
		for _, filterTag := range opts.Tags {
			if !rgdTags[strings.ToLower(filterTag)] {
				return false
			}
		}
	}

	// Search filter (case-insensitive contains on name, title, and description)
	if opts.Search != "" {
		search := strings.ToLower(opts.Search)
		nameMatch := strings.Contains(strings.ToLower(rgd.Name), search)
		titleMatch := strings.Contains(strings.ToLower(rgd.Title), search)
		descMatch := strings.Contains(strings.ToLower(rgd.Description), search)
		if !nameMatch && !titleMatch && !descMatch {
			return false
		}
	}

	return true
}

// sortRGDs sorts the RGD slice in place
func (c *RGDCache) sortRGDs(rgds []models.CatalogRGD, sortBy, sortOrder string) {
	if len(rgds) == 0 {
		return
	}

	ascending := sortOrder != "desc"

	sort.SliceStable(rgds, func(i, j int) bool {
		var less, equal bool
		switch sortBy {
		case "createdAt":
			less = rgds[i].CreatedAt.Before(rgds[j].CreatedAt)
			equal = rgds[i].CreatedAt.Equal(rgds[j].CreatedAt)
		case "updatedAt":
			less = rgds[i].UpdatedAt.Before(rgds[j].UpdatedAt)
			equal = rgds[i].UpdatedAt.Equal(rgds[j].UpdatedAt)
		default: // "name" and any unrecognized sort field
			ni := strings.ToLower(rgds[i].Name)
			nj := strings.ToLower(rgds[j].Name)
			less = ni < nj
			equal = ni == nj
		}

		// Tie-break by namespace/name for deterministic order
		if equal {
			ki := strings.ToLower(rgds[i].Namespace + "/" + rgds[i].Name)
			kj := strings.ToLower(rgds[j].Namespace + "/" + rgds[j].Name)
			if ascending {
				return ki < kj
			}
			return ki > kj
		}

		if ascending {
			return less
		}
		return !less
	})
}
