// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// TestE2E_AdminBootstrap_FirstLogin tests the complete end-to-end flow for admin first login
func TestE2E_AdminBootstrap_FirstLogin(t *testing.T) {
	// Setup test server with auth
	server, authSvc, projectService := setupE2EAuthServer(t)
	defer server.Close()

	ctx := context.Background()

	// AC-1: Default project should not exist before first login
	_, err := projectService.GetProject(ctx, auth.DefaultProjectName)
	assert.Error(t, err, "Default project should not exist before first login")

	// Make login request
	loginReq := map[string]string{
		"username": "admin",
		"password": "TestPassword123!",
	}
	reqBody, err := json.Marshal(loginReq)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	// AC-2: Login should succeed
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Admin login should succeed")

	// Decode response body (token delivered via HttpOnly cookie, not in body)
	var loginResp struct {
		ExpiresAt time.Time     `json:"expiresAt"`
		User      auth.UserInfo `json:"user"`
	}
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	require.NoError(t, err)

	// AC-3: JWT token should be returned via cookie
	var sessionToken string
	for _, c := range resp.Cookies() {
		if c.Name == "knodex_session" {
			sessionToken = c.Value
			break
		}
	}
	assert.NotEmpty(t, sessionToken, "JWT token should be returned via knodex_session cookie")

	// AC-4: Validate JWT token contains correct claims
	// Global admins don't have explicit project membership in JWT
	// They access all projects via role:serveradmin Casbin role (ArgoCD-aligned 2-role model)
	claims, err := authSvc.ValidateToken(context.Background(), sessionToken)
	require.NoError(t, err)

	hasGlobalAdminRole := false
	for _, role := range claims.CasbinRoles {
		if role == rbac.CasbinRoleServerAdmin {
			hasGlobalAdminRole = true
			break
		}
	}
	assert.True(t, hasGlobalAdminRole, "Admin should have role:serveradmin in CasbinRoles")

	// Also verify UserInfo has the Casbin role
	hasGlobalAdminRoleInUser := false
	for _, role := range loginResp.User.CasbinRoles {
		if role == rbac.CasbinRoleServerAdmin {
			hasGlobalAdminRoleInUser = true
			break
		}
	}
	assert.True(t, hasGlobalAdminRoleInUser, "Admin should have role:serveradmin in User.CasbinRoles")

	// AC-5: Default project should exist after login with ArgoCD-aligned structure
	project, err := projectService.GetProject(ctx, auth.DefaultProjectName)
	require.NoError(t, err, "Default project should exist after login")
	assert.Equal(t, auth.DefaultProjectDescription, project.Spec.Description)
	// Verify destinations contain the default namespace
	require.NotEmpty(t, project.Spec.Destinations, "Project should have destinations")
	assert.Equal(t, auth.DefaultProjectNamespace, project.Spec.Destinations[0].Namespace)

	// AC-6: Admin should have platform-admin role (via OIDC group in role's Groups)
	adminUserID := "user-local-admin"
	adminGroup := fmt.Sprintf("admin:%s", adminUserID)
	foundAdminRole := false
	for _, role := range project.Spec.Roles {
		if role.Name == "platform-admin" {
			for _, group := range role.Groups {
				if group == adminGroup {
					foundAdminRole = true
					break
				}
			}
			break
		}
	}
	assert.True(t, foundAdminRole, "Admin should have platform-admin role via group binding")
}

// TestE2E_AdminBootstrap_Idempotency tests multiple admin logins are idempotent
func TestE2E_AdminBootstrap_Idempotency(t *testing.T) {
	server, authSvc, projectService := setupE2EAuthServer(t)
	defer server.Close()

	ctx := context.Background()

	// Perform first login
	loginReq := map[string]string{
		"username": "admin",
		"password": "TestPassword123!",
	}
	reqBody, err := json.Marshal(loginReq)
	require.NoError(t, err)

	resp1, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp1.Body.Close()
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Get project after first login - count groups in platform-admin role
	project1, err := projectService.GetProject(ctx, auth.DefaultProjectName)
	require.NoError(t, err)
	var initialGroupCount int
	for _, role := range project1.Spec.Roles {
		if role.Name == "platform-admin" {
			initialGroupCount = len(role.Groups)
			break
		}
	}

	// Perform second login
	resp2, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp2.Body.Close()

	// AC-7: Second login should succeed
	assert.Equal(t, http.StatusOK, resp2.StatusCode, "Second login should succeed")

	// Extract token from cookie (not from JSON body)
	var sessionToken2 string
	for _, c := range resp2.Cookies() {
		if c.Name == "knodex_session" {
			sessionToken2 = c.Value
			break
		}
	}
	require.NotEmpty(t, sessionToken2, "JWT should be in knodex_session cookie")

	// AC-8: Token should still contain role:serveradmin
	// Global admins don't have explicit project membership in JWT
	claims2, err := authSvc.ValidateToken(context.Background(), sessionToken2)
	require.NoError(t, err)
	hasGlobalAdminRole := false
	for _, role := range claims2.CasbinRoles {
		if role == rbac.CasbinRoleServerAdmin {
			hasGlobalAdminRole = true
			break
		}
	}
	assert.True(t, hasGlobalAdminRole, "Admin should still have role:serveradmin after second login")

	// AC-9: Admin group count in platform-admin role should remain the same (no duplicates)
	project2, err := projectService.GetProject(ctx, auth.DefaultProjectName)
	require.NoError(t, err)
	var currentGroupCount int
	for _, role := range project2.Spec.Roles {
		if role.Name == "platform-admin" {
			currentGroupCount = len(role.Groups)
			break
		}
	}
	assert.Equal(t, initialGroupCount, currentGroupCount, "Group count should remain the same")

	// AC-10: Admin should still be in platform-admin role
	adminUserID := "user-local-admin"
	adminGroup := fmt.Sprintf("admin:%s", adminUserID)
	foundAdmin := false
	for _, role := range project2.Spec.Roles {
		if role.Name == "platform-admin" {
			for _, group := range role.Groups {
				if group == adminGroup {
					foundAdmin = true
					break
				}
			}
			break
		}
	}
	assert.True(t, foundAdmin)
}

// TestE2E_OIDCUserEvaluation tests OIDC user evaluation (ephemeral users)
// OIDC users are ephemeral - not persisted to CRD
// They get project membership via group mappings evaluated at login time
func TestE2E_OIDCUserEvaluation(t *testing.T) {
	// Create test services
	k8sClient := fake.NewSimpleClientset()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(schema.GroupVersion{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion},
		&rbac.Project{},
		&rbac.ProjectList{},
	)
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion})

	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	projectService := rbac.NewProjectService(k8sClient, dynamicClient, "knodex-system")

	// Create ConfigMap and Secret for AccountStore
	namespace := "default"
	accountsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "knodex-accounts",
			Namespace: namespace,
		},
		Data: map[string]string{
			"accounts.admin":         "apiKey, login",
			"accounts.admin.enabled": "true",
		},
	}
	_, err := k8sClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), accountsCM, metav1.CreateOptions{})
	require.NoError(t, err)

	accountsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "knodex-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-jwt-secret-key-for-e2e-testing"),
		},
	}
	_, err = k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), accountsSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create AccountStore
	accountStore := auth.NewAccountStore(k8sClient, namespace)

	mockRedis := redis.NewClient(&redis.Options{Addr: "localhost:16379"})
	authConfig := &auth.Config{
		JWTSecret:          "test-jwt-secret-key-for-e2e-testing",
		LocalAdminUsername: "admin",
		LocalAdminPassword: "TestPassword123!",
		LocalLoginEnabled:  true,
		JWTExpiry:          1 * time.Hour,
	}

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	require.NoError(t, err, "should create Casbin enforcer")

	// NewService now uses AccountStore instead of UserService
	authSvc, err := auth.NewService(authConfig, accountStore, projectService, k8sClient, mockRedis, casbinEnforcer)
	require.NoError(t, err)

	ctx := context.Background()

	// AC-11: First, admin logs in to create default project
	adminUserID := "user-local-admin"

	_, err = casbinEnforcer.AddUserRole(adminUserID, rbac.CasbinRoleServerAdmin)
	require.NoError(t, err, "should assign global admin role via Casbin")

	_, err = authSvc.GetBootstrapService().EnsureDefaultProject(ctx, adminUserID)
	require.NoError(t, err, "Bootstrap should create default project")

	// AC-12: Create OIDCProvisioningService and evaluate OIDC user
	// OIDC users are evaluated (ephemeral), not provisioned to CRD
	provisioningSvc := auth.NewOIDCProvisioningService(projectService, nil, casbinEnforcer, "")
	oidcGroups := []string{"engineering", "developers"}
	result, err := provisioningSvc.EvaluateOIDCUser(ctx, "oidc-subject-123", "newuser@example.com", "New User", oidcGroups)
	require.NoError(t, err, "OIDC user evaluation should succeed")

	// Verify OIDCUserInfo is populated correctly
	assert.Equal(t, "newuser@example.com", result.Email, "Email should match")
	assert.Equal(t, "New User", result.DisplayName, "DisplayName should match")
	assert.NotEmpty(t, result.UserID, "UserID should be generated")
	assert.Equal(t, oidcGroups, result.Groups, "Groups should be passed through in result")

	// AC-13: OIDC user should have no project memberships without GroupMapper
	// Project membership comes from group mappings only (ephemeral)
	assert.Empty(t, result.ProjectMemberships, "OIDC user should have no projects without GroupMapper")

	// AC-14: Verify default project exists and has ArgoCD-aligned structure
	project, err := projectService.GetProject(ctx, auth.DefaultProjectName)
	require.NoError(t, err, "Default project should exist")
	assert.Equal(t, auth.DefaultProjectDescription, project.Spec.Description)

	// OIDC user is ephemeral - not added to project roles (no User CRD)
	// Authorization happens via OIDC groups matching Project spec.roles.groups at request time
}

// TestE2E_DefaultProjectConstants validates the project constants
func TestE2E_DefaultProjectConstants(t *testing.T) {
	// AC-15: Verify project constants are correct
	assert.Equal(t, "default-project", auth.DefaultProjectName, "Project name should be DNS-1123 compliant")
	assert.Equal(t, "Default Project", auth.DefaultProjectDescription)
	assert.Equal(t, "default-project", auth.DefaultProjectNamespace)
}

// TestE2E_AdminRemovedAndReAdded tests admin being removed and re-added
func TestE2E_AdminRemovedAndReAdded(t *testing.T) {
	server, _, projectService := setupE2EAuthServer(t)
	defer server.Close()

	ctx := context.Background()

	// First login to create default project
	loginReq := map[string]string{
		"username": "admin",
		"password": "TestPassword123!",
	}
	reqBody, err := json.Marshal(loginReq)
	require.NoError(t, err)

	resp1, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	// AC-16: Simulate admin being removed from platform-admin role
	project, err := projectService.GetProject(ctx, auth.DefaultProjectName)
	require.NoError(t, err)

	adminUserID := "user-local-admin"
	adminGroup := fmt.Sprintf("admin:%s", adminUserID)

	// Remove admin group from platform-admin role
	for i, role := range project.Spec.Roles {
		if role.Name == "platform-admin" {
			filteredGroups := []string{}
			for _, group := range role.Groups {
				if group != adminGroup {
					filteredGroups = append(filteredGroups, group)
				}
			}
			project.Spec.Roles[i].Groups = filteredGroups
			break
		}
	}

	// Update project to remove admin (need to specify who is updating)
	_, err = projectService.UpdateProject(ctx, project, "test-system")
	require.NoError(t, err)

	// Verify admin was removed
	updatedProject, err := projectService.GetProject(ctx, auth.DefaultProjectName)
	require.NoError(t, err)
	for _, role := range updatedProject.Spec.Roles {
		if role.Name == "platform-admin" {
			for _, group := range role.Groups {
				assert.NotEqual(t, adminGroup, group, "Admin group should be removed")
			}
			break
		}
	}

	// Login again - admin should be re-added
	resp2, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp2.Body.Close()

	// Login should still succeed
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	// Verify admin was re-added
	finalProject, err := projectService.GetProject(ctx, auth.DefaultProjectName)
	require.NoError(t, err)

	foundAdmin := false
	for _, role := range finalProject.Spec.Roles {
		if role.Name == "platform-admin" {
			for _, group := range role.Groups {
				if group == adminGroup {
					foundAdmin = true
					break
				}
			}
			break
		}
	}
	assert.True(t, foundAdmin, "Admin should be re-added to platform-admin role")
}

// setupE2EAuthServer creates a complete E2E test server with auth configured
// Updated to use AccountStore instead of UserService
func setupE2EAuthServer(t *testing.T) (*httptest.Server, *auth.Service, *rbac.ProjectService) {
	k8sClient := fake.NewSimpleClientset()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(schema.GroupVersion{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion},
		&rbac.Project{},
		&rbac.ProjectList{},
	)
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion})
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	projectService := rbac.NewProjectService(k8sClient, dynamicClient, "knodex-system")

	// Create ConfigMap and Secret for AccountStore
	namespace := "default"
	accountsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "knodex-accounts",
			Namespace: namespace,
		},
		Data: map[string]string{
			"accounts.admin":         "apiKey, login",
			"accounts.admin.enabled": "true",
		},
	}
	_, err := k8sClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), accountsCM, metav1.CreateOptions{})
	require.NoError(t, err)

	accountsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "knodex-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-jwt-secret-key-for-e2e-testing"),
		},
	}
	_, err = k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), accountsSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create AccountStore
	accountStore := auth.NewAccountStore(k8sClient, namespace)

	mockRedis := redis.NewClient(&redis.Options{Addr: "localhost:16379"})
	authConfig := &auth.Config{
		JWTSecret:          "test-jwt-secret-key-for-e2e-testing",
		LocalAdminUsername: "admin",
		LocalAdminPassword: "TestPassword123!",
		LocalLoginEnabled:  true,
		JWTExpiry:          1 * time.Hour,
	}

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	require.NoError(t, err, "should create Casbin enforcer")

	// NewService now uses AccountStore instead of UserService
	authSvc, err := auth.NewService(authConfig, accountStore, projectService, k8sClient, mockRedis, casbinEnforcer)
	require.NoError(t, err)

	// Create a simple test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/local/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var loginReq struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		resp, err := authSvc.AuthenticateLocal(r.Context(), loginReq.Username, loginReq.Password, r.RemoteAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Set session cookie (mirrors real handler behaviour)
		http.SetCookie(w, &http.Cookie{
			Name:     "knodex_session",
			Value:    resp.Token,
			Path:     "/api",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	return server, authSvc, projectService
}
