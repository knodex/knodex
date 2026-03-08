package clients

import (
	"context"
	"crypto/tls"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/config"
)

// Redis connection timeout constants.
// These values are tuned for typical datacenter latency; adjust for high-latency environments.
const (
	// redisDialTimeout is the max time to establish a TCP connection to Redis.
	// 5s allows for DNS resolution and TCP handshake in congested networks.
	redisDialTimeout = 5 * time.Second

	// redisReadTimeout is the max time to wait for a Redis response.
	// 3s is generous for typical commands; long-running commands (SCAN, etc.) may need more.
	redisReadTimeout = 3 * time.Second

	// redisWriteTimeout is the max time to send a command to Redis.
	// 3s matches read timeout for symmetry.
	redisWriteTimeout = 3 * time.Second

	// redisPingTimeout is the timeout for health check pings during connection retry.
	// 5s matches dial timeout since initial connection may be slow.
	redisPingTimeout = 5 * time.Second

	// redisRetryBaseDelay is the initial delay between connection retries.
	// 500ms with exponential backoff prevents thundering herd on Redis restarts.
	redisRetryBaseDelay = 500 * time.Millisecond

	// redisRetryMaxDelay caps the exponential backoff to prevent excessive waits.
	// 5s keeps total retry time under 30s for 10 attempts.
	redisRetryMaxDelay = 5 * time.Second

	// redisMaxRetries is the number of connection attempts before giving up.
	// 10 retries with backoff gives Redis ~30s to become available.
	redisMaxRetries = 10
)

// NewRedisClient creates a new Redis client from configuration
// Returns nil if Redis is not configured or connection fails
// Retries connection up to 10 times with exponential backoff (max 30 seconds total wait)
func NewRedisClient(cfg *config.Redis) *redis.Client {
	if cfg.Address == "" {
		slog.Info("redis not configured, skipping client initialization")
		return nil
	}

	opts := &redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		Username:     cfg.Username,
		DialTimeout:  redisDialTimeout,
		ReadTimeout:  redisReadTimeout,
		WriteTimeout: redisWriteTimeout,
		PoolSize:     10,
		MinIdleConns: 0, // Don't pre-establish connections
	}

	if cfg.TLSEnabled {
		opts.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.TLSInsecureSkipVerify, //nolint:gosec // Operator opt-in for dev/self-signed certs
		}
		slog.Info("redis TLS enabled",
			"address", cfg.Address,
			"min_tls_version", "TLS1.2",
		)
		if cfg.TLSInsecureSkipVerify {
			slog.Warn("redis TLS certificate verification disabled (REDIS_TLS_INSECURE_SKIP_VERIFY=true) — do not use in production")
		}
	}

	client := redis.NewClient(opts)

	// Retry connection with exponential backoff
	var lastErr error

	for attempt := 0; attempt < redisMaxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
		err := client.Ping(ctx).Err()
		cancel()

		if err == nil {
			slog.Info("connected to redis",
				"address", cfg.Address,
				"attempt", attempt+1,
			)
			return client
		}

		lastErr = err

		// Calculate delay with exponential backoff (capped at max delay)
		delay := redisRetryBaseDelay * time.Duration(1<<uint(attempt))
		if delay > redisRetryMaxDelay {
			delay = redisRetryMaxDelay
		}

		if attempt < redisMaxRetries-1 {
			// Sanitize error message to prevent auth info disclosure
			errMsg := "connection failed"
			if err != nil {
				errStr := err.Error()
				// Don't log authentication-related errors that could leak info
				if !strings.Contains(errStr, "AUTH") &&
					!strings.Contains(errStr, "auth") &&
					!strings.Contains(errStr, "password") &&
					!strings.Contains(errStr, "NOAUTH") {
					errMsg = errStr
				}
			}

			slog.Debug("redis connection attempt failed, retrying",
				"error", errMsg,
				"address", cfg.Address,
				"attempt", attempt+1,
				"max_retries", redisMaxRetries,
				"retry_delay_ms", delay.Milliseconds(),
			)
			time.Sleep(delay)
		}
	}

	// Sanitize final error message as well
	finalErrMsg := "connection failed after all retries"
	if lastErr != nil {
		errStr := lastErr.Error()
		if !strings.Contains(errStr, "AUTH") &&
			!strings.Contains(errStr, "auth") &&
			!strings.Contains(errStr, "password") &&
			!strings.Contains(errStr, "NOAUTH") {
			finalErrMsg = errStr
		}
	}

	slog.Warn("failed to connect to redis after retries, continuing without redis",
		"error", finalErrMsg,
		"address", cfg.Address,
		"attempts", redisMaxRetries,
	)
	client.Close()
	return nil
}

// CloseRedisClient closes the Redis client connection
func CloseRedisClient(client *redis.Client) {
	if client != nil {
		if err := client.Close(); err != nil {
			slog.Error("failed to close redis connection", "error", err)
		} else {
			slog.Info("redis connection closed")
		}
	}
}
