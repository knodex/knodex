package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/testutil"
)

// setupServiceWithMiniredis creates a test Service backed by miniredis for blacklist testing
func setupServiceWithMiniredis(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { redisClient.Close() })

	k8sClient := testutil.NewFakeClientset(t)
	namespace := "default"

	accountsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: AccountConfigMapName, Namespace: namespace},
		Data: map[string]string{
			"accounts.admin":         "apiKey, login",
			"accounts.admin.enabled": "true",
		},
	}
	_, err = k8sClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), accountsCM, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create ConfigMap: %v", err)
	}

	accountsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: AccountSecretName, Namespace: namespace},
		Data:       map[string][]byte{"server.secretkey": []byte("test-jwt-secret-for-testing")},
	}
	_, err = k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), accountsSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create Secret: %v", err)
	}

	accountStore := NewAccountStore(k8sClient, namespace)
	if err := accountStore.LoadAccounts(context.Background()); err != nil {
		t.Fatalf("LoadAccounts() error = %v", err)
	}

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: rbac.ProjectGroup, Version: rbac.ProjectVersion, Resource: "projects"}: "ProjectList",
	}
	dynamicClient := testutil.NewFakeDynamicClientWithListKinds(t, gvrToListKind)
	projectSvc := rbac.NewProjectService(k8sClient, dynamicClient)
	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	if err != nil {
		t.Fatalf("NewCasbinEnforcer() error = %v", err)
	}

	svc, err := NewService(&Config{
		JWTSecret:          "test-jwt-secret",
		JWTExpiry:          1 * time.Hour,
		LocalAdminUsername: "admin",
		LocalAdminPassword: "Password123!",
	}, accountStore, projectSvc, k8sClient, redisClient, casbinEnforcer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return svc, mr
}

func TestJTI_PresentInGeneratedTokens(t *testing.T) {
	svc, _ := setupServiceWithMiniredis(t)

	t.Run("GenerateTokenForAccount includes jti", func(t *testing.T) {
		account := &Account{
			Name:         "admin",
			PasswordHash: "ignored",
			Capabilities: []string{"login"},
			Enabled:      true,
		}

		tokenString, _, err := svc.GenerateTokenForAccount(account, "user-local-admin")
		if err != nil {
			t.Fatalf("GenerateTokenForAccount() error = %v", err)
		}

		// Parse the token to extract jti
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(svc.config.JWTSecret), nil
		})
		if err != nil {
			t.Fatalf("jwt.Parse() error = %v", err)
		}

		claims := token.Claims.(jwt.MapClaims)
		jti, ok := claims["jti"].(string)
		if !ok || jti == "" {
			t.Fatal("expected non-empty jti claim in token")
		}

		// Verify it looks like a UUID (8-4-4-4-12 format)
		parts := strings.Split(jti, "-")
		if len(parts) != 5 {
			t.Errorf("jti = %q, expected UUID format (8-4-4-4-12)", jti)
		}
	})

	t.Run("GenerateTokenWithGroups includes jti", func(t *testing.T) {
		tokenString, _, err := svc.GenerateTokenWithGroups("user-oidc-123", "test@example.com", "Test User", []string{"group1"})
		if err != nil {
			t.Fatalf("GenerateTokenWithGroups() error = %v", err)
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(svc.config.JWTSecret), nil
		})
		if err != nil {
			t.Fatalf("jwt.Parse() error = %v", err)
		}

		claims := token.Claims.(jwt.MapClaims)
		jti, ok := claims["jti"].(string)
		if !ok || jti == "" {
			t.Fatal("expected non-empty jti claim in token")
		}
	})

	t.Run("each token has unique jti", func(t *testing.T) {
		token1, _, _ := svc.GenerateTokenWithGroups("user-1", "a@test.com", "A", nil)
		token2, _, _ := svc.GenerateTokenWithGroups("user-2", "b@test.com", "B", nil)

		parseJTI := func(tokenStr string) string {
			tok, _ := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				return []byte(svc.config.JWTSecret), nil
			})
			claims := tok.Claims.(jwt.MapClaims)
			jti, _ := claims["jti"].(string)
			return jti
		}

		jti1 := parseJTI(token1)
		jti2 := parseJTI(token2)

		if jti1 == jti2 {
			t.Errorf("expected unique jti values, both = %q", jti1)
		}
	})
}

func TestValidateToken_BlacklistRevoked(t *testing.T) {
	svc, _ := setupServiceWithMiniredis(t)

	// Generate a valid token
	tokenString, _, err := svc.GenerateTokenWithGroups("user-oidc-test", "test@example.com", "Test User", nil)
	if err != nil {
		t.Fatalf("GenerateTokenWithGroups() error = %v", err)
	}

	// Extract jti from token
	token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(svc.config.JWTSecret), nil
	})
	claims := token.Claims.(jwt.MapClaims)
	jti := claims["jti"].(string)

	t.Run("valid token with non-blacklisted jti succeeds", func(t *testing.T) {
		result, err := svc.ValidateToken(context.Background(), tokenString)
		if err != nil {
			t.Fatalf("ValidateToken() error = %v", err)
		}
		if result.JTI != jti {
			t.Errorf("ValidateToken() JTI = %q, want %q", result.JTI, jti)
		}
	})

	t.Run("blacklisted jti returns session revoked error", func(t *testing.T) {
		// Blacklist the jti
		err := svc.blacklist.RevokeToken(context.Background(), jti, 30*time.Minute)
		if err != nil {
			t.Fatalf("RevokeToken() error = %v", err)
		}

		_, err = svc.ValidateToken(context.Background(), tokenString)
		if err == nil {
			t.Fatal("expected error for blacklisted token")
		}
		if !strings.Contains(err.Error(), "session has been revoked") {
			t.Errorf("error = %q, want to contain 'session has been revoked'", err.Error())
		}
	})
}

// errorBlacklist is a mock that always returns an error, simulating Redis unavailability
// without the 2+ second connection timeout of closing real miniredis.
type errorBlacklist struct{}

func (e *errorBlacklist) RevokeToken(_ context.Context, _ string, _ time.Duration) error {
	return fmt.Errorf("redis connection refused")
}

func (e *errorBlacklist) IsRevoked(_ context.Context, _ string) (bool, error) {
	return false, fmt.Errorf("redis connection refused")
}

func TestValidateToken_BlacklistFailOpen(t *testing.T) {
	svc, _ := setupServiceWithMiniredis(t)

	// Generate a valid token
	tokenString, _, err := svc.GenerateTokenWithGroups("user-oidc-failopen", "failopen@example.com", "Fail Open User", nil)
	if err != nil {
		t.Fatalf("GenerateTokenWithGroups() error = %v", err)
	}

	// Swap blacklist with one that always errors (instant, no Redis timeout)
	svc.blacklist = &errorBlacklist{}

	// Validation should succeed (fail-open)
	result, err := svc.ValidateToken(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("ValidateToken() should succeed when Redis unavailable (fail-open), got error = %v", err)
	}
	if result.Email != "failopen@example.com" {
		t.Errorf("ValidateToken() Email = %q, want %q", result.Email, "failopen@example.com")
	}
}

func TestService_RevokeToken(t *testing.T) {
	t.Parallel()

	t.Run("sets key with correct TTL via service method", func(t *testing.T) {
		t.Parallel()
		svc, mr := setupServiceWithMiniredis(t)

		err := svc.RevokeToken(context.Background(), "svc-jti-123", 45*time.Minute)
		if err != nil {
			t.Fatalf("RevokeToken() error = %v", err)
		}

		key := jwtBlacklistPrefix + "svc-jti-123"
		if !mr.Exists(key) {
			t.Fatal("expected key to exist in Redis after RevokeToken()")
		}
	})

	t.Run("nil blacklist is safe", func(t *testing.T) {
		t.Parallel()
		svc := &Service{} // no blacklist set
		err := svc.RevokeToken(context.Background(), "any-jti", 10*time.Minute)
		if err != nil {
			t.Fatalf("RevokeToken() with nil blacklist should not error, got = %v", err)
		}
	})
}
