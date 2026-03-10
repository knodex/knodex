// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package rand provides cryptographically secure random generation utilities.
//
// All functions use crypto/rand exclusively (never math/rand) to ensure
// cryptographic security for tokens, secrets, and identifiers.
//
// This package consolidates random generation patterns previously duplicated
// across auth, bootstrap, middleware, handlers, and RBAC packages, following
// the ArgoCD util/ package structure.
//
// Usage:
//
//	// Generate random hex string (e.g., request IDs, tickets)
//	id := rand.GenerateRandomHex(16) // 32 hex chars
//
//	// Generate URL-safe base64 string (e.g., OIDC state tokens)
//	token := rand.GenerateRandomString(32) // ~43 base64 chars
//
//	// Generate a token with default 32-byte entropy
//	t := rand.GenerateToken(32)
package rand

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

// reader is the random source. It defaults to crypto/rand.Reader and is
// overridden only in tests to simulate failures.
var reader io.Reader = cryptorand.Reader

// GenerateRandomBytes returns n cryptographically secure random bytes.
// Returns an error if the system's random number generator fails.
func GenerateRandomBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("rand: negative byte count %d", n)
	}
	if n == 0 {
		return []byte{}, nil
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(reader, b); err != nil {
		return nil, fmt.Errorf("rand: failed to generate %d random bytes: %w", n, err)
	}
	return b, nil
}

// GenerateRandomString returns a URL-safe base64 encoded string from n random bytes.
// The resulting string length is approximately 4*n/3 characters.
// Panics if the system's random number generator fails (extremely rare).
func GenerateRandomString(n int) string {
	b, err := GenerateRandomBytes(n)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

// GenerateRandomHex returns a hex-encoded string from n random bytes.
// The resulting string is exactly 2*n characters long.
// Panics if the system's random number generator fails (extremely rare).
func GenerateRandomHex(n int) string {
	b, err := GenerateRandomBytes(n)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// GenerateToken returns a URL-safe base64 encoded token from n random bytes.
// This is functionally identical to GenerateRandomString and exists as a
// semantic alias for token generation use cases.
// Panics if the system's random number generator fails (extremely rare).
func GenerateToken(n int) string {
	return GenerateRandomString(n)
}
