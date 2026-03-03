package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/provops-org/knodex/server/internal/api/response"
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// RequestsPerMinute is the number of requests allowed per minute per IP
	RequestsPerMinute int
	// BurstSize is the burst size for the rate limiter
	BurstSize int
	// TrustedProxies is a list of trusted proxy IP addresses or CIDR ranges
	// Only requests from these IPs will have X-Forwarded-For headers trusted
	TrustedProxies []string
}

// IPRateLimiter manages rate limiters for different IP addresses
type IPRateLimiter struct {
	limiters    map[string]*rate.Limiter
	mu          sync.RWMutex
	config      RateLimitConfig
	stopCleanup chan struct{}
}

// NewIPRateLimiter creates a new IP rate limiter
func NewIPRateLimiter(config RateLimitConfig) *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters:    make(map[string]*rate.Limiter),
		config:      config,
		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.periodicCleanup()

	return rl
}

// GetLimiter returns the rate limiter for the given IP address
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	// First check with read lock for existing limiter
	i.mu.RLock()
	limiter, exists := i.limiters[ip]
	i.mu.RUnlock()

	if exists {
		return limiter
	}

	// Need to create new limiter, acquire write lock
	i.mu.Lock()
	defer i.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have created it)
	if limiter, exists := i.limiters[ip]; exists {
		return limiter
	}

	// Create a new rate limiter
	// rate.Limit is requests per second, so divide by 60 for per-minute rate
	limiter = rate.NewLimiter(rate.Limit(float64(i.config.RequestsPerMinute)/60.0), i.config.BurstSize)
	i.limiters[ip] = limiter

	return limiter
}

// periodicCleanup runs in the background and periodically removes inactive limiters
func (i *IPRateLimiter) periodicCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Snapshot current limiters while holding lock
			i.mu.Lock()
			snapshot := make(map[string]*rate.Limiter, len(i.limiters))
			for ip, limiter := range i.limiters {
				snapshot[ip] = limiter
			}
			i.mu.Unlock()

			// Check tokens outside of lock (Tokens() is thread-safe)
			toDelete := make([]string, 0)
			for ip, limiter := range snapshot {
				// Only delete if completely unused (all tokens available)
				// Add small epsilon to avoid floating point comparison issues
				if limiter.Tokens() >= float64(i.config.BurstSize)-0.001 {
					toDelete = append(toDelete, ip)
				}
			}

			// Delete identified limiters while holding lock
			if len(toDelete) > 0 {
				i.mu.Lock()
				for _, ip := range toDelete {
					// Recheck before deletion (limiter might have been used)
					if limiter, exists := i.limiters[ip]; exists {
						if limiter.Tokens() >= float64(i.config.BurstSize)-0.001 {
							delete(i.limiters, ip)
						}
					}
				}
				i.mu.Unlock()
			}
		case <-i.stopCleanup:
			return
		}
	}
}

// getClientIP extracts the client IP from the request with trusted proxy validation
func getClientIP(r *http.Request, trustedProxies []string) string {
	// Get the immediate peer (RemoteAddr)
	peerIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerIP = r.RemoteAddr
	}

	// Only trust X-Forwarded-For if the request comes from a trusted proxy
	if len(trustedProxies) > 0 && isTrustedProxy(peerIP, trustedProxies) {
		// Check X-Forwarded-For header (for proxied requests)
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			// X-Forwarded-For format: client, proxy1, proxy2, ...
			// We want the rightmost IP that is NOT a trusted proxy
			// This prevents header injection by untrusted sources
			ips := parseXForwardedFor(xff)

			// Traverse from right to left, skip trusted proxies
			for i := len(ips) - 1; i >= 0; i-- {
				ip := strings.TrimSpace(ips[i])
				parsedIP := net.ParseIP(ip)
				if parsedIP == nil {
					continue // Invalid IP format
				}
				// Skip if this is a trusted proxy
				if isTrustedProxy(ip, trustedProxies) {
					continue
				}
				// This is the first untrusted IP (real client)
				return ip
			}
		}

		// Check X-Real-IP header as fallback
		xri := r.Header.Get("X-Real-IP")
		if xri != "" {
			// Validate IP format
			if parsedIP := net.ParseIP(xri); parsedIP != nil {
				return xri
			}
		}
	}

	// Fall back to RemoteAddr (direct connection or untrusted proxy)
	return peerIP
}

// isTrustedProxy checks if the given IP is in the trusted proxies list
func isTrustedProxy(ip string, trustedProxies []string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, proxy := range trustedProxies {
		// Check if it's a CIDR range
		if strings.Contains(proxy, "/") {
			_, cidr, err := net.ParseCIDR(proxy)
			if err != nil {
				slog.Warn("invalid CIDR in trusted proxies", "cidr", proxy, "error", err)
				continue
			}
			if cidr.Contains(parsedIP) {
				return true
			}
		} else {
			// Direct IP comparison
			if ip == proxy {
				return true
			}
		}
	}

	return false
}

// parseXForwardedFor parses the X-Forwarded-For header
// Limits chain length to prevent abuse
func parseXForwardedFor(xff string) []string {
	const maxXFFChainLength = 10

	parts := strings.Split(xff, ",")
	if len(parts) > maxXFFChainLength {
		slog.Warn("X-Forwarded-For chain too long, truncating",
			"length", len(parts),
			"max", maxXFFChainLength)
		parts = parts[:maxXFFChainLength]
	}

	ips := make([]string, 0, len(parts))
	for _, ipStr := range parts {
		if trimmed := strings.TrimSpace(ipStr); trimmed != "" {
			ips = append(ips, trimmed)
		}
	}
	return ips
}

// RateLimit creates a rate limiting middleware
func RateLimit(config RateLimitConfig) func(http.Handler) http.Handler {
	limiter := NewIPRateLimiter(config)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r, config.TrustedProxies)
			limiter := limiter.GetLimiter(ip)

			if !limiter.Allow() {
				slog.Warn("rate limit exceeded",
					"ip", ip,
					"path", r.URL.Path,
					"method", r.Method,
				)
				response.WriteError(w, http.StatusTooManyRequests, response.ErrCodeRateLimit, "too many requests, please try again later", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
