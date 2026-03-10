// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package hash provides cryptographic hashing utilities for content
// fingerprinting, ID generation, and deterministic hashing.
//
// All functions use SHA-256. This package consolidates hashing patterns
// previously duplicated across auth/provisioning, drift, deployment/vcs,
// repository, and rbac packages, following the ArgoCD util/ package structure.
//
// Usage:
//
//	digest := hash.SHA256(data)
//	id := hash.Truncate(hash.SHA256String("input"), 12)
//	fingerprint := hash.ContentHash("part1", "part2")
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SHA256 returns the hex-encoded SHA-256 hash of the given data.
func SHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SHA256String returns the hex-encoded SHA-256 hash of a string.
func SHA256String(s string) string {
	return SHA256([]byte(s))
}

// Truncate safely truncates a hash string to the given length.
// If the hash is shorter than length, it is returned unchanged.
func Truncate(hash string, length int) string {
	if length <= 0 {
		return ""
	}
	if len(hash) <= length {
		return hash
	}
	return hash[:length]
}

// ContentHash computes a SHA-256 hash of multiple string parts concatenated
// with newline separators. Useful for creating composite content fingerprints
// (e.g., idempotency keys).
func ContentHash(parts ...string) string {
	combined := strings.Join(parts, "\n")
	return SHA256String(combined)
}
