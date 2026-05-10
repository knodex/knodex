// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

// NOTE: Tests in this file are NOT safe for t.Parallel() due to shared MockRedisClient storage map
// and OIDCService instance across subtests (e.g., TestValidateStateToken shares svc/redisClient).
// See tech-spec: go-test-mechanics-parallel-and-setup for details.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// MockRedisClient is a mock Redis client for testing
type MockRedisClient struct {
	storage map[string]string
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		storage: make(map[string]string),
	}
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.storage[key] = value.(string)
	return redis.NewStatusCmd(ctx)
}

func (m *MockRedisClient) GetDel(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	if val, ok := m.storage[key]; ok {
		delete(m.storage, key)
		cmd.SetVal(val)
		return cmd
	}
	cmd.SetErr(redis.Nil)
	return cmd
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	return redis.NewStatusCmd(ctx)
}

func (m *MockRedisClient) Close() error {
	return nil
}

// mockAuthServiceForOIDC is a mock auth service for testing OIDC
// Updated to match new ServiceInterface
type mockAuthServiceForOIDC struct {
	generateTokenForAccountFunc func(account *Account, userID string) (string, time.Time, error)
	generateTokenWithGroupsFunc func(userID, email, displayName string, groups []string) (string, time.Time, error)
	localLoginEnabled           bool // configurable; default false suits OIDC unit tests
}

func (m *mockAuthServiceForOIDC) AuthenticateLocal(ctx context.Context, username, password, sourceIP string) (*LoginResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceForOIDC) GenerateTokenForAccount(account *Account, userID string) (string, time.Time, error) {
	if m.generateTokenForAccountFunc != nil {
		return m.generateTokenForAccountFunc(account, userID)
	}
	return "mock-token", time.Now().Add(1 * time.Hour), nil
}

func (m *mockAuthServiceForOIDC) GenerateTokenWithGroups(userID, email, displayName string, groups []string) (string, time.Time, error) {
	if m.generateTokenWithGroupsFunc != nil {
		return m.generateTokenWithGroupsFunc(userID, email, displayName, groups)
	}
	return "mock-token", time.Now().Add(1 * time.Hour), nil
}

func (m *mockAuthServiceForOIDC) ValidateToken(_ context.Context, tokenString string) (*JWTClaims, error) {
	return nil, nil
}

func (m *mockAuthServiceForOIDC) RevokeToken(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *mockAuthServiceForOIDC) IsLocalLoginEnabled() bool { return m.localLoginEnabled }

// createTestOIDCProvisioningService creates a test provisioning service
// Replaces createTestProvisioningService that used UserProvisioningService
func createTestOIDCProvisioningService() (*OIDCProvisioningService, *fake.Clientset) {
	k8sClient := fake.NewSimpleClientset()
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
	_, _ = k8sClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), accountsCM, metav1.CreateOptions{})

	// Create Secret for credentials
	accountsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-jwt-secret"),
		},
	}
	_, _ = k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), accountsSecret, metav1.CreateOptions{})

	projectService := rbac.NewProjectService(k8sClient, nil, "knodex-system")
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	// Create with empty group mapper (no mappings configured), no default role
	return NewOIDCProvisioningService(projectService, nil, casbinEnforcer, ""), k8sClient
}

// createTestOIDCProvisioningServiceWithMapper creates a test provisioning service with a custom GroupMapper
// Used by integration tests that need to test group mappings
func createTestOIDCProvisioningServiceWithMapper(groupMapper *GroupMapper) *OIDCProvisioningService {
	svc, _ := createTestOIDCProvisioningServiceWithMapperAndEnforcer(groupMapper)
	return svc
}

// createTestOIDCProvisioningServiceWithMapperAndEnforcer creates a test provisioning service
// with a custom GroupMapper and returns both the service and the Casbin enforcer.
// The enforcer should be passed to OIDCService so it can retrieve roles assigned by the provisioning service.
func createTestOIDCProvisioningServiceWithMapperAndEnforcer(groupMapper *GroupMapper) (*OIDCProvisioningService, *rbac.CasbinEnforcer) {
	k8sClient := fake.NewSimpleClientset()
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
	_, _ = k8sClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), accountsCM, metav1.CreateOptions{})

	// Create Secret for credentials
	accountsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-jwt-secret"),
		},
	}
	_, _ = k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), accountsSecret, metav1.CreateOptions{})

	projectService := rbac.NewProjectService(k8sClient, nil, "knodex-system")
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	return NewOIDCProvisioningService(projectService, groupMapper, casbinEnforcer, ""), casbinEnforcer
}

// TestNewOIDCService tests the OIDCService constructor
// Updated to remove UserService and UserProvisioningService parameters
func TestNewOIDCService(t *testing.T) {
	tests := []struct {
		name                string
		config              *Config
		redisClient         RedisClient
		authService         ServiceInterface
		provisioningService *OIDCProvisioningService
		wantErr             bool
		errContains         string
	}{
		{
			name:        "nil config",
			config:      nil,
			redisClient: NewMockRedisClient(),
			authService: &mockAuthServiceForOIDC{},
			wantErr:     true,
			errContains: "config cannot be nil",
		},
		{
			name:        "nil redis client with OIDC disabled",
			config:      &Config{OIDCEnabled: false},
			redisClient: nil,
			authService: &mockAuthServiceForOIDC{},
			wantErr:     false, // Redis not required when OIDC disabled
		},
		{
			name: "nil redis client with OIDC enabled",
			config: &Config{
				OIDCEnabled: true,
			},
			redisClient: nil,
			authService: &mockAuthServiceForOIDC{},
			wantErr:     true,
			errContains: "redisClient cannot be nil when OIDC is enabled",
		},
		{
			name:        "nil auth service",
			config:      &Config{},
			redisClient: NewMockRedisClient(),
			authService: nil,
			wantErr:     true,
			errContains: "authService cannot be nil",
		},
		{
			name: "nil provisioning service with OIDC enabled",
			config: &Config{
				OIDCEnabled: true,
			},
			redisClient:         NewMockRedisClient(),
			authService:         &mockAuthServiceForOIDC{},
			provisioningService: nil,
			wantErr:             true,
			errContains:         "provisioningService cannot be nil when OIDC is enabled",
		},
		{
			name: "OIDC disabled",
			config: &Config{
				OIDCEnabled: false,
			},
			redisClient: NewMockRedisClient(),
			authService: &mockAuthServiceForOIDC{},
			wantErr:     false,
		},
		{
			name: "OIDC enabled with valid config but no providers",
			config: &Config{
				OIDCEnabled:   true,
				OIDCProviders: []OIDCProviderConfig{},
			},
			redisClient: NewMockRedisClient(),
			authService: &mockAuthServiceForOIDC{},
			wantErr:     false, // No error, but warning logged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use provided provisioning service or create a test one
			provSvc := tt.provisioningService
			if provSvc == nil && !strings.Contains(tt.errContains, "provisioningService cannot be nil") {
				provSvc, _ = createTestOIDCProvisioningService()
			}
			casbinEnforcer, _ := rbac.NewCasbinEnforcer()
			svc, err := NewOIDCService(tt.config, tt.redisClient, tt.authService, provSvc, casbinEnforcer, nil)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewOIDCService() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewOIDCService() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NewOIDCService() unexpected error = %v", err)
				return
			}
			if svc == nil {
				t.Errorf("NewOIDCService() returned nil service")
			}
		})
	}
}

// TestGenerateStateToken tests state token generation
func TestGenerateStateToken(t *testing.T) {
	redisClient := NewMockRedisClient()
	config := &Config{
		OIDCEnabled: false,
	}
	authService := &mockAuthServiceForOIDC{}
	provSvc, _ := createTestOIDCProvisioningService()
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	svc, err := NewOIDCService(config, redisClient, authService, provSvc, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Test state token generation
	providerName := "azuread"
	redirectURL := ""
	state, nonce, verifier, err := svc.GenerateStateToken(ctx, providerName, redirectURL)
	if err != nil {
		t.Fatalf("GenerateStateToken() error = %v", err)
	}

	// Verify state token is not empty
	if state == "" {
		t.Error("GenerateStateToken() returned empty state token")
	}

	// Verify nonce is not empty
	if nonce == "" {
		t.Error("GenerateStateToken() returned empty nonce")
	}

	// Verify PKCE verifier is not empty and within RFC 7636 §4.1 length bounds
	if verifier == "" {
		t.Error("GenerateStateToken() returned empty PKCE verifier")
	}
	if len(verifier) < 43 || len(verifier) > 128 {
		t.Errorf("GenerateStateToken() verifier length = %d, want 43-128 (RFC 7636 §4.1)", len(verifier))
	}

	// Verify state token is base64 encoded
	decoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		t.Errorf("GenerateStateToken() returned invalid base64: %v", err)
	}

	// Verify state token length (should be 32 bytes)
	if len(decoded) != StateTokenLength {
		t.Errorf("GenerateStateToken() token length = %d, want %d", len(decoded), StateTokenLength)
	}

	// Verify nonce is base64 encoded
	decodedNonce, err := base64.URLEncoding.DecodeString(nonce)
	if err != nil {
		t.Errorf("GenerateStateToken() returned invalid base64 nonce: %v", err)
	}

	// Verify nonce length (should be 32 bytes)
	if len(decodedNonce) != NonceLength {
		t.Errorf("GenerateStateToken() nonce length = %d, want %d", len(decodedNonce), NonceLength)
	}

	// Verify state token is stored in Redis
	key := "oidc:state:" + state
	if _, ok := redisClient.storage[key]; !ok {
		t.Error("GenerateStateToken() did not store token in Redis")
	}

	// Verify stored value is the provider name
	if redisClient.storage[key] != providerName {
		t.Errorf("GenerateStateToken() stored value = %v, want %v", redisClient.storage[key], providerName)
	}

	// Verify nonce is stored in Redis
	nonceKey := NoncePrefix + state
	if _, ok := redisClient.storage[nonceKey]; !ok {
		t.Error("GenerateStateToken() did not store nonce in Redis")
	}

	// Verify stored nonce value matches returned nonce
	if redisClient.storage[nonceKey] != nonce {
		t.Errorf("GenerateStateToken() stored nonce = %v, want %v", redisClient.storage[nonceKey], nonce)
	}

	// Verify PKCE verifier is stored in Redis
	verifierKey := PKCEVerifierPrefix + state
	if _, ok := redisClient.storage[verifierKey]; !ok {
		t.Error("GenerateStateToken() did not store PKCE verifier in Redis")
	}
	if redisClient.storage[verifierKey] != verifier {
		t.Errorf("GenerateStateToken() stored verifier = %v, want %v", redisClient.storage[verifierKey], verifier)
	}
}

// TestValidateStateToken tests state token validation
func TestValidateStateToken(t *testing.T) {
	redisClient := NewMockRedisClient()
	config := &Config{
		OIDCEnabled: false,
	}
	authService := &mockAuthServiceForOIDC{}
	provSvc, _ := createTestOIDCProvisioningService()
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	svc, err := NewOIDCService(config, redisClient, authService, provSvc, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name            string
		setupFunc       func() string
		wantErr         bool
		errContains     string
		wantProvider    string
		wantRedirectURL string
	}{
		{
			name: "valid state token",
			setupFunc: func() string {
				state, _, _, _ := svc.GenerateStateToken(ctx, "azuread", "")
				return state
			},
			wantErr:         false,
			wantProvider:    "azuread",
			wantRedirectURL: "",
		},
		{
			name: "empty state token",
			setupFunc: func() string {
				return ""
			},
			wantErr:     true,
			errContains: "state token cannot be empty",
		},
		{
			name: "non-existent state token",
			setupFunc: func() string {
				return "nonexistent"
			},
			wantErr:     true,
			errContains: "invalid or expired",
		},
		{
			name: "already used state token (double validation)",
			setupFunc: func() string {
				state, _, _, _ := svc.GenerateStateToken(ctx, "google", "")
				// Validate once (should succeed)
				svc.ValidateStateToken(ctx, state)
				// Return same token for second validation (should fail)
				return state
			},
			wantErr:     true,
			errContains: "invalid or expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.setupFunc()
			provider, redirectURL, err := svc.ValidateStateToken(ctx, state)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateStateToken() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateStateToken() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateStateToken() unexpected error = %v", err)
			}
			if provider != tt.wantProvider {
				t.Errorf("ValidateStateToken() provider = %v, want %v", provider, tt.wantProvider)
			}
			if redirectURL != tt.wantRedirectURL {
				t.Errorf("ValidateStateToken() redirectURL = %v, want %v", redirectURL, tt.wantRedirectURL)
			}
		})
	}
}

// TestListProviders tests provider listing
func TestListProviders(t *testing.T) {
	redisClient := NewMockRedisClient()
	config := &Config{
		OIDCEnabled: false,
		OIDCProviders: []OIDCProviderConfig{
			{Name: "google"},
			{Name: "azure"},
			{Name: "okta"},
		},
	}
	authService := &mockAuthServiceForOIDC{}
	provSvc, _ := createTestOIDCProvisioningService()
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	svc, err := NewOIDCService(config, redisClient, authService, provSvc, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	// Since providers are not actually initialized (OIDC disabled),
	// this should return empty list
	providers := svc.ListProviders()
	if len(providers) != 0 {
		t.Errorf("ListProviders() with OIDC disabled = %d providers, want 0", len(providers))
	}
}

// TestGetAuthCodeURL tests authorization URL generation
func TestGetAuthCodeURL(t *testing.T) {
	redisClient := NewMockRedisClient()
	config := &Config{
		OIDCEnabled: false,
	}
	authService := &mockAuthServiceForOIDC{}
	provSvc, _ := createTestOIDCProvisioningService()
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	svc, err := NewOIDCService(config, redisClient, authService, provSvc, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	tests := []struct {
		name         string
		providerName string
		state        string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "unknown provider",
			providerName: "unknown",
			state:        "test-state",
			wantErr:      true,
			errContains:  "unknown OIDC provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := svc.GetAuthCodeURL(tt.providerName, tt.state, "test-nonce", "test-verifier-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetAuthCodeURL() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GetAuthCodeURL() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("GetAuthCodeURL() unexpected error = %v", err)
				return
			}
			if url == "" {
				t.Error("GetAuthCodeURL() returned empty URL")
			}
		})
	}
}

// TestStateTokenRandomness tests that state tokens are unique
func TestStateTokenRandomness(t *testing.T) {
	redisClient := NewMockRedisClient()
	config := &Config{
		OIDCEnabled: false,
	}
	authService := &mockAuthServiceForOIDC{}
	provSvc, _ := createTestOIDCProvisioningService()
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	svc, err := NewOIDCService(config, redisClient, authService, provSvc, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Generate multiple state tokens
	tokens := make(map[string]bool)
	numTokens := 100

	for i := 0; i < numTokens; i++ {
		state, _, _, err := svc.GenerateStateToken(ctx, "azuread", "")
		if err != nil {
			t.Fatalf("GenerateStateToken() error = %v", err)
		}
		if tokens[state] {
			t.Errorf("GenerateStateToken() generated duplicate token: %s", state)
		}
		tokens[state] = true
	}

	if len(tokens) != numTokens {
		t.Errorf("Generated %d unique tokens, want %d", len(tokens), numTokens)
	}
}

// TestStateTokenTTL tests that state tokens have correct TTL
func TestStateTokenTTL(t *testing.T) {
	// This test verifies the TTL constant
	expectedTTL := 5 * time.Minute
	if StateTokenTTL != expectedTTL {
		t.Errorf("StateTokenTTL = %v, want %v", StateTokenTTL, expectedTTL)
	}
}

// TestStateTokenLength tests that state token length is correct
func TestStateTokenLength(t *testing.T) {
	// This test verifies the length constant
	expectedLength := 32
	if StateTokenLength != expectedLength {
		t.Errorf("StateTokenLength = %d, want %d", StateTokenLength, expectedLength)
	}
}

// TestSanitizeOIDCGroups tests the OIDC groups sanitization function
func TestSanitizeOIDCGroups(t *testing.T) {
	tests := []struct {
		name         string
		input        []string
		wantLen      int
		wantContains []string
		wantExcludes []string
	}{
		{
			name:         "nil input returns empty slice",
			input:        nil,
			wantLen:      0,
			wantContains: []string{},
		},
		{
			name:         "empty slice returns empty slice",
			input:        []string{},
			wantLen:      0,
			wantContains: []string{},
		},
		{
			name:         "valid groups are preserved",
			input:        []string{"engineering", "platform-team", "admins"},
			wantLen:      3,
			wantContains: []string{"engineering", "platform-team", "admins"},
		},
		{
			name:         "skip empty strings",
			input:        []string{"valid", "", "group"},
			wantLen:      2,
			wantContains: []string{"valid", "group"},
		},
		{
			name:         "filter groups with control characters (null byte)",
			input:        []string{"valid", "bad\x00group", "good"},
			wantLen:      2,
			wantContains: []string{"valid", "good"},
			wantExcludes: []string{"bad\x00group"},
		},
		{
			name:         "filter groups with control characters (newline)",
			input:        []string{"valid", "bad\ngroup", "good"},
			wantLen:      2,
			wantContains: []string{"valid", "good"},
			wantExcludes: []string{"bad\ngroup"},
		},
		{
			name:         "filter groups with control characters (tab)",
			input:        []string{"valid", "bad\tgroup", "good"},
			wantLen:      2,
			wantContains: []string{"valid", "good"},
			wantExcludes: []string{"bad\tgroup"},
		},
		{
			name:         "filter invalid UTF-8",
			input:        []string{"valid", "\xff\xfe", "good"},
			wantLen:      2,
			wantContains: []string{"valid", "good"},
		},
		{
			name:    "truncate long group names",
			input:   []string{strings.Repeat("a", 300)},
			wantLen: 1,
		},
		{
			name:         "preserve nested paths",
			input:        []string{"/org/team/subteam", "flat-group"},
			wantLen:      2,
			wantContains: []string{"/org/team/subteam", "flat-group"},
		},
		{
			name:         "preserve special characters in valid groups",
			input:        []string{"group-name", "group_name", "group.name", "group:scope"},
			wantLen:      4,
			wantContains: []string{"group-name", "group_name", "group.name", "group:scope"},
		},
		{
			name:         "preserve unicode characters",
			input:        []string{"группа", "グループ", "grupo"},
			wantLen:      3,
			wantContains: []string{"группа", "グループ", "grupo"},
		},
		{
			name:         "single valid group",
			input:        []string{"admins"},
			wantLen:      1,
			wantContains: []string{"admins"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeOIDCGroups(tt.input)

			if len(result) != tt.wantLen {
				t.Errorf("sanitizeOIDCGroups() len = %d, want %d", len(result), tt.wantLen)
			}

			for _, want := range tt.wantContains {
				if !containsString(result, want) {
					t.Errorf("sanitizeOIDCGroups() missing expected group %q, got %v", want, result)
				}
			}

			for _, exclude := range tt.wantExcludes {
				if containsString(result, exclude) {
					t.Errorf("sanitizeOIDCGroups() should not contain %q", exclude)
				}
			}
		})
	}
}

// TestSanitizeOIDCGroups_TruncateLongGroupName tests truncation of long group names
func TestSanitizeOIDCGroups_TruncateLongGroupName(t *testing.T) {
	longName := strings.Repeat("a", 300)
	result := sanitizeOIDCGroups([]string{longName})

	if len(result) != 1 {
		t.Fatalf("sanitizeOIDCGroups() expected 1 result, got %d", len(result))
	}

	if len(result[0]) != MaxOIDCGroupNameLength {
		t.Errorf("sanitizeOIDCGroups() group length = %d, want %d (MaxOIDCGroupNameLength)",
			len(result[0]), MaxOIDCGroupNameLength)
	}
}

// TestSanitizeOIDCGroups_TruncateUTF8Safe tests that truncation doesn't break multi-byte UTF-8
func TestSanitizeOIDCGroups_TruncateUTF8Safe(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "multi-byte at boundary",
			// 254 'a' + "中中中" (9 bytes) = 263 bytes, must truncate safely
			input: strings.Repeat("a", MaxOIDCGroupNameLength-2) + "中中中",
		},
		{
			name: "emoji at boundary",
			// 253 'a' + "🎉🎉" (8 bytes) = 261 bytes
			input: strings.Repeat("a", MaxOIDCGroupNameLength-3) + "🎉🎉",
		},
		{
			name: "mixed multi-byte throughout",
			// Mix of ASCII and multi-byte chars: 100 * "日本語" = 100 * 9 bytes = 900 bytes
			input: strings.Repeat("日本語", 100),
		},
		{
			name: "2-byte chars at boundary",
			// 255 'a' + "üü" (4 bytes) = 259 bytes
			input: strings.Repeat("a", MaxOIDCGroupNameLength-1) + "üü",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeOIDCGroups([]string{tt.input})

			if len(result) != 1 {
				t.Fatalf("expected 1 result, got %d", len(result))
			}

			truncated := result[0]

			// Must be valid UTF-8
			if !utf8.ValidString(truncated) {
				t.Errorf("truncated string is not valid UTF-8: %q", truncated)
			}

			// Must not exceed max length
			if len(truncated) > MaxOIDCGroupNameLength {
				t.Errorf("truncated length %d exceeds max %d", len(truncated), MaxOIDCGroupNameLength)
			}

			// Must be at least somewhat close to max (not over-truncated)
			// Allow up to 3 bytes less for the largest UTF-8 char (4 bytes - 1)
			if len(truncated) < MaxOIDCGroupNameLength-3 {
				t.Errorf("truncated too aggressively: got %d bytes, expected at least %d",
					len(truncated), MaxOIDCGroupNameLength-3)
			}

			// Must be a prefix of the original (content preserved)
			if !strings.HasPrefix(tt.input, truncated) {
				t.Errorf("truncated string is not a prefix of original")
			}
		})
	}
}

// TestTruncateUTF8 tests the truncateUTF8 helper function directly
func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxBytes int
		wantLen  int // expected length in bytes
	}{
		{
			name:     "empty string",
			input:    "",
			maxBytes: 10,
			wantLen:  0,
		},
		{
			name:     "string shorter than max",
			input:    "hello",
			maxBytes: 10,
			wantLen:  5,
		},
		{
			name:     "string exactly at max",
			input:    "hello",
			maxBytes: 5,
			wantLen:  5,
		},
		{
			name:     "ASCII truncation",
			input:    "hello world",
			maxBytes: 5,
			wantLen:  5,
		},
		{
			name:     "truncate before 2-byte char",
			input:    "aaa" + "ü", // 3 + 2 = 5 bytes
			maxBytes: 4,           // can't fit ü, should get "aaa"
			wantLen:  3,
		},
		{
			name:     "truncate before 3-byte char",
			input:    "aa" + "中", // 2 + 3 = 5 bytes
			maxBytes: 4,          // can't fit 中, should get "aa"
			wantLen:  2,
		},
		{
			name:     "truncate before 4-byte emoji",
			input:    "a" + "🎉", // 1 + 4 = 5 bytes
			maxBytes: 4,         // can't fit 🎉, should get "a"
			wantLen:  1,
		},
		{
			name:     "maxBytes zero",
			input:    "hello",
			maxBytes: 0,
			wantLen:  0,
		},
		{
			name:     "maxBytes one with multi-byte start",
			input:    "中文", // starts with 3-byte char
			maxBytes: 1,
			wantLen:  0, // can't fit any complete rune
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateUTF8(tt.input, tt.maxBytes)

			if len(result) != tt.wantLen {
				t.Errorf("truncateUTF8(%q, %d) len = %d, want %d",
					tt.input, tt.maxBytes, len(result), tt.wantLen)
			}

			if !utf8.ValidString(result) {
				t.Errorf("truncateUTF8(%q, %d) produced invalid UTF-8: %q",
					tt.input, tt.maxBytes, result)
			}

			if len(result) > 0 && !strings.HasPrefix(tt.input, result) {
				t.Errorf("truncateUTF8(%q, %d) = %q is not a prefix of input",
					tt.input, tt.maxBytes, result)
			}
		})
	}
}

// TestSanitizeOIDCGroups_TruncateExcessiveGroups tests DoS protection for excessive groups
func TestSanitizeOIDCGroups_TruncateExcessiveGroups(t *testing.T) {
	// Generate more groups than the maximum
	excessiveGroups := make([]string, MaxOIDCGroups+100)
	for i := 0; i < len(excessiveGroups); i++ {
		excessiveGroups[i] = "group-" + strings.Repeat("x", i%10)
	}

	result := sanitizeOIDCGroups(excessiveGroups)

	if len(result) > MaxOIDCGroups {
		t.Errorf("sanitizeOIDCGroups() returned %d groups, want <= %d (MaxOIDCGroups)",
			len(result), MaxOIDCGroups)
	}
}

// TestSanitizeOIDCGroups_MemorySafety tests memory safety with extreme inputs
func TestSanitizeOIDCGroups_MemorySafety(t *testing.T) {
	// Test 1: Very large number of groups
	largeInput := make([]string, 10000)
	for i := 0; i < len(largeInput); i++ {
		largeInput[i] = "group-" + string(rune('a'+i%26))
	}
	result := sanitizeOIDCGroups(largeInput)
	if len(result) > MaxOIDCGroups {
		t.Errorf("sanitizeOIDCGroups() failed to limit groups: got %d, max %d", len(result), MaxOIDCGroups)
	}

	// Test 2: Groups with very long names
	longNames := make([]string, 10)
	for i := 0; i < 10; i++ {
		longNames[i] = strings.Repeat("a", 10000)
	}
	result = sanitizeOIDCGroups(longNames)
	for _, group := range result {
		if len(group) > MaxOIDCGroupNameLength {
			t.Errorf("sanitizeOIDCGroups() failed to truncate group: len=%d, max=%d", len(group), MaxOIDCGroupNameLength)
		}
	}
}

// TestSanitizeOIDCGroups_Constants tests that security constants are set correctly
func TestSanitizeOIDCGroups_Constants(t *testing.T) {
	// Verify MaxOIDCGroups is a reasonable value
	if MaxOIDCGroups < 100 || MaxOIDCGroups > 10000 {
		t.Errorf("MaxOIDCGroups = %d, expected between 100 and 10000", MaxOIDCGroups)
	}

	// Verify MaxOIDCGroupNameLength is a reasonable value
	if MaxOIDCGroupNameLength < 64 || MaxOIDCGroupNameLength > 1024 {
		t.Errorf("MaxOIDCGroupNameLength = %d, expected between 64 and 1024", MaxOIDCGroupNameLength)
	}
}

// TestSanitizeOIDCGroups_EdgeCases tests edge cases
func TestSanitizeOIDCGroups_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantLen int
	}{
		{
			name:    "all empty strings",
			input:   []string{"", "", ""},
			wantLen: 0,
		},
		{
			name:    "all invalid UTF-8",
			input:   []string{"\xff\xfe", "\xfe\xff"},
			wantLen: 0,
		},
		{
			name:    "all control characters",
			input:   []string{"\x00", "\x01\x02"},
			wantLen: 0,
		},
		{
			name:    "mixed valid and invalid",
			input:   []string{"valid", "\x00", "", "also-valid", "\xff\xfe"},
			wantLen: 2,
		},
		{
			name:    "whitespace only groups",
			input:   []string{"   ", "\t\t", "valid"},
			wantLen: 2, // spaces are valid, tabs are control chars and filtered
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeOIDCGroups(tt.input)
			if len(result) != tt.wantLen {
				t.Errorf("sanitizeOIDCGroups() len = %d, want %d, result = %v", len(result), tt.wantLen, result)
			}
		})
	}
}

// TestReloadProviders_EmptySet tests that ReloadProviders with empty set clears providers
func TestReloadProviders_EmptySet(t *testing.T) {
	redisClient := NewMockRedisClient()
	config := &Config{
		OIDCEnabled: false,
	}
	authService := &mockAuthServiceForOIDC{}
	provSvc, _ := createTestOIDCProvisioningService()
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	svc, err := NewOIDCService(config, redisClient, authService, provSvc, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	// Reload with empty provider set
	err = svc.ReloadProviders(context.Background(), []OIDCProviderConfig{})
	if err != nil {
		t.Errorf("ReloadProviders() with empty set error = %v", err)
	}

	// After reload, should have zero providers
	providers := svc.ListProviders()
	if len(providers) != 0 {
		t.Errorf("ListProviders() after empty reload = %d, want 0", len(providers))
	}
}

// TestReloadProviders_NilSet tests that ReloadProviders with nil set clears providers
func TestReloadProviders_NilSet(t *testing.T) {
	redisClient := NewMockRedisClient()
	config := &Config{
		OIDCEnabled: false,
	}
	authService := &mockAuthServiceForOIDC{}
	provSvc, _ := createTestOIDCProvisioningService()
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	svc, err := NewOIDCService(config, redisClient, authService, provSvc, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	err = svc.ReloadProviders(context.Background(), nil)
	if err != nil {
		t.Errorf("ReloadProviders() with nil set error = %v", err)
	}

	providers := svc.ListProviders()
	if len(providers) != 0 {
		t.Errorf("ListProviders() after nil reload = %d, want 0", len(providers))
	}
}

// TestReloadProviders_InvalidProvider tests that invalid providers are skipped (not fatal)
func TestReloadProviders_InvalidProvider(t *testing.T) {
	redisClient := NewMockRedisClient()
	config := &Config{
		OIDCEnabled: false,
	}
	authService := &mockAuthServiceForOIDC{}
	provSvc, _ := createTestOIDCProvisioningService()
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	svc, err := NewOIDCService(config, redisClient, authService, provSvc, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	// Reload with an invalid provider (bad issuer URL — OIDC discovery will fail)
	err = svc.ReloadProviders(context.Background(), []OIDCProviderConfig{
		{
			Name:      "bad-provider",
			IssuerURL: "https://invalid.example.com/nonexistent",
			ClientID:  "id",
		},
	})
	// ReloadProviders returns an error reporting failed providers (partial failure)
	if err == nil {
		t.Error("ReloadProviders() expected error for invalid provider, got nil")
	}

	// The bad provider should not appear in the list
	providers := svc.ListProviders()
	if len(providers) != 0 {
		t.Errorf("ListProviders() = %d, want 0 (bad provider should be skipped)", len(providers))
	}
}

// TestReloadProviders_ConcurrentSafety tests that concurrent reloads and reads don't panic
func TestReloadProviders_ConcurrentSafety(t *testing.T) {
	redisClient := NewMockRedisClient()
	config := &Config{
		OIDCEnabled: false,
	}
	authService := &mockAuthServiceForOIDC{}
	provSvc, _ := createTestOIDCProvisioningService()
	casbinEnforcer, _ := rbac.NewCasbinEnforcer()

	svc, err := NewOIDCService(config, redisClient, authService, provSvc, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = svc.ListProviders()
			}
		}()
	}

	// Concurrent writers (reload with empty set — this is fast and safe)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = svc.ReloadProviders(ctx, []OIDCProviderConfig{})
			}
		}()
	}

	// Concurrent GetAuthCodeURL calls (tests RLock on providers map)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				// Will return error (no provider named "test"), but tests lock safety
				_, _ = svc.GetAuthCodeURL("test-provider", "fake-state", "fake-nonce", "fake-verifier")
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success — no panic, no deadlock
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent test timed out — possible deadlock")
	}
}

// TestDefaultHTTPClient_TransportConfig verifies that the production HTTP client
// has correct timeout values and wires http.ProxyFromEnvironment (STORY-361 AC #1).
// SSRF dialer removed per ArgoCD-aligned posture; see tech-spec
// oidc-http-client-argocd-alignment.
func TestDefaultHTTPClient_TransportConfig(t *testing.T) {
	t.Parallel()
	svc := &OIDCService{} // nil httpClient → defaultHTTPClient() will be used
	client := svc.defaultHTTPClient()

	// Same OIDCService must return the same *http.Client across calls so the
	// transport's idle-connection pool is reused across discovery / JWKS /
	// token exchange.
	if client2 := svc.defaultHTTPClient(); client2 != client {
		t.Errorf("defaultHTTPClient() returned different *http.Client across calls (%p vs %p) — connection pool not reused", client, client2)
	}

	// Verify overall request timeout (AC #1)
	if client.Timeout != OIDCClientTimeout {
		t.Errorf("client.Timeout = %v, want %v", client.Timeout, OIDCClientTimeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("client.Transport is not *http.Transport")
	}

	// Verify transport timeouts and pool sizes (AC #1)
	if transport.TLSHandshakeTimeout != OIDCTLSHandshakeTimeout {
		t.Errorf("TLSHandshakeTimeout = %v, want %v", transport.TLSHandshakeTimeout, OIDCTLSHandshakeTimeout)
	}
	if transport.IdleConnTimeout != OIDCIdleConnTimeout {
		t.Errorf("IdleConnTimeout = %v, want %v", transport.IdleConnTimeout, OIDCIdleConnTimeout)
	}
	if transport.MaxIdleConns != OIDCMaxIdleConns {
		t.Errorf("MaxIdleConns = %d, want %d", transport.MaxIdleConns, OIDCMaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != OIDCMaxIdleConnsPerHost {
		t.Errorf("MaxIdleConnsPerHost = %d, want %d", transport.MaxIdleConnsPerHost, OIDCMaxIdleConnsPerHost)
	}
	if !transport.ForceAttemptHTTP2 {
		t.Errorf("ForceAttemptHTTP2 = false, want true (matches http.DefaultTransport and other custom transports in this codebase)")
	}

	// Verify the transport.Proxy is wired to http.ProxyFromEnvironment via
	// function-pointer equality. Pointer equality is order-independent and does
	// not invoke the proxy func, so it is unaffected by net/http's process-wide
	// caching of HTTPS_PROXY (which makes t.Setenv-based behavioral assertions
	// silently ineffective).
	if transport.Proxy == nil {
		t.Fatal("transport.Proxy is nil — http.ProxyFromEnvironment not wired")
	}
	want := reflect.ValueOf(http.ProxyFromEnvironment).Pointer()
	got := reflect.ValueOf(transport.Proxy).Pointer()
	if got != want {
		t.Errorf("transport.Proxy is not http.ProxyFromEnvironment (got=%x want=%x)", got, want)
	}

	// Dial timeout is captured inside the dialer.DialContext closure and is
	// unreachable through *http.Transport. Assert directly against the package
	// helper that builds the dialer.
	if d := defaultOIDCDialer(); d.Timeout != OIDCDialTimeout {
		t.Errorf("defaultOIDCDialer().Timeout = %v, want %v", d.Timeout, OIDCDialTimeout)
	}
}

// TestInitializeProviderInto_AllowsPrivateNetwork verifies that OIDC provider
// initialization succeeds against a loopback issuer URL with the production
// HTTP client (no s.httpClient override). Locks in the ArgoCD-aligned posture
// where private/in-cluster IdPs are reachable. Inverse of the deleted
// TestInitializeProviderInto_UsesSSRFSafeDialer.
//
// HTTPS-vs-HTTP asymmetry: the mock issuer uses http://, and initializeProviderInto
// accepts any scheme — HTTPS-only enforcement lives at the API layer
// (validateProviderRequest in sso_settings.go), not at the OIDC service layer.
func TestInitializeProviderInto_AllowsPrivateNetwork(t *testing.T) {
	t.Parallel()
	discovery := minimalOIDCDiscovery(t)

	svc := &OIDCService{} // nil httpClient → production defaultHTTPClient() exercised
	target := make(map[string]*oidcProvider)
	cfg := OIDCProviderConfig{
		Name:         "loopback",
		IssuerURL:    discovery.URL, // http://127.0.0.1:NNNN
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := svc.initializeProviderInto(ctx, cfg, target); err != nil {
		t.Fatalf("initializeProviderInto() unexpected error: %v", err)
	}
	if target["loopback"] == nil {
		t.Fatal("provider not stored after loopback discovery")
	}
}

// recordingRT records every URL passed through it, then delegates to inner.
// Used by TestExchangeCodeForToken_UsesConfiguredClient to prove that token
// exchange routes through s.httpClient (and not http.DefaultClient).
type recordingRT struct {
	inner http.RoundTripper
	mu    sync.Mutex
	urls  []string
}

func (r *recordingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.urls = append(r.urls, req.URL.String())
	r.mu.Unlock()
	return r.inner.RoundTrip(req)
}

func (r *recordingRT) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.urls...)
}

// newOIDCServiceForExchangeTest constructs an OIDCService wired for the token-
// exchange unit test below. Built around the OIDCEnabled: false short-circuit
// (which skips the constructor's seed validation) plus direct assignment of the
// runtime fields ExchangeCodeForToken actually reads. Centralized so that if
// OIDCService gains a new required field, the change shows up here as a single
// edit rather than getting silently spread across an inline test setup.
func newOIDCServiceForExchangeTest(t *testing.T, transport http.RoundTripper) *OIDCService {
	t.Helper()
	cfg := &Config{OIDCEnabled: false}
	svc, err := NewOIDCService(cfg, nil, &mockAuthServiceForIntegration{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewOIDCService: %v", err)
	}
	provisioningService, _ := createTestOIDCProvisioningService()
	svc.redisClient = NewMockRedisClient()
	svc.provisioningService = provisioningService
	svc.httpClient = &http.Client{Transport: transport}
	return svc
}

// TestExchangeCodeForToken_UsesConfiguredClient proves that ExchangeCodeForToken
// routes the OAuth2 token-exchange HTTP call through s.httpClient (the configured
// client) rather than falling through to http.DefaultClient. The recorder is set
// during initialization; we snapshot the recorded URLs before Exchange runs and
// then assert that NEW entries are added post-Exchange and at least one of those
// new entries is the mock issuer's /token endpoint.
//
// The load-bearing assertion is the pre/post snapshot. A naive `len(rt.urls) > 0`
// would false-pass because initializeProviderInto already records discovery
// URLs against the same recording client.
//
// Out of scope: this test does not validate PKCE semantics. The mock /token
// handler in MockOIDCProvider does not check `code_verifier`, so the verifier
// passed here is dummy text. PKCE behavior is covered by the dedicated tests
// in oidc_integration_test.go.
func TestExchangeCodeForToken_UsesConfiguredClient(t *testing.T) {
	t.Parallel()

	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("NewMockOIDCProvider: %v", err)
	}
	defer mockProvider.Close()

	rt := &recordingRT{inner: http.DefaultTransport}
	svc := newOIDCServiceForExchangeTest(t, rt)

	// Register the mock provider — discovery runs through the recording client
	// and fills rt.urls with discovery-time URLs.
	providerCfg := OIDCProviderConfig{
		Name:         "mock",
		IssuerURL:    mockProvider.issuerURL,
		ClientID:     mockProvider.clientID,
		ClientSecret: "test-secret",
		RedirectURL:  mockProvider.redirectURI,
		Scopes:       []string{"openid", "email", "profile"},
	}
	if err := svc.initializeProviderInto(context.Background(), providerCfg, svc.providers); err != nil {
		t.Fatalf("initializeProviderInto: %v", err)
	}

	// Seed an authorization code with a matching nonce so the mock /token
	// handler returns a valid ID token.
	const (
		authCode = "test-auth-code"
		nonce    = "test-nonce"
	)
	mockProvider.AddAuthCodeWithNonce(authCode, "user@example.com", "user-sub", "User", true, nonce)

	// Snapshot pre-exchange — discovery has already added entries.
	preCount := len(rt.snapshot())
	if preCount == 0 {
		t.Fatalf("expected discovery to have recorded at least one URL, got 0")
	}

	if _, err := svc.ExchangeCodeForToken(context.Background(), "mock", authCode, nonce, "test-verifier"); err != nil {
		t.Fatalf("ExchangeCodeForToken: %v", err)
	}

	post := rt.snapshot()
	if len(post) <= preCount {
		t.Fatalf("recorder did not grow after ExchangeCodeForToken (pre=%d, post=%d) — token exchange likely fell through to http.DefaultClient", preCount, len(post))
	}

	tokenURL := mockProvider.issuerURL + "/token"
	foundToken := false
	for _, u := range post[preCount:] {
		if u == tokenURL {
			foundToken = true
			break
		}
	}
	if !foundToken {
		t.Errorf("expected token endpoint %q in post-exchange URLs, got %v", tokenURL, post[preCount:])
	}
}

// containsString checks if a slice contains a string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// minimalOIDCDiscovery starts a test OIDC discovery server on a random port and
// returns it. The server is automatically stopped via t.Cleanup.
func minimalOIDCDiscovery(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	issuer := "http://" + srv.Listener.Addr().String()
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		doc := map[string]interface{}{
			"issuer":                                issuer,
			"authorization_endpoint":                issuer + "/authorize",
			"token_endpoint":                        issuer + "/token",
			"jwks_uri":                              issuer + "/jwks",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc) //nolint:errcheck
	})
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

// failingRoundTripper fails any HTTP attempt. Used to prove that the explicit-endpoint
// bypass path makes zero HTTP calls during provider initialization.
type failingRoundTripper struct {
	t *testing.T
}

func (f *failingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	f.t.Errorf("unexpected HTTP request during explicit-endpoint bypass: %s %s", req.Method, req.URL.String())
	return nil, fmt.Errorf("HTTP forbidden in this test")
}

// TestInitializeProviderInto_ExplicitEndpoints_SkipsDiscovery verifies that when
// AuthorizationURL, TokenURL, and JWKSURL are all set, OIDC discovery is bypassed
// entirely (no HTTP request is issued). The IssuerURL is set to a host that would
// fail discovery, so a successful initialization proves the bypass path is taken.
func TestInitializeProviderInto_ExplicitEndpoints_SkipsDiscovery(t *testing.T) {
	svc := &OIDCService{
		httpClient: &http.Client{Transport: &failingRoundTripper{t: t}},
	}
	target := make(map[string]*oidcProvider)

	cfg := OIDCProviderConfig{
		Name:             "supabase-gotrue",
		IssuerURL:        "https://discovery-would-fail.invalid",
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
		RedirectURL:      "http://localhost/callback",
		AuthorizationURL: "https://example.supabase.co/auth/v1/authorize",
		TokenURL:         "https://example.supabase.co/auth/v1/token",
		JWKSURL:          "https://example.supabase.co/auth/v1/.well-known/jwks.json",
	}

	if err := svc.initializeProviderInto(context.Background(), cfg, target); err != nil {
		t.Fatalf("initializeProviderInto() unexpected error: %v", err)
	}

	p, ok := target["supabase-gotrue"]
	if !ok {
		t.Fatal("provider not stored after explicit-endpoint initialization")
	}
	if p.oauth2Config.Endpoint.AuthURL != cfg.AuthorizationURL {
		t.Errorf("oauth2Config.Endpoint.AuthURL = %q, want %q", p.oauth2Config.Endpoint.AuthURL, cfg.AuthorizationURL)
	}
	if p.oauth2Config.Endpoint.TokenURL != cfg.TokenURL {
		t.Errorf("oauth2Config.Endpoint.TokenURL = %q, want %q", p.oauth2Config.Endpoint.TokenURL, cfg.TokenURL)
	}
}

// TestInitializeProviderInto_PartialExplicitEndpoints_FallsBackToDiscovery verifies
// that when fewer than all three explicit URLs are set, the discovery path runs
// unchanged (all-or-nothing semantic). Two of three fields populated → discovery
// is invoked and the discovered authorization endpoint is used, not the partial input.
func TestInitializeProviderInto_PartialExplicitEndpoints_FallsBackToDiscovery(t *testing.T) {
	discovery := minimalOIDCDiscovery(t)

	svc := &OIDCService{httpClient: http.DefaultClient}
	target := make(map[string]*oidcProvider)

	cfg := OIDCProviderConfig{
		Name:         "partial",
		IssuerURL:    discovery.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
		// Only two of three set — bypass must NOT activate.
		AuthorizationURL: "https://ignored.example/authorize",
		TokenURL:         "https://ignored.example/token",
	}

	if err := svc.initializeProviderInto(context.Background(), cfg, target); err != nil {
		t.Fatalf("initializeProviderInto() unexpected error: %v", err)
	}

	p, ok := target["partial"]
	if !ok {
		t.Fatal("provider not stored after fallback initialization")
	}
	// Discovery ran: the resulting auth URL must come from the discovery doc, not
	// from the partial-input ignored.example value.
	wantAuthURL := discovery.URL + "/authorize"
	if p.oauth2Config.Endpoint.AuthURL != wantAuthURL {
		t.Errorf("oauth2Config.Endpoint.AuthURL = %q, want discovered %q (partial input must not be used)",
			p.oauth2Config.Endpoint.AuthURL, wantAuthURL)
	}
}

// TestInitializeProviderInto_AuthMethod verifies auth method inference and validation
// across the five canonical cases for tokenEndpointAuthMethod.
func TestInitializeProviderInto_AuthMethod(t *testing.T) {
	discovery := minimalOIDCDiscovery(t)

	tests := []struct {
		name        string
		method      string
		secret      string
		wantErr     bool
		errContains string
		wantSecret  string
	}{
		{
			name:       "explicit_none_no_secret",
			method:     "none",
			secret:     "",
			wantErr:    false,
			wantSecret: "",
		},
		{
			name:       "explicit_basic_with_secret",
			method:     "client_secret_basic",
			secret:     "my-secret",
			wantErr:    false,
			wantSecret: "my-secret",
		},
		{
			name:        "explicit_basic_empty_secret",
			method:      "client_secret_basic",
			secret:      "",
			wantErr:     true,
			errContains: "client secret cannot be empty",
		},
		{
			name:       "infer_none_from_empty_method_and_secret",
			method:     "",
			secret:     "",
			wantErr:    false,
			wantSecret: "",
		},
		{
			name:        "unsupported_method",
			method:      "private_key_jwt",
			secret:      "",
			wantErr:     true,
			errContains: "unsupported token_endpoint_auth_method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &OIDCService{httpClient: http.DefaultClient}
			target := make(map[string]*oidcProvider)
			cfg := OIDCProviderConfig{
				Name:                    "test-provider",
				IssuerURL:               discovery.URL,
				ClientID:                "test-client-id",
				ClientSecret:            tt.secret,
				RedirectURL:             "http://localhost/callback",
				TokenEndpointAuthMethod: tt.method,
			}

			err := svc.initializeProviderInto(context.Background(), cfg, target)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("initializeProviderInto() expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("initializeProviderInto() unexpected error: %v", err)
			}

			p, ok := target["test-provider"]
			if !ok {
				t.Fatal("provider not stored in target map after successful initialization")
			}
			if p.oauth2Config.ClientSecret != tt.wantSecret {
				t.Errorf("oauth2Config.ClientSecret = %q, want %q", p.oauth2Config.ClientSecret, tt.wantSecret)
			}
		})
	}
}
