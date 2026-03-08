package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// NOTE: MockUserService removed
// Local user storage now uses AccountStore (ConfigMap/Secret pattern)

// MockProjectService is a mock implementation of ProjectService for integration testing
type MockProjectService struct {
	mu       sync.RWMutex
	projects map[string]*rbac.Project
}

func NewMockProjectService() *MockProjectService {
	return &MockProjectService{
		projects: make(map[string]*rbac.Project),
	}
}

func (m *MockProjectService) CreateProject(ctx context.Context, name string, spec rbac.ProjectSpec) (*rbac.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project := &rbac.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.kro.run/v1alpha1",
			Kind:       "Project",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}

	m.projects[name] = project
	return project, nil
}

func (m *MockProjectService) GetProject(ctx context.Context, projectID string) (*rbac.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, errors.New("project not found")
	}
	return project, nil
}

func (m *MockProjectService) AddGroupToRole(ctx context.Context, projectID, roleName, groupName, addedBy string) (*rbac.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, errors.New("project not found")
	}

	// Find the role and add the group
	for i, role := range project.Spec.Roles {
		if role.Name == roleName {
			// Check if group already exists
			for _, g := range role.Groups {
				if g == groupName {
					return nil, fmt.Errorf("group %s already in role %s", groupName, roleName)
				}
			}
			project.Spec.Roles[i].Groups = append(project.Spec.Roles[i].Groups, groupName)
			return project, nil
		}
	}

	return nil, fmt.Errorf("role %s not found", roleName)
}

// newMockRedisClient creates a mock Redis client for testing
func newMockRedisClient() *redis.Client {
	// Create a Redis client that won't actually connect (used in tests)
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:16379", // Non-existent server for tests
	})
	return client
}

// setupAuthTestServer creates a test HTTP server with auth configured
// Updated to use AccountStore instead of UserService
func setupAuthTestServer(t *testing.T) (*httptest.Server, *auth.Service) {
	// Create fake Kubernetes clients
	k8sClient := fake.NewSimpleClientset()
	scheme := runtime.NewScheme()
	// Register Project types with the scheme
	scheme.AddKnownTypes(schema.GroupVersion{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion},
		&rbac.Project{},
		&rbac.ProjectList{},
	)
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion})
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create real RBAC services
	projectService := rbac.NewProjectService(k8sClient, dynamicClient)

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
	if err != nil {
		t.Fatalf("failed to create accounts ConfigMap: %v", err)
	}

	accountsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "knodex-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-jwt-secret-key-for-integration-testing"),
		},
	}
	_, err = k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), accountsSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create accounts Secret: %v", err)
	}

	// Create AccountStore
	accountStore := auth.NewAccountStore(k8sClient, namespace)

	// Create mock Redis client
	mockRedis := newMockRedisClient()

	// Create auth service with test configuration
	authConfig := &auth.Config{
		JWTSecret:          "test-jwt-secret-key-for-integration-testing",
		LocalAdminUsername: "admin",
		LocalAdminPassword: "TestPassword123!", // SECURITY: Meets complexity requirements (upper, lower, digit, special)
		JWTExpiry:          1 * time.Hour,
	}

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("failed to create casbin enforcer: %v", err)
	}

	// NewService now uses AccountStore instead of UserService
	authSvc, err := auth.NewService(authConfig, accountStore, projectService, k8sClient, mockRedis, casbinEnforcer)
	if err != nil {
		t.Fatalf("failed to create auth service: %v", err)
	}

	// Create router with auth service
	routerConfig := RouterConfig{
		AuthService: authSvc,
	}

	router := NewRouterWithConfig(nil, nil, nil, nil, routerConfig)

	// Create test server
	server := httptest.NewServer(router)

	return server, authSvc
}

func TestIntegration_LocalLogin_Success(t *testing.T) {
	server, authService := setupAuthTestServer(t)
	defer server.Close()

	// Prepare login request
	loginReq := auth.LocalLoginRequest{
		Username: "admin",
		Password: "TestPassword123!",
	}
	bodyBytes, _ := json.Marshal(loginReq)

	// Make HTTP request
	resp, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Check Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", contentType)
	}

	// Parse response body (token is NOT in body, delivered via HttpOnly cookie)
	var loginResp struct {
		ExpiresAt time.Time     `json:"expiresAt"`
		User      auth.UserInfo `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Extract token from Set-Cookie header
	var sessionToken string
	for _, c := range resp.Cookies() {
		if c.Name == "knodex_session" {
			sessionToken = c.Value
			break
		}
	}
	if sessionToken == "" {
		t.Fatal("expected knodex_session cookie, got none")
	}

	// Validate user information
	if loginResp.User.ID != "user-local-admin" {
		t.Errorf("user ID = %v, want user-local-admin", loginResp.User.ID)
	}
	if loginResp.User.Email != "admin@local" {
		t.Errorf("user email = %v, want admin@local", loginResp.User.Email)
	}

	hasGlobalAdminRole := false
	for _, role := range loginResp.User.CasbinRoles {
		if role == "role:serveradmin" {
			hasGlobalAdminRole = true
			break
		}
	}
	if !hasGlobalAdminRole {
		t.Error("user should have global admin role in CasbinRoles")
	}

	// Validate token expiry is in the future
	if loginResp.ExpiresAt.Before(time.Now()) {
		t.Error("token expiry should be in the future")
	}

	// Validate JWT token from cookie can be parsed
	claims, err := authService.ValidateToken(context.Background(), sessionToken)
	if err != nil {
		t.Errorf("failed to validate returned token: %v", err)
	}

	// Validate JWT claims
	if claims.UserID != "user-local-admin" {
		t.Errorf("JWT userID = %v, want user-local-admin", claims.UserID)
	}
	if claims.Email != "admin@local" {
		t.Errorf("JWT email = %v, want admin@local", claims.Email)
	}

	hasGlobalAdminClaim := false
	for _, role := range claims.CasbinRoles {
		if role == "role:serveradmin" {
			hasGlobalAdminClaim = true
			break
		}
	}
	if !hasGlobalAdminClaim {
		t.Error("JWT should have role:serveradmin in CasbinRoles")
	}
}

func TestIntegration_LocalLogin_InvalidCredentials(t *testing.T) {
	server, _ := setupAuthTestServer(t)
	defer server.Close()

	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "wrong password",
			username: "admin",
			password: "wrongpassword",
		},
		{
			name:     "wrong username",
			username: "wronguser",
			password: "TestPassword123!",
		},
		{
			name:     "both wrong",
			username: "wronguser",
			password: "wrongpassword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loginReq := auth.LocalLoginRequest{
				Username: tt.username,
				Password: tt.password,
			}
			bodyBytes, _ := json.Marshal(loginReq)

			resp, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			// Should return 401 Unauthorized
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status code = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
			}

			// Parse error response
			var errResp map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			// Verify error message contains "invalid credentials"
			message, ok := errResp["message"].(string)
			if !ok || message != "invalid credentials" {
				t.Errorf("error message = %v, want 'invalid credentials'", message)
			}
		})
	}
}

func TestIntegration_LocalLogin_ValidationErrors(t *testing.T) {
	server, _ := setupAuthTestServer(t)
	defer server.Close()

	tests := []struct {
		name        string
		requestBody interface{}
		wantStatus  int
		wantMessage string
	}{
		{
			name: "missing username",
			requestBody: auth.LocalLoginRequest{
				Username: "",
				Password: "TestPassword123!",
			},
			wantStatus:  http.StatusBadRequest,
			wantMessage: "username and password are required",
		},
		{
			name: "missing password",
			requestBody: auth.LocalLoginRequest{
				Username: "admin",
				Password: "",
			},
			wantStatus:  http.StatusBadRequest,
			wantMessage: "username and password are required",
		},
		{
			name:        "malformed JSON",
			requestBody: `{\"username\": \"admin\", \"password\":`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyBytes []byte
			if str, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, _ = json.Marshal(tt.requestBody)
			}

			resp, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status code = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			var errResp map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			message, ok := errResp["message"].(string)
			if !ok || message != tt.wantMessage {
				t.Errorf("error message = %v, want %v", message, tt.wantMessage)
			}
		})
	}
}

func TestIntegration_LocalLogin_RateLimiting(t *testing.T) {
	server, _ := setupAuthTestServer(t)
	defer server.Close()

	loginReq := auth.LocalLoginRequest{
		Username: "admin",
		Password: "TestPassword123!",
	}
	bodyBytes, _ := json.Marshal(loginReq)

	// Rate limit is 5 requests per minute with burst of 5
	// Send requests rapidly in goroutines to ensure they hit within the time window
	const numRequests = 6
	type result struct {
		reqNum     int
		statusCode int
		errCode    string
		errMessage string
	}
	resultsChan := make(chan result, numRequests)

	// Launch all requests concurrently to trigger rate limiting
	for i := 1; i <= numRequests; i++ {
		go func(reqNum int) {
			resp, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				t.Errorf("request %d failed: %v", reqNum, err)
				resultsChan <- result{reqNum: reqNum, statusCode: -1}
				return
			}
			defer resp.Body.Close()

			res := result{reqNum: reqNum, statusCode: resp.StatusCode}

			// If it's an error response, decode it
			if resp.StatusCode != http.StatusOK {
				var errResp map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
					res.errCode, _ = errResp["code"].(string)
					res.errMessage, _ = errResp["message"].(string)
				}
			}

			resultsChan <- res
		}(i)
		// Small delay to ensure requests are ordered but still rapid
		time.Sleep(10 * time.Millisecond)
	}

	// Collect results
	results := make([]result, 0, numRequests)
	for i := 0; i < numRequests; i++ {
		results = append(results, <-resultsChan)
	}

	// Count successful and rate-limited requests
	successCount := 0
	rateLimitedCount := 0

	for _, res := range results {
		if res.statusCode == http.StatusOK {
			successCount++
		} else if res.statusCode == http.StatusTooManyRequests {
			rateLimitedCount++
			// Verify error response for rate-limited requests
			if res.errCode != "RATE_LIMIT_EXCEEDED" {
				t.Errorf("rate-limited request: error code = %v, want RATE_LIMIT_EXCEEDED", res.errCode)
			}
			if res.errMessage != "too many requests, please try again later" {
				t.Errorf("rate-limited request: error message = %v, want 'too many requests, please try again later'", res.errMessage)
			}
		}
	}

	// With burst of 5, we expect 5 successful and 1 rate-limited
	if successCount != 5 {
		t.Errorf("expected 5 successful requests, got %d", successCount)
	}
	if rateLimitedCount != 1 {
		t.Errorf("expected 1 rate-limited request, got %d", rateLimitedCount)
	}
}

func TestIntegration_LocalLogin_JWTTokenValidation(t *testing.T) {
	server, authService := setupAuthTestServer(t)
	defer server.Close()

	// Login and get token
	loginReq := auth.LocalLoginRequest{
		Username: "admin",
		Password: "TestPassword123!",
	}
	bodyBytes, _ := json.Marshal(loginReq)

	resp, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Extract token from cookie (not from JSON body)
	var sessionToken string
	for _, c := range resp.Cookies() {
		if c.Name == "knodex_session" {
			sessionToken = c.Value
			break
		}
	}
	if sessionToken == "" {
		t.Fatal("expected knodex_session cookie")
	}

	// Parse token manually to verify structure
	token, _, err := new(jwt.Parser).ParseUnverified(sessionToken, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to cast claims to MapClaims")
	}

	// Verify standard JWT claims
	if _, exists := claims["exp"]; !exists {
		t.Error("token missing 'exp' claim")
	}
	if _, exists := claims["iat"]; !exists {
		t.Error("token missing 'iat' claim")
	}

	// Verify custom claims
	if sub, _ := claims["sub"].(string); sub != "user-local-admin" {
		t.Errorf("sub claim = %v, want user-local-admin", sub)
	}
	if email, _ := claims["email"].(string); email != "admin@local" {
		t.Errorf("email claim = %v, want admin@local", email)
	}

	if casbinRoles, ok := claims["casbin_roles"].([]interface{}); !ok {
		t.Error("casbin_roles claim should be present")
	} else {
		hasRole := false
		for _, role := range casbinRoles {
			if role == "role:serveradmin" {
				hasRole = true
				break
			}
		}
		if !hasRole {
			t.Error("casbin_roles claim should contain role:serveradmin")
		}
	}

	// Verify token can be validated by auth service
	validatedClaims, err := authService.ValidateToken(context.Background(), sessionToken)
	if err != nil {
		t.Errorf("auth service failed to validate token: %v", err)
	}

	if validatedClaims.UserID != "user-local-admin" {
		t.Errorf("validated userID = %v, want user-local-admin", validatedClaims.UserID)
	}
}

func TestIntegration_LocalLogin_ConcurrentRequests(t *testing.T) {
	server, _ := setupAuthTestServer(t)
	defer server.Close()

	loginReq := auth.LocalLoginRequest{
		Username: "admin",
		Password: "TestPassword123!",
	}
	bodyBytes, _ := json.Marshal(loginReq)

	// Test concurrent requests to ensure thread safety
	// Use 3 to stay under rate limit (rate limit is 5 per minute)
	const concurrentRequests = 3
	errChan := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func() {
			resp, err := http.Post(server.URL+"/api/v1/auth/local/login", "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				errChan <- err
				return
			}
			defer resp.Body.Close()

			// All requests should succeed (within rate limit)
			if resp.StatusCode != http.StatusOK {
				errChan <- errors.New("unexpected status code")
				return
			}

			// Verify token is in cookie
			var hasSessionCookie bool
			for _, c := range resp.Cookies() {
				if c.Name == "knodex_session" && c.Value != "" {
					hasSessionCookie = true
					break
				}
			}
			if !hasSessionCookie {
				errChan <- errors.New("missing knodex_session cookie")
				return
			}

			errChan <- nil
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < concurrentRequests; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent request failed: %v", err)
		}
	}
}
