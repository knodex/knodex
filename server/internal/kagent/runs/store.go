// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/util/sanitize"
)

const (
	// runKeyPrefix is the per-run JSON payload key: agentruns:run:{id}.
	runKeyPrefix = "agentruns:run:"

	// indexKey is the ZSET index of run IDs scored by invocation time
	// (UnixMilli), trimmed to maxEntries.
	indexKey = "agentruns:index"

	// maxEntries caps the number of retained runs; the oldest beyond the cap
	// are pruned (OSS storage is ephemeral by design — AC #6).
	maxEntries = 1000

	// runTTL bounds every key's lifetime so an idle store self-expires. The
	// index TTL is refreshed on every Create, so an active store never loses
	// its index before its runs.
	runTTL = 7 * 24 * time.Hour
)

// Filter narrows List results. Pagination and namespace visibility are
// applied by the handler AFTER authz filtering — the store only filters by
// exact agentType/status and returns newest-first.
type Filter struct {
	AgentType string
	Status    string
}

// Store is the run-record persistence seam. OSS uses RedisStore; EE (49.5)
// layers Postgres persistence behind this same interface via
// app.SetAgentRunStore without touching handlers.
type Store interface {
	Create(ctx context.Context, run *Run) error
	Update(ctx context.Context, run *Run) error
	// List returns runs matching the filter, newest-first.
	List(ctx context.Context, filter Filter) ([]Run, error)
}

// RedisStore implements Store on Redis: per-run JSON payload keys plus a
// capped ZSET index (the internal/groups/store.go pattern extended with
// per-record payloads).
type RedisStore struct {
	client *redis.Client
	// now returns the current time; overridable in tests for deterministic
	// index scoring. Defaults to time.Now.
	now func() time.Time
}

// NewRedisStore creates a new Redis-backed run store.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client, now: time.Now}
}

// sanitizeRun strips control characters from every stored string field so
// attacker-influenced input (message text, A2A response artifacts) cannot
// smuggle terminal escapes into stored records.
func sanitizeRun(run *Run) {
	run.ID = sanitize.RemoveControlChars(run.ID)
	run.Actor = sanitize.RemoveControlChars(run.Actor)
	run.AgentType = sanitize.RemoveControlChars(run.AgentType)
	run.AgentNamespace = sanitize.RemoveControlChars(run.AgentNamespace)
	run.ContextRef = sanitize.RemoveControlChars(run.ContextRef)
	run.KagentSessionID = sanitize.RemoveControlChars(run.KagentSessionID)
	run.ConversationID = sanitize.RemoveControlChars(run.ConversationID)
	run.InputSummary = sanitize.RemoveControlChars(run.InputSummary)
	run.RecommendationSummary = sanitize.RemoveControlChars(run.RecommendationSummary)
	run.ActionTaken = sanitize.RemoveControlChars(run.ActionTaken)
	run.Status = sanitize.RemoveControlChars(run.Status)
	run.TriggerType = sanitize.RemoveControlChars(run.TriggerType)
}

// runKey returns the Redis key for a run's JSON payload.
func runKey(id string) string {
	return runKeyPrefix + id
}

// Create persists a new run and indexes it by invocation time, pruning the
// index beyond maxEntries.
func (s *RedisStore) Create(ctx context.Context, run *Run) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("agent run store: redis client unavailable")
	}
	if run == nil || run.ID == "" {
		return fmt.Errorf("agent run store: run with non-empty ID required")
	}

	sanitizeRun(run)
	payload, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("marshal agent run: %w", err)
	}

	// If the ZSET is at maxEntries, adding one more will evict the oldest run
	// (rank 0). Eagerly delete its result key so it doesn't accumulate as an
	// orphan until resultTTL (7d since Story 50.6): ZRemRangeByRank removes the
	// ZSET member but leaves the separate agentruns:result:{id} key untouched.
	var evictID string
	if n, err := s.client.ZCard(ctx, indexKey).Result(); err == nil && n >= int64(maxEntries) {
		if ids, err := s.client.ZRange(ctx, indexKey, 0, 0).Result(); err == nil && len(ids) > 0 {
			evictID = ids[0]
		}
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, runKey(run.ID), payload, runTTL)
	pipe.ZAdd(ctx, indexKey, redis.Z{Score: float64(run.Timestamp.UnixMilli()), Member: run.ID})
	// Retain only the maxEntries most-recent runs: drop the lowest-scored
	// (oldest) index members beyond the cap. Their payload keys expire via TTL.
	pipe.ZRemRangeByRank(ctx, indexKey, 0, -(maxEntries + 1))
	// Refresh the index TTL so an actively-used store never expires.
	pipe.Expire(ctx, indexKey, runTTL)
	if evictID != "" {
		pipe.Del(ctx, resultKey(evictID))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("create agent run: %w", err)
	}
	return nil
}

// Update overwrites an existing run's payload (same key, TTL refreshed). The
// index score keeps the invocation time — ordering is by invocation, not by
// completion.
func (s *RedisStore) Update(ctx context.Context, run *Run) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("agent run store: redis client unavailable")
	}
	if run == nil || run.ID == "" {
		return fmt.Errorf("agent run store: run with non-empty ID required")
	}

	sanitizeRun(run)
	payload, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("marshal agent run: %w", err)
	}
	if err := s.client.Set(ctx, runKey(run.ID), payload, runTTL).Err(); err != nil {
		return fmt.Errorf("update agent run: %w", err)
	}
	return nil
}

// List returns runs newest-first, filtered by exact agentType/status when
// set. At ≤maxEntries records this is one ZREVRANGE + one MGET + in-memory
// filtering — trivially within NFR-A1's 500ms; no per-filter secondary
// indexes by design.
func (s *RedisStore) List(ctx context.Context, filter Filter) ([]Run, error) {
	if s == nil || s.client == nil {
		return []Run{}, nil
	}

	ids, err := s.client.ZRevRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("list agent runs: %w", err)
	}
	if len(ids) == 0 {
		return []Run{}, nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = runKey(id)
	}
	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("list agent runs payloads: %w", err)
	}

	out := make([]Run, 0, len(vals))
	for i, v := range vals {
		raw, ok := v.(string)
		if !ok {
			// Payload key expired before its index entry — lazily drop the
			// stale index member (best-effort; ignore the error).
			s.client.ZRem(ctx, indexKey, ids[i])
			continue
		}
		var run Run
		if err := json.Unmarshal([]byte(raw), &run); err != nil {
			continue
		}
		if filter.AgentType != "" && run.AgentType != filter.AgentType {
			continue
		}
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		out = append(out, run)
	}
	return out, nil
}
