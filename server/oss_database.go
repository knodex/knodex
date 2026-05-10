// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides the OSS (non-enterprise) database initialization stub.
package main

import (
	"context"
	"io"

	"github.com/knodex/knodex/server/internal/config"
)

// InitDatabaseManager is a no-op in OSS builds.
// No PostgreSQL dependency is introduced; the OSS binary has zero DB packages.
func InitDatabaseManager(_ context.Context, _ *config.Config) (io.Closer, error) {
	return nil, nil
}
