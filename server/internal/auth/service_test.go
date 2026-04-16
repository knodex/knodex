// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

// NOTE: Tests in this file are NOT safe for t.Parallel() due to shared K8s fake client
// and AccountStore state across subtests (setupTestAccountStore creates shared ConfigMaps/Secrets).
// See tech-spec: go-test-mechanics-parallel-and-setup for details.

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/testutil"
)

// setupTestAccountStore creates a test AccountStore with fake kubernetes clients
// Replaces setupTestServices that used UserService
func setupTestAccountStore(t *testing.T) (*AccountStore, *rbac.ProjectService, *fake.Clientset) {
	k8sClient := testutil.NewFakeClientset(t)
	namespace := "default"

	// Create ConfigMap for accounts
	accountsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountConfigMapName,
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

	// Create Secret for credentials with JWT secret
	accountsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-jwt-secret-for-testing-purposes"),
		},
	}
	_, err = k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), accountsSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create accounts Secret: %v", err)
	}

	accountStore := NewAccountStore(k8sClient, namespace)

	// ListKind mapping required for Project CRD List operations
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion, Resource: "projects"}: "ProjectList",
	}
	dynamicClient := testutil.NewFakeDynamicClientWithListKinds(t, gvrToListKind)

	projectService := rbac.NewProjectService(k8sClient, dynamicClient, "knodex-system")

	return accountStore, projectService, k8sClient
}

func TestNewService(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		accountStore func(t *testing.T) *AccountStore
		wantErr      bool
		errContains  string
	}{
		{
			name:         "nil config",
			config:       nil,
			accountStore: nil,
			wantErr:      true,
			errContains:  "config cannot be nil",
		},
		{
			name: "nil accountStore",
			config: &Config{
				JWTSecret:          "test-secret",
				JWTExpiry:          1 * time.Hour,
				LocalAdminUsername: "admin",
				LocalAdminPassword: "Password123!",
			},
			accountStore: nil,
			wantErr:      true,
			errContains:  "accountStore cannot be nil",
		},
		{
			name: "valid config",
			config: &Config{
				JWTSecret:          "test-secret",
				JWTExpiry:          1 * time.Hour,
				LocalAdminUsername: "admin",
				LocalAdminPassword: "Password123!",
			},
			accountStore: func(t *testing.T) *AccountStore {
				as, _, _ := setupTestAccountStore(t)
				return as
			},
			wantErr: false,
		},
		{
			name: "zero JWT expiry uses default",
			config: &Config{
				JWTSecret:          "test-secret",
				JWTExpiry:          0,
				LocalAdminUsername: "admin",
				LocalAdminPassword: "Password123!",
			},
			accountStore: func(t *testing.T) *AccountStore {
				as, _, _ := setupTestAccountStore(t)
				return as
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var accountStore *AccountStore
			var projectSvc *rbac.ProjectService
			var k8sClient *fake.Clientset

			if tt.accountStore != nil {
				accountStore = tt.accountStore(t)
				_, projectSvc, k8sClient = setupTestAccountStore(t)
			} else {
				// For nil accountStore test, only create projectSvc
				_, projectSvc, k8sClient = setupTestAccountStore(t)
			}

			// Create a real redis client mock
			redisClient := NewMockRedisClientAdapter()

			svc, err := NewService(tt.config, accountStore, projectSvc, k8sClient, redisClient, nil)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewService() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewService() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NewService() unexpected error = %v", err)
				return
			}

			if svc == nil {
				t.Error("NewService() returned nil service")
				return
			}

			// Verify JWT secret was set
			if svc.config.JWTSecret == "" {
				t.Error("NewService() did not set JWT secret")
			}

			// Verify JWT expiry was set
			if svc.config.JWTExpiry == 0 {
				t.Error("NewService() did not set JWT expiry")
			}
		})
	}
}

// TestNewService_MissingAdminAccountLogsWarning verifies that NewService detects
// when the admin password is bootstrapped but the admin account is missing from the
// knodex-accounts ConfigMap. This catches the scenario where a Helm chart creates the
// ConfigMap without the admin account entry — the server starts fine but login silently
// fails with "invalid credentials".
func TestNewService_MissingAdminAccountLogsWarning(t *testing.T) {
	k8sClient := testutil.NewFakeClientset(t)
	namespace := "default"

	// Create ConfigMap WITHOUT admin account (simulates a Helm chart that forgot it)
	accountsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"accounts.test":         "apiKey, login",
			"accounts.test.enabled": "true",
			// NOTE: accounts.admin is intentionally missing
		},
	}
	_, err := k8sClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), accountsCM, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create ConfigMap: %v", err)
	}

	// Create Secret with JWT key
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-jwt-secret"),
		},
	}
	_, err = k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create Secret: %v", err)
	}

	accountStore := NewAccountStore(k8sClient, namespace)

	dynamicClient := testutil.NewFakeDynamicClient(t)
	projectSvc := rbac.NewProjectService(k8sClient, dynamicClient, "knodex-system")

	redisClient := NewMockRedisClientAdapter()

	// NewService should succeed (it's non-fatal) but the admin account won't be usable.
	// The validation log happens inside NewService — we verify the account is actually missing.
	svc, err := NewService(&Config{
		JWTSecret:          "test-jwt-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, nil)
	if err != nil {
		t.Fatalf("NewService() should not fail, got: %v", err)
	}

	// Verify that login actually fails (the scenario we're warning about)
	_, err = svc.AuthenticateLocal(context.Background(), "admin", "Password123!", "127.0.0.1")
	if err == nil {
		t.Error("AuthenticateLocal() should fail when admin account is missing from ConfigMap")
	}
}

// TestNewService_DisabledAdminAccountLogsWarning verifies that NewService detects
// when the admin account exists but has login disabled.
func TestNewService_DisabledAdminAccountLogsWarning(t *testing.T) {
	k8sClient := testutil.NewFakeClientset(t)
	namespace := "default"

	// Create ConfigMap with admin account DISABLED
	accountsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"accounts.admin":         "apiKey", // no "login" capability
			"accounts.admin.enabled": "true",
		},
	}
	_, err := k8sClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), accountsCM, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create ConfigMap: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-jwt-secret"),
		},
	}
	_, err = k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create Secret: %v", err)
	}

	accountStore := NewAccountStore(k8sClient, namespace)

	dynamicClient := testutil.NewFakeDynamicClient(t)
	projectSvc := rbac.NewProjectService(k8sClient, dynamicClient, "knodex-system")

	redisClient := NewMockRedisClientAdapter()

	svc, err := NewService(&Config{
		JWTSecret:          "test-jwt-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, nil)
	if err != nil {
		t.Fatalf("NewService() should not fail, got: %v", err)
	}

	// Verify login fails because account lacks "login" capability
	_, err = svc.AuthenticateLocal(context.Background(), "admin", "Password123!", "127.0.0.1")
	if err == nil {
		t.Error("AuthenticateLocal() should fail when admin account lacks login capability")
	}
}

func TestAuthenticateLocal(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	redisClient := NewMockRedisClientAdapter()

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	tests := []struct {
		name        string
		username    string
		password    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid credentials",
			username: "admin",
			password: "Password123!",
			wantErr:  false,
		},
		{
			name:        "invalid username",
			username:    "wronguser",
			password:    "Password123!",
			wantErr:     true,
			errContains: "invalid credentials",
		},
		{
			name:        "invalid password",
			username:    "admin",
			password:    "wrongpassword",
			wantErr:     true,
			errContains: "invalid credentials",
		},
		{
			name:        "empty username",
			username:    "",
			password:    "Password123!",
			wantErr:     true,
			errContains: "invalid credentials",
		},
		{
			name:        "empty password",
			username:    "admin",
			password:    "",
			wantErr:     true,
			errContains: "invalid credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.AuthenticateLocal(context.Background(), tt.username, tt.password, "127.0.0.1")

			if tt.wantErr {
				if err == nil {
					t.Errorf("AuthenticateLocal() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("AuthenticateLocal() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("AuthenticateLocal() unexpected error = %v", err)
				return
			}

			if resp == nil {
				t.Error("AuthenticateLocal() returned nil response")
				return
			}

			// Verify JWT token
			if resp.Token == "" {
				t.Error("AuthenticateLocal() returned empty token")
			}

			// Verify expiry
			if resp.ExpiresAt.IsZero() {
				t.Error("AuthenticateLocal() returned zero expiry time")
			}

			// Verify user info
			if resp.User.ID == "" {
				t.Error("AuthenticateLocal() returned empty user ID")
			}
			if resp.User.Email != "admin@local" {
				t.Errorf("AuthenticateLocal() user email = %v, want admin@local", resp.User.Email)
			}

			hasGlobalAdminRole := false
			for _, role := range resp.User.CasbinRoles {
				if role == "role:serveradmin" {
					hasGlobalAdminRole = true
					break
				}
			}
			if !hasGlobalAdminRole {
				t.Error("AuthenticateLocal() user should have role:serveradmin in CasbinRoles")
			}
		})
	}
}

func TestGenerateTokenWithGroups(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	redisClient := NewMockRedisClientAdapter()

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Test parameters
	userID := "user-test-123"
	email := "test@example.com"
	displayName := "Test User"
	groups := []string{"engineering", "platform"}

	// Add user to global admin role for testing
	if _, err := casbinEnforcer.AddUserRole(userID, rbac.CasbinRoleServerAdmin); err != nil {
		t.Fatalf("AddUserRole() error = %v", err)
	}

	tokenString, expiresAt, err := svc.GenerateTokenWithGroups(userID, email, displayName, groups)
	if err != nil {
		t.Fatalf("GenerateTokenWithGroups() error = %v", err)
	}

	if tokenString == "" {
		t.Error("GenerateTokenWithGroups() returned empty token")
	}

	if expiresAt.IsZero() {
		t.Error("GenerateTokenWithGroups() returned zero expiry time")
	}

	// Verify expiry is approximately 1 hour from now
	expectedExpiry := time.Now().Add(1 * time.Hour)
	if expiresAt.Before(expectedExpiry.Add(-1*time.Minute)) || expiresAt.After(expectedExpiry.Add(1*time.Minute)) {
		t.Errorf("GenerateTokenWithGroups() expiry = %v, want ~%v", expiresAt, expectedExpiry)
	}

	// Validate token structure
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(svc.config.JWTSecret), nil
	})

	if err != nil {
		t.Fatalf("jwt.Parse() error = %v", err)
	}

	if !token.Valid {
		t.Error("GenerateTokenWithGroups() produced invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("GenerateTokenWithGroups() token claims not MapClaims")
	}

	// Verify claims
	if claims["sub"] != userID {
		t.Errorf("token claim sub = %v, want %v", claims["sub"], userID)
	}
	if claims["email"] != email {
		t.Errorf("token claim email = %v, want %v", claims["email"], email)
	}
	if claims["name"] != displayName {
		t.Errorf("token claim name = %v, want %v", claims["name"], displayName)
	}

	casbinRoles, ok := claims["casbin_roles"].([]interface{})
	if !ok {
		t.Error("token should have casbin_roles claim")
	} else {
		hasGlobalAdmin := false
		for _, role := range casbinRoles {
			if role == "role:serveradmin" {
				hasGlobalAdmin = true
				break
			}
		}
		if !hasGlobalAdmin {
			t.Error("token casbin_roles should contain role:serveradmin")
		}
	}
}

func TestGenerateTokenForAccount(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	redisClient := NewMockRedisClientAdapter()

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Test account
	account := &Account{
		Name:         "testuser",
		Enabled:      true,
		Capabilities: []string{"login", "apiKey"},
	}
	userID := "user-local-testuser"

	// Add user to global admin role for testing
	if _, err := casbinEnforcer.AddUserRole(userID, rbac.CasbinRoleServerAdmin); err != nil {
		t.Fatalf("AddUserRole() error = %v", err)
	}

	tokenString, expiresAt, err := svc.GenerateTokenForAccount(account, userID)
	if err != nil {
		t.Fatalf("GenerateTokenForAccount() error = %v", err)
	}

	if tokenString == "" {
		t.Error("GenerateTokenForAccount() returned empty token")
	}

	if expiresAt.IsZero() {
		t.Error("GenerateTokenForAccount() returned zero expiry time")
	}

	// Validate token structure
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(svc.config.JWTSecret), nil
	})

	if err != nil {
		t.Fatalf("jwt.Parse() error = %v", err)
	}

	if !token.Valid {
		t.Error("GenerateTokenForAccount() produced invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("GenerateTokenForAccount() token claims not MapClaims")
	}

	// Verify claims
	if claims["sub"] != userID {
		t.Errorf("token claim sub = %v, want %v", claims["sub"], userID)
	}
	if claims["email"] != "testuser@local" {
		t.Errorf("token claim email = %v, want %v", claims["email"], "testuser@local")
	}
}

func TestValidateToken(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	redisClient := NewMockRedisClientAdapter()

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Create test user info
	userID := "user-test-123"
	email := "test@example.com"
	displayName := "Test User"

	if _, err := casbinEnforcer.AddUserRole(userID, rbac.CasbinRoleServerAdmin); err != nil {
		t.Fatalf("AddUserRole() error = %v", err)
	}

	// Generate valid token using GenerateTokenWithGroups
	validToken, _, err := svc.GenerateTokenWithGroups(userID, email, displayName, nil)
	if err != nil {
		t.Fatalf("GenerateTokenWithGroups() error = %v", err)
	}

	// Create an expired token (includes valid iss/aud so failure is due to expiration)
	expiredClaims := jwt.MapClaims{
		"sub":          "user-expired",
		"email":        "expired@example.com",
		"name":         "Expired User",
		"casbin_roles": []string{},
		"iss":          "knodex",
		"aud":          "knodex-api",
		"exp":          time.Now().Add(-1 * time.Hour).Unix(),
		"iat":          time.Now().Add(-2 * time.Hour).Unix(),
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenString, _ := expiredToken.SignedString([]byte(svc.config.JWTSecret))

	// Create a token with wrong signature
	wrongSigToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-wrong-sig",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	wrongSigTokenString, _ := wrongSigToken.SignedString([]byte("wrong-secret"))

	tests := []struct {
		name        string
		token       string
		wantErr     bool
		errContains string
		checkClaims func(*testing.T, *JWTClaims)
	}{
		{
			name:    "valid token",
			token:   validToken,
			wantErr: false,
			checkClaims: func(t *testing.T, claims *JWTClaims) {
				if claims.UserID != userID {
					t.Errorf("claims.UserID = %v, want %v", claims.UserID, userID)
				}
				if claims.Email != email {
					t.Errorf("claims.Email = %v, want %v", claims.Email, email)
				}

				hasGlobalAdminRole := false
				for _, role := range claims.CasbinRoles {
					if role == rbac.CasbinRoleServerAdmin {
						hasGlobalAdminRole = true
						break
					}
				}
				if !hasGlobalAdminRole {
					t.Error("claims.CasbinRoles should contain role:serveradmin")
				}
			},
		},
		{
			name:        "expired token",
			token:       expiredTokenString,
			wantErr:     true,
			errContains: "token is expired",
		},
		{
			name:        "invalid signature",
			token:       wrongSigTokenString,
			wantErr:     true,
			errContains: "failed to parse token",
		},
		{
			name:        "empty token",
			token:       "",
			wantErr:     true,
			errContains: "failed to parse token",
		},
		{
			name:        "malformed token",
			token:       "not.a.valid.jwt",
			wantErr:     true,
			errContains: "failed to parse token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := svc.ValidateToken(context.Background(), tt.token)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateToken() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateToken() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateToken() unexpected error = %v", err)
				return
			}

			if claims == nil {
				t.Error("ValidateToken() returned nil claims")
				return
			}

			if tt.checkClaims != nil {
				tt.checkClaims(t, claims)
			}
		})
	}
}

func TestValidateToken_PasswordChangeInvalidation(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	redisClient := NewMockRedisClientAdapter()

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Assign admin role
	if _, err := casbinEnforcer.AddUserRole("user-local-admin", rbac.CasbinRoleServerAdmin); err != nil {
		t.Fatalf("AddUserRole() error = %v", err)
	}

	// Generate a token for the local admin
	account, err := accountStore.GetAccount(context.Background(), "admin")
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	token, _, err := svc.GenerateTokenForAccount(account, "user-local-admin")
	if err != nil {
		t.Fatalf("GenerateTokenForAccount() error = %v", err)
	}

	// Token should be valid before password change
	claims, err := svc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken() before password change: unexpected error = %v", err)
	}
	if claims.UserID != "user-local-admin" {
		t.Errorf("claims.UserID = %v, want user-local-admin", claims.UserID)
	}

	// Simulate a password change by directly setting a future mtime in the K8s secret.
	// This is deterministic (no time.Sleep) and tests the IsTokenValid path.
	futureMtime := time.Now().Add(2 * time.Second).UTC().Format(time.RFC3339)
	secret, err := k8sClient.CoreV1().Secrets("default").Get(context.Background(), AccountSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get secret: %v", err)
	}
	secret.Data["admin.passwordMtime"] = []byte(futureMtime)
	_, err = k8sClient.CoreV1().Secrets("default").Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update secret: %v", err)
	}
	// Clear cached accounts so GetPasswordMtime reads fresh data
	if err := accountStore.LoadAccounts(context.Background()); err != nil {
		t.Fatalf("LoadAccounts() error = %v", err)
	}

	// Token should now be invalid (iat < future passwordMtime)
	_, err = svc.ValidateToken(context.Background(), token)
	if err == nil {
		t.Error("ValidateToken() after password change: expected error, got nil")
	} else if !contains(err.Error(), "token invalidated by password change") {
		t.Errorf("ValidateToken() error = %v, want error containing 'token invalidated by password change'", err)
	}

	// Generate a new token - it should be valid (iat >= passwordMtime in same-second tolerance)
	// Reset mtime to now so new tokens are valid
	nowMtime := time.Now().UTC().Format(time.RFC3339)
	secret, err = k8sClient.CoreV1().Secrets("default").Get(context.Background(), AccountSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get secret for reset: %v", err)
	}
	secret.Data["admin.passwordMtime"] = []byte(nowMtime)
	_, err = k8sClient.CoreV1().Secrets("default").Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update secret for reset: %v", err)
	}
	if err := accountStore.LoadAccounts(context.Background()); err != nil {
		t.Fatalf("LoadAccounts() error = %v", err)
	}

	account, err = accountStore.GetAccount(context.Background(), "admin")
	if err != nil {
		t.Fatalf("GetAccount() after password change: error = %v", err)
	}
	newToken, _, err := svc.GenerateTokenForAccount(account, "user-local-admin")
	if err != nil {
		t.Fatalf("GenerateTokenForAccount() after password change: error = %v", err)
	}

	claims, err = svc.ValidateToken(context.Background(), newToken)
	if err != nil {
		t.Fatalf("ValidateToken() new token after password change: unexpected error = %v", err)
	}
	if claims.UserID != "user-local-admin" {
		t.Errorf("claims.UserID = %v, want user-local-admin", claims.UserID)
	}
}

func TestValidateToken_NonLocalUserSkipsPasswordCheck(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	redisClient := NewMockRedisClientAdapter()

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Create an OIDC user token (non-local user)
	oidcUserID := "oidc:provider1:user123"
	email := "oidc-user@example.com"
	displayName := "OIDC User"

	if _, err := casbinEnforcer.AddUserRole(oidcUserID, rbac.CasbinRoleServerAdmin); err != nil {
		t.Fatalf("AddUserRole() error = %v", err)
	}

	token, _, err := svc.GenerateTokenWithGroups(oidcUserID, email, displayName, nil)
	if err != nil {
		t.Fatalf("GenerateTokenWithGroups() error = %v", err)
	}

	// Token should be valid (non-local user skips password check)
	claims, err := svc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken() for OIDC user: unexpected error = %v", err)
	}
	if claims.UserID != oidcUserID {
		t.Errorf("claims.UserID = %v, want %v", claims.UserID, oidcUserID)
	}
}

func TestValidateToken_DeletedAccountRejectsToken(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	redisClient := NewMockRedisClientAdapter()

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Assign admin role
	if _, err := casbinEnforcer.AddUserRole("user-local-admin", rbac.CasbinRoleServerAdmin); err != nil {
		t.Fatalf("AddUserRole() error = %v", err)
	}

	// Generate a valid token
	account, err := accountStore.GetAccount(context.Background(), "admin")
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	token, _, err := svc.GenerateTokenForAccount(account, "user-local-admin")
	if err != nil {
		t.Fatalf("GenerateTokenForAccount() error = %v", err)
	}

	// Token should be valid before deletion
	_, err = svc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken() before deletion: unexpected error = %v", err)
	}

	// Delete the account by removing it from the ConfigMap, then reload
	cm, err := k8sClient.CoreV1().ConfigMaps("default").Get(context.Background(), AccountConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get configmap: %v", err)
	}
	delete(cm.Data, "accounts.admin")
	delete(cm.Data, "accounts.admin.enabled")
	_, err = k8sClient.CoreV1().ConfigMaps("default").Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update configmap: %v", err)
	}

	// Force cache reload so the deleted account is reflected
	if err := accountStore.LoadAccounts(context.Background()); err != nil {
		t.Fatalf("LoadAccounts() error = %v", err)
	}

	// Token should now be rejected (account not found → error path)
	_, err = svc.ValidateToken(context.Background(), token)
	if err == nil {
		t.Error("ValidateToken() after account deletion: expected error, got nil")
	} else if contains(err.Error(), "token invalidated by password change") {
		t.Errorf("ValidateToken() error should NOT say 'password change' for deleted account, got: %v", err)
	}
}

func TestBcryptCost(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)
	password := "test-password-123!A"

	redisClient := NewMockRedisClientAdapter()

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: password,
	}, accountStore, projectSvc, k8sClient, redisClient, nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Get the account to verify password was stored
	ctx := context.Background()
	account, err := svc.accountStore.GetAccount(ctx, "admin")
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}

	// Verify bcrypt cost is exactly 12
	cost, err := bcrypt.Cost([]byte(account.PasswordHash))
	if err != nil {
		t.Fatalf("bcrypt.Cost() error = %v", err)
	}

	if cost != BcryptCostAccountStore {
		t.Errorf("bcrypt cost = %d, want %d", cost, BcryptCostAccountStore)
	}

	// Verify password can be verified
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		t.Errorf("bcrypt.CompareHashAndPassword() error = %v", err)
	}

	// Verify wrong password fails
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte("wrong-password")); err == nil {
		t.Error("bcrypt.CompareHashAndPassword() succeeded for wrong password")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestComputePermissions tests the computePermissions function for various role scenarios
func TestComputePermissions(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)
	redisClient := NewMockRedisClientAdapter()

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	tests := []struct {
		name           string
		userID         string
		casbinRoles    []string
		expectedKeys   []string
		unexpectedKeys []string
	}{
		{
			name:        "admin_role_gets_all_permissions",
			userID:      "admin@example.com",
			casbinRoles: []string{rbac.CasbinRoleServerAdmin},
			expectedKeys: []string{
				"*:*",
				"settings:get",
				"settings:update",
				"projects:get",
				"projects:create",
				"projects:update",
				"projects:delete",
				"instances:get",
				"instances:create",
				"instances:delete",
				"repositories:get",
				"repositories:create",
				"repositories:delete",
				"rgds:get",
			},
		},
		{
			name:        "project_readonly_no_wildcard",
			userID:      "viewer@example.com",
			casbinRoles: []string{"proj:my-project:readonly"},
			unexpectedKeys: []string{
				"*:*",
			},
		},
		{
			name:        "project_admin_role_scoped_permissions",
			userID:      "proj-admin@example.com",
			casbinRoles: []string{"proj:my-project:admin"},
			expectedKeys: []string{
				"projects:update:my-project",
				"projects:get:my-project",
				"instances:create:my-project",
				"instances:delete:my-project",
				"instances:get:my-project",
				"repositories:create:my-project",
				"repositories:delete:my-project",
				"repositories:get:my-project",
			},
			unexpectedKeys: []string{
				"*:*",
				"projects:create",
				"projects:delete",
			},
		},
		{
			name:        "project_developer_role_limited_permissions",
			userID:      "dev@example.com",
			casbinRoles: []string{"proj:dev-team:developer"},
			expectedKeys: []string{
				"projects:get:dev-team",
				"instances:create:dev-team",
				"instances:delete:dev-team",
				"instances:get:dev-team",
				"repositories:get:dev-team",
			},
			unexpectedKeys: []string{
				"*:*",
				"projects:update:dev-team",
				"repositories:create:dev-team",
			},
		},
		{
			name:        "project_readonly_role_view_only",
			userID:      "viewer@example.com",
			casbinRoles: []string{"proj:test-project:readonly"},
			expectedKeys: []string{
				"projects:get:test-project",
				"instances:get:test-project",
				"repositories:get:test-project",
			},
			unexpectedKeys: []string{
				"*:*",
				"instances:create:test-project",
				"instances:delete:test-project",
			},
		},
		{
			name:        "multiple_project_roles",
			userID:      "multi-role@example.com",
			casbinRoles: []string{"proj:project-a:admin", "proj:project-b:developer"},
			expectedKeys: []string{
				// Project A - admin
				"projects:update:project-a",
				"instances:create:project-a",
				"repositories:create:project-a",
				// Project B - developer
				"projects:get:project-b",
				"instances:create:project-b",
			},
			unexpectedKeys: []string{
				"*:*",
				"projects:update:project-b", // developer can't update project
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			permissions := svc.computePermissions(tt.userID, tt.casbinRoles)

			// Check expected keys are present and true
			for _, key := range tt.expectedKeys {
				if !permissions[key] {
					t.Errorf("expected permission %q to be true, got false or missing", key)
				}
			}

			// Check unexpected keys are not present or false
			for _, key := range tt.unexpectedKeys {
				if permissions[key] {
					t.Errorf("unexpected permission %q is true, expected false or missing", key)
				}
			}
		})
	}
}

// TestCanI tests the CanI function for real-time permission checking
func TestCanI(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)
	redisClient := NewMockRedisClientAdapter()

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Setup test users with roles
	adminUserID := "admin-user@example.com"
	regularUserID := "regular-user@example.com"
	developerUserID := "developer@example.com"

	// Assign admin role to admin user
	if _, err := casbinEnforcer.AddUserRole(adminUserID, rbac.CasbinRoleServerAdmin); err != nil {
		t.Fatalf("failed to add admin role: %v", err)
	}

	// Assign project-scoped readonly role to regular user (global readonly was removed)
	if _, err := casbinEnforcer.AddUserRole(regularUserID, "proj:default:readonly"); err != nil {
		t.Fatalf("failed to add readonly role: %v", err)
	}

	// Add built-in readonly policies for the project-scoped role
	// (normally loaded via loadProjectPoliciesLocked when a Project CRD exists)
	readonlyPolicies := [][]string{
		{"proj:default:readonly", "projects/default", "get", "allow"},
		{"proj:default:readonly", "instances/default/*", "get", "allow"},
		{"proj:default:readonly", "rgds/default/*", "get", "allow"},
		{"proj:default:readonly", "repositories/default/*", "get", "allow"},
	}
	for _, p := range readonlyPolicies {
		if _, err := casbinEnforcer.AddPolicy(p[0], p[1], p[2], p[3]); err != nil {
			t.Fatalf("failed to add readonly policy: %v", err)
		}
	}

	// Add specific policy for developer user via direct policy
	// p, developer@example.com, instances/dev-project, create, allow
	// Note: Object is "instances/dev-project" because CanI constructs object as resource + "/" + subresource
	if _, err := casbinEnforcer.AddPolicy(developerUserID, "instances/dev-project", "create", "allow"); err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}

	tests := []struct {
		name        string
		userID      string
		groups      []string
		resource    string
		action      string
		subresource string
		expected    bool
	}{
		{
			name:        "admin_can_create_instances",
			userID:      adminUserID,
			groups:      nil,
			resource:    "instances",
			action:      "create",
			subresource: "any-project",
			expected:    true,
		},
		{
			name:        "admin_can_delete_projects",
			userID:      adminUserID,
			groups:      nil,
			resource:    "projects",
			action:      "delete",
			subresource: "-",
			expected:    true,
		},
		{
			name:        "admin_can_update_settings",
			userID:      adminUserID,
			groups:      nil,
			resource:    "settings",
			action:      "update",
			subresource: "-",
			expected:    true,
		},
		{
			name:        "readonly_can_get_instances_in_project",
			userID:      regularUserID,
			groups:      nil,
			resource:    "instances",
			action:      "get",
			subresource: "default",
			expected:    true,
		},
		{
			name:        "readonly_can_get_instances_in_any_project",
			userID:      regularUserID,
			groups:      nil,
			resource:    "instances",
			action:      "get",
			subresource: "-",
			expected:    true, // checkAnyProjectPermission finds project-scoped policy
		},
		{
			name:        "readonly_cannot_create_instances",
			userID:      regularUserID,
			groups:      nil,
			resource:    "instances",
			action:      "create",
			subresource: "default",
			expected:    false,
		},
		{
			name:        "readonly_cannot_delete_projects",
			userID:      regularUserID,
			groups:      nil,
			resource:    "projects",
			action:      "delete",
			subresource: "-",
			expected:    false,
		},
		{
			name:        "developer_can_create_in_allowed_project",
			userID:      developerUserID,
			groups:      nil,
			resource:    "instances",
			action:      "create",
			subresource: "dev-project",
			expected:    true,
		},
		{
			name:        "developer_cannot_create_in_other_project",
			userID:      developerUserID,
			groups:      nil,
			resource:    "instances",
			action:      "create",
			subresource: "other-project",
			expected:    false,
		},
		{
			name:        "developer_cannot_update_settings",
			userID:      developerUserID,
			groups:      nil,
			resource:    "settings",
			action:      "update",
			subresource: "-",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := svc.CanI(tt.userID, tt.groups, tt.resource, tt.action, tt.subresource)
			if err != nil {
				t.Fatalf("CanI() error = %v", err)
			}
			if allowed != tt.expected {
				t.Errorf("CanI(%s, %s, %s, %s) = %v, want %v",
					tt.userID, tt.resource, tt.action, tt.subresource,
					allowed, tt.expected)
			}
		})
	}
}

// TestCanI_OIDCGroups tests CanI with OIDC group-based policies
func TestCanI_OIDCGroups(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)
	redisClient := NewMockRedisClientAdapter()

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Add group-based policy: group:developers can create instances in dev-*
	if _, err := casbinEnforcer.AddPolicy("group:developers", "instances/dev-*/*", "create", "allow"); err != nil {
		t.Fatalf("failed to add group policy: %v", err)
	}

	// Add group-based policy: group:platform-team can do anything
	if _, err := casbinEnforcer.AddPolicy("group:platform-team", "*", "*", "allow"); err != nil {
		t.Fatalf("failed to add group policy: %v", err)
	}

	tests := []struct {
		name        string
		userID      string
		groups      []string
		resource    string
		action      string
		subresource string
		expected    bool
	}{
		{
			name:        "user_with_developer_group_can_create_in_dev",
			userID:      "user1@example.com",
			groups:      []string{"developers"},
			resource:    "instances",
			action:      "create",
			subresource: "dev-team",
			expected:    true,
		},
		{
			name:        "user_with_developer_group_cannot_create_in_prod",
			userID:      "user1@example.com",
			groups:      []string{"developers"},
			resource:    "instances",
			action:      "create",
			subresource: "prod-team",
			expected:    false,
		},
		{
			name:        "platform_team_can_do_anything",
			userID:      "platform-eng@example.com",
			groups:      []string{"platform-team"},
			resource:    "settings",
			action:      "update",
			subresource: "-",
			expected:    true,
		},
		{
			name:        "user_without_groups_denied",
			userID:      "nobody@example.com",
			groups:      nil,
			resource:    "instances",
			action:      "create",
			subresource: "dev-team",
			expected:    false,
		},
		{
			name:        "user_with_multiple_groups_allowed_if_any_match",
			userID:      "multi-group@example.com",
			groups:      []string{"viewers", "developers"},
			resource:    "instances",
			action:      "create",
			subresource: "dev-alpha",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := svc.CanI(tt.userID, tt.groups, tt.resource, tt.action, tt.subresource)
			if err != nil {
				t.Fatalf("CanI() error = %v", err)
			}
			if allowed != tt.expected {
				t.Errorf("CanI(%s, groups=%v, %s, %s, %s) = %v, want %v",
					tt.userID, tt.groups, tt.resource, tt.action, tt.subresource,
					allowed, tt.expected)
			}
		})
	}
}

// TestCanI_WildcardFallback tests that CanI checks wildcard patterns for project-scoped permissions
// This is important for the ArgoCD-style pattern where policies like "repositories/project/*, *, allow"
// should also allow checking "repositories/project" (the project context without a specific resource)
func TestCanI_WildcardFallback(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)
	redisClient := NewMockRedisClientAdapter()

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// User with project-scoped repository permission
	// Policy: "repositories/my-project/*, *, allow" (can manage any repo in the project)
	projectAdminUserID := "project-admin@example.com"
	if _, err := casbinEnforcer.AddPolicy(projectAdminUserID, "repositories/my-project/*", "*", "allow"); err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}

	tests := []struct {
		name        string
		userID      string
		groups      []string
		resource    string
		action      string
		subresource string
		expected    bool
	}{
		{
			name:        "project_admin_can_create_repos_in_project_with_wildcard",
			userID:      projectAdminUserID,
			groups:      nil,
			resource:    "repositories",
			action:      "create",
			subresource: "my-project/*", // Explicit wildcard
			expected:    true,
		},
		{
			name:        "project_admin_can_create_repos_in_project_context",
			userID:      projectAdminUserID,
			groups:      nil,
			resource:    "repositories",
			action:      "create",
			subresource: "my-project", // Just project name - should fallback to wildcard check
			expected:    true,
		},
		{
			name:        "project_admin_can_get_specific_repo",
			userID:      projectAdminUserID,
			groups:      nil,
			resource:    "repositories",
			action:      "get",
			subresource: "my-project/https://github.com/myrepo", // Specific repo URL
			expected:    true,
		},
		{
			name:        "project_admin_cannot_access_other_project_context",
			userID:      projectAdminUserID,
			groups:      nil,
			resource:    "repositories",
			action:      "create",
			subresource: "other-project", // Different project
			expected:    false,
		},
		{
			name:        "project_admin_cannot_access_other_project_wildcard",
			userID:      projectAdminUserID,
			groups:      nil,
			resource:    "repositories",
			action:      "create",
			subresource: "other-project/*", // Different project wildcard
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := svc.CanI(tt.userID, tt.groups, tt.resource, tt.action, tt.subresource)
			if err != nil {
				t.Fatalf("CanI() error = %v", err)
			}
			if allowed != tt.expected {
				t.Errorf("CanI(%s, %s, %s, %s) = %v, want %v",
					tt.userID, tt.resource, tt.action, tt.subresource,
					allowed, tt.expected)
			}
		})
	}
}

// TestCanI_NilEnforcer tests that CanI returns error when enforcer is not configured
func TestCanI_NilEnforcer(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)
	redisClient := NewMockRedisClientAdapter()

	// Create service WITHOUT casbin enforcer
	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = svc.CanI("user@example.com", nil, "instances", "create", "-")
	if err == nil {
		t.Error("CanI() should return error when enforcer is nil")
	}
	if !stringContains(err.Error(), "casbin enforcer not configured") {
		t.Errorf("error should mention 'casbin enforcer not configured', got: %v", err)
	}
}

// TestCanI_GenericProjectScopedPermission tests that CanI correctly handles the case where
// frontend checks for generic permission ("can I create instances?") without specifying a project,
// but user has project-scoped policies like "instances/proj-a/*, create, allow".
// This is the scenario where:
// - Frontend calls: useCanI('instances', 'create') -> /api/v1/account/can-i/instances/create/-
// - User has project policy: "instances/proj-a/*, create, allow"
// - Expected: allowed=true (user can create instances in their project)
func TestCanI_GenericProjectScopedPermission(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)
	redisClient := NewMockRedisClientAdapter()

	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Set up project-scoped role with policies (simulating what ProjectService creates)
	// This mimics what happens when a user is admin of project "proj-azuread-staging"
	projectRole := "proj:proj-azuread-staging:admin"
	if _, err := casbinEnforcer.AddPolicy(projectRole, "instances/proj-azuread-staging/*", "*", "allow"); err != nil {
		t.Fatalf("failed to add instances policy: %v", err)
	}
	if _, err := casbinEnforcer.AddPolicy(projectRole, "repositories/proj-azuread-staging/*", "*", "allow"); err != nil {
		t.Fatalf("failed to add repositories policy: %v", err)
	}
	if _, err := casbinEnforcer.AddPolicy(projectRole, "compliance/*", "get", "allow"); err != nil {
		t.Fatalf("failed to add compliance get policy: %v", err)
	}
	if _, err := casbinEnforcer.AddPolicy(projectRole, "compliance/*", "list", "allow"); err != nil {
		t.Fatalf("failed to add compliance list policy: %v", err)
	}

	// Add OIDC group mapping: Azure AD group -> project role
	oidcGroup := "group:7e24cb11-e404-4b4d-9e2c-96d6e7b4733c"
	if _, err := casbinEnforcer.AddUserRole(oidcGroup, projectRole); err != nil {
		t.Fatalf("failed to add group to role: %v", err)
	}

	tests := []struct {
		name        string
		userID      string
		groups      []string
		resource    string
		action      string
		subresource string
		expected    bool
		description string
	}{
		{
			name:        "project_admin_can_create_instances_generic",
			userID:      "user@azuread.test",
			groups:      []string{"7e24cb11-e404-4b4d-9e2c-96d6e7b4733c"}, // Azure AD group
			resource:    "instances",
			action:      "create",
			subresource: "-", // Generic check (no specific project)
			expected:    true,
			description: "User with project admin role should be able to create instances (deploy button should show)",
		},
		{
			name:        "project_admin_can_create_instances_in_own_project",
			userID:      "user@azuread.test",
			groups:      []string{"7e24cb11-e404-4b4d-9e2c-96d6e7b4733c"},
			resource:    "instances",
			action:      "create",
			subresource: "proj-azuread-staging",
			expected:    true,
			description: "User should be able to create instances in their specific project",
		},
		{
			name:        "project_admin_cannot_create_instances_in_other_project",
			userID:      "user@azuread.test",
			groups:      []string{"7e24cb11-e404-4b4d-9e2c-96d6e7b4733c"},
			resource:    "instances",
			action:      "create",
			subresource: "other-project",
			expected:    false,
			description: "User should NOT be able to create instances in other projects",
		},
		{
			name:        "project_admin_can_view_compliance_generic",
			userID:      "user@azuread.test",
			groups:      []string{"7e24cb11-e404-4b4d-9e2c-96d6e7b4733c"},
			resource:    "compliance",
			action:      "get",
			subresource: "-",
			expected:    true,
			description: "User with project admin role should see compliance page",
		},
		{
			name:        "user_without_project_role_cannot_create_instances",
			userID:      "other@test.com",
			groups:      []string{"some-other-group"},
			resource:    "instances",
			action:      "create",
			subresource: "-",
			expected:    false,
			description: "User without project role should NOT see deploy button",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := svc.CanI(tt.userID, tt.groups, tt.resource, tt.action, tt.subresource)
			if err != nil {
				t.Fatalf("CanI() error = %v", err)
			}
			if allowed != tt.expected {
				t.Errorf("CanI(%s, groups=%v, %s, %s, %s) = %v, want %v\nDescription: %s",
					tt.userID, tt.groups, tt.resource, tt.action, tt.subresource,
					allowed, tt.expected, tt.description)
			}
		})
	}
}

// TestComputePermissions_NilEnforcer tests that computePermissions returns nil when enforcer is not configured
func TestComputePermissions_NilEnforcer(t *testing.T) {
	accountStore, projectSvc, k8sClient := setupTestAccountStore(t)
	redisClient := NewMockRedisClientAdapter()

	// Create service WITHOUT casbin enforcer
	svc, err := NewService(&Config{
		JWTSecret:          "test-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	permissions := svc.computePermissions("user@example.com", []string{"role:serveradmin"})
	if permissions != nil {
		t.Error("computePermissions() should return nil when enforcer is nil")
	}
}
