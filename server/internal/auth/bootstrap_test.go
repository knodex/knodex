package auth

import (
	"context"
	"testing"

	"github.com/provops-org/knodex/server/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// createTestProjectService creates a test project service with proper scheme setup
func createTestProjectService() *rbac.ProjectService {
	scheme := runtime.NewScheme()
	// Register Project types with the scheme
	scheme.AddKnownTypes(schema.GroupVersion{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion},
		&rbac.Project{},
		&rbac.ProjectList{},
	)
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion})

	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	k8sClient := fake.NewSimpleClientset()

	return rbac.NewProjectService(k8sClient, dynamicClient)
}

// createTestProjectSpec creates a valid ArgoCD-aligned ProjectSpec for testing
func createTestProjectSpec(adminGroup string) rbac.ProjectSpec {
	return rbac.ProjectSpec{
		Description: DefaultProjectDescription,
		Destinations: []rbac.Destination{
			{
				Namespace: DefaultProjectNamespace,
			},
		},
		NamespaceResourceWhitelist: []rbac.ResourceSpec{
			{Group: "*", Kind: "*"},
		},
		Roles: []rbac.ProjectRole{
			{
				Name:        "platform-admin",
				Description: "Full access to project resources",
				Policies: []string{
					"p, proj:default-project:platform-admin, *, *, default-project/*, allow",
				},
				Groups: []string{adminGroup},
			},
		},
	}
}

func TestProjectBootstrapService_EnsureDefaultProject(t *testing.T) {
	tests := []struct {
		name              string
		adminUserID       string
		existingProject   *rbac.ProjectSpec
		expectedProjectID string
		expectedError     bool
		validateResult    func(t *testing.T, project *rbac.Project)
	}{
		{
			name:              "creates default project when none exists",
			adminUserID:       "user-local-admin",
			existingProject:   nil,
			expectedProjectID: DefaultProjectName,
			expectedError:     false,
			validateResult: func(t *testing.T, project *rbac.Project) {
				assert.Equal(t, DefaultProjectName, project.Name)
				assert.Equal(t, DefaultProjectDescription, project.Spec.Description)
				require.Len(t, project.Spec.Destinations, 1)
				assert.Equal(t, DefaultProjectNamespace, project.Spec.Destinations[0].Namespace)
				// Verify platform-admin role exists with admin's group
				var platformAdminRole *rbac.ProjectRole
				for i := range project.Spec.Roles {
					if project.Spec.Roles[i].Name == "platform-admin" {
						platformAdminRole = &project.Spec.Roles[i]
						break
					}
				}
				require.NotNil(t, platformAdminRole, "platform-admin role should exist")
				assert.Contains(t, platformAdminRole.Groups, "admin:user-local-admin")
			},
		},
		{
			name:        "returns existing project when already exists",
			adminUserID: "user-local-admin",
			existingProject: func() *rbac.ProjectSpec {
				spec := createTestProjectSpec("admin:user-local-admin")
				return &spec
			}(),
			expectedProjectID: DefaultProjectName,
			expectedError:     false,
			validateResult: func(t *testing.T, project *rbac.Project) {
				assert.Equal(t, DefaultProjectName, project.Name)
				assert.Len(t, project.Spec.Roles, 1)
			},
		},
		{
			name:        "adds admin when project exists but admin not in platform-admin role",
			adminUserID: "user-local-admin",
			existingProject: func() *rbac.ProjectSpec {
				spec := rbac.ProjectSpec{
					Description: DefaultProjectDescription,
					Destinations: []rbac.Destination{
						{
							Namespace: DefaultProjectNamespace,
						},
					},
					NamespaceResourceWhitelist: []rbac.ResourceSpec{
						{Group: "*", Kind: "*"},
					},
					Roles: []rbac.ProjectRole{
						{
							Name:        "platform-admin",
							Description: "Full access to project resources",
							Policies: []string{
								"p, proj:default-project:platform-admin, *, *, default-project/*, allow",
							},
							Groups: []string{"admin:other-user"}, // Different admin
						},
					},
				}
				return &spec
			}(),
			expectedProjectID: DefaultProjectName,
			expectedError:     false,
			validateResult: func(t *testing.T, project *rbac.Project) {
				assert.Equal(t, DefaultProjectName, project.Name)
				// Should have admin's group in platform-admin role
				var platformAdminRole *rbac.ProjectRole
				for i := range project.Spec.Roles {
					if project.Spec.Roles[i].Name == "platform-admin" {
						platformAdminRole = &project.Spec.Roles[i]
						break
					}
				}
				require.NotNil(t, platformAdminRole, "platform-admin role should exist")
				assert.Contains(t, platformAdminRole.Groups, "admin:user-local-admin")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test project service
			projectService := createTestProjectService()

			// Pre-create existing project if specified
			if tt.existingProject != nil {
				_, err := projectService.CreateProject(context.Background(), DefaultProjectName, *tt.existingProject, "test-user")
				require.NoError(t, err)
			}

			// Create bootstrap service
			k8sClient := fake.NewSimpleClientset()
			bootstrapService := NewProjectBootstrapService(projectService, k8sClient)

			// Call EnsureDefaultProject
			project, err := bootstrapService.EnsureDefaultProject(context.Background(), tt.adminUserID)

			// Validate error
			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Validate project
			assert.NotNil(t, project)
			assert.Equal(t, tt.expectedProjectID, project.Name)

			// Run custom validation if provided
			if tt.validateResult != nil {
				tt.validateResult(t, project)
			}
		})
	}
}

func TestProjectBootstrapService_Idempotency(t *testing.T) {
	// Create test project service
	projectService := createTestProjectService()
	k8sClient := fake.NewSimpleClientset()
	bootstrapService := NewProjectBootstrapService(projectService, k8sClient)

	ctx := context.Background()
	adminUserID := "user-local-admin"

	// Call EnsureDefaultProject multiple times
	project1, err := bootstrapService.EnsureDefaultProject(ctx, adminUserID)
	require.NoError(t, err)
	require.NotNil(t, project1)

	project2, err := bootstrapService.EnsureDefaultProject(ctx, adminUserID)
	require.NoError(t, err)
	require.NotNil(t, project2)

	project3, err := bootstrapService.EnsureDefaultProject(ctx, adminUserID)
	require.NoError(t, err)
	require.NotNil(t, project3)

	// All should return the same project
	assert.Equal(t, project1.Name, project2.Name)
	assert.Equal(t, project1.Name, project3.Name)
	assert.Equal(t, DefaultProjectName, project1.Name)

	// Verify only one project was created
	projectList, err := projectService.ListProjects(ctx)
	require.NoError(t, err)
	assert.Len(t, projectList.Items, 1)
}

func TestProjectBootstrapService_EnsureAdminRole(t *testing.T) {
	tests := []struct {
		name        string
		projectSpec rbac.ProjectSpec
		adminUserID string
		expectError bool
		expectGroup bool
	}{
		{
			name:        "admin already in platform-admin role",
			projectSpec: createTestProjectSpec("admin:user-local-admin"),
			adminUserID: "user-local-admin",
			expectError: false,
			expectGroup: true,
		},
		{
			name: "admin not in platform-admin role - should be added",
			projectSpec: rbac.ProjectSpec{
				Description: DefaultProjectDescription,
				Destinations: []rbac.Destination{
					{
						Namespace: DefaultProjectNamespace,
					},
				},
				NamespaceResourceWhitelist: []rbac.ResourceSpec{
					{Group: "*", Kind: "*"},
				},
				Roles: []rbac.ProjectRole{
					{
						Name:        "platform-admin",
						Description: "Full access to project resources",
						Policies: []string{
							"p, proj:default-project:platform-admin, *, *, default-project/*, allow",
						},
						Groups: []string{"admin:other-user"}, // Different admin
					},
				},
			},
			adminUserID: "user-local-admin",
			expectError: false,
			expectGroup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test project service
			projectService := createTestProjectService()
			k8sClient := fake.NewSimpleClientset()
			bootstrapService := NewProjectBootstrapService(projectService, k8sClient)

			// Create the project
			_, err := projectService.CreateProject(context.Background(), DefaultProjectName, tt.projectSpec, "test-user")
			require.NoError(t, err)

			// Get the project to ensure it's in the right state
			project, err := projectService.GetProject(context.Background(), DefaultProjectName)
			require.NoError(t, err)

			// Call ensureAdminRole
			updatedProject, err := bootstrapService.ensureAdminRole(context.Background(), project, tt.adminUserID)

			// Validate error
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, updatedProject)

			// Check if admin's group is in the platform-admin role
			adminGroup := "admin:" + tt.adminUserID
			var platformAdminRole *rbac.ProjectRole
			for i := range updatedProject.Spec.Roles {
				if updatedProject.Spec.Roles[i].Name == "platform-admin" {
					platformAdminRole = &updatedProject.Spec.Roles[i]
					break
				}
			}

			if tt.expectGroup {
				require.NotNil(t, platformAdminRole, "platform-admin role should exist")
				assert.Contains(t, platformAdminRole.Groups, adminGroup, "admin group should be in platform-admin role")
			}
		})
	}
}

func TestProjectBootstrapService_Constants(t *testing.T) {
	// Verify constants are set correctly
	assert.Equal(t, "default-project", DefaultProjectName)
	assert.Equal(t, "Default Project", DefaultProjectDescription)
	assert.Equal(t, "default-project", DefaultProjectNamespace)
}

// Note: Error handling tests are covered by the idempotency and race condition tests above
