package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/provops-org/knodex/server/internal/testutil"
)

// setupRateLimitTestStore creates an AccountStore with a known admin password
// and a short rate limit window for testing.
func setupRateLimitTestStore(t *testing.T) *AccountStore {
	t.Helper()
	k8sClient := testutil.NewFakeClientset(t)
	namespace := "default"

	// Create ConfigMap with admin account
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: AccountConfigMapName, Namespace: namespace},
		Data: map[string]string{
			"accounts.admin":         "apiKey, login",
			"accounts.admin.enabled": "true",
		},
	}
	if _, err := k8sClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	// Create Secret with a known bcrypt hash for "Password123!"
	// Generated with bcrypt cost 12
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: AccountSecretName, Namespace: namespace},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-secret"),
		},
	}
	if _, err := k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	store := NewAccountStore(k8sClient, namespace)
	// Use a short window for testing
	store.attemptWindow = 10 * time.Second
	store.maxAttempts = 3
	// Use minimum bcrypt cost to avoid slow hashing that can cause
	// timing-dependent rate limit tests to fail on slow CI runners.
	store.bcryptCost = bcrypt.MinCost

	// Set admin password
	if err := store.LoadAccounts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPassword(context.Background(), "admin", "Password123!"); err != nil {
		t.Fatal(err)
	}

	return store
}

func TestRateLimit_IPBased_BlocksAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	store := setupRateLimitTestStore(t)
	ctx := context.Background()
	ip := "10.0.0.1"

	// Exhaust rate limit with wrong passwords
	for i := 0; i < store.maxAttempts; i++ {
		_, err := store.ValidatePassword(ctx, "admin", "wrong", ip)
		if err == nil {
			t.Fatal("expected error for wrong password")
		}
	}

	// Next attempt from same IP should be rate-limited
	_, err := store.ValidatePassword(ctx, "admin", "Password123!", ip)
	if err == nil {
		t.Fatal("expected rate limit error")
	}

	var rateLimitErr *ErrRateLimited
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected ErrRateLimited, got %T: %v", err, err)
	}
	if rateLimitErr.RetryAfter <= 0 {
		t.Errorf("RetryAfter should be positive, got %v", rateLimitErr.RetryAfter)
	}
}

func TestRateLimit_DifferentIP_NotBlocked(t *testing.T) {
	t.Parallel()

	store := setupRateLimitTestStore(t)
	ctx := context.Background()

	// Exhaust rate limit for IP1
	for i := 0; i < store.maxAttempts; i++ {
		store.ValidatePassword(ctx, "admin", "wrong", "10.0.0.1")
	}

	// IP2 should still be able to authenticate
	account, err := store.ValidatePassword(ctx, "admin", "Password123!", "10.0.0.2")
	if err != nil {
		t.Fatalf("different IP should not be rate-limited: %v", err)
	}
	if account == nil {
		t.Fatal("expected account, got nil")
	}
}

func TestRateLimit_SuccessfulLogin_ClearsAttempts(t *testing.T) {
	t.Parallel()

	store := setupRateLimitTestStore(t)
	ctx := context.Background()
	ip := "10.0.0.3"

	// Record some failed attempts (but not enough to trigger limit)
	for i := 0; i < store.maxAttempts-1; i++ {
		store.ValidatePassword(ctx, "admin", "wrong", ip)
	}

	// Successful login should clear attempts
	_, err := store.ValidatePassword(ctx, "admin", "Password123!", ip)
	if err != nil {
		t.Fatalf("expected successful login: %v", err)
	}

	// Should be able to fail again without hitting the limit immediately
	for i := 0; i < store.maxAttempts-1; i++ {
		_, err := store.ValidatePassword(ctx, "admin", "wrong", ip)
		if err == nil {
			t.Fatal("expected error for wrong password")
		}
		var rateLimitErr *ErrRateLimited
		if errors.As(err, &rateLimitErr) {
			t.Fatalf("should not be rate-limited after only %d failures post-clear", i+1)
		}
	}
}

func TestRateLimit_RetryAfterDuration(t *testing.T) {
	t.Parallel()

	store := setupRateLimitTestStore(t)
	store.attemptWindow = 5 * time.Minute
	ctx := context.Background()
	ip := "10.0.0.4"

	// Exhaust rate limit
	for i := 0; i < store.maxAttempts; i++ {
		store.ValidatePassword(ctx, "admin", "wrong", ip)
	}

	_, err := store.ValidatePassword(ctx, "admin", "Password123!", ip)
	var rateLimitErr *ErrRateLimited
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected ErrRateLimited, got %T: %v", err, err)
	}

	// RetryAfter should be approximately the window duration (within a few seconds)
	if rateLimitErr.RetryAfter < 4*time.Minute || rateLimitErr.RetryAfter > 5*time.Minute+time.Second {
		t.Errorf("RetryAfter = %v, expected ~5 minutes", rateLimitErr.RetryAfter)
	}
}

func TestErrRateLimited_ErrorString(t *testing.T) {
	t.Parallel()

	err := &ErrRateLimited{RetryAfter: 30 * time.Second}
	expected := "too many failed login attempts, try again in 30 seconds"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

// --- Redis-backed rate limiting tests ---

// setupRedisRateLimitTestStore creates an AccountStore with Redis-backed rate limiting.
func setupRedisRateLimitTestStore(t *testing.T) (*AccountStore, *miniredis.Miniredis) {
	t.Helper()
	mr, redisClient := testutil.NewRedis(t)

	k8sClient := testutil.NewFakeClientset(t)
	namespace := "default"

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: AccountConfigMapName, Namespace: namespace},
		Data: map[string]string{
			"accounts.admin":         "apiKey, login",
			"accounts.admin.enabled": "true",
		},
	}
	if _, err := k8sClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: AccountSecretName, Namespace: namespace},
		Data: map[string][]byte{
			"server.secretkey": []byte("test-secret"),
		},
	}
	if _, err := k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	store := NewAccountStoreWithRedis(k8sClient, namespace, redisClient)
	store.attemptWindow = 10 * time.Second
	store.maxAttempts = 3
	// Use minimum bcrypt cost to avoid slow hashing that can cause
	// timing-dependent rate limit tests to fail on slow CI runners.
	store.bcryptCost = bcrypt.MinCost

	if err := store.LoadAccounts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPassword(context.Background(), "admin", "Password123!"); err != nil {
		t.Fatal(err)
	}

	return store, mr
}

func TestRedisRateLimit_BlocksAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	store, _ := setupRedisRateLimitTestStore(t)
	ctx := context.Background()
	ip := "10.0.0.1"

	// Exhaust rate limit with wrong passwords
	for i := 0; i < store.maxAttempts; i++ {
		_, err := store.ValidatePassword(ctx, "admin", "wrong", ip)
		if err == nil {
			t.Fatal("expected error for wrong password")
		}
	}

	// Next attempt should be rate-limited
	_, err := store.ValidatePassword(ctx, "admin", "Password123!", ip)
	if err == nil {
		t.Fatal("expected rate limit error")
	}

	var rateLimitErr *ErrRateLimited
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected ErrRateLimited, got %T: %v", err, err)
	}
	if rateLimitErr.RetryAfter <= 0 {
		t.Errorf("RetryAfter should be positive, got %v", rateLimitErr.RetryAfter)
	}
}

func TestRedisRateLimit_DifferentIP_NotBlocked(t *testing.T) {
	t.Parallel()

	store, _ := setupRedisRateLimitTestStore(t)
	ctx := context.Background()

	// Exhaust rate limit for IP1
	for i := 0; i < store.maxAttempts; i++ {
		store.ValidatePassword(ctx, "admin", "wrong", "10.0.0.1")
	}

	// IP2 should still be able to authenticate
	account, err := store.ValidatePassword(ctx, "admin", "Password123!", "10.0.0.2")
	if err != nil {
		t.Fatalf("different IP should not be rate-limited: %v", err)
	}
	if account == nil {
		t.Fatal("expected account, got nil")
	}
}

func TestRedisRateLimit_SuccessfulLogin_ClearsAttempts(t *testing.T) {
	t.Parallel()

	store, _ := setupRedisRateLimitTestStore(t)
	ctx := context.Background()
	ip := "10.0.0.3"

	// Record some failed attempts (but not enough to trigger limit)
	for i := 0; i < store.maxAttempts-1; i++ {
		store.ValidatePassword(ctx, "admin", "wrong", ip)
	}

	// Successful login should clear attempts in Redis
	_, err := store.ValidatePassword(ctx, "admin", "Password123!", ip)
	if err != nil {
		t.Fatalf("expected successful login: %v", err)
	}

	// Should be able to fail again without hitting limit immediately
	for i := 0; i < store.maxAttempts-1; i++ {
		_, err := store.ValidatePassword(ctx, "admin", "wrong", ip)
		if err == nil {
			t.Fatal("expected error for wrong password")
		}
		var rateLimitErr *ErrRateLimited
		if errors.As(err, &rateLimitErr) {
			t.Fatalf("should not be rate-limited after only %d failures post-clear", i+1)
		}
	}
}

func TestRedisRateLimit_FallsBackToInMemoryOnRedisFailure(t *testing.T) {
	t.Parallel()

	store, mr := setupRedisRateLimitTestStore(t)
	ctx := context.Background()
	ip := "10.0.0.5"

	// Close Redis to simulate failure
	mr.Close()

	// Should still work via in-memory fallback
	for i := 0; i < store.maxAttempts; i++ {
		_, err := store.ValidatePassword(ctx, "admin", "wrong", ip)
		if err == nil {
			t.Fatal("expected error for wrong password")
		}
		// Should not be a rate limit error yet
		if i < store.maxAttempts-1 {
			var rateLimitErr *ErrRateLimited
			if errors.As(err, &rateLimitErr) {
				t.Fatalf("should not be rate-limited after only %d failures", i+1)
			}
		}
	}

	// Should be rate-limited via in-memory fallback
	_, err := store.ValidatePassword(ctx, "admin", "Password123!", ip)
	var rateLimitErr *ErrRateLimited
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected ErrRateLimited via in-memory fallback, got %T: %v", err, err)
	}
}

func TestRedisRateLimit_WindowExpiry(t *testing.T) {
	// NOT parallel: this test uses time.Sleep for real window expiry and relies
	// on Redis being responsive. Under race+parallel, context deadlines can cause
	// Redis fallback to in-memory (which has no recorded failures), producing
	// false results.

	store, _ := setupRedisRateLimitTestStore(t)
	ctx := context.Background()
	ip := "10.0.0.6"

	// Use a 2-second window. With bcrypt.MinCost (set in setup), hashing is
	// near-instant so all attempts complete well within this window.
	// Sorted set scores use real time.Now(), so we must sleep for real expiry.
	store.attemptWindow = 2 * time.Second

	// Exhaust rate limit
	for i := 0; i < store.maxAttempts; i++ {
		store.ValidatePassword(ctx, "admin", "wrong", ip)
	}

	// Should be rate-limited
	_, err := store.ValidatePassword(ctx, "admin", "Password123!", ip)
	var rateLimitErr *ErrRateLimited
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected ErrRateLimited, got %T: %v", err, err)
	}

	// Wait for the window to expire (sorted set scores are real timestamps)
	time.Sleep(3 * time.Second)

	// Should no longer be rate-limited
	account, err := store.ValidatePassword(ctx, "admin", "Password123!", ip)
	if err != nil {
		t.Fatalf("should not be rate-limited after window expiry: %v", err)
	}
	if account == nil {
		t.Fatal("expected account, got nil")
	}
}

func TestRedisRateLimit_NilClient_UsesInMemory(t *testing.T) {
	t.Parallel()

	// Create store without Redis client
	store := setupRateLimitTestStore(t) // Uses NewAccountStore (no Redis)
	ctx := context.Background()
	ip := "10.0.0.7"

	// Should work using in-memory rate limiting
	for i := 0; i < store.maxAttempts; i++ {
		_, err := store.ValidatePassword(ctx, "admin", "wrong", ip)
		if err == nil {
			t.Fatal("expected error for wrong password")
		}
	}

	_, err := store.ValidatePassword(ctx, "admin", "Password123!", ip)
	var rateLimitErr *ErrRateLimited
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected ErrRateLimited, got %T: %v", err, err)
	}
}
