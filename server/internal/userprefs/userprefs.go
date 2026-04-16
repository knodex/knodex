// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package userprefs provides user preference storage backed by Redis.
package userprefs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/util/sanitize"
)

const (
	// prefsTTL is the TTL for user preferences (90 days).
	prefsTTL = 90 * 24 * time.Hour

	// MaxFavorites is the maximum number of favorite RGDs.
	MaxFavorites = 10

	// MaxRecent is the maximum number of recent RGDs.
	MaxRecent = 20
)

// UserPreferences holds a user's favorites and recent items.
type UserPreferences struct {
	FavoriteRgds []string `json:"favoriteRgds"`
	RecentRgds   []string `json:"recentRgds"`
}

// Store defines the interface for user preference persistence.
type Store interface {
	Get(ctx context.Context, userID string) (*UserPreferences, error)
	Put(ctx context.Context, userID string, prefs *UserPreferences) error
}

// RedisStore implements Store using Redis.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new Redis-backed preference store.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func redisKey(userID string) string {
	return fmt.Sprintf("user:prefs:%s", sanitize.RedisKey(userID))
}

// Get retrieves user preferences from Redis.
// Returns empty preferences (not an error) if no prefs exist.
func (s *RedisStore) Get(ctx context.Context, userID string) (*UserPreferences, error) {
	data, err := s.client.Get(ctx, redisKey(userID)).Bytes()
	if err == redis.Nil {
		return &UserPreferences{
			FavoriteRgds: []string{},
			RecentRgds:   []string{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get user prefs: %w", err)
	}

	var prefs UserPreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, fmt.Errorf("unmarshal user prefs: %w", err)
	}

	// Ensure non-nil slices for JSON serialization
	if prefs.FavoriteRgds == nil {
		prefs.FavoriteRgds = []string{}
	}
	if prefs.RecentRgds == nil {
		prefs.RecentRgds = []string{}
	}

	return &prefs, nil
}

// Put stores user preferences in Redis with a 90-day TTL.
// Arrays are truncated to their max sizes before storage.
func (s *RedisStore) Put(ctx context.Context, userID string, prefs *UserPreferences) error {
	// Truncate to max sizes
	if len(prefs.FavoriteRgds) > MaxFavorites {
		prefs.FavoriteRgds = prefs.FavoriteRgds[:MaxFavorites]
	}
	if len(prefs.RecentRgds) > MaxRecent {
		prefs.RecentRgds = prefs.RecentRgds[:MaxRecent]
	}

	data, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("marshal user prefs: %w", err)
	}

	if err := s.client.Set(ctx, redisKey(userID), data, prefsTTL).Err(); err != nil {
		return fmt.Errorf("redis set user prefs: %w", err)
	}

	return nil
}
