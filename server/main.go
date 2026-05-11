// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"os"
	"strconv"

	"github.com/knodex/knodex/server/app"
	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/logger"
)

// shouldMigrateOnly reports whether the binary was invoked in migrate-only
// mode (Helm pre-install/pre-upgrade Job entrypoint). Env var takes precedence
// over the CLI flag. The Helm Job uses the CLI flag (args: ["--migrate-only"]);
// the env var (KNODEX_MIGRATE_ONLY=true) is available for manual invocations.
//
// The flag.NewFlagSet/io.Discard pattern is used (rather than flag.Parse) so
// regular invocations that pass other args don't trip on an unknown flag.
func shouldMigrateOnly() bool {
	if v, ok := os.LookupEnv("KNODEX_MIGRATE_ONLY"); ok {
		// strconv.ParseBool("") returns (false, error); empty env var is falsy.
		b, _ := strconv.ParseBool(v)
		return b
	}
	fs := flag.NewFlagSet("knodex-server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	migrate := fs.Bool("migrate-only", false, "run pending DB migrations and exit (enterprise only)")
	_ = fs.Parse(os.Args[1:])
	return *migrate
}

func main() {
	// Capture migrate-only intent before config.Load(). Note: config.Load()
	// still runs unconditionally below — RunMigrationsOnly needs cfg.Database.URL.
	migrateOnly := shouldMigrateOnly()

	// Load configuration (needed for logger setup and for RunMigrationsOnly)
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

	// Helm pre-install/pre-upgrade Job entrypoint: run migrations and exit
	// without booting the HTTP server. OSS builds return a clear error here.
	if migrateOnly {
		if err := RunMigrationsOnly(context.Background(), cfg); err != nil {
			slog.Error("migrate-only failed", "error", err)
			os.Exit(1)
		}
		slog.Info("migrate-only completed successfully")
		os.Exit(0)
	}

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
	a.SetDatabaseManagerInitFunc(InitDatabaseManager)

	if err := a.Run(context.Background()); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
