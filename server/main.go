// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/knodex/knodex/server/app"
	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/logger"
)

func main() {
	// Load configuration first (needed for logger setup)
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize structured logging with configuration
	logger.Setup(&cfg.Log)
	slog.Info("logger initialized",
		"level", cfg.Log.Level,
		"format", cfg.Log.Format,
	)

	// Create composable application container
	a := app.New(cfg)

	// Set enterprise services via build-tag dispatch functions.
	// In OSS builds, these return nil/noop. In EE builds, these initialize real services.
	a.SetOrganizationFilter(InitOrganizationFilter(cfg))
	a.SetLicenseService(InitLicenseService(cfg.License.Path, cfg.License.Text))

	// Register init functions for services that need runtime dependencies (wsHub, redisClient, rgdWatcher).
	// These are called during Run() after those dependencies are created.
	a.SetComplianceInitFunc(InitComplianceService)
	a.SetViolationHistoryInitFunc(InitViolationHistoryService)
	a.SetCategoryInitFunc(InitCategoryService)
	a.SetAuditRecorderInitFunc(InitAuditRecorder)
	a.SetAuditLoginMiddlewareInitFunc(InitAuditLoginMiddleware)
	a.SetAuditMiddlewareInitFunc(InitAuditMiddleware)
	a.SetAuditAPIServiceInitFunc(InitAuditAPIService)

	if err := a.Run(context.Background()); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
