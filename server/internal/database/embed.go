// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"embed"
)

// sharedMigrationsFS holds the edition-neutral migration SQL applied on every
// edition (OSS, Enterprise, Cloud). The embed declaration must be co-located
// with the .sql files it embeds. The first file landed in Story 15.2
// (0005_create_identity_schema); do not remove the last .sql file without also
// removing this directive (an empty embed glob fails to compile).
//
//go:embed migrations/*.sql
var sharedMigrationsFS embed.FS

// SharedMigrationSource returns the edition-neutral migration source for the
// shared runner (NewManager via Config.MigrationSources). The composition root
// supplies it on every edition; EE additionally appends the enterprise source.
func SharedMigrationSource() MigrationSource {
	return MigrationSource{
		Name: "shared",
		FS:   sharedMigrationsFS,
		Dir:  "migrations",
	}
}
