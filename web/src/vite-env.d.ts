// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/// <reference types="vite/client" />

/**
 * Enterprise feature flag
 * Defined in vite.config.ts based on build mode
 * - Development: false by default
 * - Enterprise build: true when using `npm run build:enterprise`
 */
declare const __ENTERPRISE__: boolean;

/**
 * Application version injected by Vite at build time from package.json.
 * Managed by Release Please for automated version bumps.
 */
declare const __APP_VERSION__: string;
