// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

// MockDynamicClient implements dynamic.Interface for testing
type MockDynamicClient struct {
	mock.Mock
}

func (m *MockDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	args := m.Called(resource)
	return args.Get(0).(dynamic.NamespaceableResourceInterface)
}

// MockNamespaceableResourceInterface implements dynamic.NamespaceableResourceInterface
type MockNamespaceableResourceInterface struct {
	mock.Mock
}

func (m *MockNamespaceableResourceInterface) Namespace(namespace string) dynamic.ResourceInterface {
	args := m.Called(namespace)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(dynamic.ResourceInterface)
}

func (m *MockNamespaceableResourceInterface) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, obj, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockNamespaceableResourceInterface) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockNamespaceableResourceInterface) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, obj, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockNamespaceableResourceInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, obj, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockNamespaceableResourceInterface) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	args := m.Called(ctx, name, opts, subresources)
	return args.Error(0)
}

func (m *MockNamespaceableResourceInterface) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	args := m.Called(ctx, opts, listOpts)
	return args.Error(0)
}

func (m *MockNamespaceableResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.UnstructuredList), args.Error(1)
}

func (m *MockNamespaceableResourceInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(watch.Interface), args.Error(1)
}

func (m *MockNamespaceableResourceInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, pt, data, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockNamespaceableResourceInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, obj, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockNamespaceableResourceInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, obj, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

// MockResourceInterface implements dynamic.ResourceInterface for testing (used for Namespace() returns)
type MockResourceInterface struct {
	mock.Mock
}

func (m *MockResourceInterface) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, obj, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockResourceInterface) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockResourceInterface) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, obj, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockResourceInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, obj, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockResourceInterface) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	args := m.Called(ctx, name, opts, subresources)
	return args.Error(0)
}

func (m *MockResourceInterface) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	args := m.Called(ctx, opts, listOpts)
	return args.Error(0)
}

func (m *MockResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.UnstructuredList), args.Error(1)
}

func (m *MockResourceInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(watch.Interface), args.Error(1)
}

func (m *MockResourceInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, pt, data, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockResourceInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, obj, opts, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockResourceInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, obj, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

// Test fixtures
func newTestProjectSpec() ProjectSpec {
	return ProjectSpec{
		Description: "Test project",
		Destinations: []Destination{
			{
				Namespace: "default",
			},
		},
		Roles: []ProjectRole{
			{
				Name:        "admin",
				Description: "Administrator",
				Policies: []string{
					"p, proj:test:admin, applications, *, test/*, allow",
				},
				Groups: []string{"user:admin-user"},
			},
			{
				Name:        "viewer",
				Description: "Viewer",
				Policies: []string{
					"p, proj:test:viewer, applications, get, test/*, allow",
				},
				Groups: []string{"user:viewer-user"},
			},
		},
	}
}

func newTestUnstructuredProject(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": ProjectGroup + "/" + ProjectVersion,
			"kind":       ProjectKind,
			"metadata": map[string]interface{}{
				"name":              name,
				"resourceVersion":   "1",
				"uid":               "test-uid-123",
				"creationTimestamp": "2024-01-15T10:00:00Z",
				"labels": map[string]interface{}{
					"knodex.io/created-by": "test-user",
				},
				"annotations": map[string]interface{}{
					"knodex.io/created-at": "2024-01-15T10:00:00Z",
				},
			},
			"spec": map[string]interface{}{
				"description": "Test project",
				"destinations": []interface{}{
					map[string]interface{}{
						"namespace": "default",
					},
				},
				"roles": []interface{}{
					map[string]interface{}{
						"name":        "admin",
						"description": "Administrator",
						"policies": []interface{}{
							"p, proj:test:admin, applications, *, test/*, allow",
						},
						"groups": []interface{}{
							"user:admin-user",
						},
					},
					map[string]interface{}{
						"name":        "viewer",
						"description": "Viewer",
						"policies": []interface{}{
							"p, proj:test:viewer, applications, get, test/*, allow",
						},
						"groups": []interface{}{
							"user:viewer-user",
						},
					},
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"lastTransitionTime": "2024-01-15T10:00:00Z",
						"reason":             "Created",
						"message":            "Project created successfully",
					},
				},
			},
		},
	}
}

func newTestUnstructuredProjectList(names ...string) *unstructured.UnstructuredList {
	items := make([]unstructured.Unstructured, 0, len(names))
	for _, name := range names {
		items = append(items, *newTestUnstructuredProject(name))
	}
	return &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": ProjectGroup + "/" + ProjectVersion,
			"kind":       ProjectKind + "List",
			"metadata":   map[string]interface{}{},
		},
		Items: items,
	}
}

func setupMockDynamicClient() (*MockDynamicClient, *MockNamespaceableResourceInterface) {
	mockClient := new(MockDynamicClient)
	mockResource := new(MockNamespaceableResourceInterface)
	mockClient.On("Resource", ProjectGVR).Return(mockResource)
	return mockClient, mockResource
}

// ==============================================
// CreateProject Tests
// ==============================================

func TestProjectService_CreateProject(t *testing.T) {
	tests := []struct {
		name          string
		projectName   string
		spec          ProjectSpec
		createdBy     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
	}{
		{
			name:        "successful creation",
			projectName: "test-project",
			spec:        newTestProjectSpec(),
			createdBy:   "test-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError: false,
		},
		{
			name:        "invalid project name - empty",
			projectName: "",
			spec:        newTestProjectSpec(),
			createdBy:   "test-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				// Should not be called - validation fails first
			},
			expectError:   true,
			errorContains: "invalid project name",
		},
		{
			name:        "invalid project name - special characters",
			projectName: "invalid_project!",
			spec:        newTestProjectSpec(),
			createdBy:   "test-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				// Should not be called - validation fails first
			},
			expectError:   true,
			errorContains: "invalid project name",
		},
		{
			name:        "invalid spec - empty destinations",
			projectName: "test-project",
			spec: ProjectSpec{
				Description:  "Test",
				Destinations: []Destination{},
			},
			createdBy: "test-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				// Should not be called - validation fails first
			},
			expectError:   true,
			errorContains: "invalid project spec",
		},
		{
			name:        "kubernetes already exists error",
			projectName: "existing-project",
			spec:        newTestProjectSpec(),
			createdBy:   "test-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewAlreadyExists(
						schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource},
						"existing-project",
					))
			},
			expectError:   true,
			errorContains: "already exists",
		},
		{
			name:        "kubernetes API error",
			projectName: "test-project",
			spec:        newTestProjectSpec(),
			createdBy:   "test-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("connection refused"))
			},
			expectError:   true,
			errorContains: "connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.CreateProject(context.Background(), tt.projectName, tt.spec, tt.createdBy)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.projectName, result.Name)
			}
		})
	}
}

func TestProjectService_CreateProject_SetsLabelsAndAnnotations(t *testing.T) {
	mockClient, mockResource := setupMockDynamicClient()

	// Capture the created object to verify labels/annotations
	var capturedObj *unstructured.Unstructured
	mockResource.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedObj = args.Get(1).(*unstructured.Unstructured)
		}).
		Return(newTestUnstructuredProject("test-project"), nil)

	svc := NewProjectService(nil, mockClient)
	_, err := svc.CreateProject(context.Background(), "test-project", newTestProjectSpec(), "creator-user")

	require.NoError(t, err)
	require.NotNil(t, capturedObj)

	// Verify labels
	labels := capturedObj.GetLabels()
	assert.Equal(t, "knodex", labels["app.kubernetes.io/managed-by"])

	// Verify annotations
	annotations := capturedObj.GetAnnotations()
	assert.Equal(t, "creator-user", annotations["knodex.io/created-by"])
	assert.Contains(t, annotations, "knodex.io/created-at")
}

// ==============================================
// GetProject Tests
// ==============================================

func TestProjectService_GetProject(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
		validateFunc  func(*testing.T, *Project)
	}{
		{
			name:      "successful get",
			projectID: "test-project",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError: false,
			validateFunc: func(t *testing.T, p *Project) {
				assert.Equal(t, "test-project", p.Name)
				assert.Equal(t, "Test project", p.Spec.Description)
				assert.Len(t, p.Spec.Destinations, 1)
				assert.Len(t, p.Spec.Roles, 2)
				assert.Equal(t, "test-uid-123", string(p.UID))
				assert.Equal(t, "1", p.ResourceVersion)
			},
		},
		{
			name:      "not found error",
			projectID: "nonexistent",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "nonexistent", mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(
						schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource},
						"nonexistent",
					))
			},
			expectError:   true,
			errorContains: "nonexistent",
		},
		{
			name:      "kubernetes API error",
			projectID: "test-project",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("connection timeout"))
			},
			expectError:   true,
			errorContains: "connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.GetProject(context.Background(), tt.projectID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validateFunc != nil {
					tt.validateFunc(t, result)
				}
			}
		})
	}
}

// ==============================================
// ListProjects Tests
// ==============================================

func TestProjectService_ListProjects(t *testing.T) {
	tests := []struct {
		name         string
		mockSetup    func(*MockNamespaceableResourceInterface)
		expectError  bool
		expectCount  int
		validateFunc func(*testing.T, *ProjectList)
	}{
		{
			name: "successful list with multiple projects",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("List", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProjectList("project-1", "project-2", "project-3"), nil)
			},
			expectError: false,
			expectCount: 3,
			validateFunc: func(t *testing.T, pl *ProjectList) {
				assert.Len(t, pl.Items, 3)
				names := make([]string, len(pl.Items))
				for i, p := range pl.Items {
					names[i] = p.Name
				}
				assert.Contains(t, names, "project-1")
				assert.Contains(t, names, "project-2")
				assert.Contains(t, names, "project-3")
			},
		},
		{
			name: "empty list",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("List", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProjectList(), nil)
			},
			expectError: false,
			expectCount: 0,
		},
		{
			name: "kubernetes API error",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("List", mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("connection refused"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.ListProjects(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Len(t, result.Items, tt.expectCount)
				if tt.validateFunc != nil {
					tt.validateFunc(t, result)
				}
			}
		})
	}
}

// ==============================================
// UpdateProject Tests
// ==============================================

func TestProjectService_UpdateProject(t *testing.T) {
	tests := []struct {
		name          string
		project       *Project
		updatedBy     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
	}{
		{
			name: "successful update",
			project: &Project{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ProjectGroup + "/" + ProjectVersion,
					Kind:       ProjectKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-project",
					ResourceVersion: "1",
				},
				Spec: newTestProjectSpec(),
			},
			updatedBy: "update-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				updated := newTestUnstructuredProject("test-project")
				updated.Object["metadata"].(map[string]interface{})["resourceVersion"] = "2"
				m.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(updated, nil)
			},
			expectError: false,
		},
		{
			name: "not found error",
			project: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "nonexistent",
					ResourceVersion: "1",
				},
				Spec: newTestProjectSpec(),
			},
			updatedBy: "update-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(
						schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource},
						"nonexistent",
					))
			},
			expectError:   true,
			errorContains: "nonexistent",
		},
		{
			name: "conflict error - resource version mismatch",
			project: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-project",
					ResourceVersion: "1",
				},
				Spec: newTestProjectSpec(),
			},
			updatedBy: "update-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewConflict(
						schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource},
						"test-project",
						fmt.Errorf("resource version mismatch"),
					))
			},
			expectError:   true,
			errorContains: "resource version mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.UpdateProject(context.Background(), tt.project, tt.updatedBy)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestProjectService_UpdateProject_SetsUpdatedAnnotations(t *testing.T) {
	mockClient, mockResource := setupMockDynamicClient()

	var capturedObj *unstructured.Unstructured
	mockResource.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedObj = args.Get(1).(*unstructured.Unstructured)
		}).
		Return(newTestUnstructuredProject("test-project"), nil)

	svc := NewProjectService(nil, mockClient)
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-project",
			ResourceVersion: "1",
		},
		Spec: newTestProjectSpec(),
	}
	_, err := svc.UpdateProject(context.Background(), project, "updater-user")

	require.NoError(t, err)
	require.NotNil(t, capturedObj)

	annotations := capturedObj.GetAnnotations()
	assert.Equal(t, "updater-user", annotations["knodex.io/updated-by"])
	assert.Contains(t, annotations, "knodex.io/updated-at")
}

// ==============================================
// DeleteProject Tests
// ==============================================

func TestProjectService_DeleteProject(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
	}{
		{
			name:      "successful delete",
			projectID: "test-project",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Delete", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(nil)
			},
			expectError: false,
		},
		{
			name:      "not found error",
			projectID: "nonexistent",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Delete", mock.Anything, "nonexistent", mock.Anything, mock.Anything).
					Return(errors.NewNotFound(
						schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource},
						"nonexistent",
					))
			},
			expectError:   true,
			errorContains: "nonexistent",
		},
		{
			name:      "kubernetes API error",
			projectID: "test-project",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Delete", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(fmt.Errorf("internal server error"))
			},
			expectError:   true,
			errorContains: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			err := svc.DeleteProject(context.Background(), tt.projectID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ==============================================
// GetUserRole Tests
// ==============================================

func TestProjectService_GetUserRole(t *testing.T) {
	tests := []struct {
		name         string
		projectID    string
		userID       string
		mockSetup    func(*MockNamespaceableResourceInterface)
		expectError  bool
		expectedRole string
	}{
		{
			name:      "user is admin",
			projectID: "test-project",
			userID:    "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:  false,
			expectedRole: "admin",
		},
		{
			name:      "user is viewer",
			projectID: "test-project",
			userID:    "viewer-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:  false,
			expectedRole: "viewer",
		},
		{
			name:      "user has no role",
			projectID: "test-project",
			userID:    "unknown-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:  false,
			expectedRole: "", // No role
		},
		{
			name:      "project not found",
			projectID: "nonexistent",
			userID:    "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "nonexistent", mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(
						schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource},
						"nonexistent",
					))
			},
			expectError:  true,
			expectedRole: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			role, err := svc.GetUserRole(context.Background(), tt.projectID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRole, role)
			}
		})
	}
}

// ==============================================
// GetProjectRoles Tests
// ==============================================

func TestProjectService_GetProjectRoles(t *testing.T) {
	tests := []struct {
		name        string
		projectID   string
		mockSetup   func(*MockNamespaceableResourceInterface)
		expectError bool
		expectCount int
	}{
		{
			name:      "successful get roles",
			projectID: "test-project",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError: false,
			expectCount: 2, // admin and viewer
		},
		{
			name:      "project not found",
			projectID: "nonexistent",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "nonexistent", mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(
						schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource},
						"nonexistent",
					))
			},
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			roles, err := svc.GetProjectRoles(context.Background(), tt.projectID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, roles, tt.expectCount)
			}
		})
	}
}

// ==============================================
// AddRole Tests
// ==============================================

func TestProjectService_AddRole(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		role          ProjectRole
		updatedBy     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
	}{
		{
			name:      "successful add role",
			projectID: "test-project",
			role: ProjectRole{
				Name:        "developer",
				Description: "Developer role",
				Policies:    []string{"p, proj:test:developer, applications, sync, test/*, allow"},
			},
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
				m.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError: false,
		},
		{
			name:      "role already exists",
			projectID: "test-project",
			role: ProjectRole{
				Name:        "admin", // Already exists in fixture
				Description: "Another admin",
				Policies:    []string{"p, proj:test:admin, applications, *, test/*, allow"},
			},
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:   true,
			errorContains: "already exists",
		},
		{
			name:      "invalid role - empty name",
			projectID: "test-project",
			role: ProjectRole{
				Name:        "",
				Description: "Invalid role",
			},
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:   true,
			errorContains: "invalid role",
		},
		{
			name:      "project not found",
			projectID: "nonexistent",
			role: ProjectRole{
				Name:        "developer",
				Description: "Developer role",
			},
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "nonexistent", mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(
						schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource},
						"nonexistent",
					))
			},
			expectError:   true,
			errorContains: "nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.AddRole(context.Background(), tt.projectID, tt.role, tt.updatedBy)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// ==============================================
// RemoveRole Tests
// ==============================================

func TestProjectService_RemoveRole(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		roleName      string
		updatedBy     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
	}{
		{
			name:      "successful remove role",
			projectID: "test-project",
			roleName:  "viewer", // Exists in fixture
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
				m.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError: false,
		},
		{
			name:      "role not found",
			projectID: "test-project",
			roleName:  "nonexistent-role",
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:      "project not found",
			projectID: "nonexistent",
			roleName:  "admin",
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "nonexistent", mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(
						schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource},
						"nonexistent",
					))
			},
			expectError:   true,
			errorContains: "nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.RemoveRole(context.Background(), tt.projectID, tt.roleName, tt.updatedBy)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// ==============================================
// UpdateRole Tests
// ==============================================

func TestProjectService_UpdateRole(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		roleName      string
		updatedRole   ProjectRole
		updatedBy     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
	}{
		{
			name:      "successful update role",
			projectID: "test-project",
			roleName:  "admin",
			updatedRole: ProjectRole{
				Name:        "admin",
				Description: "Updated Administrator",
				Policies:    []string{"p, proj:test:admin, applications, *, test/*, allow"},
			},
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
				m.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError: false,
		},
		{
			name:      "role not found",
			projectID: "test-project",
			roleName:  "nonexistent-role",
			updatedRole: ProjectRole{
				Name:        "nonexistent-role",
				Description: "Should fail",
				Policies:    []string{"p, proj:test:viewer, applications, get, test/*, allow"},
			},
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:      "invalid updated role - empty name",
			projectID: "test-project",
			roleName:  "admin",
			updatedRole: ProjectRole{
				Name:        "", // Invalid
				Description: "Invalid",
			},
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:   true,
			errorContains: "invalid role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.UpdateRole(context.Background(), tt.projectID, tt.roleName, tt.updatedRole, tt.updatedBy)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// ==============================================
// AddGroupToRole Tests
// ==============================================

func TestProjectService_AddGroupToRole(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		roleName      string
		groupName     string
		updatedBy     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
	}{
		{
			name:      "successful add group",
			projectID: "test-project",
			roleName:  "admin",
			groupName: "user:new-admin",
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
				m.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError: false,
		},
		{
			name:      "group already exists in role",
			projectID: "test-project",
			roleName:  "admin",
			groupName: "user:admin-user", // Already in fixture
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:   true,
			errorContains: "already in role",
		},
		{
			name:      "role not found",
			projectID: "test-project",
			roleName:  "nonexistent-role",
			groupName: "user:new-user",
			updatedBy: "admin-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.AddGroupToRole(context.Background(), tt.projectID, tt.roleName, tt.groupName, tt.updatedBy)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// ==============================================
// GetProjectByDestinationNamespace Tests
// ==============================================

func TestProjectService_GetProjectByDestinationNamespace(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
		expectedName  string
	}{
		{
			name:      "find by exact namespace",
			namespace: "default",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("List", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProjectList("test-project"), nil)
			},
			expectError:  false,
			expectedName: "test-project",
		},
		{
			name:      "namespace not found",
			namespace: "nonexistent-namespace",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("List", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProjectList("test-project"), nil)
			},
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:      "empty project list",
			namespace: "default",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("List", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProjectList(), nil)
			},
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.GetProjectByDestinationNamespace(context.Background(), tt.namespace)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedName, result.Name)
			}
		})
	}
}

// ==============================================
// GetUserProjectsByGroup Tests
// ==============================================

func TestProjectService_GetUserProjectsByGroup(t *testing.T) {
	tests := []struct {
		name        string
		userGroups  []string
		mockSetup   func(*MockNamespaceableResourceInterface)
		expectError bool
		expectCount int
		expectNames []string
	}{
		{
			name:       "user has access via admin group",
			userGroups: []string{"user:admin-user"},
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("List", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProjectList("project-1", "project-2"), nil)
			},
			expectError: false,
			expectCount: 2,
			expectNames: []string{"project-1", "project-2"},
		},
		{
			name:       "user has no access",
			userGroups: []string{"user:unknown-user"},
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("List", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProjectList("project-1"), nil)
			},
			expectError: false,
			expectCount: 0,
		},
		{
			name:       "empty user groups",
			userGroups: []string{},
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("List", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProjectList("project-1"), nil)
			},
			expectError: false,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.GetUserProjectsByGroup(context.Background(), tt.userGroups)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectCount)
				if tt.expectNames != nil && len(result) > 0 {
					names := make([]string, len(result))
					for i, p := range result {
						names[i] = p.Name
					}
					for _, expectedName := range tt.expectNames {
						assert.Contains(t, names, expectedName)
					}
				}
			}
		})
	}
}

// ==============================================
// UpdateProjectStatus Tests
// ==============================================

func TestProjectService_UpdateProjectStatus(t *testing.T) {
	tests := []struct {
		name          string
		project       *Project
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
	}{
		{
			name: "successful status update",
			project: &Project{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ProjectGroup + "/" + ProjectVersion,
					Kind:       ProjectKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-project",
					ResourceVersion: "1",
				},
				Spec: newTestProjectSpec(),
				Status: ProjectStatus{
					Conditions: []ProjectCondition{
						{
							Type:               "Ready",
							Status:             "True",
							LastTransitionTime: metav1.Time{Time: time.Now()},
							Reason:             "Updated",
							Message:            "Status updated",
						},
					},
				},
			},
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectError: false,
		},
		{
			name: "status update fails",
			project: &Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-project",
					ResourceVersion: "1",
				},
				Status: ProjectStatus{},
			},
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("forbidden"))
			},
			expectError:   true,
			errorContains: "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.UpdateProjectStatus(context.Background(), tt.project)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// ==============================================
// matchesWildcard Tests
// ==============================================

func TestMatchesWildcard(t *testing.T) {
	tests := []struct {
		pattern  string
		value    string
		expected bool
	}{
		{"*", "anything", true},
		{"*", "", true},
		{"prefix-*", "prefix-test", true},
		{"prefix-*", "prefix-", true},
		{"prefix-*", "other", false},
		{"*-suffix", "test-suffix", true},
		{"*-suffix", "-suffix", true},
		{"*-suffix", "test", false},
		{"exact", "exact", true},
		{"exact", "notexact", false},
		{"ns-*", "ns-dev", true},
		{"ns-*", "ns-prod", true},
		{"ns-*", "other-dev", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.pattern, tt.value), func(t *testing.T) {
			result := matchesWildcard(tt.pattern, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==============================================
// CreateProjectWithDescription Tests
// ==============================================

func TestProjectService_CreateProjectWithDescription(t *testing.T) {
	tests := []struct {
		name          string
		description   string
		spec          ProjectSpec
		createdBy     string
		mockSetup     func(*MockNamespaceableResourceInterface)
		expectError   bool
		errorContains string
	}{
		{
			name:        "successful creation with description",
			description: "My Test Project",
			spec:        newTestProjectSpec(),
			createdBy:   "test-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("my-test-project"), nil)
			},
			expectError: false,
		},
		{
			name:        "description becomes spec description if not set",
			description: "Auto-generated description",
			spec: ProjectSpec{
				Description:  "", // Empty, should be filled from description
				Destinations: []Destination{{Namespace: "*"}},
			},
			createdBy: "test-user",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						obj := args.Get(1).(*unstructured.Unstructured)
						spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
						assert.Equal(t, "Auto-generated description", spec["description"])
					}).
					Return(newTestUnstructuredProject("auto-generated-description"), nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			result, err := svc.CreateProjectWithDescription(context.Background(), tt.description, tt.spec, tt.createdBy)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// ==============================================
// GVR Verification Tests
// ==============================================

func TestProjectService_UsesCorrectGVR(t *testing.T) {
	mockClient := new(MockDynamicClient)
	mockResource := new(MockNamespaceableResourceInterface)

	// Verify Resource is called with correct GVR
	mockClient.On("Resource", ProjectGVR).Return(mockResource)
	mockResource.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newTestUnstructuredProject("test"), nil)

	svc := NewProjectService(nil, mockClient)
	_, _ = svc.GetProject(context.Background(), "test")

	// Verify GVR values
	assert.Equal(t, ProjectGroup, ProjectGVR.Group)
	assert.Equal(t, ProjectVersion, ProjectGVR.Version)
	assert.Equal(t, ProjectResource, ProjectGVR.Resource)

	mockClient.AssertCalled(t, "Resource", ProjectGVR)
}

// ==============================================
// Exists Tests
// ==============================================

func TestProjectService_Exists_Success(t *testing.T) {
	mockClient, mockResource := setupMockDynamicClient()

	// Mock successful Get - project exists
	mockResource.On("Get", mock.Anything, "existing-project", mock.Anything, mock.Anything).
		Return(newTestUnstructuredProject("existing-project"), nil)

	svc := NewProjectService(nil, mockClient)

	exists, err := svc.Exists(context.Background(), "existing-project")

	assert.NoError(t, err)
	assert.True(t, exists)
	mockResource.AssertCalled(t, "Get", mock.Anything, "existing-project", mock.Anything, mock.Anything)
}

func TestProjectService_Exists_NotFound(t *testing.T) {
	mockClient, mockResource := setupMockDynamicClient()

	// Mock Get returns "not found" error
	notFoundErr := errors.NewNotFound(schema.GroupResource{Group: ProjectGroup, Resource: ProjectResource}, "non-existent")
	mockResource.On("Get", mock.Anything, "non-existent", mock.Anything, mock.Anything).
		Return(nil, notFoundErr)

	svc := NewProjectService(nil, mockClient)

	exists, err := svc.Exists(context.Background(), "non-existent")

	assert.NoError(t, err) // Should NOT return error for not found
	assert.False(t, exists)
}

func TestProjectService_Exists_APIError(t *testing.T) {
	mockClient, mockResource := setupMockDynamicClient()

	// Mock Get returns internal server error (not a "not found" error)
	internalErr := fmt.Errorf("internal server error: connection refused")
	mockResource.On("Get", mock.Anything, "some-project", mock.Anything, mock.Anything).
		Return(nil, internalErr)

	svc := NewProjectService(nil, mockClient)

	exists, err := svc.Exists(context.Background(), "some-project")

	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestProjectService_Exists_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		projectName  string
		mockSetup    func(*MockNamespaceableResourceInterface)
		expectExists bool
		expectError  bool
	}{
		{
			name:        "project exists",
			projectName: "test-project",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "test-project", mock.Anything, mock.Anything).
					Return(newTestUnstructuredProject("test-project"), nil)
			},
			expectExists: true,
			expectError:  false,
		},
		{
			name:        "project not found with 'not found' message",
			projectName: "missing-project",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "missing-project", mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("project missing-project not found"))
			},
			expectExists: false,
			expectError:  false,
		},
		{
			name:        "project not found with 'NotFound' message",
			projectName: "another-missing",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "another-missing", mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("NotFound: project does not exist"))
			},
			expectExists: false,
			expectError:  false,
		},
		{
			name:        "API error - not a not-found error",
			projectName: "error-project",
			mockSetup: func(m *MockNamespaceableResourceInterface) {
				m.On("Get", mock.Anything, "error-project", mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("timeout: server unavailable"))
			},
			expectExists: false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mockResource := setupMockDynamicClient()
			tt.mockSetup(mockResource)

			svc := NewProjectService(nil, mockClient)

			exists, err := svc.Exists(context.Background(), tt.projectName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectExists, exists)
		})
	}
}

// ==============================================
// Helper Function Tests
// ==============================================

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "error with 'not found' lowercase",
			err:      fmt.Errorf("resource not found"),
			expected: true,
		},
		{
			name:     "error with 'NotFound' camelCase",
			err:      fmt.Errorf("NotFound: resource does not exist"),
			expected: true,
		},
		{
			name:     "error with both patterns",
			err:      fmt.Errorf("NotFound: resource not found in namespace"),
			expected: true,
		},
		{
			name:     "kubernetes NotFound error",
			err:      errors.NewNotFound(schema.GroupResource{Group: "test", Resource: "projects"}, "test-project"),
			expected: true,
		},
		{
			name:     "other error - no match",
			err:      fmt.Errorf("connection refused"),
			expected: false,
		},
		{
			name:     "timeout error",
			err:      fmt.Errorf("timeout waiting for response"),
			expected: false,
		},
		{
			name:     "empty error message",
			err:      fmt.Errorf(""),
			expected: false,
		},
		{
			name:     "error starting with 'not found'",
			err:      fmt.Errorf("not found in database"),
			expected: true,
		},
		{
			name:     "error ending with 'not found'",
			err:      fmt.Errorf("resource was not found"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFoundError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "empty string and empty substr",
			s:        "",
			substr:   "",
			expected: true,
		},
		{
			name:     "non-empty string, empty substr",
			s:        "hello",
			substr:   "",
			expected: true,
		},
		{
			name:     "empty string, non-empty substr",
			s:        "",
			substr:   "test",
			expected: false,
		},
		{
			name:     "exact match",
			s:        "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substr at start",
			s:        "hello world",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substr at end",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "substr in middle",
			s:        "hello world foo",
			substr:   "world",
			expected: true,
		},
		{
			name:     "substr not present",
			s:        "hello world",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "case sensitive - no match",
			s:        "Hello World",
			substr:   "hello",
			expected: false,
		},
		{
			name:     "case sensitive - match",
			s:        "Hello World",
			substr:   "Hello",
			expected: true,
		},
		{
			name:     "single character match",
			s:        "hello",
			substr:   "l",
			expected: true,
		},
		{
			name:     "single character no match",
			s:        "hello",
			substr:   "x",
			expected: false,
		},
		{
			name:     "substr longer than string",
			s:        "hi",
			substr:   "hello",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsAt(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "empty substr in non-empty string",
			s:        "hello",
			substr:   "",
			expected: true,
		},
		{
			name:     "empty string, empty substr",
			s:        "",
			substr:   "",
			expected: true,
		},
		{
			name:     "substr at start",
			s:        "hello world",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substr at end",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "substr in middle",
			s:        "foo bar baz",
			substr:   "bar",
			expected: true,
		},
		{
			name:     "substr not found",
			s:        "hello world",
			substr:   "xyz",
			expected: false,
		},
		{
			name:     "exact match",
			s:        "test",
			substr:   "test",
			expected: true,
		},
		{
			name:     "substr longer than string",
			s:        "hi",
			substr:   "hello",
			expected: false,
		},
		{
			name:     "partial match at boundary",
			s:        "not found",
			substr:   "not found",
			expected: true,
		},
		{
			name:     "NotFound pattern",
			s:        "error: NotFound",
			substr:   "NotFound",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAt(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
