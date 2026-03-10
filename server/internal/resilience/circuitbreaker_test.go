// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package resilience

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sony/gobreaker/v2"
)

func TestNewCircuitBreakers(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakers()
	require.NotNil(t, cb)

	// Verify both breakers exist
	assert.NotNil(t, cb.Get(CBKubernetesAPI))
	assert.NotNil(t, cb.Get(CBRedis))
}

func TestCircuitBreakerInitialState(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakers()

	assert.Equal(t, gobreaker.StateClosed, cb.State(CBKubernetesAPI))
	assert.Equal(t, gobreaker.StateClosed, cb.State(CBRedis))
	assert.Equal(t, "closed", cb.StateString(CBKubernetesAPI))
}

func TestCircuitBreakerTripsAfterConsecutiveFailures(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakers()
	breaker := cb.Get(CBRedis)

	errFail := errors.New("connection refused")

	// Execute 5 consecutive failures to trip the breaker
	for i := 0; i < 5; i++ {
		_, _ = breaker.Execute(func() (any, error) {
			return nil, errFail
		})
	}

	assert.Equal(t, gobreaker.StateOpen, cb.State(CBRedis))
}

func TestCircuitBreakerRejectsWhenOpen(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakers()
	breaker := cb.Get(CBRedis)

	errFail := errors.New("connection refused")

	// Trip the breaker
	for i := 0; i < 5; i++ {
		_, _ = breaker.Execute(func() (any, error) {
			return nil, errFail
		})
	}

	// Next call should be rejected by the circuit breaker
	_, err := breaker.Execute(func() (any, error) {
		return "should not execute", nil
	})
	assert.Error(t, err)
	assert.Equal(t, gobreaker.ErrOpenState, err)
}

func TestCircuitBreakerSuccessKeepsClosed(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakers()
	breaker := cb.Get(CBKubernetesAPI)

	// Successful calls keep breaker closed
	for i := 0; i < 10; i++ {
		result, err := breaker.Execute(func() (any, error) {
			return "ok", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "ok", result)
	}

	assert.Equal(t, gobreaker.StateClosed, cb.State(CBKubernetesAPI))
}

func TestCircuitBreakerCounts(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakers()
	breaker := cb.Get(CBRedis)

	errFail := errors.New("fail")

	// 2 successes, 3 failures
	for i := 0; i < 2; i++ {
		_, _ = breaker.Execute(func() (any, error) { return nil, nil })
	}
	for i := 0; i < 3; i++ {
		_, _ = breaker.Execute(func() (any, error) { return nil, errFail })
	}

	counts := cb.Counts(CBRedis)
	assert.Equal(t, uint32(3), counts.ConsecutiveFailures)
	assert.Equal(t, uint32(5), counts.Requests)
}

func TestCircuitBreakerUnknownName(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakers()

	// Unknown name returns nil/defaults
	assert.Nil(t, cb.Get("unknown"))
	assert.Equal(t, gobreaker.StateClosed, cb.State("unknown"))
	assert.Equal(t, gobreaker.Counts{}, cb.Counts("unknown"))
}
