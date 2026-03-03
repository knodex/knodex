package resilience

import (
	"log/slog"
	"time"

	"github.com/sony/gobreaker/v2"
)

// CircuitBreakerName identifies a circuit breaker for a downstream dependency.
type CircuitBreakerName string

const (
	CBKubernetesAPI CircuitBreakerName = "kubernetes-api"
	CBRedis         CircuitBreakerName = "redis"
)

// CircuitBreakers holds circuit breakers for all downstream dependencies.
type CircuitBreakers struct {
	breakers map[CircuitBreakerName]*gobreaker.CircuitBreaker[any]
}

// NewCircuitBreakers creates circuit breakers for all known downstream dependencies.
func NewCircuitBreakers() *CircuitBreakers {
	cb := &CircuitBreakers{
		breakers: make(map[CircuitBreakerName]*gobreaker.CircuitBreaker[any]),
	}

	cb.breakers[CBKubernetesAPI] = gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        string(CBKubernetesAPI),
		MaxRequests: 3,                // Allow 3 requests in half-open state
		Interval:    30 * time.Second, // Reset failure count after 30s of no failures
		Timeout:     15 * time.Second, // Move to half-open after 15s in open state
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip after 5 consecutive failures
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			slog.Warn("circuit breaker state change",
				"name", name,
				"from", from.String(),
				"to", to.String(),
			)
		},
	})

	cb.breakers[CBRedis] = gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        string(CBRedis),
		MaxRequests: 2,                // Allow 2 requests in half-open state
		Interval:    30 * time.Second, // Reset failure count after 30s
		Timeout:     10 * time.Second, // Move to half-open after 10s
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip after 5 consecutive failures
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			slog.Warn("circuit breaker state change",
				"name", name,
				"from", from.String(),
				"to", to.String(),
			)
		},
	})

	return cb
}

// Get returns the circuit breaker for the given dependency name.
func (cb *CircuitBreakers) Get(name CircuitBreakerName) *gobreaker.CircuitBreaker[any] {
	return cb.breakers[name]
}

// State returns the current state of a circuit breaker.
func (cb *CircuitBreakers) State(name CircuitBreakerName) gobreaker.State {
	if b, ok := cb.breakers[name]; ok {
		return b.State()
	}
	return gobreaker.StateClosed
}

// Counts returns the current failure counts for a circuit breaker.
func (cb *CircuitBreakers) Counts(name CircuitBreakerName) gobreaker.Counts {
	if b, ok := cb.breakers[name]; ok {
		return b.Counts()
	}
	return gobreaker.Counts{}
}

// StateString returns the state as a human-readable string.
func (cb *CircuitBreakers) StateString(name CircuitBreakerName) string {
	return cb.State(name).String()
}
