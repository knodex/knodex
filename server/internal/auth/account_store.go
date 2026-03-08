// Package auth provides authentication and authorization services for knodex.
// This file implements the ArgoCD-style local user storage pattern using ConfigMap/Secret.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	utilrand "github.com/knodex/knodex/server/internal/util/rand"
)

const (
	// AccountConfigMapName is the ConfigMap containing account definitions
	AccountConfigMapName = "knodex-accounts"
	// AccountSecretName is the Secret containing credentials
	AccountSecretName = "knodex-secret"

	// DefaultMaxUsernameLength is the maximum allowed username length
	DefaultMaxUsernameLength = 32

	// AccountCapabilityAPIKey allows API key generation
	AccountCapabilityAPIKey = "apiKey"
	// AccountCapabilityLogin allows interactive login
	AccountCapabilityLogin = "login"

	// secretKeyServerSecretKey is the key for JWT signing secret in the Secret
	secretKeyServerSecretKey = "server.secretkey"

	// BcryptCostAccountStore is the bcrypt cost factor for password hashing
	BcryptCostAccountStore = 12

	// DefaultAccountCacheTTL is the default time-to-live for the account cache.
	// Reduced from 30s to 5s for faster cross-replica propagation of
	// account changes (e.g., password resets). This trades slightly more
	// K8s API calls for better multi-replica consistency (AC #4).
	DefaultAccountCacheTTL = 5 * time.Second

	// DefaultRateLimitCleanupInterval is how often to clean up stale rate limit entries
	DefaultRateLimitCleanupInterval = 10 * time.Minute

	// MaxRateLimitEntries is the maximum number of source IPs to track for rate limiting
	// This prevents memory exhaustion from brute-force attacks with unique IPs
	MaxRateLimitEntries = 10000
)

// ErrRateLimited is returned when a source IP has exceeded the maximum number
// of failed login attempts within the rate limit window.
type ErrRateLimited struct {
	// RetryAfter is the duration until the rate limit window expires
	RetryAfter time.Duration
}

func (e *ErrRateLimited) Error() string {
	return fmt.Sprintf("too many failed login attempts, try again in %d seconds", int(e.RetryAfter.Seconds()))
}

// Account represents a local user account
type Account struct {
	// Name is the username (max 32 chars)
	Name string `json:"name"`
	// Capabilities is a list of capabilities (apiKey, login)
	Capabilities []string `json:"capabilities"`
	// Enabled indicates if the account is active
	Enabled bool `json:"enabled"`
	// PasswordHash is the bcrypt hash of the password (loaded from Secret)
	PasswordHash string `json:"-"`
	// PasswordMtime is the RFC3339 timestamp of last password change
	PasswordMtime time.Time `json:"-"`
	// Tokens is the list of API tokens (loaded from Secret)
	Tokens []APIToken `json:"-"`
}

// APIToken represents an API token for an account
type APIToken struct {
	ID        string    `json:"id"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp,omitempty"`
}

// HasCapability checks if the account has a specific capability
func (a *Account) HasCapability(cap string) bool {
	for _, c := range a.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// CanLogin returns true if the account can perform interactive login
func (a *Account) CanLogin() bool {
	return a.Enabled && a.HasCapability(AccountCapabilityLogin)
}

// CanGenerateAPIKey returns true if the account can generate API keys
func (a *Account) CanGenerateAPIKey() bool {
	return a.Enabled && a.HasCapability(AccountCapabilityAPIKey)
}

// AccountStore provides operations on local user accounts
// following the ArgoCD ConfigMap/Secret pattern
type AccountStore struct {
	k8sClient kubernetes.Interface
	namespace string

	// Cache for accounts (refreshed periodically or on demand)
	mu         sync.RWMutex
	accounts   map[string]*Account
	lastLoaded time.Time     // Track when cache was last loaded
	cacheTTL   time.Duration // How long cache is valid

	// Redis client for cross-replica rate limiting (optional)
	redisClient *redis.Client

	// In-memory rate limiting fallback (used when Redis unavailable)
	failedAttempts        map[string][]time.Time
	failedAttemptsMu      sync.Mutex
	maxAttempts           int
	attemptWindow         time.Duration
	lastRateLimitCleanup  time.Time // Track last cleanup time
	rateLimitCleanupEvery time.Duration

	// bcryptCost is the bcrypt cost factor used by SetPassword.
	// Defaults to BcryptCostAccountStore (12). Tests may lower this to
	// bcrypt.MinCost to avoid slow bcrypt operations that can cause
	// timing-dependent tests to fail on slow CI runners.
	bcryptCost int
}

// rateLimitRedisKeyPrefix is the Redis key prefix for login rate limiting.
// Key pattern: ratelimit:login:{ip}
// Value type: Redis sorted set (score = Unix timestamp in seconds)
const rateLimitRedisKeyPrefix = "ratelimit:login:"

// NewAccountStore creates a new AccountStore
func NewAccountStore(k8sClient kubernetes.Interface, namespace string) *AccountStore {
	if namespace == "" {
		namespace = "default"
	}
	return &AccountStore{
		k8sClient:             k8sClient,
		namespace:             namespace,
		accounts:              make(map[string]*Account),
		cacheTTL:              DefaultAccountCacheTTL,
		failedAttempts:        make(map[string][]time.Time),
		maxAttempts:           5,               // 5 attempts per window (ArgoCD default)
		attemptWindow:         5 * time.Minute, // 300 seconds (ArgoCD default)
		rateLimitCleanupEvery: DefaultRateLimitCleanupInterval,
		bcryptCost:            BcryptCostAccountStore,
	}
}

// NewAccountStoreWithRedis creates a new AccountStore with Redis-backed rate limiting.
// When Redis is available, rate limiting is shared across all replicas.
// When Redis is unavailable, falls back to per-replica in-memory rate limiting.
func NewAccountStoreWithRedis(k8sClient kubernetes.Interface, namespace string, redisClient *redis.Client) *AccountStore {
	store := NewAccountStore(k8sClient, namespace)
	store.redisClient = redisClient
	if redisClient != nil {
		slog.Info("account store: Redis-backed rate limiting enabled for cross-replica consistency")
	}
	return store
}

// LoadAccounts loads all accounts from ConfigMap and Secret
func (s *AccountStore) LoadAccounts(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load ConfigMap for account definitions
	cm, err := s.k8sClient.CoreV1().ConfigMaps(s.namespace).Get(ctx, AccountConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap doesn't exist - create default with admin account
			if err := s.createDefaultConfigMap(ctx); err != nil {
				return fmt.Errorf("failed to create default ConfigMap: %w", err)
			}
			cm, err = s.k8sClient.CoreV1().ConfigMaps(s.namespace).Get(ctx, AccountConfigMapName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get ConfigMap after creation: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get ConfigMap: %w", err)
		}
	}

	// Load Secret for credentials
	secret, err := s.k8sClient.CoreV1().Secrets(s.namespace).Get(ctx, AccountSecretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Secret doesn't exist - create with auto-generated server secret key
			if err := s.createDefaultSecret(ctx); err != nil {
				return fmt.Errorf("failed to create default Secret: %w", err)
			}
			secret, err = s.k8sClient.CoreV1().Secrets(s.namespace).Get(ctx, AccountSecretName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get Secret after creation: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get Secret: %w", err)
		}
	}

	// Parse accounts from ConfigMap
	accounts := make(map[string]*Account)
	for key, value := range cm.Data {
		// Parse accounts.{username} entries
		if strings.HasPrefix(key, "accounts.") {
			parts := strings.Split(key, ".")
			if len(parts) < 2 {
				continue
			}
			username := parts[1]

			// Skip if username is too long
			if len(username) > DefaultMaxUsernameLength {
				slog.Warn("skipping account with username exceeding max length",
					"username_length", len(username),
					"max_length", DefaultMaxUsernameLength,
				)
				continue
			}

			// Get or create account
			account, exists := accounts[username]
			if !exists {
				account = &Account{
					Name:    username,
					Enabled: true, // Default enabled
				}
				accounts[username] = account
			}

			// Parse the value based on key suffix
			if len(parts) == 2 {
				// accounts.{username} = capabilities
				account.Capabilities = parseCapabilities(value)
			} else if len(parts) == 3 {
				// accounts.{username}.{property}
				switch parts[2] {
				case "enabled":
					account.Enabled = value == "true"
				}
			}
		}
	}

	// Load credentials from Secret using helper function for admin backward compatibility
	for username, account := range accounts {
		// Password hash
		if hash, ok := getSecretValue(secret, username, "password"); ok {
			account.PasswordHash = string(hash)
		}

		// Password mtime
		if mtime, ok := getSecretValue(secret, username, "passwordMtime"); ok {
			if t, err := time.Parse(time.RFC3339, string(mtime)); err == nil {
				account.PasswordMtime = t
			}
		}

		// Tokens
		if tokens, ok := getSecretValue(secret, username, "tokens"); ok {
			account.Tokens = parseTokens(string(tokens))
		}
	}

	s.accounts = accounts
	s.lastLoaded = time.Now() // Track cache freshness
	return nil
}

// GetAccount returns an account by username
func (s *AccountStore) GetAccount(ctx context.Context, username string) (*Account, error) {
	s.mu.RLock()
	account, exists := s.accounts[username]
	cacheStale := time.Since(s.lastLoaded) > s.cacheTTL
	s.mu.RUnlock()

	if !exists {
		// Only reload if cache TTL has expired to prevent K8s API abuse
		if cacheStale {
			if err := s.LoadAccounts(ctx); err != nil {
				return nil, fmt.Errorf("failed to reload accounts: %w", err)
			}

			s.mu.RLock()
			account, exists = s.accounts[username]
			s.mu.RUnlock()
		}

		if !exists {
			return nil, fmt.Errorf("account not found: %s", username)
		}
	}

	return account, nil
}

// ValidatePassword validates a password for an account.
// Rate limiting is keyed on sourceIP (the client's IP address) to prevent
// attackers from locking out legitimate users by spamming bad passwords.
// Returns *ErrRateLimited if the source IP has exceeded the attempt limit.
func (s *AccountStore) ValidatePassword(ctx context.Context, username, password, sourceIP string) (*Account, error) {
	// Check rate limiting by source IP
	if retryAfter, limited := s.checkRateLimit(sourceIP); limited {
		return nil, &ErrRateLimited{RetryAfter: retryAfter}
	}

	account, err := s.GetAccount(ctx, username)
	if err != nil {
		s.recordFailedAttempt(sourceIP)
		return nil, fmt.Errorf("invalid credentials")
	}

	if !account.CanLogin() {
		s.recordFailedAttempt(sourceIP)
		return nil, fmt.Errorf("account is not enabled for login")
	}

	if account.PasswordHash == "" {
		s.recordFailedAttempt(sourceIP)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password using bcrypt (timing-safe comparison)
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		s.recordFailedAttempt(sourceIP)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Clear failed attempts on successful login
	s.clearFailedAttempts(sourceIP)

	return account, nil
}

// SetPassword sets the password for an account and updates passwordMtime
func (s *AccountStore) SetPassword(ctx context.Context, username, password string) error {
	// Validate password complexity
	if err := validatePasswordComplexity(password); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Get current secret
	secret, err := s.k8sClient.CoreV1().Secrets(s.namespace).Get(ctx, AccountSecretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Determine the correct keys based on username
	var passwordKey, mtimeKey string
	if username == "admin" {
		passwordKey = "admin.password"
		mtimeKey = "admin.passwordMtime"
	} else {
		passwordKey = fmt.Sprintf("accounts.%s.password", username)
		mtimeKey = fmt.Sprintf("accounts.%s.passwordMtime", username)
	}

	// Update secret data
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[passwordKey] = hashedPassword
	secret.Data[mtimeKey] = []byte(time.Now().UTC().Format(time.RFC3339))

	// Update the secret
	_, err = s.k8sClient.CoreV1().Secrets(s.namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	// Reload accounts to refresh cache
	return s.LoadAccounts(ctx)
}

// GetServerSecretKey returns the JWT signing key from the secret.
// If the secret exists but the key is missing, it auto-generates and stores one.
// If the secret doesn't exist at all, it creates a new one with an auto-generated key.
func (s *AccountStore) GetServerSecretKey(ctx context.Context) (string, error) {
	secret, err := s.k8sClient.CoreV1().Secrets(s.namespace).Get(ctx, AccountSecretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Secret doesn't exist - create with auto-generated server secret key
			if err := s.createDefaultSecret(ctx); err != nil {
				return "", fmt.Errorf("failed to create default secret: %w", err)
			}
			// Retrieve the newly created secret
			secret, err = s.k8sClient.CoreV1().Secrets(s.namespace).Get(ctx, AccountSecretName, metav1.GetOptions{})
			if err != nil {
				return "", fmt.Errorf("failed to get secret after creation: %w", err)
			}
		} else {
			return "", fmt.Errorf("failed to get secret: %w", err)
		}
	}

	key, ok := secret.Data[secretKeyServerSecretKey]
	if !ok || len(key) == 0 {
		// Secret exists but key is missing - generate and store it
		slog.Info("server secret key missing, auto-generating...")

		// Generate random server secret key
		keyBytes, err := utilrand.GenerateRandomBytes(32)
		if err != nil {
			return "", fmt.Errorf("failed to generate server secret key: %w", err)
		}
		serverKey := base64.StdEncoding.EncodeToString(keyBytes)

		// Update the secret with the new key
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[secretKeyServerSecretKey] = []byte(serverKey)

		_, err = s.k8sClient.CoreV1().Secrets(s.namespace).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to update secret with server key: %w", err)
		}

		slog.Info("auto-generated server secret key stored in knodex-secret")
		return serverKey, nil
	}

	return string(key), nil
}

// GetPasswordMtime returns the password modification time for token invalidation
func (s *AccountStore) GetPasswordMtime(ctx context.Context, username string) (time.Time, error) {
	account, err := s.GetAccount(ctx, username)
	if err != nil {
		return time.Time{}, err
	}
	return account.PasswordMtime, nil
}

// IsTokenValid checks if a token's issued-at time is after the password mtime
// Tokens issued before a password change should be rejected
func (s *AccountStore) IsTokenValid(ctx context.Context, username string, issuedAt time.Time) (bool, error) {
	mtime, err := s.GetPasswordMtime(ctx, username)
	if err != nil {
		return false, err
	}

	// If no mtime is set, all tokens are valid
	if mtime.IsZero() {
		return true, nil
	}

	// Token is invalid only if issued strictly before the password change.
	// Tokens issued at the same second as the change are valid (covers bootstrap
	// and immediate re-login after password reset).
	return !issuedAt.Before(mtime), nil
}

// ListAccounts returns all accounts
func (s *AccountStore) ListAccounts(ctx context.Context) ([]*Account, error) {
	if err := s.LoadAccounts(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	accounts := make([]*Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// createDefaultConfigMap creates the default ConfigMap with admin account
func (s *AccountStore) createDefaultConfigMap(ctx context.Context) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountConfigMapName,
			Namespace: s.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "knodex",
				"app.kubernetes.io/component": "auth",
			},
		},
		Data: map[string]string{
			"accounts.admin":         "apiKey, login",
			"accounts.admin.enabled": "true",
		},
	}

	_, err := s.k8sClient.CoreV1().ConfigMaps(s.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// createDefaultSecret creates the default Secret with auto-generated server key
func (s *AccountStore) createDefaultSecret(ctx context.Context) error {
	// Generate random server secret key
	keyBytes, err := utilrand.GenerateRandomBytes(32)
	if err != nil {
		return fmt.Errorf("failed to generate server secret key: %w", err)
	}
	serverKey := base64.StdEncoding.EncodeToString(keyBytes)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AccountSecretName,
			Namespace: s.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "knodex",
				"app.kubernetes.io/component": "auth",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			secretKeyServerSecretKey: []byte(serverKey),
			// Admin password will be set via bootstrap or environment variable
		},
	}

	_, err = s.k8sClient.CoreV1().Secrets(s.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// Rate limiting helpers

// checkRateLimit checks whether the given key (source IP) is rate-limited.
// Uses Redis when available for cross-replica consistency, falls back to in-memory.
// Returns (retryAfter, true) if rate-limited, or (0, false) if allowed.
func (s *AccountStore) checkRateLimit(key string) (time.Duration, bool) {
	if s.redisClient != nil {
		retryAfter, limited, err := s.checkRateLimitRedis(key)
		if err == nil {
			return retryAfter, limited
		}
		// Redis error, fall through to in-memory
	}
	return s.checkRateLimitInMemory(key)
}

// checkRateLimitRedis checks rate limiting using Redis sorted sets.
// Scores use millisecond precision to avoid boundary issues with short windows.
// All operations are wrapped in a MULTI/EXEC transaction for atomicity across replicas.
// Returns (retryAfter, limited, err). Non-nil err means Redis is unavailable.
func (s *AccountStore) checkRateLimitRedis(key string) (time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	redisKey := rateLimitRedisKeyPrefix + key
	now := time.Now()
	windowStart := now.Add(-s.attemptWindow)

	// Execute cleanup + count + oldest-entry lookup atomically via MULTI/EXEC
	pipe := s.redisClient.TxPipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "-inf", fmt.Sprintf("%d", windowStart.UnixMilli()))
	cardCmd := pipe.ZCard(ctx, redisKey)
	rangeCmd := pipe.ZRangeWithScores(ctx, redisKey, 0, 0)

	_, err := pipe.Exec(ctx)
	if err != nil {
		slog.Warn("rate limit: Redis pipeline failed, falling back to in-memory", "error", err)
		return 0, false, err
	}

	count := cardCmd.Val()
	if count < int64(s.maxAttempts) {
		return 0, false, nil
	}

	// Rate-limited: compute retryAfter from the oldest attempt
	oldestEntries := rangeCmd.Val()
	if len(oldestEntries) == 0 {
		return s.attemptWindow, true, nil // Conservative: return full window
	}

	oldestTimestamp := time.UnixMilli(int64(oldestEntries[0].Score))
	retryAfter := oldestTimestamp.Add(s.attemptWindow).Sub(now)
	if retryAfter < 0 {
		retryAfter = 0
	}
	return retryAfter, true, nil
}

// checkRateLimitInMemory is the in-memory fallback for rate limiting.
func (s *AccountStore) checkRateLimitInMemory(key string) (time.Duration, bool) {
	s.failedAttemptsMu.Lock()
	defer s.failedAttemptsMu.Unlock()

	now := time.Now()
	windowStart := now.Add(-s.attemptWindow)

	// Periodically cleanup stale entries to prevent memory exhaustion
	s.cleanupStaleRateLimitEntriesLocked(now)

	// Get attempts for this key
	attempts, exists := s.failedAttempts[key]
	if !exists {
		return 0, false
	}

	// Filter to only recent attempts within the window
	recentAttempts := make([]time.Time, 0)
	for _, attempt := range attempts {
		if attempt.After(windowStart) {
			recentAttempts = append(recentAttempts, attempt)
		}
	}
	s.failedAttempts[key] = recentAttempts

	// Check if under limit
	if len(recentAttempts) < s.maxAttempts {
		return 0, false
	}

	// Rate-limited: compute how long until the oldest attempt falls out of the window
	oldest := recentAttempts[0]
	retryAfter := oldest.Add(s.attemptWindow).Sub(now)
	if retryAfter < 0 {
		retryAfter = 0
	}
	return retryAfter, true
}

// cleanupStaleRateLimitEntriesLocked removes expired rate limit entries.
// Must be called with failedAttemptsMu held.
func (s *AccountStore) cleanupStaleRateLimitEntriesLocked(now time.Time) {
	// Only cleanup periodically or when map is too large
	shouldCleanup := time.Since(s.lastRateLimitCleanup) > s.rateLimitCleanupEvery ||
		len(s.failedAttempts) > MaxRateLimitEntries

	if !shouldCleanup {
		return
	}

	windowStart := now.Add(-s.attemptWindow)
	toDelete := make([]string, 0)

	for key, attempts := range s.failedAttempts {
		// Check if all attempts are stale
		hasRecent := false
		for _, attempt := range attempts {
			if attempt.After(windowStart) {
				hasRecent = true
				break
			}
		}
		if !hasRecent {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		delete(s.failedAttempts, key)
	}

	s.lastRateLimitCleanup = now
	if len(toDelete) > 0 {
		slog.Debug("cleaned up stale rate limit entries", "count", len(toDelete))
	}
}

// recordFailedAttempt records a failed login attempt.
// Uses Redis when available for cross-replica consistency.
func (s *AccountStore) recordFailedAttempt(key string) {
	if s.redisClient != nil {
		if s.recordFailedAttemptRedis(key) {
			return
		}
		// Redis error, fall through to in-memory
	}
	s.recordFailedAttemptInMemory(key)
}

// recordFailedAttemptRedis records a failed attempt in Redis sorted set.
// Scores use millisecond precision to match checkRateLimitRedis.
// Returns true on success, false on Redis error.
func (s *AccountStore) recordFailedAttemptRedis(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	redisKey := rateLimitRedisKeyPrefix + key
	now := time.Now()

	// Use a pipeline to add the attempt and set TTL atomically
	pipe := s.redisClient.TxPipeline()

	// Add attempt with score = Unix millisecond timestamp, member = unique string.
	// Random suffix prevents collisions when multiple replicas record at the same nanosecond.
	rndSuffix := utilrand.GenerateRandomHex(4)
	pipe.ZAdd(ctx, redisKey, redis.Z{
		Score:  float64(now.UnixMilli()),
		Member: fmt.Sprintf("%d:%s", now.UnixNano(), rndSuffix),
	})

	// Set key expiry to the attempt window (auto-cleanup)
	pipe.Expire(ctx, redisKey, s.attemptWindow+time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		slog.Warn("rate limit: Redis recordFailedAttempt failed, falling back to in-memory", "error", err)
		return false
	}
	return true
}

func (s *AccountStore) recordFailedAttemptInMemory(key string) {
	s.failedAttemptsMu.Lock()
	defer s.failedAttemptsMu.Unlock()

	s.failedAttempts[key] = append(s.failedAttempts[key], time.Now())
}

// clearFailedAttempts clears failed login attempts on successful login.
// Uses Redis when available for cross-replica consistency.
func (s *AccountStore) clearFailedAttempts(key string) {
	if s.redisClient != nil {
		if s.clearFailedAttemptsRedis(key) {
			return
		}
		// Redis error, fall through to in-memory
	}
	s.clearFailedAttemptsInMemory(key)
}

// clearFailedAttemptsRedis removes the rate limit key from Redis.
// Returns true on success, false on Redis error.
func (s *AccountStore) clearFailedAttemptsRedis(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	redisKey := rateLimitRedisKeyPrefix + key
	err := s.redisClient.Del(ctx, redisKey).Err()
	if err != nil {
		slog.Warn("rate limit: Redis clearFailedAttempts failed", "error", err)
		return false
	}
	return true
}

func (s *AccountStore) clearFailedAttemptsInMemory(key string) {
	s.failedAttemptsMu.Lock()
	defer s.failedAttemptsMu.Unlock()

	delete(s.failedAttempts, key)
}

// Helper functions

// getSecretValue retrieves a value from the secret with admin backward compatibility.
// For the admin user, it first tries the short key format (e.g., "admin.password")
// then falls back to the standard format (e.g., "accounts.admin.password").
// For other users, it only uses the standard format.
func getSecretValue(secret *corev1.Secret, username, suffix string) ([]byte, bool) {
	standardKey := fmt.Sprintf("accounts.%s.%s", username, suffix)

	if username == "admin" {
		// Admin uses short key for backward compatibility
		shortKey := fmt.Sprintf("admin.%s", suffix)
		if value, ok := secret.Data[shortKey]; ok {
			return value, true
		}
	}

	// Standard key format for all users
	if value, ok := secret.Data[standardKey]; ok {
		return value, true
	}

	return nil, false
}

func parseCapabilities(value string) []string {
	// Parse comma-separated capabilities: "apiKey, login"
	parts := strings.Split(value, ",")
	capabilities := make([]string, 0, len(parts))
	for _, part := range parts {
		cap := strings.TrimSpace(part)
		if cap != "" {
			capabilities = append(capabilities, cap)
		}
	}
	return capabilities
}

func parseTokens(value string) []APIToken {
	if value == "" {
		return nil
	}

	var tokens []APIToken
	if err := json.Unmarshal([]byte(value), &tokens); err != nil {
		slog.Warn("failed to parse API tokens", "error", err)
		return nil
	}
	return tokens
}
