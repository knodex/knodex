// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package drift provides Redis-based GitOps drift detection for instances.
//
// When a gitops/hybrid instance spec is edited and pushed to Git, the desired
// spec hash and full spec are stored in Redis. On subsequent reads, the live
// spec hash is compared to the stored desired hash — if they differ, the
// instance is marked as drifted. When the InstanceTracker detects that the
// live spec matches the desired spec (ArgoCD/Flux reconciled), the drift key
// is deleted.
package drift

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/util/collection"
	utilhash "github.com/knodex/knodex/server/internal/util/hash"
)

// safetyTTL is a safety-net expiry for drift entries.
// Drift entries are normally cleared by reconciliation detection (CheckAndClearIfReconciled),
// but if ArgoCD sync is permanently broken, entries would grow unbounded without this TTL.
// 30 days is generous enough to cover extended sync outages while preventing memory leaks.
const safetyTTL = 30 * 24 * time.Hour

// DriftEntry is the value stored in Redis for a drift key.
type DriftEntry struct {
	DesiredSpecHash string                 `json:"desiredSpecHash"`
	DesiredSpec     map[string]interface{} `json:"desiredSpec"`
	PushedAt        string                 `json:"pushedAt"`
}

// Service manages GitOps drift state in Redis.
type Service struct {
	client       *redis.Client
	logger       *slog.Logger
	organization string
}

// NewService creates a new drift detection service.
// If client is nil, all operations are no-ops (graceful degradation).
// If organization is empty, it defaults to "default".
func NewService(client *redis.Client, logger *slog.Logger, organization string) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if organization == "" {
		organization = "default"
	}
	return &Service{
		client:       client,
		logger:       logger.With("component", "drift-service"),
		organization: organization,
	}
}

// driftKey builds the Redis key for a drift entry.
func (s *Service) driftKey(namespace, kind, name string) string {
	return fmt.Sprintf("drift:%s/%s/%s/%s", s.organization, namespace, kind, name)
}

// HashSpec computes a SHA-256 hash of a spec map.
// Uses canonicalJSON to produce deterministic output regardless of Go map iteration order.
func HashSpec(spec map[string]interface{}) (string, error) {
	data, err := canonicalJSON(spec)
	if err != nil {
		return "", fmt.Errorf("marshal spec for hashing: %w", err)
	}
	return "sha256:" + utilhash.SHA256(data), nil
}

// canonicalJSON produces a deterministic JSON encoding by sorting map keys recursively.
// Go's json.Marshal does not guarantee key order for map[string]interface{}, so we
// must sort keys ourselves to ensure identical specs always produce identical hashes.
func canonicalJSON(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := collection.SortedKeys(val)

		buf := []byte("{")
		for i, k := range keys {
			if i > 0 {
				buf = append(buf, ',')
			}
			keyBytes, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			buf = append(buf, keyBytes...)
			buf = append(buf, ':')
			valBytes, err := canonicalJSON(val[k])
			if err != nil {
				return nil, err
			}
			buf = append(buf, valBytes...)
		}
		buf = append(buf, '}')
		return buf, nil
	case []interface{}:
		buf := []byte("[")
		for i, elem := range val {
			if i > 0 {
				buf = append(buf, ',')
			}
			elemBytes, err := canonicalJSON(elem)
			if err != nil {
				return nil, err
			}
			buf = append(buf, elemBytes...)
		}
		buf = append(buf, ']')
		return buf, nil
	default:
		return json.Marshal(v)
	}
}

// StoreDrift stores the desired spec after a successful Git push.
// The entry persists until cleared by reconciliation (no TTL).
func (s *Service) StoreDrift(ctx context.Context, namespace, kind, name string, desiredSpec map[string]interface{}) error {
	if s.client == nil {
		return nil
	}

	hash, err := HashSpec(desiredSpec)
	if err != nil {
		return err
	}

	entry := DriftEntry{
		DesiredSpecHash: hash,
		DesiredSpec:     desiredSpec,
		PushedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal drift entry: %w", err)
	}

	key := s.driftKey(namespace, kind, name)
	if err := s.client.Set(ctx, key, data, safetyTTL).Err(); err != nil {
		s.logger.Warn("failed to store drift entry", "key", key, "error", err)
		return fmt.Errorf("redis set drift: %w", err)
	}

	s.logger.Debug("stored drift entry", "key", key, "hash", hash)
	return nil
}

// CheckDrift checks if an instance has drift by comparing the live spec hash
// to the stored desired spec hash. Returns (isDrifted, desiredSpec, driftedAt, error).
// If no drift entry exists or Redis is unavailable, returns (false, nil, nil, nil).
func (s *Service) CheckDrift(ctx context.Context, namespace, kind, name string, liveSpec map[string]interface{}) (bool, map[string]interface{}, *time.Time, error) {
	if s.client == nil {
		return false, nil, nil, nil
	}

	key := s.driftKey(namespace, kind, name)
	data, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil, nil, nil
	}
	if err != nil {
		s.logger.Warn("failed to get drift entry", "key", key, "error", err)
		return false, nil, nil, nil // Graceful degradation
	}

	var entry DriftEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		s.logger.Warn("failed to unmarshal drift entry", "key", key, "error", err)
		return false, nil, nil, nil
	}

	liveHash, err := HashSpec(liveSpec)
	if err != nil {
		return false, nil, nil, nil
	}

	if liveHash == entry.DesiredSpecHash {
		// Reconciled but not cleared here — callers use CheckAndClearIfReconciled
		// or ClearDrift explicitly so that the clear event can be broadcast via WebSocket.
		return false, nil, nil, nil
	}

	var driftedAt *time.Time
	if entry.PushedAt != "" {
		if t, err := time.Parse(time.RFC3339, entry.PushedAt); err == nil {
			driftedAt = &t
		}
	}

	return true, entry.DesiredSpec, driftedAt, nil
}

// DriftCheckInput identifies a single instance to check for drift.
type DriftCheckInput struct {
	Namespace string
	Kind      string
	Name      string
	LiveSpec  map[string]interface{}
}

// DriftCheckResult contains the drift state for a single instance.
type DriftCheckResult struct {
	IsDrifted   bool
	DesiredSpec map[string]interface{}
	DriftedAt   *time.Time
}

// BatchCheckDrift checks multiple instances for drift in a single Redis MGET round-trip.
// Returns a slice of results corresponding 1:1 with the inputs.
// Gracefully degrades: returns all-false on Redis errors.
func (s *Service) BatchCheckDrift(ctx context.Context, inputs []DriftCheckInput) []DriftCheckResult {
	results := make([]DriftCheckResult, len(inputs))
	if s.client == nil || len(inputs) == 0 {
		return results
	}

	keys := make([]string, len(inputs))
	for i, in := range inputs {
		keys[i] = s.driftKey(in.Namespace, in.Kind, in.Name)
	}

	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		s.logger.Warn("BatchCheckDrift MGet failed", "error", err)
		return results // graceful degradation
	}

	for i, val := range vals {
		if val == nil {
			continue
		}

		str, ok := val.(string)
		if !ok {
			continue
		}

		var entry DriftEntry
		if err := json.Unmarshal([]byte(str), &entry); err != nil {
			s.logger.Debug("BatchCheckDrift: unmarshal failed", "key", keys[i], "error", err)
			continue
		}

		liveHash, err := HashSpec(inputs[i].LiveSpec)
		if err != nil {
			s.logger.Debug("BatchCheckDrift: hash failed", "key", keys[i], "error", err)
			continue
		}

		if liveHash != entry.DesiredSpecHash {
			res := DriftCheckResult{
				IsDrifted:   true,
				DesiredSpec: entry.DesiredSpec,
			}
			if entry.PushedAt != "" {
				if t, err := time.Parse(time.RFC3339, entry.PushedAt); err == nil {
					res.DriftedAt = &t
				}
			}
			results[i] = res
		}
		// If hashes match: reconciled but not cleared here (same as CheckDrift)
	}

	return results
}

// ClearDrift removes the drift entry for an instance (reconciliation complete).
func (s *Service) ClearDrift(ctx context.Context, namespace, kind, name string) error {
	if s.client == nil {
		return nil
	}

	key := s.driftKey(namespace, kind, name)
	if err := s.client.Del(ctx, key).Err(); err != nil {
		s.logger.Warn("failed to clear drift entry", "key", key, "error", err)
		return fmt.Errorf("redis del drift: %w", err)
	}

	s.logger.Debug("cleared drift entry", "key", key)
	return nil
}

// checkAndDeleteScript atomically reads, compares desiredSpecHash, and deletes if matched.
// KEYS[1] = drift key, ARGV[1] = expected desiredSpecHash
// Returns 1 if deleted (reconciled), 0 if not matched or key absent.
const checkAndDeleteScript = `
local data = redis.call('GET', KEYS[1])
if not data then return 0 end
local entry = cjson.decode(data)
if entry.desiredSpecHash == ARGV[1] then
    redis.call('DEL', KEYS[1])
    return 1
end
return 0
`

// CheckAndClearIfReconciled checks if the live spec matches the desired spec
// and atomically clears the drift entry if reconciliation is complete.
// Uses a Lua script to eliminate the TOCTOU race between check and delete.
// Returns true if drift was cleared (reconciliation detected).
func (s *Service) CheckAndClearIfReconciled(ctx context.Context, namespace, kind, name string, liveSpec map[string]interface{}) bool {
	if s.client == nil {
		return false
	}

	liveHash, err := HashSpec(liveSpec)
	if err != nil {
		return false
	}

	key := s.driftKey(namespace, kind, name)
	result, err := s.client.Eval(ctx, checkAndDeleteScript, []string{key}, liveHash).Int()
	if err != nil {
		return false // Redis unavailable or script error
	}

	if result == 1 {
		s.logger.Info("drift reconciled",
			"namespace", namespace,
			"kind", kind,
			"name", name,
		)
		return true
	}

	return false
}
