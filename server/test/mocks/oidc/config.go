// Package oidc provides a mock OIDC server for deterministic E2E testing.
// It implements the OpenID Connect Discovery, Token, JWKS, and UserInfo endpoints.
package oidc

import (
	"crypto/rsa"
	"time"
)

// Config holds the configuration for the mock OIDC server.
type Config struct {
	// Port is the port to listen on (default: 8081)
	Port int

	// IssuerURL is the base URL for the OIDC issuer.
	// If empty, it will be set to http://localhost:{Port} on Start().
	IssuerURL string

	// ClientID is the expected client_id for token requests
	ClientID string

	// ClientSecret is the expected client_secret for token requests
	ClientSecret string

	// RedirectURL is the expected redirect_uri for authorization
	RedirectURL string

	// PrivateKey is the RSA private key for signing tokens.
	// If nil, a new key will be generated on Start().
	PrivateKey *rsa.PrivateKey

	// TokenExpiry is the duration for token validity (default: 1 hour)
	TokenExpiry time.Duration

	// Scenarios holds edge case simulation configuration
	Scenarios *ScenarioConfig
}

// ScenarioConfig enables edge case simulation for testing error scenarios.
type ScenarioConfig struct {
	// TokenExpirySeconds overrides default token expiry (default: 3600)
	TokenExpirySeconds int

	// RejectAllTokens causes the token endpoint to always return an error
	RejectAllTokens bool

	// InvalidSignature causes tokens to be signed with a different key
	InvalidSignature bool

	// InvalidAudience causes tokens to have wrong audience claim
	InvalidAudience bool

	// MissingEmailClaim causes tokens to omit the email claim
	MissingEmailClaim bool

	// ExpiredTokens causes all tokens to be issued as already expired
	ExpiredTokens bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Port:         8081,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:8080/api/v1/auth/oidc/callback",
		TokenExpiry:  time.Hour,
		Scenarios:    &ScenarioConfig{},
	}
}

// Option is a function that configures the mock OIDC server.
type Option func(*Config)

// WithPort sets the port for the mock server.
func WithPort(port int) Option {
	return func(c *Config) {
		c.Port = port
	}
}

// WithIssuerURL sets the issuer URL for the mock server.
func WithIssuerURL(url string) Option {
	return func(c *Config) {
		c.IssuerURL = url
	}
}

// WithClientCredentials sets the client_id and client_secret.
func WithClientCredentials(clientID, clientSecret string) Option {
	return func(c *Config) {
		c.ClientID = clientID
		c.ClientSecret = clientSecret
	}
}

// WithRedirectURL sets the expected redirect_uri.
func WithRedirectURL(url string) Option {
	return func(c *Config) {
		c.RedirectURL = url
	}
}

// WithPrivateKey sets the RSA private key for signing tokens.
func WithPrivateKey(key *rsa.PrivateKey) Option {
	return func(c *Config) {
		c.PrivateKey = key
	}
}

// WithTokenExpiry sets the token expiry duration.
func WithTokenExpiry(d time.Duration) Option {
	return func(c *Config) {
		c.TokenExpiry = d
	}
}

// WithScenarios sets the scenario configuration for edge case testing.
func WithScenarios(scenarios *ScenarioConfig) Option {
	return func(c *Config) {
		c.Scenarios = scenarios
	}
}

// EnableExpiredTokens causes all issued tokens to be expired.
func EnableExpiredTokens() Option {
	return func(c *Config) {
		if c.Scenarios == nil {
			c.Scenarios = &ScenarioConfig{}
		}
		c.Scenarios.ExpiredTokens = true
	}
}

// EnableInvalidSignature causes tokens to be signed with a wrong key.
func EnableInvalidSignature() Option {
	return func(c *Config) {
		if c.Scenarios == nil {
			c.Scenarios = &ScenarioConfig{}
		}
		c.Scenarios.InvalidSignature = true
	}
}

// EnableRejectAllTokens causes the token endpoint to always fail.
func EnableRejectAllTokens() Option {
	return func(c *Config) {
		if c.Scenarios == nil {
			c.Scenarios = &ScenarioConfig{}
		}
		c.Scenarios.RejectAllTokens = true
	}
}
