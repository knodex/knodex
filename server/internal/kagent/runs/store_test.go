// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package runs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock is a controllable clock for deterministic index scoring.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestStore(t *testing.T) (*RedisStore, *miniredis.Miniredis, *fakeClock) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0).UTC()}
	store := NewRedisStore(client)
	store.now = clk.now
	return store, mr, clk
}

func newRun(id, agentType, status string, ts time.Time) *Run {
	return &Run{
		ID:             id,
		Actor:          "dev@example.com",
		AgentType:      agentType,
		AgentNamespace: "alpha-apps",
		InputSummary:   "do the thing",
		Timestamp:      ts,
		Status:         status,
		TriggerType:    TriggerOnDemand,
	}
}

func runIDs(runs []Run) []string {
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = r.ID
	}
	return out
}

func TestRedisStore_List_EmptyWhenNothingCreated(t *testing.T) {
	t.Parallel()
	store, _, _ := newTestStore(t)

	got, err := store.List(context.Background(), Filter{})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRedisStore_CreateListRoundtrip_NewestFirst(t *testing.T) {
	t.Parallel()
	store, _, clk := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, newRun("run-1", "helper", StatusRunning, clk.now())))
	clk.advance(2 * time.Second)
	require.NoError(t, store.Create(ctx, newRun("run-2", "helper", StatusRunning, clk.now())))
	clk.advance(2 * time.Second)
	require.NoError(t, store.Create(ctx, newRun("run-3", "helper", StatusRunning, clk.now())))

	got, err := store.List(ctx, Filter{})
	require.NoError(t, err)
	assert.Equal(t, []string{"run-3", "run-2", "run-1"}, runIDs(got))

	// Full field roundtrip on the newest record.
	first := got[0]
	assert.Equal(t, "dev@example.com", first.Actor)
	assert.Equal(t, "helper", first.AgentType)
	assert.Equal(t, "alpha-apps", first.AgentNamespace)
	assert.Equal(t, "do the thing", first.InputSummary)
	assert.Equal(t, StatusRunning, first.Status)
	assert.Equal(t, TriggerOnDemand, first.TriggerType)
	assert.Nil(t, first.CompletedAt)
}

func TestRedisStore_Update_MutatesStatusSessionAndCompletedAt(t *testing.T) {
	t.Parallel()
	store, _, clk := newTestStore(t)
	ctx := context.Background()

	run := newRun("run-1", "helper", StatusRunning, clk.now())
	require.NoError(t, store.Create(ctx, run))

	clk.advance(5 * time.Second)
	completed := clk.now()
	run.Status = StatusCompleted
	run.KagentSessionID = "session-abc"
	run.RecommendationSummary = "scale to 3 replicas"
	run.CompletedAt = &completed
	require.NoError(t, store.Update(ctx, run))

	got, err := store.List(ctx, Filter{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, StatusCompleted, got[0].Status)
	assert.Equal(t, "session-abc", got[0].KagentSessionID)
	assert.Equal(t, "scale to 3 replicas", got[0].RecommendationSummary)
	require.NotNil(t, got[0].CompletedAt)
	assert.True(t, got[0].CompletedAt.Equal(completed))
}

func TestRedisStore_List_FilterByAgentTypeAndStatus(t *testing.T) {
	t.Parallel()
	store, _, clk := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, newRun("run-1", "helper", StatusCompleted, clk.now())))
	clk.advance(time.Second)
	require.NoError(t, store.Create(ctx, newRun("run-2", "other", StatusCompleted, clk.now())))
	clk.advance(time.Second)
	require.NoError(t, store.Create(ctx, newRun("run-3", "helper", StatusFailed, clk.now())))

	byType, err := store.List(ctx, Filter{AgentType: "helper"})
	require.NoError(t, err)
	assert.Equal(t, []string{"run-3", "run-1"}, runIDs(byType))

	byStatus, err := store.List(ctx, Filter{Status: StatusCompleted})
	require.NoError(t, err)
	assert.Equal(t, []string{"run-2", "run-1"}, runIDs(byStatus))

	both, err := store.List(ctx, Filter{AgentType: "helper", Status: StatusFailed})
	require.NoError(t, err)
	assert.Equal(t, []string{"run-3"}, runIDs(both))
}

func TestRedisStore_Create_CapEvictsOldestBeyondMaxEntries(t *testing.T) {
	t.Parallel()
	store, _, clk := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < maxEntries+5; i++ {
		require.NoError(t, store.Create(ctx, newRun(fmt.Sprintf("run-%04d", i), "helper", StatusCompleted, clk.now())))
		clk.advance(time.Second)
	}

	got, err := store.List(ctx, Filter{})
	require.NoError(t, err)
	require.Len(t, got, maxEntries)
	// Newest survives, the 5 oldest were evicted.
	assert.Equal(t, fmt.Sprintf("run-%04d", maxEntries+4), got[0].ID)
	assert.Equal(t, "run-0005", got[len(got)-1].ID)
}

func TestRedisStore_Create_SetsTTLOnKeys(t *testing.T) {
	t.Parallel()
	store, mr, clk := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, newRun("run-1", "helper", StatusRunning, clk.now())))

	assert.Greater(t, mr.TTL(runKey("run-1")), time.Duration(0), "run payload key must carry a TTL")
	assert.Greater(t, mr.TTL(indexKey), time.Duration(0), "index key must carry a TTL")
}

func TestRedisStore_List_SkipsAndPrunesExpiredRunKeys(t *testing.T) {
	t.Parallel()
	store, mr, clk := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, newRun("run-1", "helper", StatusCompleted, clk.now())))
	clk.advance(time.Second)
	require.NoError(t, store.Create(ctx, newRun("run-2", "helper", StatusCompleted, clk.now())))

	// Simulate the payload key expiring while the index entry lingers.
	mr.Del(runKey("run-1"))

	got, err := store.List(ctx, Filter{})
	require.NoError(t, err)
	assert.Equal(t, []string{"run-2"}, runIDs(got))

	// The stale index member was lazily pruned.
	ids, err := store.client.ZRevRange(ctx, indexKey, 0, -1).Result()
	require.NoError(t, err)
	assert.Equal(t, []string{"run-2"}, ids)
}

func TestRedisStore_Create_SanitizesControlChars(t *testing.T) {
	t.Parallel()
	store, _, clk := newTestStore(t)
	ctx := context.Background()

	run := newRun("run-1", "helper", StatusRunning, clk.now())
	run.InputSummary = "do\x1b[31m the thing\x00"
	run.Actor = "dev@example.com\r\n"
	require.NoError(t, store.Create(ctx, run))

	got, err := store.List(ctx, Filter{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.NotContains(t, got[0].InputSummary, "\x1b")
	assert.NotContains(t, got[0].InputSummary, "\x00")
	assert.Equal(t, "dev@example.com", got[0].Actor)
}

func TestRedisStore_NilSafety(t *testing.T) {
	t.Parallel()
	var store *RedisStore

	got, err := store.List(context.Background(), Filter{})
	require.NoError(t, err)
	assert.Empty(t, got)

	assert.Error(t, store.Create(context.Background(), newRun("x", "h", StatusRunning, time.Now())))
	assert.Error(t, store.Update(context.Background(), newRun("x", "h", StatusRunning, time.Now())))
}

func TestRedisStore_Create_RejectsEmptyID(t *testing.T) {
	t.Parallel()
	store, _, clk := newTestStore(t)

	run := newRun("", "helper", StatusRunning, clk.now())
	assert.Error(t, store.Create(context.Background(), run))
}
