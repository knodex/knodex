package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserRateLimit_AuthenticatedUser(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 60, // 1 per second
		BurstSize:         1,
		FallbackToIP:      false,
	}

	middleware := UserRateLimit(config)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// First request should succeed
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	userCtx := &UserContext{
		UserID: "user-123",
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w.Code)
	}

	// Second immediate request should be rate limited
	req2 := httptest.NewRequest("GET", "/api/v1/test", nil)
	ctx2 := context.WithValue(req2.Context(), UserContextKey, userCtx)
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected status 429, got %d", w2.Code)
	}
}

func TestUserRateLimit_DifferentUsers(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		FallbackToIP:      false,
	}

	middleware := UserRateLimit(config)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// User 1 makes a request
	req1 := httptest.NewRequest("GET", "/api/v1/test", nil)
	userCtx1 := &UserContext{
		UserID: "user-123",
	}
	ctx1 := context.WithValue(req1.Context(), UserContextKey, userCtx1)
	req1 = req1.WithContext(ctx1)
	w1 := httptest.NewRecorder()

	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("user 1: expected status 200, got %d", w1.Code)
	}

	// User 2 makes a request (different user, should not be rate limited)
	req2 := httptest.NewRequest("GET", "/api/v1/test", nil)
	userCtx2 := &UserContext{
		UserID: "user-456",
	}
	ctx2 := context.WithValue(req2.Context(), UserContextKey, userCtx2)
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("user 2: expected status 200, got %d", w2.Code)
	}
}

func TestUserRateLimit_UnauthenticatedNoFallback(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		FallbackToIP:      false,
	}

	middleware := UserRateLimit(config)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Unauthenticated request with no fallback should pass through
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestUserRateLimit_UnauthenticatedWithFallback(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		FallbackToIP:      true,
	}

	middleware := UserRateLimit(config)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// First unauthenticated request should succeed
	req1 := httptest.NewRequest("GET", "/api/v1/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()

	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Second immediate request from same IP should be rate limited
	req2 := httptest.NewRequest("GET", "/api/v1/test", nil)
	req2.RemoteAddr = "192.168.1.1:54321"
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected status 429, got %d", w2.Code)
	}
}

func TestUserRateLimit_BurstAllowance(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         3, // Allow burst of 3 requests
		FallbackToIP:      false,
	}

	middleware := UserRateLimit(config)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	userCtx := &UserContext{
		UserID: "user-123",
	}

	// First 3 requests should succeed (within burst)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// 4th request should be rate limited
	req4 := httptest.NewRequest("GET", "/api/v1/test", nil)
	ctx4 := context.WithValue(req4.Context(), UserContextKey, userCtx)
	req4 = req4.WithContext(ctx4)
	w4 := httptest.NewRecorder()

	handler.ServeHTTP(w4, req4)

	if w4.Code != http.StatusTooManyRequests {
		t.Errorf("4th request: expected status 429, got %d", w4.Code)
	}
}

func TestUserRateLimiter_GetLimiter(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		FallbackToIP:      false,
	}

	limiter := NewUserRateLimiter(config)

	// Get limiter for user 1
	limiter1 := limiter.GetLimiter("user-123")
	if limiter1 == nil {
		t.Error("expected non-nil limiter for user-123")
	}

	// Get same limiter again
	limiter1Again := limiter.GetLimiter("user-123")
	if limiter1 != limiter1Again {
		t.Error("expected same limiter instance for same user")
	}

	// Get limiter for different user
	limiter2 := limiter.GetLimiter("user-456")
	if limiter2 == nil {
		t.Error("expected non-nil limiter for user-456")
	}
	if limiter1 == limiter2 {
		t.Error("expected different limiter instances for different users")
	}
}

func TestUserRateLimiter_Cleanup(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         5,
		FallbackToIP:      false,
	}

	limiter := NewUserRateLimiter(config)
	defer limiter.Stop() // Clean up background goroutine

	// Create limiters for multiple users
	limiter.GetLimiter("user-1")
	limiter.GetLimiter("user-2")
	limiter.GetLimiter("user-3")

	// Check that limiters were created
	limiter.mu.RLock()
	count := len(limiter.limiters)
	limiter.mu.RUnlock()

	if count != 3 {
		t.Errorf("expected 3 limiters, got %d", count)
	}

	// Note: Background cleanup runs every 5 minutes automatically
	// We can't easily test the timing-based cleanup in a unit test
	// The important thing is that the limiters are created and work correctly
	// Manual cleanup testing would require waiting 5+ minutes which is impractical for unit tests

	// Verify that we can still get limiters after creation
	l4 := limiter.GetLimiter("user-4")
	if l4 == nil {
		t.Error("expected to get limiter for user-4")
	}

	// Verify count increased
	limiter.mu.RLock()
	newCount := len(limiter.limiters)
	limiter.mu.RUnlock()

	if newCount != 4 {
		t.Errorf("expected 4 limiters, got %d", newCount)
	}
}

func TestUserRateLimit_RateLimitErrorResponse(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		FallbackToIP:      false,
	}

	middleware := UserRateLimit(config)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	userCtx := &UserContext{
		UserID: "user-123",
	}

	// First request to consume the token
	req1 := httptest.NewRequest("GET", "/api/v1/test", nil)
	ctx1 := context.WithValue(req1.Context(), UserContextKey, userCtx)
	req1 = req1.WithContext(ctx1)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	// Second request should be rate limited
	req2 := httptest.NewRequest("GET", "/api/v1/test", nil)
	ctx2 := context.WithValue(req2.Context(), UserContextKey, userCtx)
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w2.Code)
	}

	// Check response body contains error
	body := w2.Body.String()
	if body == "" {
		t.Error("expected non-empty error response body")
	}
}

func TestUserRateLimit_RetryAfterHeader(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		FallbackToIP:      false,
	}

	mw := UserRateLimit(config)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(testHandler)

	userCtx := &UserContext{
		UserID: "user-retry",
	}

	// First request to consume the token
	req1 := httptest.NewRequest("GET", "/api/v1/test", nil)
	ctx1 := context.WithValue(req1.Context(), UserContextKey, userCtx)
	req1 = req1.WithContext(ctx1)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	// Second request should be rate limited with Retry-After header
	req2 := httptest.NewRequest("GET", "/api/v1/test", nil)
	ctx2 := context.WithValue(req2.Context(), UserContextKey, userCtx)
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w2.Code)
	}

	retryAfter := w2.Header().Get("Retry-After")
	if retryAfter != "60" {
		t.Errorf("expected Retry-After header '60', got %q", retryAfter)
	}
}

func TestUserRateLimit_SSOBurstOf5(t *testing.T) {
	t.Parallel()

	// Simulate SSO mutation rate limit: 1 req/min sustained, burst of 5
	config := UserRateLimitConfig{
		RequestsPerMinute: 1,
		BurstSize:         5,
		FallbackToIP:      false,
	}

	mw := UserRateLimit(config)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(testHandler)

	userCtx := &UserContext{
		UserID: "admin-sso",
	}

	// First 5 requests should succeed (within burst)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/v1/settings/sso/providers", nil)
		ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// 6th request should be rate limited
	req6 := httptest.NewRequest("POST", "/api/v1/settings/sso/providers", nil)
	ctx6 := context.WithValue(req6.Context(), UserContextKey, userCtx)
	req6 = req6.WithContext(ctx6)
	w6 := httptest.NewRecorder()
	handler.ServeHTTP(w6, req6)

	if w6.Code != http.StatusTooManyRequests {
		t.Errorf("6th request: expected status 429, got %d", w6.Code)
	}

	retryAfter := w6.Header().Get("Retry-After")
	if retryAfter != "60" {
		t.Errorf("expected Retry-After header '60', got %q", retryAfter)
	}
}

func TestUserRateLimit_SSOPerUserIndependent(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 1,
		BurstSize:         5,
		FallbackToIP:      false,
	}

	mw := UserRateLimit(config)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(testHandler)

	// User 1 makes 5 requests (exhausts burst)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/v1/settings/sso/providers", nil)
		userCtx := &UserContext{UserID: "admin-1"}
		ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("user1 request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// User 2 should still be able to make 5 requests independently
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/v1/settings/sso/providers", nil)
		userCtx := &UserContext{UserID: "admin-2"}
		ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("user2 request %d: expected status 200, got %d", i+1, w.Code)
		}
	}
}

func TestNewUserRateLimiter_WithIPFallback(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         5,
		FallbackToIP:      true,
	}

	limiter := NewUserRateLimiter(config)

	if limiter.ipLimiter == nil {
		t.Error("expected ipLimiter to be initialized when FallbackToIP is true")
	}

	if len(limiter.limiters) != 0 {
		t.Error("expected limiters map to be empty initially")
	}

	if limiter.config.RequestsPerMinute != 100 {
		t.Errorf("expected RequestsPerMinute 100, got %d", limiter.config.RequestsPerMinute)
	}
}

func TestNewUserRateLimiter_WithoutIPFallback(t *testing.T) {
	t.Parallel()

	config := UserRateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         5,
		FallbackToIP:      false,
	}

	limiter := NewUserRateLimiter(config)

	if limiter.ipLimiter != nil {
		t.Error("expected ipLimiter to be nil when FallbackToIP is false")
	}
}
