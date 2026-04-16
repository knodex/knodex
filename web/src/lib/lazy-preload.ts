// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { lazy, type ComponentType, type LazyExoticComponent } from "react";

/**
 * A lazy component that exposes a `.preload()` method for programmatic prefetching.
 * Calling `.preload()` triggers the dynamic import without rendering the component,
 * so the chunk is ready when the user navigates.
 */
export type PreloadableComponent<T extends ComponentType<unknown>> =
  LazyExoticComponent<T> & {
    preload: () => Promise<{ default: T }>;
  };

/**
 * Wraps `React.lazy()` but attaches the factory function as `.preload()`.
 *
 * Usage:
 * ```ts
 * const MyPage = lazyWithPreload(() => import("./MyPage"));
 * // Later, on hover or idle:
 * MyPage.preload();
 * ```
 */
export function lazyWithPreload<T extends ComponentType<unknown>>(
  factory: () => Promise<{ default: T }>
): PreloadableComponent<T> {
  const Component = lazy(factory) as PreloadableComponent<T>;
  let cached: Promise<{ default: T }> | null = null;
  Component.preload = () => {
    cached ??= factory();
    return cached;
  };
  return Component;
}
