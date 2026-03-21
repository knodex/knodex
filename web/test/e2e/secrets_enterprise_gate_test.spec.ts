// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';

/**
 * Secrets Enterprise Gate E2E Tests
 *
 * Tests that secrets endpoints return 402 Payment Required in OSS builds
 * and that the Secrets nav item is absent from the sidebar.
 *
 * These tests run in OSS builds only (skipped when ENTERPRISE_BUILD=true).
 */

test.describe('Secrets Enterprise Gate (OSS)', () => {
  // Only run in OSS builds — enterprise builds have secrets enabled
  test.skip(!!process.env.ENTERPRISE_BUILD, 'OSS-only tests, skipped in enterprise builds');

  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test('AC-6.2: Secrets API returns 402 in OSS build', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');

    const resp = await page.evaluate(async () => {
      const token = localStorage.getItem('jwt_token');
      const res = await fetch('/api/v1/secrets?project=default', {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {},
      });
      return { status: res.status, body: await res.json() };
    });

    expect(resp.status).toBe(402);
    expect(resp.body.code).toBe('ENTERPRISE_REQUIRED');
  });

  test('AC-6.3: Secrets nav item absent in sidebar', async ({ page }) => {
    await page.goto('/catalog');
    // Wait for the catalog content to be visible (signals auth + render complete)
    await page.waitForSelector('nav[aria-label="Main navigation"]', { state: 'visible', timeout: 10000 });

    // In OSS builds, isEnterprise() is false so the Secrets nav link is never
    // rendered in the DOM — checking element count is more reliable than
    // hovering and checking CSS visibility.
    const secretsNavCount = await page.locator('nav a[href="/secrets"]').count();
    expect(secretsNavCount).toBe(0);
  });
});
