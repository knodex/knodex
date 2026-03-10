// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// redisRoleKeyPrefix is the Redis key prefix for persisted user-role assignments.
	// Key pattern: casbin:user_roles:{userID}
	// Value type: Redis SET of role strings
	redisRoleKeyPrefix = "casbin:user_roles:"

	// redisRoleScanCount is the batch size for SCAN operations when loading all user roles.
	redisRoleScanCount = 100
)

// RedisRoleStore persists user-to-role grouping policies (Casbin g policies) to Redis.
// This is a side-channel persistence layer — it does NOT replace Casbin's in-memory adapter.
// Only user-role assignments are persisted; built-in and project policies are not.
//
// Redis key design:
//
//	Key:   casbin:user_roles:{userID}
//	Type:  SET (set of role strings)
//	TTL:   Configurable (default 24h)
//
// Thread safety: All methods are safe for concurrent use (Redis operations are atomic).
type RedisRoleStore struct {
	client *redis.Client
	ttl    time.Duration
	logger *slog.Logger
}

// NewRedisRoleStore creates a new RedisRoleStore.
// If client is nil, all operations gracefully degrade (return empty/nil, log warning).
func NewRedisRoleStore(client *redis.Client, ttl time.Duration, logger *slog.Logger) *RedisRoleStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisRoleStore{
		client: client,
		ttl:    ttl,
		logger: logger,
	}
}

// SaveUserRoles persists a user's roles to Redis, replacing any existing roles.
// The key is set with a TTL so roles expire automatically.
// If Redis is unavailable, logs a warning and returns nil (graceful degradation).
func (s *RedisRoleStore) SaveUserRoles(ctx context.Context, userID string, roles []string) error {
	if s.client == nil {
		s.logger.Warn("redis role store: skipping save, redis client is nil",
			"user", userID,
		)
		return nil
	}

	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	key := redisRoleKeyPrefix + userID

	// Use a transaction pipeline (MULTI/EXEC) to atomically replace the role set.
	// TxPipeline ensures DEL + SADD + EXPIRE execute as a single atomic operation,
	// preventing partial state if the process crashes mid-operation.
	pipe := s.client.TxPipeline()

	// Delete existing key (clears old roles)
	pipe.Del(ctx, key)

	// Add new roles (if any)
	if len(roles) > 0 {
		members := make([]interface{}, len(roles))
		for i, r := range roles {
			members[i] = r
		}
		pipe.SAdd(ctx, key, members...)
	}

	// Set TTL
	if len(roles) > 0 {
		pipe.Expire(ctx, key, s.ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		s.logger.Warn("redis role store: failed to save user roles",
			"user", userID,
			"roles_count", len(roles),
			"error", err,
		)
		return nil // Graceful degradation
	}

	s.logger.Debug("redis role store: saved user roles",
		"user", userID,
		"roles_count", len(roles),
		"ttl", s.ttl,
	)
	return nil
}

// LoadUserRoles retrieves a user's persisted roles from Redis.
// Returns nil (not error) if Redis is unavailable or user has no persisted roles.
func (s *RedisRoleStore) LoadUserRoles(ctx context.Context, userID string) ([]string, error) {
	if s.client == nil {
		s.logger.Warn("redis role store: skipping load, redis client is nil",
			"user", userID,
		)
		return nil, nil
	}

	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}

	key := redisRoleKeyPrefix + userID

	roles, err := s.client.SMembers(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No roles persisted
		}
		s.logger.Warn("redis role store: failed to load user roles",
			"user", userID,
			"error", err,
		)
		return nil, nil // Graceful degradation
	}

	return roles, nil
}

// LoadAllUserRoles retrieves all persisted user-role assignments from Redis.
// Uses SCAN to iterate over all casbin:user_roles:* keys.
// Returns an empty map (not error) if Redis is unavailable.
func (s *RedisRoleStore) LoadAllUserRoles(ctx context.Context) (map[string][]string, error) {
	if s.client == nil {
		s.logger.Warn("redis role store: skipping load all, redis client is nil")
		return nil, nil
	}

	result := make(map[string][]string)
	pattern := redisRoleKeyPrefix + "*"
	var cursor uint64

	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, pattern, redisRoleScanCount).Result()
		if err != nil {
			s.logger.Warn("redis role store: failed to scan user role keys",
				"error", err,
			)
			return nil, nil // Graceful degradation
		}

		for _, key := range keys {
			// Extract userID from key
			userID := strings.TrimPrefix(key, redisRoleKeyPrefix)
			if userID == "" {
				continue
			}

			roles, err := s.client.SMembers(ctx, key).Result()
			if err != nil {
				s.logger.Warn("redis role store: failed to load roles for user during scan",
					"user", userID,
					"error", err,
				)
				continue // Skip this user, continue with others
			}

			if len(roles) > 0 {
				result[userID] = roles
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break // SCAN complete
		}
	}

	if len(result) > 0 {
		s.logger.Info("redis role store: loaded all persisted user roles",
			"users_count", len(result),
		)
	} else {
		s.logger.Debug("redis role store: no persisted user roles found in Redis")
	}
	return result, nil
}

// DeleteUserRoles removes a user's persisted roles from Redis.
// If Redis is unavailable, logs a warning and returns nil (graceful degradation).
func (s *RedisRoleStore) DeleteUserRoles(ctx context.Context, userID string) error {
	if s.client == nil {
		s.logger.Warn("redis role store: skipping delete, redis client is nil",
			"user", userID,
		)
		return nil
	}

	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	key := redisRoleKeyPrefix + userID

	err := s.client.Del(ctx, key).Err()
	if err != nil {
		s.logger.Warn("redis role store: failed to delete user roles",
			"user", userID,
			"error", err,
		)
		return nil // Graceful degradation
	}

	s.logger.Debug("redis role store: deleted user roles",
		"user", userID,
	)
	return nil
}
