// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"github.com/redis/go-redis/v9"
)

// NewMockRedisClientAdapter creates a mock Redis client for testing purposes
// This creates a redis.Client that won't actually connect (used in non-OIDC tests)
func NewMockRedisClientAdapter() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:16379", // Non-existent server, but tests won't use it
	})
	return client
}
