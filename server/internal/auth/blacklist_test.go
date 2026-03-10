// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { client.Close() })

	return client, mr
}

func TestRedisJWTBlacklist_RevokeToken(t *testing.T) {
	t.Parallel()

	t.Run("sets Redis key with correct TTL", func(t *testing.T) {
		t.Parallel()
		client, mr := setupTestRedis(t)
		bl := newRedisJWTBlacklist(client)

		err := bl.RevokeToken(context.Background(), "test-jti-123", 30*time.Minute)
		if err != nil {
			t.Fatalf("RevokeToken() error = %v", err)
		}

		// Verify key exists
		key := jwtBlacklistPrefix + "test-jti-123"
		if !mr.Exists(key) {
			t.Fatal("expected key to exist in Redis")
		}

		// Verify TTL is set (miniredis stores TTL)
		ttl := mr.TTL(key)
		if ttl <= 0 {
			t.Fatal("expected TTL to be positive")
		}
		if ttl > 30*time.Minute {
			t.Errorf("TTL = %v, want <= 30m", ttl)
		}
	})

	t.Run("skips revocation for expired token (TTL <= 0)", func(t *testing.T) {
		t.Parallel()
		client, mr := setupTestRedis(t)
		bl := newRedisJWTBlacklist(client)

		err := bl.RevokeToken(context.Background(), "expired-jti", 0)
		if err != nil {
			t.Fatalf("RevokeToken() error = %v", err)
		}

		key := jwtBlacklistPrefix + "expired-jti"
		if mr.Exists(key) {
			t.Fatal("expected key NOT to exist for expired token")
		}
	})

	t.Run("skips revocation for negative TTL", func(t *testing.T) {
		t.Parallel()
		client, mr := setupTestRedis(t)
		bl := newRedisJWTBlacklist(client)

		err := bl.RevokeToken(context.Background(), "negative-ttl-jti", -5*time.Minute)
		if err != nil {
			t.Fatalf("RevokeToken() error = %v", err)
		}

		key := jwtBlacklistPrefix + "negative-ttl-jti"
		if mr.Exists(key) {
			t.Fatal("expected key NOT to exist for negative TTL")
		}
	})

	t.Run("skips revocation for empty jti", func(t *testing.T) {
		t.Parallel()
		client, mr := setupTestRedis(t)
		bl := newRedisJWTBlacklist(client)

		err := bl.RevokeToken(context.Background(), "", 30*time.Minute)
		if err != nil {
			t.Fatalf("RevokeToken() error = %v", err)
		}

		// Ensure no key with bare prefix was created
		key := jwtBlacklistPrefix
		if mr.Exists(key) {
			t.Fatal("expected no key for empty jti")
		}
	})
}

func TestRedisJWTBlacklist_IsRevoked(t *testing.T) {
	t.Parallel()

	t.Run("returns true for blacklisted jti", func(t *testing.T) {
		t.Parallel()
		client, _ := setupTestRedis(t)
		bl := newRedisJWTBlacklist(client)

		// Blacklist a jti
		err := bl.RevokeToken(context.Background(), "revoked-jti", 10*time.Minute)
		if err != nil {
			t.Fatalf("RevokeToken() error = %v", err)
		}

		revoked, err := bl.IsRevoked(context.Background(), "revoked-jti")
		if err != nil {
			t.Fatalf("IsRevoked() error = %v", err)
		}
		if !revoked {
			t.Fatal("expected IsRevoked() = true for blacklisted jti")
		}
	})

	t.Run("returns false for non-blacklisted jti", func(t *testing.T) {
		t.Parallel()
		client, _ := setupTestRedis(t)
		bl := newRedisJWTBlacklist(client)

		revoked, err := bl.IsRevoked(context.Background(), "not-revoked-jti")
		if err != nil {
			t.Fatalf("IsRevoked() error = %v", err)
		}
		if revoked {
			t.Fatal("expected IsRevoked() = false for non-blacklisted jti")
		}
	})

	t.Run("returns false after TTL expires", func(t *testing.T) {
		t.Parallel()
		client, mr := setupTestRedis(t)
		bl := newRedisJWTBlacklist(client)

		err := bl.RevokeToken(context.Background(), "expiring-jti", 1*time.Second)
		if err != nil {
			t.Fatalf("RevokeToken() error = %v", err)
		}

		// Fast-forward miniredis past TTL
		mr.FastForward(2 * time.Second)

		revoked, err := bl.IsRevoked(context.Background(), "expiring-jti")
		if err != nil {
			t.Fatalf("IsRevoked() error = %v", err)
		}
		if revoked {
			t.Fatal("expected IsRevoked() = false after TTL expires")
		}
	})

	t.Run("returns error when Redis unavailable", func(t *testing.T) {
		t.Parallel()
		client, mr := setupTestRedis(t)
		bl := newRedisJWTBlacklist(client)

		// Close Redis to simulate unavailability
		mr.Close()

		_, err := bl.IsRevoked(context.Background(), "any-jti")
		if err == nil {
			t.Fatal("expected error when Redis is unavailable")
		}
	})

	t.Run("returns false for empty jti", func(t *testing.T) {
		t.Parallel()
		client, _ := setupTestRedis(t)
		bl := newRedisJWTBlacklist(client)

		revoked, err := bl.IsRevoked(context.Background(), "")
		if err != nil {
			t.Fatalf("IsRevoked() error = %v", err)
		}
		if revoked {
			t.Fatal("expected IsRevoked() = false for empty jti")
		}
	})
}
