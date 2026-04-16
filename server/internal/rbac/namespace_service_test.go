// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// MockProjectService is a mock implementation of ProjectServiceInterface for testing
type MockProjectService struct {
	mock.Mock
}

func (m *MockProjectService) CreateProject(ctx context.Context, name string, spec ProjectSpec, createdBy string) (*Project, error) {
	args := m.Called(ctx, name, spec, createdBy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Project), args.Error(1)
}

func (m *MockProjectService) GetProject(ctx context.Context, name string) (*Project, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Project), args.Error(1)
}

func (m *MockProjectService) ListProjects(ctx context.Context) (*ProjectList, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ProjectList), args.Error(1)
}

func (m *MockProjectService) UpdateProject(ctx context.Context, project *Project, updatedBy string) (*Project, error) {
	args := m.Called(ctx, project, updatedBy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Project), args.Error(1)
}

func (m *MockProjectService) DeleteProject(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockProjectService) Exists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockProjectService) UpdateProjectStatus(ctx context.Context, project *Project) (*Project, error) {
	args := m.Called(ctx, project)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Project), args.Error(1)
}

// TestNamespaceMatchesPattern tests the glob pattern matching function
func TestNamespaceMatchesPattern(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		pattern   string
		expected  bool
	}{
		// Full wildcard tests
		{
			name:      "wildcard matches any namespace",
			namespace: "anything",
			pattern:   "*",
			expected:  true,
		},
		{
			name:      "wildcard matches system namespace",
			namespace: "kube-system",
			pattern:   "*",
			expected:  true,
		},

		// Prefix wildcard tests
		{
			name:      "prefix pattern matches",
			namespace: "dev-team1",
			pattern:   "dev-*",
			expected:  true,
		},
		{
			name:      "prefix pattern matches longer name",
			namespace: "dev-team-alpha",
			pattern:   "dev-*",
			expected:  true,
		},
		{
			name:      "prefix pattern does not match different prefix",
			namespace: "staging-team1",
			pattern:   "dev-*",
			expected:  false,
		},
		{
			name:      "prefix pattern exact boundary",
			namespace: "dev-",
			pattern:   "dev-*",
			expected:  true,
		},

		// Suffix wildcard tests
		{
			name:      "suffix pattern matches",
			namespace: "team-a-prod",
			pattern:   "*-prod",
			expected:  true,
		},
		{
			name:      "suffix pattern does not match different suffix",
			namespace: "team-a-staging",
			pattern:   "*-prod",
			expected:  false,
		},

		// Single character wildcard tests
		{
			name:      "single char pattern matches",
			namespace: "team-a-prod",
			pattern:   "team-?-prod",
			expected:  true,
		},
		{
			name:      "single char pattern does not match longer",
			namespace: "team-aa-prod",
			pattern:   "team-?-prod",
			expected:  false,
		},
		{
			name:      "single char pattern does not match empty",
			namespace: "team--prod",
			pattern:   "team-?-prod",
			expected:  false,
		},

		// Exact match tests
		{
			name:      "exact match succeeds",
			namespace: "production",
			pattern:   "production",
			expected:  true,
		},
		{
			name:      "exact match fails on different name",
			namespace: "staging",
			pattern:   "production",
			expected:  false,
		},
		{
			name:      "partial name does not match exact",
			namespace: "prod",
			pattern:   "production",
			expected:  false,
		},
		{
			name:      "longer name does not match exact",
			namespace: "production-v2",
			pattern:   "production",
			expected:  false,
		},

		// Edge cases
		{
			name:      "empty pattern does not match",
			namespace: "anything",
			pattern:   "",
			expected:  false,
		},
		{
			name:      "empty namespace does not match",
			namespace: "",
			pattern:   "dev-*",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NamespaceMatchesPattern(tt.namespace, tt.pattern)
			assert.Equal(t, tt.expected, result, "NamespaceMatchesPattern(%q, %q) = %v, want %v",
				tt.namespace, tt.pattern, result, tt.expected)
		})
	}
}

// TestFilterNamespacesByPatterns tests the batch filtering function
func TestFilterNamespacesByPatterns(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		patterns   []string
		expected   []string
	}{
		{
			name:       "filter with multiple patterns",
			namespaces: []string{"dev-team1", "dev-team2", "staging", "production"},
			patterns:   []string{"dev-*", "staging"},
			expected:   []string{"dev-team1", "dev-team2", "staging"},
		},
		{
			name:       "wildcard matches all",
			namespaces: []string{"dev-team1", "staging", "production"},
			patterns:   []string{"*"},
			expected:   []string{"dev-team1", "production", "staging"},
		},
		{
			name:       "empty patterns returns empty",
			namespaces: []string{"dev-team1", "staging"},
			patterns:   []string{},
			expected:   []string{},
		},
		{
			name:       "empty namespaces returns empty",
			namespaces: []string{},
			patterns:   []string{"dev-*"},
			expected:   []string{},
		},
		{
			name:       "no matches returns empty",
			namespaces: []string{"production", "staging"},
			patterns:   []string{"dev-*"},
			expected:   []string{},
		},
		{
			name:       "deduplicate matching namespaces",
			namespaces: []string{"dev-team1", "dev-team1", "staging"},
			patterns:   []string{"dev-*", "dev-team1"},
			expected:   []string{"dev-team1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterNamespacesByPatterns(tt.namespaces, tt.patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNamespaceService_ListNamespaces tests listing cluster namespaces
func TestNamespaceService_ListNamespaces(t *testing.T) {
	ctx := context.Background()

	// Create fake Kubernetes client with test namespaces
	fakeClient := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-public"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-node-lease"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev-team1"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev-team2"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "staging"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "production"}},
	)

	mockProjectService := new(MockProjectService)
	service := NewNamespaceService(fakeClient, mockProjectService)

	t.Run("exclude system namespaces", func(t *testing.T) {
		namespaces, err := service.ListNamespaces(ctx, true)
		require.NoError(t, err)

		// Should not include system namespaces
		assert.NotContains(t, namespaces, "kube-system")
		assert.NotContains(t, namespaces, "kube-public")
		assert.NotContains(t, namespaces, "kube-node-lease")

		// Should include regular namespaces
		assert.Contains(t, namespaces, "default")
		assert.Contains(t, namespaces, "dev-team1")
		assert.Contains(t, namespaces, "dev-team2")
		assert.Contains(t, namespaces, "staging")
		assert.Contains(t, namespaces, "production")
	})

	t.Run("include system namespaces", func(t *testing.T) {
		namespaces, err := service.ListNamespaces(ctx, false)
		require.NoError(t, err)

		// Should include all namespaces
		assert.Contains(t, namespaces, "kube-system")
		assert.Contains(t, namespaces, "kube-public")
		assert.Contains(t, namespaces, "kube-node-lease")
		assert.Contains(t, namespaces, "default")
		assert.Contains(t, namespaces, "dev-team1")
	})

	t.Run("namespaces are sorted", func(t *testing.T) {
		namespaces, err := service.ListNamespaces(ctx, true)
		require.NoError(t, err)

		// Verify sorted order
		for i := 1; i < len(namespaces); i++ {
			assert.True(t, namespaces[i-1] <= namespaces[i],
				"namespaces should be sorted: %s should come before %s",
				namespaces[i-1], namespaces[i])
		}
	})
}

// TestNamespaceService_ListProjectNamespaces tests listing namespaces for a project
func TestNamespaceService_ListProjectNamespaces(t *testing.T) {
	ctx := context.Background()

	// Create fake Kubernetes client with test namespaces
	fakeClient := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev-team1"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev-team2"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "staging"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "production"}},
	)

	t.Run("project with wildcard prefix pattern", func(t *testing.T) {
		mockProjectService := new(MockProjectService)
		mockProjectService.On("GetProject", ctx, "alpha").Return(&Project{
			ObjectMeta: metav1.ObjectMeta{Name: "alpha"},
			Spec: ProjectSpec{
				Destinations: []Destination{
					{Namespace: "dev-*"},
					{Namespace: "staging"},
				},
			},
		}, nil)

		service := NewNamespaceService(fakeClient, mockProjectService)

		namespaces, err := service.ListProjectNamespaces(ctx, "alpha")
		require.NoError(t, err)

		// Should match dev-* and staging
		assert.Contains(t, namespaces, "dev-team1")
		assert.Contains(t, namespaces, "dev-team2")
		assert.Contains(t, namespaces, "staging")

		// Should not match others
		assert.NotContains(t, namespaces, "production")
		assert.NotContains(t, namespaces, "default")

		mockProjectService.AssertExpectations(t)
	})

	t.Run("project with full wildcard", func(t *testing.T) {
		mockProjectService := new(MockProjectService)
		mockProjectService.On("GetProject", ctx, "global").Return(&Project{
			ObjectMeta: metav1.ObjectMeta{Name: "global"},
			Spec: ProjectSpec{
				Destinations: []Destination{
					{Namespace: "*"},
				},
			},
		}, nil)

		service := NewNamespaceService(fakeClient, mockProjectService)

		namespaces, err := service.ListProjectNamespaces(ctx, "global")
		require.NoError(t, err)

		// Should match all non-system namespaces
		assert.Contains(t, namespaces, "dev-team1")
		assert.Contains(t, namespaces, "dev-team2")
		assert.Contains(t, namespaces, "staging")
		assert.Contains(t, namespaces, "production")
		assert.Contains(t, namespaces, "default")

		// Should not include system namespaces
		assert.NotContains(t, namespaces, "kube-system")

		mockProjectService.AssertExpectations(t)
	})

	t.Run("project with no destinations returns empty", func(t *testing.T) {
		mockProjectService := new(MockProjectService)
		mockProjectService.On("GetProject", ctx, "empty").Return(&Project{
			ObjectMeta: metav1.ObjectMeta{Name: "empty"},
			Spec:       ProjectSpec{},
		}, nil)

		service := NewNamespaceService(fakeClient, mockProjectService)

		namespaces, err := service.ListProjectNamespaces(ctx, "empty")
		require.NoError(t, err)
		assert.Empty(t, namespaces)

		mockProjectService.AssertExpectations(t)
	})

	t.Run("project with no matching namespaces returns empty", func(t *testing.T) {
		mockProjectService := new(MockProjectService)
		mockProjectService.On("GetProject", ctx, "nomatch").Return(&Project{
			ObjectMeta: metav1.ObjectMeta{Name: "nomatch"},
			Spec: ProjectSpec{
				Destinations: []Destination{
					{Namespace: "nonexistent-*"},
				},
			},
		}, nil)

		service := NewNamespaceService(fakeClient, mockProjectService)

		namespaces, err := service.ListProjectNamespaces(ctx, "nomatch")
		require.NoError(t, err)
		assert.Empty(t, namespaces)

		mockProjectService.AssertExpectations(t)
	})

	t.Run("project not found returns error", func(t *testing.T) {
		mockProjectService := new(MockProjectService)
		mockProjectService.On("GetProject", ctx, "nonexistent").Return(nil, assert.AnError)

		service := NewNamespaceService(fakeClient, mockProjectService)

		namespaces, err := service.ListProjectNamespaces(ctx, "nonexistent")
		require.Error(t, err)
		assert.Nil(t, namespaces)

		mockProjectService.AssertExpectations(t)
	})

	t.Run("destination with empty namespace is skipped", func(t *testing.T) {
		mockProjectService := new(MockProjectService)
		mockProjectService.On("GetProject", ctx, "mixed").Return(&Project{
			ObjectMeta: metav1.ObjectMeta{Name: "mixed"},
			Spec: ProjectSpec{
				Destinations: []Destination{
					{Namespace: ""},         // Empty namespace
					{Name: "cluster.local"}, // Only name specified
					{Namespace: "staging"},  // Valid namespace
				},
			},
		}, nil)

		service := NewNamespaceService(fakeClient, mockProjectService)

		namespaces, err := service.ListProjectNamespaces(ctx, "mixed")
		require.NoError(t, err)

		// Should only contain staging
		assert.Equal(t, []string{"staging"}, namespaces)

		mockProjectService.AssertExpectations(t)
	})
}

// MockAuthorizer is a mock implementation of Authorizer for testing
type MockAuthorizer struct {
	mock.Mock
}

func (m *MockAuthorizer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	args := m.Called(ctx, user, object, action)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthorizer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	args := m.Called(ctx, user, groups, object, action)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthorizer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	args := m.Called(ctx, user, projectName, action)
	return args.Error(0)
}

func (m *MockAuthorizer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	args := m.Called(ctx, user, groups)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAuthorizer) HasRole(ctx context.Context, user, role string) (bool, error) {
	args := m.Called(ctx, user, role)
	return args.Bool(0), args.Error(1)
}

// TestNamespaceService_ListNamespacesForUser tests project-scoped namespace filtering
func TestNamespaceService_ListNamespacesForUser(t *testing.T) {
	ctx := context.Background()

	fakeClient := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev-team1"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev-team2"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "staging"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "production"}},
	)

	t.Run("user with one project sees filtered namespaces", func(t *testing.T) {
		mockProjectSvc := new(MockProjectService)
		mockProjectSvc.On("GetProject", ctx, "alpha").Return(&Project{
			ObjectMeta: metav1.ObjectMeta{Name: "alpha"},
			Spec: ProjectSpec{
				Destinations: []Destination{
					{Namespace: "dev-*"},
					{Namespace: "staging"},
				},
			},
		}, nil)

		mockAuth := new(MockAuthorizer)
		mockAuth.On("GetAccessibleProjects", ctx, "user1", []string{"alpha-devs"}).
			Return([]string{"alpha"}, nil)

		service := NewNamespaceService(fakeClient, mockProjectSvc)
		namespaces, err := service.ListNamespacesForUser(ctx, mockAuth, "user1", []string{"alpha-devs"}, true)
		require.NoError(t, err)

		assert.Contains(t, namespaces, "dev-team1")
		assert.Contains(t, namespaces, "dev-team2")
		assert.Contains(t, namespaces, "staging")
		assert.NotContains(t, namespaces, "production")
		assert.NotContains(t, namespaces, "default")

		mockAuth.AssertExpectations(t)
		mockProjectSvc.AssertExpectations(t)
	})

	t.Run("user with no accessible projects sees empty list", func(t *testing.T) {
		mockProjectSvc := new(MockProjectService)
		mockAuth := new(MockAuthorizer)
		mockAuth.On("GetAccessibleProjects", ctx, "nobody", []string(nil)).
			Return([]string{}, nil)

		service := NewNamespaceService(fakeClient, mockProjectSvc)
		namespaces, err := service.ListNamespacesForUser(ctx, mockAuth, "nobody", nil, true)
		require.NoError(t, err)
		assert.Empty(t, namespaces)

		mockAuth.AssertExpectations(t)
	})

	t.Run("skips deleted project gracefully", func(t *testing.T) {
		mockProjectSvc := new(MockProjectService)
		mockProjectSvc.On("GetProject", ctx, "deleted-project").Return(nil, assert.AnError)
		mockProjectSvc.On("GetProject", ctx, "alpha").Return(&Project{
			ObjectMeta: metav1.ObjectMeta{Name: "alpha"},
			Spec: ProjectSpec{
				Destinations: []Destination{
					{Namespace: "staging"},
				},
			},
		}, nil)

		mockAuth := new(MockAuthorizer)
		mockAuth.On("GetAccessibleProjects", ctx, "user1", []string{"grp"}).
			Return([]string{"deleted-project", "alpha"}, nil)

		service := NewNamespaceService(fakeClient, mockProjectSvc)
		namespaces, err := service.ListNamespacesForUser(ctx, mockAuth, "user1", []string{"grp"}, true)
		require.NoError(t, err)

		// Should still return alpha's namespaces despite deleted project error
		assert.Equal(t, []string{"staging"}, namespaces)

		mockAuth.AssertExpectations(t)
		mockProjectSvc.AssertExpectations(t)
	})

	t.Run("GetAccessibleProjects error propagates", func(t *testing.T) {
		mockProjectSvc := new(MockProjectService)
		mockAuth := new(MockAuthorizer)
		mockAuth.On("GetAccessibleProjects", ctx, "user1", []string(nil)).
			Return(nil, assert.AnError)

		service := NewNamespaceService(fakeClient, mockProjectSvc)
		_, err := service.ListNamespacesForUser(ctx, mockAuth, "user1", nil, true)
		require.Error(t, err)

		mockAuth.AssertExpectations(t)
	})
}
