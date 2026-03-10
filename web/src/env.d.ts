// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/// <reference types="vite/client" />

/**
 * Enterprise Edition flag injected by Vite at build time.
 *
 * This constant is defined in vite.config.ts via the `define` option:
 * - OSS builds: __ENTERPRISE__ = false
 * - Enterprise builds: __ENTERPRISE__ = true (when mode='enterprise')
 *
 * Usage:
 * ```typescript
 * if (__ENTERPRISE__) {
 *   // Dynamic import for tree-shaking in OSS builds
 *   const { CompliancePanel } = await import('@/ee/components/CompliancePanel');
 * }
 * ```
 */
declare const __ENTERPRISE__: boolean;

/**
 * Application version injected by Vite at build time from package.json.
 * Managed by Release Please for automated version bumps.
 */
declare const __APP_VERSION__: string;
