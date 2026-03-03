package oidc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	require.NotNil(t, config)
	assert.Equal(t, 8081, config.Port)
	assert.Equal(t, "test-client-id", config.ClientID)
	assert.Equal(t, "test-client-secret", config.ClientSecret)
	assert.Equal(t, "http://localhost:8080/api/v1/auth/oidc/callback", config.RedirectURL)
	assert.Equal(t, time.Hour, config.TokenExpiry)
	assert.NotNil(t, config.Scenarios)
}

func TestWithPort(t *testing.T) {
	config := DefaultConfig()
	WithPort(9999)(config)

	assert.Equal(t, 9999, config.Port)
}

func TestWithIssuerURL(t *testing.T) {
	config := DefaultConfig()
	WithIssuerURL("http://custom-issuer:8888")(config)

	assert.Equal(t, "http://custom-issuer:8888", config.IssuerURL)
}

func TestWithClientCredentials(t *testing.T) {
	config := DefaultConfig()
	WithClientCredentials("custom-client", "custom-secret")(config)

	assert.Equal(t, "custom-client", config.ClientID)
	assert.Equal(t, "custom-secret", config.ClientSecret)
}

func TestWithRedirectURL(t *testing.T) {
	config := DefaultConfig()
	WithRedirectURL("http://myapp:3000/callback")(config)

	assert.Equal(t, "http://myapp:3000/callback", config.RedirectURL)
}

func TestWithTokenExpiry(t *testing.T) {
	config := DefaultConfig()
	WithTokenExpiry(30 * time.Minute)(config)

	assert.Equal(t, 30*time.Minute, config.TokenExpiry)
}

func TestWithScenarios(t *testing.T) {
	config := DefaultConfig()
	scenarios := &ScenarioConfig{
		ExpiredTokens:     true,
		InvalidSignature:  true,
		MissingEmailClaim: true,
		RejectAllTokens:   true,
	}
	WithScenarios(scenarios)(config)

	require.NotNil(t, config.Scenarios)
	assert.True(t, config.Scenarios.ExpiredTokens)
	assert.True(t, config.Scenarios.InvalidSignature)
	assert.True(t, config.Scenarios.MissingEmailClaim)
	assert.True(t, config.Scenarios.RejectAllTokens)
}

func TestChainingOptions(t *testing.T) {
	config := DefaultConfig()

	// Apply multiple options
	options := []Option{
		WithPort(9000),
		WithIssuerURL("http://test:9000"),
		WithClientCredentials("client1", "secret1"),
		WithTokenExpiry(2 * time.Hour),
		WithScenarios(&ScenarioConfig{ExpiredTokens: true}),
	}

	for _, opt := range options {
		opt(config)
	}

	assert.Equal(t, 9000, config.Port)
	assert.Equal(t, "http://test:9000", config.IssuerURL)
	assert.Equal(t, "client1", config.ClientID)
	assert.Equal(t, "secret1", config.ClientSecret)
	assert.Equal(t, 2*time.Hour, config.TokenExpiry)
	require.NotNil(t, config.Scenarios)
	assert.True(t, config.Scenarios.ExpiredTokens)
}

func TestScenarioConfig(t *testing.T) {
	t.Run("empty scenario config", func(t *testing.T) {
		scenarios := &ScenarioConfig{}

		assert.False(t, scenarios.ExpiredTokens)
		assert.False(t, scenarios.InvalidSignature)
		assert.False(t, scenarios.MissingEmailClaim)
		assert.False(t, scenarios.RejectAllTokens)
	})

	t.Run("full scenario config", func(t *testing.T) {
		scenarios := &ScenarioConfig{
			ExpiredTokens:      true,
			InvalidSignature:   true,
			InvalidAudience:    true,
			MissingEmailClaim:  true,
			RejectAllTokens:    true,
			TokenExpirySeconds: 300,
		}

		assert.True(t, scenarios.ExpiredTokens)
		assert.True(t, scenarios.InvalidSignature)
		assert.True(t, scenarios.InvalidAudience)
		assert.True(t, scenarios.MissingEmailClaim)
		assert.True(t, scenarios.RejectAllTokens)
		assert.Equal(t, 300, scenarios.TokenExpirySeconds)
	})
}

func TestEnableExpiredTokens(t *testing.T) {
	config := DefaultConfig()
	EnableExpiredTokens()(config)

	require.NotNil(t, config.Scenarios)
	assert.True(t, config.Scenarios.ExpiredTokens)
}

func TestEnableInvalidSignature(t *testing.T) {
	config := DefaultConfig()
	EnableInvalidSignature()(config)

	require.NotNil(t, config.Scenarios)
	assert.True(t, config.Scenarios.InvalidSignature)
}

func TestEnableRejectAllTokens(t *testing.T) {
	config := DefaultConfig()
	EnableRejectAllTokens()(config)

	require.NotNil(t, config.Scenarios)
	assert.True(t, config.Scenarios.RejectAllTokens)
}
