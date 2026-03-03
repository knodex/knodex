// Package main provides OSS (non-enterprise) views stubs.
package main

import (
	"github.com/provops-org/knodex/server/internal/services"
	"github.com/provops-org/knodex/server/internal/watcher"
)

// InitViewsService returns nil for OSS builds.
// The custom views feature requires an Enterprise license.
func InitViewsService(_ *watcher.RGDWatcher, _ string) services.ViewsService {
	return nil
}
