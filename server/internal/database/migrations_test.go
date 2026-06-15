// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

// TestAdvisoryLockKeyStable guards against accidental edits to the advisory-lock
// constant. The value is a contract shared with the startup runner and the Helm
// migrate Job — if it changes, both startup paths must change in lockstep.
func TestAdvisoryLockKeyStable(t *testing.T) {
	const want int64 = 7242042020042442
	if AdvisoryLockKey != want {
		t.Errorf("AdvisoryLockKey changed: got %d, want %d (see adr-postgres-migration-tooling.md §1)", AdvisoryLockKey, want)
	}
}

// TestMergeSourcesInterleavesByPrefix verifies files from multiple sources are
// folded into one FS keyed by bare filename, so golang-migrate orders them
// globally by numeric prefix regardless of which source they came from.
func TestMergeSourcesInterleavesByPrefix(t *testing.T) {
	ee := MigrationSource{
		Name: "enterprise",
		FS: fstest.MapFS{
			"migrations/0001_a.up.sql":   {Data: []byte("-- 1 up")},
			"migrations/0001_a.down.sql": {Data: []byte("-- 1 down")},
			"migrations/0004_d.up.sql":   {Data: []byte("-- 4 up")},
			"migrations/0004_d.down.sql": {Data: []byte("-- 4 down")},
		},
		Dir: "migrations",
	}
	shared := MigrationSource{
		Name: "shared",
		FS: fstest.MapFS{
			"migrations/0005_e.up.sql":   {Data: []byte("-- 5 up")},
			"migrations/0005_e.down.sql": {Data: []byte("-- 5 down")},
		},
		Dir: "migrations",
	}

	merged, err := mergeSources([]MigrationSource{ee, shared})
	if err != nil {
		t.Fatalf("mergeSources() error = %v", err)
	}

	entries, err := fs.ReadDir(merged, ".")
	if err != nil {
		t.Fatalf("ReadDir(.) error = %v", err)
	}
	if got, want := len(entries), 6; got != want {
		t.Fatalf("merged entry count = %d, want %d", got, want)
	}
	for _, name := range []string{"0001_a.up.sql", "0004_d.up.sql", "0005_e.up.sql"} {
		if _, err := fs.Stat(merged, name); err != nil {
			t.Errorf("expected merged file %q: %v", name, err)
		}
	}
}

// TestMergeSourcesRejectsDuplicateVersion verifies the runner fails fast when
// two different sources contribute the same numeric version.
func TestMergeSourcesRejectsDuplicateVersion(t *testing.T) {
	a := MigrationSource{
		Name: "enterprise",
		FS:   fstest.MapFS{"migrations/0004_seats.up.sql": {Data: []byte("-- ee")}},
		Dir:  "migrations",
	}
	b := MigrationSource{
		Name: "shared",
		FS:   fstest.MapFS{"migrations/0004_identity.up.sql": {Data: []byte("-- shared")}},
		Dir:  "migrations",
	}

	_, err := mergeSources([]MigrationSource{a, b})
	if err == nil {
		t.Fatal("mergeSources() with colliding version 4: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate migration version 4") {
		t.Errorf("error = %q, want it to mention duplicate migration version 4", err.Error())
	}
	if !strings.Contains(err.Error(), "enterprise") || !strings.Contains(err.Error(), "shared") {
		t.Errorf("error = %q, want it to name both colliding sources", err.Error())
	}
}

// TestMergeSourcesAllowsUpDownSameVersion verifies up + down of one version
// within a single source is NOT treated as a collision.
func TestMergeSourcesAllowsUpDownSameVersion(t *testing.T) {
	s := MigrationSource{
		Name: "enterprise",
		FS: fstest.MapFS{
			"migrations/0002_x.up.sql":   {Data: []byte("-- up")},
			"migrations/0002_x.down.sql": {Data: []byte("-- down")},
		},
		Dir: "migrations",
	}
	if _, err := mergeSources([]MigrationSource{s}); err != nil {
		t.Errorf("mergeSources() with up+down of same version: unexpected error %v", err)
	}
}

// TestMergeSourcesEmpty verifies zero sources yields an empty (non-nil) FS,
// matching this story's OSS state where no migrations are contributed.
func TestMergeSourcesEmpty(t *testing.T) {
	merged, err := mergeSources(nil)
	if err != nil {
		t.Fatalf("mergeSources(nil) error = %v", err)
	}
	entries, err := fs.ReadDir(merged, ".")
	if err != nil {
		t.Fatalf("ReadDir(.) error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("merged empty source count = %d, want 0", len(entries))
	}
}

// TestParseVersionRejectsBadPrefix verifies filenames without a numeric prefix
// are rejected and valid prefixes parse.
func TestParseVersionRejectsBadPrefix(t *testing.T) {
	for _, name := range []string{"noprefix.up.sql", "_leading.up.sql", "abcd_x.up.sql"} {
		if _, err := parseVersion(name); err == nil {
			t.Errorf("parseVersion(%q): expected error, got nil", name)
		}
	}
	if v, err := parseVersion("0007_thing.up.sql"); err != nil || v != 7 {
		t.Errorf("parseVersion(0007_thing.up.sql) = %d, %v; want 7, nil", v, err)
	}
}
