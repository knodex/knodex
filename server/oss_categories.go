// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides the OSS categories service initialization.
package main

import (
	"github.com/knodex/knodex/server/internal/categories"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/services"
)

// InitCategoryService initializes the OSS categories service.
// Categories are auto-discovered from knodex.io/category annotations on live RGDs
// in the watcher's in-memory cache. No ConfigMap or enterprise license required.
func InitCategoryService(rgdWatcher *watcher.RGDWatcher) services.CategoryService {
	return categories.NewService(rgdWatcher, nil)
}
