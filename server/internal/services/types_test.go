package services

import (
	"testing"
	"time"

	"github.com/provops-org/knodex/server/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestToRGDResponse(t *testing.T) {
	now := time.Now()
	rgd := &models.CatalogRGD{
		Name:        "test-rgd",
		Title:       "Test RGD Display Title",
		Namespace:   "default",
		Description: "Test RGD description",
		Version:     "v1.0.0",
		Tags:        []string{"database", "postgres"},
		Category:    "Databases",
		Icon:        "postgres-icon",
		Labels: map[string]string{
			"knodex.io/project": "my-project",
		},
		APIVersion:    "kro.io/v1alpha1",
		Kind:          "ResourceGraphDefinition",
		CreatedAt:     now,
		UpdatedAt:     now,
		InstanceCount: 5,
	}

	resp := ToRGDResponse(rgd, 10)

	assert.Equal(t, "test-rgd", resp.Name)
	assert.Equal(t, "Test RGD Display Title", resp.Title)
	assert.Equal(t, "default", resp.Namespace)
	assert.Equal(t, "Test RGD description", resp.Description)
	assert.Equal(t, "v1.0.0", resp.Version)
	assert.Equal(t, []string{"database", "postgres"}, resp.Tags)
	assert.Equal(t, "Databases", resp.Category)
	assert.Equal(t, "postgres-icon", resp.Icon)
	assert.Equal(t, map[string]string{"knodex.io/project": "my-project"}, resp.Labels)
	assert.Equal(t, 10, resp.Instances) // Uses provided count, not rgd.InstanceCount
	assert.Equal(t, "kro.io/v1alpha1", resp.APIVersion)
	assert.Equal(t, "ResourceGraphDefinition", resp.Kind)
	assert.Equal(t, now.Format("2006-01-02T15:04:05Z"), resp.CreatedAt)
	assert.Equal(t, now.Format("2006-01-02T15:04:05Z"), resp.UpdatedAt)
}

func TestToRGDResponse_NilTags(t *testing.T) {
	rgd := &models.CatalogRGD{
		Name:      "test-rgd",
		Namespace: "default",
		Tags:      nil, // nil tags
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := ToRGDResponse(rgd, 0)

	// Should return empty slice, not nil
	assert.NotNil(t, resp.Tags)
	assert.Empty(t, resp.Tags)
}

func TestToRGDResponse_NilLabels(t *testing.T) {
	rgd := &models.CatalogRGD{
		Name:      "test-rgd",
		Namespace: "default",
		Labels:    nil, // nil labels
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := ToRGDResponse(rgd, 0)

	// Should return empty map, not nil
	assert.NotNil(t, resp.Labels)
	assert.Empty(t, resp.Labels)
}

func TestToRGDResponse_DateFormatting(t *testing.T) {
	// Test specific date formatting
	created := time.Date(2026, 1, 15, 10, 30, 45, 0, time.UTC)
	updated := time.Date(2026, 1, 20, 14, 45, 30, 0, time.UTC)

	rgd := &models.CatalogRGD{
		Name:      "test-rgd",
		Namespace: "default",
		CreatedAt: created,
		UpdatedAt: updated,
	}

	resp := ToRGDResponse(rgd, 0)

	assert.Equal(t, "2026-01-15T10:30:45Z", resp.CreatedAt)
	assert.Equal(t, "2026-01-20T14:45:30Z", resp.UpdatedAt)
}

func TestRGDFiltersStruct(t *testing.T) {
	filters := RGDFilters{
		Namespace: "production",
		Category:  "Databases",
		Tags:      []string{"postgres", "ha"},
		Search:    "my-db",
		Page:      2,
		PageSize:  50,
		SortBy:    "createdAt",
		SortOrder: "desc",
	}

	assert.Equal(t, "production", filters.Namespace)
	assert.Equal(t, "Databases", filters.Category)
	assert.Equal(t, []string{"postgres", "ha"}, filters.Tags)
	assert.Equal(t, "my-db", filters.Search)
	assert.Equal(t, 2, filters.Page)
	assert.Equal(t, 50, filters.PageSize)
	assert.Equal(t, "createdAt", filters.SortBy)
	assert.Equal(t, "desc", filters.SortOrder)
}

func TestListRGDsResultStruct(t *testing.T) {
	result := ListRGDsResult{
		Items: []RGDResponse{
			{Name: "rgd-1"},
			{Name: "rgd-2"},
		},
		TotalCount: 100,
		Page:       1,
		PageSize:   20,
	}

	assert.Len(t, result.Items, 2)
	assert.Equal(t, 100, result.TotalCount)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.PageSize)
}

func TestRGDFilterOptionsStruct(t *testing.T) {
	opts := RGDFilterOptions{
		Projects:   []string{"project-a", "project-b"},
		Tags:       []string{"database", "cache"},
		Categories: []string{"Databases", "Caching"},
	}

	assert.Equal(t, []string{"project-a", "project-b"}, opts.Projects)
	assert.Equal(t, []string{"database", "cache"}, opts.Tags)
	assert.Equal(t, []string{"Databases", "Caching"}, opts.Categories)
}

func TestCountResultStruct(t *testing.T) {
	result := CountResult{Count: 42}
	assert.Equal(t, 42, result.Count)
}

func TestUserAuthContextStruct(t *testing.T) {
	authCtx := UserAuthContext{
		UserID:               "user-123",
		Groups:               []string{"engineering", "devops"},
		Roles:                []string{"role:serveradmin"},
		AccessibleProjects:   []string{"project-a", "project-b"},
		AccessibleNamespaces: []string{"ns-a", "ns-b"},
		IsGlobalAccess:       false,
	}

	assert.Equal(t, "user-123", authCtx.UserID)
	assert.Equal(t, []string{"engineering", "devops"}, authCtx.Groups)
	assert.Len(t, authCtx.Roles, 1, "should have exactly one role assigned")
	assert.Equal(t, []string{"project-a", "project-b"}, authCtx.AccessibleProjects)
	assert.Equal(t, []string{"ns-a", "ns-b"}, authCtx.AccessibleNamespaces)
	assert.False(t, authCtx.IsGlobalAccess)
}

func TestUserAuthContextGlobalAccess(t *testing.T) {
	authCtx := UserAuthContext{
		UserID:               "admin",
		AccessibleNamespaces: nil, // nil = global access
		IsGlobalAccess:       true,
	}

	assert.Nil(t, authCtx.AccessibleNamespaces)
	assert.True(t, authCtx.IsGlobalAccess)
}

func TestToRGDResponse_AllowedDeploymentModes(t *testing.T) {
	tests := []struct {
		name          string
		allowedModes  []string
		expectedModes []string
	}{
		{
			name:          "nil allowed modes - all modes allowed",
			allowedModes:  nil,
			expectedModes: nil,
		},
		{
			name:          "single mode - gitops only",
			allowedModes:  []string{"gitops"},
			expectedModes: []string{"gitops"},
		},
		{
			name:          "two modes - direct and hybrid",
			allowedModes:  []string{"direct", "hybrid"},
			expectedModes: []string{"direct", "hybrid"},
		},
		{
			name:          "all three modes",
			allowedModes:  []string{"direct", "gitops", "hybrid"},
			expectedModes: []string{"direct", "gitops", "hybrid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rgd := &models.CatalogRGD{
				Name:                   "test-rgd",
				Namespace:              "default",
				AllowedDeploymentModes: tt.allowedModes,
				CreatedAt:              time.Now(),
				UpdatedAt:              time.Now(),
			}

			resp := ToRGDResponse(rgd, 0)

			assert.Equal(t, tt.expectedModes, resp.AllowedDeploymentModes)
		})
	}
}
