// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package services provides business logic services following clean architecture principles.
// Services encapsulate business logic that was previously scattered across HTTP handlers,
// improving testability, maintainability, and adherence to the Single Responsibility Principle.
package services

import (
	"errors"

	"github.com/knodex/knodex/server/internal/api/middleware"
	kroparser "github.com/knodex/knodex/server/internal/kro/parser"
	"github.com/knodex/knodex/server/internal/models"
)

// Common service errors for consistent error handling across services.
var (
	// ErrNotFound indicates the requested resource was not found.
	ErrNotFound = errors.New("resource not found")
	// ErrForbidden indicates the user does not have access to the resource.
	ErrForbidden = errors.New("access forbidden")
	// ErrServiceUnavailable indicates the service is not available.
	ErrServiceUnavailable = errors.New("service unavailable")
)

// UserAuthContext contains pre-computed authorization data for a user.
// This consolidates the repeated pattern of fetching accessible projects and namespaces
// that was duplicated across 10+ handlers.
type UserAuthContext struct {
	// UserID is the unique identifier for the user (from JWT sub claim)
	UserID string

	// Groups contains OIDC groups for the user (for runtime authorization)
	Groups []string

	// Roles contains Casbin roles from the JWT token (e.g., "role:serveradmin", "proj:acme:developer").
	// NOTE: These are for informational/display purposes only. Server-side authorization
	// uses GetImplicitRolesForUser from Casbin's authoritative state (STORY-228).
	Roles []string

	// AccessibleProjects contains project names the user can access based on Casbin policies.
	// Global admins will have all projects (via wildcard policy match, not special code path).
	AccessibleProjects []string

	// AccessibleNamespaces contains Kubernetes namespaces the user can access.
	// ["*"] = user can access all namespaces (global admin)
	// empty slice = user has no namespace access (secure default, fail-closed)
	// non-empty = user can only access these namespaces
	// SECURITY: Never nil. A zero-value []string{} means "no access".
	// Admin access requires an explicit ["*"] entry from the authorization service.
	AccessibleNamespaces []string
}

// NewUserAuthContextFromMiddleware creates a UserAuthContext from the middleware UserContext.
// This is a helper for cases where you only need the basic user info without the computed
// accessible projects/namespaces.
func NewUserAuthContextFromMiddleware(userCtx *middleware.UserContext) *UserAuthContext {
	if userCtx == nil {
		return nil
	}
	return &UserAuthContext{
		UserID:               userCtx.UserID,
		Groups:               userCtx.Groups,
		Roles:                userCtx.CasbinRoles,
		AccessibleProjects:   []string{}, // fail-closed: no projects without auth service
		AccessibleNamespaces: []string{}, // fail-closed: no namespaces without auth service
	}
}

// RGDFilters represents filter criteria for RGD listing.
// This consolidates query parameter parsing that was repeated in handlers.
type RGDFilters struct {
	// Namespace filters RGDs by their Kubernetes namespace
	Namespace string

	// Category filters RGDs by category annotation
	Category string

	// Tags filters RGDs that have any of the specified tags
	Tags []string

	// ExtendsKind filters RGDs that extend the specified Kind
	ExtendsKind string

	// Search performs case-insensitive search on name, title, and description
	Search string

	// DependsOnKind filters RGDs that depend on a specific Kind via externalRef
	DependsOnKind string

	// ProducesKind filters RGDs that produce a specific Kind as a non-external resource
	ProducesKind string

	// ProducesGroup narrows ProducesKind filtering to a specific API group (optional)
	ProducesGroup string

	// Status filters RGDs by status (e.g., "Active", "Inactive"). Empty = all statuses.
	Status string

	// Page is the page number (1-indexed)
	Page int

	// PageSize is the number of items per page (max 100)
	PageSize int

	// SortBy specifies the field to sort by (name, namespace, createdAt, updatedAt, category)
	SortBy string

	// SortOrder specifies the sort direction (asc, desc)
	SortOrder string
}

// DefaultRGDFilters returns filters with sensible defaults.
func DefaultRGDFilters() RGDFilters {
	return RGDFilters{
		Page:      1,
		PageSize:  20,
		SortBy:    "name",
		SortOrder: "asc",
	}
}

// ListRGDsResult represents the result of listing RGDs.
type ListRGDsResult struct {
	// Items contains the RGD responses for the current page
	Items []RGDResponse

	// TotalCount is the total number of RGDs matching the filters
	TotalCount int

	// Page is the current page number
	Page int

	// PageSize is the number of items per page
	PageSize int
}

// RGDResponse represents an RGD in the API response.
// This matches the existing handler response format for backward compatibility.
type RGDResponse struct {
	Name                   string                `json:"name"`
	Title                  string                `json:"title"`
	Namespace              string                `json:"namespace"`
	Description            string                `json:"description"`
	Tags                   []string              `json:"tags"`
	Category               string                `json:"category"`
	Icon                   string                `json:"icon,omitempty"`
	DocsURL                string                `json:"docsUrl,omitempty"`
	CatalogTier            string                `json:"catalogTier"`
	Labels                 map[string]string     `json:"labels"`
	Instances              int                   `json:"instances"`
	APIVersion             string                `json:"apiVersion,omitempty"`
	Kind                   string                `json:"kind,omitempty"`
	ExtendsKinds           []string              `json:"extendsKinds,omitempty"`
	Status                 string                `json:"status,omitempty"`
	DependsOnKinds         []string              `json:"dependsOnKinds,omitempty"`
	ProducesKinds          []models.GVKRef       `json:"producesKinds,omitempty"`
	SecretRefs             []kroparser.SecretRef `json:"secretRefs,omitempty"`
	AllowedDeploymentModes []string              `json:"allowedDeploymentModes,omitempty"`
	// IsClusterScoped indicates the RGD produces cluster-scoped (not namespaced) instance CRDs.
	// Intentionally omits `omitempty` so the field is always present in JSON responses,
	// allowing frontends to distinguish "namespace-scoped" (false) from "field absent".
	IsClusterScoped bool   `json:"isClusterScoped"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// RGDFilterOptions represents available filter values for the catalog UI.
type RGDFilterOptions struct {
	// Projects contains project names visible to the user
	Projects []string `json:"projects"`

	// Tags contains all unique tags from visible RGDs
	Tags []string `json:"tags"`

	// Categories contains all unique categories from visible RGDs
	Categories []string `json:"categories"`
}

// GraphRevisionProvider provides access to GraphRevision data.
// Implementations: *watcher.GraphRevisionWatcher
type GraphRevisionProvider interface {
	ListRevisions(rgdName string) models.GraphRevisionList
	GetRevision(rgdName string, revision int) (*models.GraphRevision, bool)
	GetLatestRevision(rgdName string) (*models.GraphRevision, bool)
}

// InstanceUIDProvider resolves Kubernetes UIDs to cached Instance objects.
// Implementations: *watcher.InstanceTracker
type InstanceUIDProvider interface {
	GetInstanceByUID(uid string) (*models.Instance, bool)
}

// CountResult represents a simple count response.
type CountResult struct {
	Count int `json:"count"`
}

// ToRGDResponse converts a CatalogRGD to an API response format.
// userNamespaces filters the instance count:
// - nil: user can see all instances (global admin)
// - empty slice: user has no namespace access, count will be 0
// - non-empty: count only instances in these namespaces
func ToRGDResponse(rgd *models.CatalogRGD, instanceCount int) RGDResponse {
	tags := rgd.Tags
	if tags == nil {
		tags = []string{}
	}

	labels := rgd.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	return RGDResponse{
		Name:                   rgd.Name,
		Title:                  rgd.Title,
		Namespace:              rgd.Namespace,
		Description:            rgd.Description,
		Tags:                   tags,
		Category:               rgd.Category,
		Icon:                   rgd.Icon,
		DocsURL:                rgd.DocsURL,
		CatalogTier:            rgd.CatalogTier,
		Labels:                 labels,
		Instances:              instanceCount,
		APIVersion:             rgd.APIVersion,
		Kind:                   rgd.Kind,
		ExtendsKinds:           rgd.ExtendsKinds,
		Status:                 rgd.Status,
		DependsOnKinds:         rgd.DependsOnKinds,
		ProducesKinds:          rgd.ProducesKinds,
		SecretRefs:             rgd.SecretRefs,
		AllowedDeploymentModes: rgd.AllowedDeploymentModes,
		IsClusterScoped:        rgd.IsClusterScoped,
		CreatedAt:              rgd.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:              rgd.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
