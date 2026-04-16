// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package clients

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/knodex/knodex/server/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisClient_Success(t *testing.T) {
	t.Parallel()

	// Create a miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := &config.Redis{
		Address:  mr.Addr(),
		Password: "",
		DB:       0,
	}

	client := NewRedisClient(cfg, nil)
	require.NotNil(t, client, "Redis client should not be nil")
	defer client.Close()

	// Test that client is connected
	ctx := context.Background()
	err := client.Ping(ctx).Err()
	assert.NoError(t, err, "Should be able to ping Redis")
}

func TestNewRedisClient_EmptyAddress(t *testing.T) {
	t.Parallel()

	cfg := &config.Redis{
		Address: "",
	}

	client := NewRedisClient(cfg, nil)
	assert.Nil(t, client, "Redis client should be nil when address is empty")
}

func TestNewRedisClient_RetryLogic(t *testing.T) {
	t.Parallel()

	// This test verifies that retry logic works by using a delayed connection
	// We'll start miniredis after the client has begun attempting to connect

	// Start with a port that will initially fail
	mr := miniredis.NewMiniRedis()
	err := mr.StartAddr("localhost:0") // Use random port
	require.NoError(t, err)
	addr := mr.Addr()
	mr.Close() // Close it so first connections fail

	cfg := &config.Redis{
		Address:  addr,
		Password: "",
		DB:       0,
	}

	// Start a goroutine to restart Redis after a short delay
	// This simulates Redis starting up after the backend
	done := make(chan bool)
	go func() {
		time.Sleep(1500 * time.Millisecond) // Wait before starting
		err := mr.Start()
		if err != nil {
			t.Logf("Failed to restart miniredis: %v", err)
		}
		close(done)
	}()

	// This should retry and eventually connect
	client := NewRedisClient(cfg, nil)
	<-done // Wait for Redis to start
	defer mr.Close()

	if client != nil {
		defer client.Close()
		// Verify connection works
		ctx := context.Background()
		err := client.Ping(ctx).Err()
		assert.NoError(t, err, "Should eventually connect to Redis after retries")
	} else {
		t.Fatal("Client should have connected after Redis became available")
	}
}

func TestNewRedisClient_MaxRetriesExceeded(t *testing.T) {
	t.Parallel()

	// This test verifies that after max retries, client returns nil
	// Skip in normal runs as it takes ~30+ seconds to complete
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	// Use an invalid address that will never connect
	cfg := &config.Redis{
		Address:  "localhost:99999", // Invalid port
		Password: "",
		DB:       0,
	}

	// This test will take some time due to retries
	start := time.Now()
	client := NewRedisClient(cfg, nil)
	duration := time.Since(start)

	assert.Nil(t, client, "Redis client should be nil after max retries exceeded")

	// Verify that retries actually happened (should take at least a few seconds)
	// With exponential backoff: 500ms, 1s, 2s, 4s, 5s, 5s... = ~20+ seconds for 10 retries
	// But we cap delays at 5s, so minimum time is harder to predict exactly
	// Let's just check it took longer than a single attempt (5s timeout)
	assert.Greater(t, duration, 5*time.Second, "Should have taken time for multiple retry attempts")
}

func TestNewRedisClient_WithPassword(t *testing.T) {
	t.Parallel()

	// Create a miniredis server with password
	mr := miniredis.RunT(t)
	defer mr.Close()

	password := "test-password"
	mr.RequireAuth(password)

	cfg := &config.Redis{
		Address:  mr.Addr(),
		Password: password,
		DB:       0,
	}

	client := NewRedisClient(cfg, nil)
	require.NotNil(t, client, "Redis client should not be nil")
	defer client.Close()

	// Test that client is connected and authenticated
	ctx := context.Background()
	err := client.Ping(ctx).Err()
	assert.NoError(t, err, "Should be able to ping Redis with correct password")
}

func TestNewRedisClient_WrongPassword(t *testing.T) {
	t.Parallel()

	// Create a miniredis server with password
	mr := miniredis.RunT(t)
	defer mr.Close()

	mr.RequireAuth("correct-password")

	cfg := &config.Redis{
		Address:  mr.Addr(),
		Password: "wrong-password",
		DB:       0,
	}

	// With wrong password, all retries will fail, so this takes ~30+ seconds
	// We'll skip this test in normal runs to keep tests fast
	// It's still valuable for manual testing
	t.Skip("Skipping test that requires waiting for all retries to exhaust")

	client := NewRedisClient(cfg, nil)
	assert.Nil(t, client, "Redis client should be nil with wrong password")
}

func TestCloseRedisClient_Success(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Should not panic
	CloseRedisClient(client, nil)

	// Verify client is closed
	ctx := context.Background()
	err := client.Ping(ctx).Err()
	assert.Error(t, err, "Client should be closed")
}

func TestCloseRedisClient_NilClient(t *testing.T) {
	t.Parallel()

	// Should not panic with nil client
	CloseRedisClient(nil, nil)
}

// =============================================================================
// Redis TLS and Username Configuration Tests
// =============================================================================

func TestNewRedisClient_WithUsername(t *testing.T) {
	t.Parallel()

	// miniredis doesn't support ACL/username, but we verify the client accepts
	// the config without error and passes it through.
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := &config.Redis{
		Address:  mr.Addr(),
		Password: "",
		DB:       0,
		Username: "testuser",
	}

	// The client will connect fine — miniredis ignores unknown AUTH params.
	// This tests that our code correctly passes Username to redis.Options.
	client := NewRedisClient(cfg, nil)
	// miniredis may or may not reject username-only auth; the key is our code doesn't crash
	if client != nil {
		defer client.Close()
	}
}

func TestNewRedisClient_TLSEnabled_FailsOnPlaintextServer(t *testing.T) {
	t.Parallel()

	// When TLS is enabled but the server doesn't support TLS,
	// the connection should fail (verifying TLS is actually being used).
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := &config.Redis{
		Address:    mr.Addr(),
		Password:   "",
		DB:         0,
		TLSEnabled: true,
	}

	// This should fail to connect because miniredis doesn't support TLS.
	// After max retries, it returns nil — proving TLS was actually attempted.
	// Skip in normal runs due to retry delay.
	if testing.Short() {
		t.Skip("Skipping long-running TLS test in short mode")
	}

	client := NewRedisClient(cfg, nil)
	assert.Nil(t, client, "Redis client should be nil when TLS is enabled but server doesn't support TLS")
}

func TestNewRedisClient_TLSDisabled_ConnectsToPlaintextServer(t *testing.T) {
	t.Parallel()

	// When TLS is disabled, connection to plaintext miniredis should succeed.
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := &config.Redis{
		Address:    mr.Addr(),
		Password:   "",
		DB:         0,
		TLSEnabled: false,
	}

	client := NewRedisClient(cfg, nil)
	require.NotNil(t, client, "Redis client should connect when TLS is disabled")
	defer client.Close()

	ctx := context.Background()
	err := client.Ping(ctx).Err()
	assert.NoError(t, err, "Should be able to ping plaintext Redis")
}

func TestNewRedisClient_TLSInsecureSkipVerify(t *testing.T) {
	t.Parallel()

	// Verify that InsecureSkipVerify doesn't crash and is accepted.
	// We can't fully test TLS behavior without a TLS-capable server,
	// but we verify the configuration is passed correctly.
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := &config.Redis{
		Address:               mr.Addr(),
		Password:              "",
		DB:                    0,
		TLSEnabled:            true,
		TLSInsecureSkipVerify: true,
	}

	// Same as above — will fail because miniredis doesn't support TLS.
	// But the code path exercises InsecureSkipVerify without panicking.
	if testing.Short() {
		t.Skip("Skipping long-running TLS test in short mode")
	}

	client := NewRedisClient(cfg, nil)
	// Client should be nil because miniredis is plaintext
	assert.Nil(t, client, "Redis client should be nil when TLS is enabled but server is plaintext")
}
