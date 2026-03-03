package models

import (
	"strings"
	"time"
)

// Annotation keys for RGD catalog discovery
const (
	// CatalogAnnotation indicates the RGD should be included in the catalog
	CatalogAnnotation = "knodex.io/catalog"
	// DescriptionAnnotation provides a human-readable description
	DescriptionAnnotation = "knodex.io/description"
	// TagsAnnotation provides comma-separated tags for filtering
	TagsAnnotation = "knodex.io/tags"
	// CategoryAnnotation provides the catalog category
	CategoryAnnotation = "knodex.io/category"
	// IconAnnotation provides the UI icon hint
	IconAnnotation = "knodex.io/icon"
	// VersionAnnotation provides the version (if not in spec)
	VersionAnnotation = "knodex.io/version"
	// TitleAnnotation provides a human-readable display title for the catalog
	TitleAnnotation = "knodex.io/title"
	// DeploymentModesAnnotation restricts allowed deployment modes (comma-separated: direct,gitops,hybrid)
	// If missing or empty, all modes are allowed (backward compatible)
	DeploymentModesAnnotation = "knodex.io/deployment-modes"
)

// Label keys for RBAC project filtering on cluster-scoped RGDs
// These are different from instance labels since RGDs don't have namespaces
const (
	// RGDProjectLabel associates an RGD with a specific project
	// Value should be the namespace name (e.g., "alpha-team"), not project ID (e.g., "proj-alpha-team")
	// When set, restricts visibility to project members only
	RGDProjectLabel = "knodex.io/project"
	// RGDOrganizationLabel scopes the RGD to a specific organization
	// Empty/missing means the RGD is shared across all organizations
	RGDOrganizationLabel = "knodex.io/organization"
)

// KRO CRD information
const (
	// RGDGVR is the GroupVersionResource for ResourceGraphDefinitions
	RGDGroup    = "kro.run"
	RGDVersion  = "v1alpha1"
	RGDResource = "resourcegraphdefinitions"
	RGDKind     = "ResourceGraphDefinition"
)

// CatalogRGD represents a ResourceGraphDefinition in the catalog
// This is the internal model used by the watcher and cache
type CatalogRGD struct {
	// Name is the RGD resource name
	Name string `json:"name"`
	// Title is a human-readable display name from annotations (falls back to Name)
	Title string `json:"title"`
	// Namespace is the RGD namespace
	Namespace string `json:"namespace"`
	// Description is extracted from annotations
	Description string `json:"description"`
	// Version is extracted from annotations or spec
	Version string `json:"version"`
	// Tags are extracted from annotations (comma-separated)
	Tags []string `json:"tags"`
	// Category is extracted from annotations
	Category string `json:"category"`
	// Icon is the UI icon hint from annotations
	Icon string `json:"icon"`
	// Organization is the org scope from labels (empty = shared/public RGD)
	Organization string `json:"organization,omitempty"`
	// Labels from the RGD metadata
	Labels map[string]string `json:"labels"`
	// Annotations from the RGD metadata
	Annotations map[string]string `json:"annotations"`
	// InstanceCount tracks how many instances of this RGD exist
	// This will be populated by the instance tracker later
	InstanceCount int `json:"instanceCount"`
	// APIVersion is the API version of CRs created by this RGD
	APIVersion string `json:"apiVersion,omitempty"`
	// Kind is the Kind of CRs created by this RGD
	Kind string `json:"kind,omitempty"`
	// Status is the KRO processing state (e.g., "Active", "Inactive")
	// Empty/missing status means KRO has not yet processed the RGD
	Status string `json:"status"`
	// AllowedDeploymentModes restricts which deployment modes can be used
	// Valid values: "direct", "gitops", "hybrid" (case-insensitive)
	// Empty slice means all modes are allowed (default, backward compatible)
	AllowedDeploymentModes []string `json:"allowedDeploymentModes,omitempty"`
	// CreatedAt is when the RGD was created in the cluster
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the RGD was last updated
	UpdatedAt time.Time `json:"updatedAt"`
	// ResourceVersion for optimistic concurrency
	ResourceVersion string `json:"-"`
	// RawSpec stores the raw RGD spec for dependency parsing
	RawSpec map[string]interface{} `json:"-"`
}

// CatalogRGDList represents a paginated list of RGDs
type CatalogRGDList struct {
	Items      []CatalogRGD `json:"items"`
	TotalCount int          `json:"totalCount"`
	Page       int          `json:"page"`
	PageSize   int          `json:"pageSize"`
}

// ListOptions contains options for listing RGDs
type ListOptions struct {
	// Namespace filters by namespace (empty = all namespaces)
	// Deprecated: Use Namespaces for multi-namespace filtering
	Namespace string
	// Namespaces filters by multiple namespaces (empty = all namespaces)
	// If both Namespace and Namespaces are set, Namespaces takes precedence
	Namespaces []string
	// Projects filters RGDs by project label (for cluster-scoped RGDs)
	// RGDs must have the knodex.io/project label matching one of these values
	// Empty = all projects (global admin view)
	Projects []string
	// Organization filters RGDs by organization annotation (enterprise feature)
	// Empty = no org filtering (OSS default or global admin override)
	// Non-empty = filter: show shared RGDs (no org annotation) + matching org RGDs
	Organization string
	// IncludePublic includes public RGDs (catalog: true with no project label) regardless of project
	// When true, users see: public RGDs + their project RGDs
	// When false with empty/nil Projects: no visibility filtering (admin view)
	IncludePublic bool
	// Tags filters by tags (AND logic)
	Tags []string
	// Category filters by category
	Category string
	// Search filters by name/title/description (case-insensitive contains)
	Search string
	// Page is the page number (1-indexed)
	Page int
	// PageSize is the number of items per page
	PageSize int
	// SortBy is the field to sort by (name, createdAt, updatedAt)
	SortBy string
	// SortOrder is asc or desc
	SortOrder string
}

// DefaultListOptions returns default list options
func DefaultListOptions() ListOptions {
	return ListOptions{
		Page:      1,
		PageSize:  20,
		SortBy:    "name",
		SortOrder: "asc",
	}
}

// ValidDeploymentModes contains all valid deployment mode values
var ValidDeploymentModes = map[string]bool{
	"direct": true,
	"gitops": true,
	"hybrid": true,
}

// ParseDeploymentModesResult contains the result of parsing deployment modes
type ParseDeploymentModesResult struct {
	// ValidModes contains all valid, lowercase mode strings
	ValidModes []string
	// InvalidModes contains any modes that were not recognized
	InvalidModes []string
}

// ParseDeploymentModes parses a comma-separated deployment modes annotation value
// Returns a slice of valid, lowercase mode strings
// Invalid values are ignored (with warning logged by caller)
// Empty or whitespace-only input returns nil (all modes allowed)
func ParseDeploymentModes(annotation string) []string {
	result := ParseDeploymentModesWithInvalid(annotation)
	return result.ValidModes
}

// ParseDeploymentModesWithInvalid parses deployment modes and also returns invalid modes
// This is useful for logging warnings about unrecognized mode values
func ParseDeploymentModesWithInvalid(annotation string) ParseDeploymentModesResult {
	annotation = strings.TrimSpace(annotation)
	if annotation == "" {
		return ParseDeploymentModesResult{}
	}

	parts := strings.Split(annotation, ",")
	var validModes []string
	var invalidModes []string
	seen := make(map[string]bool)

	for _, part := range parts {
		mode := strings.ToLower(strings.TrimSpace(part))
		if mode == "" {
			continue
		}
		// Only include valid modes, skip duplicates
		if ValidDeploymentModes[mode] {
			if !seen[mode] {
				validModes = append(validModes, mode)
				seen[mode] = true
			}
		} else {
			invalidModes = append(invalidModes, mode)
		}
	}

	if len(validModes) == 0 {
		validModes = nil
	}
	return ParseDeploymentModesResult{
		ValidModes:   validModes,
		InvalidModes: invalidModes,
	}
}

// IsDeploymentModeAllowed checks if a deployment mode is allowed for an RGD.
//
// Backward Compatibility Design:
//   - If allowedModes is nil or empty, ALL modes are allowed (direct, gitops, hybrid).
//   - This ensures existing RGDs without the knodex.io/deployment-modes annotation
//     continue to work with any deployment mode after upgrading.
//   - Similarly, if the annotation contains only invalid values, ParseDeploymentModes
//     returns nil, resulting in unrestricted mode access.
//
// Parameters:
//   - allowedModes: slice of lowercase mode strings from ParseDeploymentModes(), or nil/empty for unrestricted
//   - mode: the deployment mode to check (case-insensitive)
//
// Returns true if the mode is allowed, false otherwise.
func IsDeploymentModeAllowed(allowedModes []string, mode string) bool {
	// Backward compatibility: empty/nil means all modes are allowed
	// This is intentional - existing RGDs without annotation should not break
	if len(allowedModes) == 0 {
		return true
	}
	modeLower := strings.ToLower(mode)
	for _, allowed := range allowedModes {
		if strings.ToLower(allowed) == modeLower {
			return true
		}
	}
	return false
}
