// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"log/slog"
	"testing"
)

// TestNormalizeDSN verifies that postgres:// and postgresql:// URLs are converted
// to pgx5:// for golang-migrate's pgx5 driver, while other schemes pass through unchanged.
func TestNormalizeDSN(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"postgres scheme", "postgres://user:pass@host:5432/db", "pgx5://user:pass@host:5432/db"},
		{"postgresql scheme", "postgresql://user:pass@host:5432/db", "pgx5://user:pass@host:5432/db"},
		{"already pgx5", "pgx5://user:pass@host:5432/db", "pgx5://user:pass@host:5432/db"},
		{"unknown scheme passthrough", "mysql://user:pass@host/db", "mysql://user:pass@host/db"},
		{"empty string", "", ""},
		{"sslmode param preserved", "postgres://u:p@h:5432/db?sslmode=disable", "pgx5://u:p@h:5432/db?sslmode=disable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeDSN(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeDSN(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestRunMigrations_BadDSN verifies that runMigrations returns a connection error
// when the DSN is unreachable. Port 1 is connection-refused on every OS.
func TestRunMigrations_BadDSN(t *testing.T) {
	err := runMigrations(context.Background(), "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1", nil, slog.Default())
	if err == nil {
		t.Fatal("expected connection error from bad DSN, got nil")
	}
}
