// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides OSS (non-enterprise) views stubs.
package main

import (
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/services"
)

// InitViewsService returns nil for OSS builds.
// The custom views feature requires an Enterprise license.
func InitViewsService(_ *watcher.RGDWatcher, _ string) services.ViewsService {
	return nil
}
