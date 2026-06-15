// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * retryImport — run a dynamic-import thunk, retrying a few times on failure.
 *
 * Lazy route/chunk loads fail transiently for boring reasons: a network blip,
 * or (most commonly) a stale hashed chunk after a redeploy. A single cheap retry
 * recovers from nearly all of these before the caller has to surface an error.
 *
 * The caller passes the `() => import(...)` thunk so the literal `import()` stays
 * lexically at the call site, keeping dead-code-elimination / tree-shaking intact.
 */
export async function retryImport<T>(
  importFn: () => Promise<T>,
  opts: { retries?: number; delayMs?: number } = {},
): Promise<T> {
  const retries = opts.retries ?? 1;
  const delayMs = opts.delayMs ?? 500;
  let lastError: unknown;
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      return await importFn();
    } catch (err) {
      lastError = err;
      if (attempt < retries) {
        await new Promise((resolve) => setTimeout(resolve, delayMs));
      }
    }
  }
  throw lastError;
}
