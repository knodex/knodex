// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"testing/fstest"
)

// AdvisoryLockKey is the pg_advisory_lock identifier acquired by every code path
// that runs migrations against the cluster (startup runner + Helm migrate Job).
// All must use this exact value so concurrent runners serialize.
//
// The constant is a stable 64-bit integer documented in
// _bmad-output/planning-artifacts/architecture/adr-postgres-migration-tooling.md §1.
// Do not change it without updating every consumer in lockstep.
const AdvisoryLockKey int64 = 7242042020042442

// MigrationSource is one contributor of migration files to the merged runner.
// Files are read from Dir within FS and must follow golang-migrate's
// NNNN_title.{up,down}.sql naming, where NNNN is the numeric version prefix.
type MigrationSource struct {
	// Name is a human-readable label used in collision error messages.
	Name string

	// FS holds the migration files (e.g. an embed.FS).
	FS fs.FS

	// Dir is the subdirectory within FS containing the .sql files.
	// An empty Dir means the FS root (".").
	Dir string
}

// mergeSources folds every source's NNNN_*.sql files into a single in-memory
// fs.FS, keyed by bare filename so golang-migrate orders them globally by the
// numeric prefix regardless of which source they came from. It fails fast if
// two different sources contribute the same version number.
func mergeSources(sources []MigrationSource) (fs.FS, error) {
	merged := fstest.MapFS{}
	versionOwner := map[uint]string{}

	for _, s := range sources {
		if s.FS == nil {
			continue
		}
		dir := s.Dir
		if dir == "" {
			dir = "."
		}

		entries, err := fs.ReadDir(s.FS, dir)
		if err != nil {
			return nil, fmt.Errorf("read source %q dir %q: %w", s.Name, dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}
			name := entry.Name()

			version, err := parseVersion(name)
			if err != nil {
				return nil, fmt.Errorf("source %q: %w", s.Name, err)
			}

			// Reject a version owned by a *different* source. Multiple files of
			// the same version within one source (up + down) are expected.
			if owner, ok := versionOwner[version]; ok && owner != s.Name {
				return nil, fmt.Errorf("duplicate migration version %d in sources %q and %q", version, owner, s.Name)
			}
			versionOwner[version] = s.Name

			data, err := fs.ReadFile(s.FS, joinFSPath(dir, name))
			if err != nil {
				return nil, fmt.Errorf("read migration %q from source %q: %w", name, s.Name, err)
			}
			if _, exists := merged[name]; exists {
				return nil, fmt.Errorf("duplicate migration filename %q across sources", name)
			}
			merged[name] = &fstest.MapFile{Data: data}
		}
	}

	return merged, nil
}

// parseVersion extracts the leading numeric version prefix from a migration
// filename (the digits before the first underscore).
func parseVersion(name string) (uint, error) {
	idx := strings.IndexByte(name, '_')
	if idx <= 0 {
		return 0, fmt.Errorf("invalid migration filename %q: missing NNNN_ version prefix", name)
	}
	n, err := strconv.ParseUint(name[:idx], 10, strconv.IntSize)
	if err != nil {
		return 0, fmt.Errorf("invalid migration version prefix in %q: %w", name, err)
	}
	return uint(n), nil
}

// joinFSPath joins a directory and filename for fs.FS lookups. The root
// directory (".") is elided so keys match the on-disk layout iofs expects.
func joinFSPath(dir, name string) string {
	if dir == "." || dir == "" {
		return name
	}
	return dir + "/" + name
}
