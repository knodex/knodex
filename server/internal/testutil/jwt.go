// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package testutil

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// DefaultJWTSecret is the default test JWT secret.
const DefaultJWTSecret = "test-jwt-secret-for-unit-tests-32bytes!"

// JWTClaims configures JWT token generation for tests.
type JWTClaims struct {
	Subject     string
	Email       string
	Name        string
	Projects    []string
	CasbinRoles []string
	Groups      []string
	Issuer      string
	Audience    string
	ExpiresIn   time.Duration
}

// GenerateJWT creates a signed HS256 JWT with the given claims and secret.
func GenerateJWT(t *testing.T, opts JWTClaims, secret string) string {
	t.Helper()
	name := opts.Name
	if name == "" && opts.Email != "" {
		name = strings.Split(opts.Email, "@")[0]
	}
	if name == "" && opts.Subject != "" {
		name = strings.Split(opts.Subject, "@")[0]
	}

	expiresIn := opts.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 1 * time.Hour
	}

	issuer := opts.Issuer
	if issuer == "" {
		issuer = "knodex"
	}
	audience := opts.Audience
	if audience == "" {
		audience = "knodex-api"
	}

	claims := jwt.MapClaims{
		"sub":      opts.Subject,
		"email":    opts.Email,
		"name":     name,
		"projects": opts.Projects,
		"iss":      issuer,
		"aud":      audience,
		"exp":      time.Now().Add(expiresIn).Unix(),
		"iat":      time.Now().Unix(),
	}

	if len(opts.CasbinRoles) > 0 {
		claims["casbin_roles"] = opts.CasbinRoles
	}

	if len(opts.Groups) > 0 {
		claims["groups"] = opts.Groups
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal("testutil: failed to sign JWT:", err)
	}
	return tokenString
}

// GenerateSimpleJWT creates a minimal JWT for the given user and projects.
func GenerateSimpleJWT(t *testing.T, email string, projects []string, secret string) string {
	t.Helper()
	return GenerateJWT(t, JWTClaims{
		Subject:  email,
		Email:    email,
		Projects: projects,
	}, secret)
}

// GenerateAdminJWT creates a JWT with role:serveradmin for the given email.
func GenerateAdminJWT(t *testing.T, email string, secret string) string {
	t.Helper()
	return GenerateJWT(t, JWTClaims{
		Subject:     email,
		Email:       email,
		CasbinRoles: []string{"role:serveradmin"},
	}, secret)
}
