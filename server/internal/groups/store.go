// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package groups provides a passive, Redis-backed store of the distinct OIDC
// group strings Knodex observes at login. It exists so the team/role editor can
// offer a typeahead of real, observed groups instead of forcing operators to
// type raw group strings blind — with no IdP admin credentials and working with
// any OIDC provider (FR-T3, NFR-T6).
//
// Privacy: the store records only the group identifier and a last-seen
// timestamp — never per-user membership or which user contributed a group. It
// is bounded (capped to maxEntries) and prunable so it cannot grow without
// limit. This is a discovery aid, not an audit log.
package groups

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/util/sanitize"
)

const (
	// observedKey is the single global Redis sorted-set key holding observed
	// group strings. Members are sanitized group identifiers; scores are the
	// last-seen unix epoch (seconds). There is no per-user dimension (AC #3).
	observedKey = "groups:observed"

	// maxEntries caps the number of distinct observed groups retained. When the
	// store exceeds this, the least-recently-seen members are pruned (AC #3).
	maxEntries = 1000

	// observedTTL is a coarse TTL on the whole key so a long-idle store
	// self-expires (90 days). Not required by the ACs; chosen so a deployment
	// that stops seeing logins eventually forgets stale group strings. The TTL
	// is refreshed on every Record, so an actively-used store never expires.
	observedTTL = 90 * 24 * time.Hour
)

// ObservedGroup is a single distinct group string with the time it was last
// seen in a login token.
type ObservedGroup struct {
	Name     string    `json:"name"`
	LastSeen time.Time `json:"lastSeen"`
}

// RedisStore implements the observed-groups store on a Redis sorted set.
type RedisStore struct {
	client *redis.Client
	// now returns the current time; overridable in tests for deterministic
	// last-seen scoring. Defaults to time.Now.
	now func() time.Time
}

// NewRedisStore creates a new Redis-backed observed-groups store.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client, now: time.Now}
}

// Record records each distinct group string with the current time as its
// last-seen score, deduplicating existing members (a re-add updates the score)
// and pruning the store to the most-recently-seen maxEntries. Control
// characters are stripped and empty/whitespace-only groups are skipped.
//
// Recording is best-effort from the caller's perspective: it returns an error
// only on a Redis failure so the caller can log it. Callers on the login hot
// path MUST invoke this off the request path (non-blocking) — see
// auth.Service.GenerateTokenWithGroups.
func (s *RedisStore) Record(ctx context.Context, groupStrings []string) error {
	if s == nil || s.client == nil {
		return nil
	}

	now := float64(s.now().Unix())
	pipe := s.client.Pipeline()
	added := 0
	for _, g := range groupStrings {
		clean := sanitize.RemoveControlChars(g) // also trims surrounding whitespace
		if clean == "" {
			continue
		}
		pipe.ZAdd(ctx, observedKey, redis.Z{Score: now, Member: clean})
		added++
	}

	if added == 0 {
		return nil
	}

	// Retain only the maxEntries most-recently-seen members: remove the lowest-
	// ranked (oldest last-seen) entries beyond the cap.
	pipe.ZRemRangeByRank(ctx, observedKey, 0, -(maxEntries + 1))
	// Refresh the coarse key TTL so an actively-used store never expires.
	pipe.Expire(ctx, observedKey, observedTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("record observed groups: %w", err)
	}
	return nil
}

// List returns the distinct observed groups ordered most-recently-seen first,
// in a shape a typeahead can consume directly (AC #2).
func (s *RedisStore) List(ctx context.Context) ([]ObservedGroup, error) {
	if s == nil || s.client == nil {
		return []ObservedGroup{}, nil
	}

	zs, err := s.client.ZRevRangeWithScores(ctx, observedKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("list observed groups: %w", err)
	}

	out := make([]ObservedGroup, 0, len(zs))
	for _, z := range zs {
		name, ok := z.Member.(string)
		if !ok {
			continue
		}
		out = append(out, ObservedGroup{
			Name:     name,
			LastSeen: time.Unix(int64(z.Score), 0).UTC(),
		})
	}
	return out, nil
}
