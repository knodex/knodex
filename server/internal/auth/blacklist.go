package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const jwtBlacklistPrefix = "jwt:blacklist:"

// JWTBlacklistInterface defines the server-side token revocation contract
type JWTBlacklistInterface interface {
	RevokeToken(ctx context.Context, jti string, remainingTTL time.Duration) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

type redisJWTBlacklist struct {
	client *redis.Client
}

func newRedisJWTBlacklist(client *redis.Client) JWTBlacklistInterface {
	return &redisJWTBlacklist{client: client}
}

func (b *redisJWTBlacklist) RevokeToken(ctx context.Context, jti string, remainingTTL time.Duration) error {
	if jti == "" {
		return nil // no jti to blacklist
	}
	if remainingTTL <= 0 {
		return nil // already expired, no need to blacklist
	}
	key := jwtBlacklistPrefix + jti
	return b.client.Set(ctx, key, "1", remainingTTL).Err()
}

func (b *redisJWTBlacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil // no jti means token predates blacklist support
	}
	key := jwtBlacklistPrefix + jti
	result, err := b.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis blacklist check failed: %w", err)
	}
	return result > 0, nil
}
