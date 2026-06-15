// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package database provides the shared PostgreSQL plumbing used by every edition
// (OSS, Enterprise, Cloud): a pgxpool-based connection manager, a dual-source
// schema-migration runner, and the CheckoutWithOrg RLS helper. It was promoted
// (behavior-preserving) from server/ee/database so a single Postgres data plane
// can back all editions. Edition-specific migration SQL is supplied to the
// runner via Config.MigrationSources by the composition root; this package
// embeds no SQL of its own.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds PostgreSQL connection configuration for the database manager.
// The audit, compliance, and identity pools all connect to the same URL with
// independent budgets (NFR-PG4).
//
// Connection budget: NewManager opens 1 advisory-lock connection (released after migrations)
// plus AuditMaxConns + ComplianceMaxConns + IdentityMaxConns pooled connections per replica.
// With defaults (10 + 10 + 10), ~3 replicas approach Postgres' default max_connections=100.
// Operators scaling beyond that should raise Postgres max_connections or reduce these limits.
//
// The identity pool backs the edition-neutral identity store (Story 15.2) and is opened on
// every edition. The audit and compliance pools are EE concerns; on OSS they connect to the
// same URL but their schemas are not migrated (the EE migration source is omitted) — they
// stay idle.
type Config struct {
	URL                string
	AuditMaxConns      int32 // Default 10 when zero
	ComplianceMaxConns int32 // Default 10 when zero
	IdentityMaxConns   int32 // Default 10 when zero

	// MigrationSources is the ordered set of migration sources the runner merges
	// (by numeric filename prefix) before applying. The composition root supplies
	// each edition's source(s); an empty slice applies no migrations.
	MigrationSources []MigrationSource
}

// TokenProvider abstracts IAM-based token refresh for managed Postgres services
// (AWS RDS IAM, Cloud SQL IAM, Azure DB AAD). Callers implement; no provider
// code ships in this story.
type TokenProvider interface {
	Token(ctx context.Context, dsn string) (string, error)
}

// Manager holds connection pools for the audit, compliance, and identity subsystems.
type Manager struct {
	audit      *pgxpool.Pool
	compliance *pgxpool.Pool
	identity   *pgxpool.Pool
}

// globalManager is the package-level registry used by the EE audit/compliance/license stores.
// atomic.Pointer makes RegisterManager/GetManager safe under concurrent access
// (e.g., handler goroutines reading while a hot-reload writes).
var globalManager atomic.Pointer[Manager]

// RegisterManager stores m as the package-level manager for later retrieval.
// Safe to call concurrently; pass nil to clear the registration.
func RegisterManager(m *Manager) { globalManager.Store(m) }

// GetManager returns the registered manager, or nil if RegisterManager has not been called.
// Safe to call concurrently.
func GetManager() *Manager { return globalManager.Load() }

// NewManager runs schema migrations under advisory lock, then opens and pings
// three connection pools (audit, compliance, identity) backed by the same
// DATABASE_URL. On OSS only the identity pool's schema is migrated/used; the
// audit and compliance pools stay idle (their EE migration source is omitted).
func NewManager(ctx context.Context, cfg Config) (*Manager, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database URL must not be empty")
	}
	if cfg.AuditMaxConns == 0 {
		cfg.AuditMaxConns = 10
	}
	if cfg.ComplianceMaxConns == 0 {
		cfg.ComplianceMaxConns = 10
	}
	if cfg.IdentityMaxConns == 0 {
		cfg.IdentityMaxConns = 10
	}

	if err := runMigrations(ctx, cfg.URL, cfg.MigrationSources, slog.Default()); err != nil {
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	auditPool, err := openPool(ctx, cfg.URL, "audit", cfg.AuditMaxConns)
	if err != nil {
		return nil, err
	}

	compliancePool, err := openPool(ctx, cfg.URL, "compliance", cfg.ComplianceMaxConns)
	if err != nil {
		auditPool.Close()
		return nil, err
	}

	identityPool, err := openPool(ctx, cfg.URL, "identity", cfg.IdentityMaxConns)
	if err != nil {
		auditPool.Close()
		compliancePool.Close()
		return nil, err
	}

	slog.Info("database connection pools established",
		"audit_max_conns", cfg.AuditMaxConns,
		"compliance_max_conns", cfg.ComplianceMaxConns,
		"identity_max_conns", cfg.IdentityMaxConns,
	)
	return &Manager{audit: auditPool, compliance: compliancePool, identity: identityPool}, nil
}

// openPool parses the URL, overrides MaxConns, opens the pool, and pings it.
func openPool(ctx context.Context, url, subsystem string, maxConns int32) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse %s database URL: %w", subsystem, err)
	}
	poolCfg.MaxConns = maxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("open %s pool: %w", subsystem, err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping %s database: %w", subsystem, err)
	}
	return pool, nil
}

// Close closes both connection pools. Implements io.Closer.
// Nil-guarded so a zero-value Manager (used in unit tests via fake constructors)
// can be safely closed without panicking.
func (m *Manager) Close() error {
	if m.audit != nil {
		m.audit.Close()
	}
	if m.compliance != nil {
		m.compliance.Close()
	}
	if m.identity != nil {
		m.identity.Close()
	}
	return nil
}

// AuditPool returns the connection pool for the audit subsystem.
func (m *Manager) AuditPool() *pgxpool.Pool { return m.audit }

// CompliancePool returns the connection pool for the compliance subsystem.
func (m *Manager) CompliancePool() *pgxpool.Pool { return m.compliance }

// IdentityPool returns the connection pool for the edition-neutral identity
// subsystem (Story 15.2). Backs the base identity store on every edition.
func (m *Manager) IdentityPool() *pgxpool.Pool { return m.identity }
