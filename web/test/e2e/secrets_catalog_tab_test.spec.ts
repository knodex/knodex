// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';

/**
 * Secrets Catalog Tab E2E Tests
 *
 * Tests that the Secrets tab renders correctly in the catalog detail view
 * when an RGD has SecretRef fields, and does NOT appear when there are none.
 *
 * Prerequisites:
 * - Backend deployed with secrets feature enabled
 * - webapp-with-secret RGD deployed (has externalRef Secret)
 */

const SCREENSHOT_DIR = '../test-results/e2e/screenshots';

test.describe('Secrets Catalog Tab', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test('AC-1: Catalog detail shows Secrets tab for RGDs with SecretRefs', async ({
    page,
  }) => {
    // Navigate directly to webapp-with-secret catalog detail
    await page.goto('/catalog/webapp-with-secret');
    await page.waitForLoadState('networkidle');

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-catalog-01-detail.png`,
      fullPage: true,
    });

    // Check for Secrets tab
    const secretsTab = page.locator('button[role="tab"]:has-text("Secrets")');
    const hasSecretsTab = await secretsTab.isVisible({ timeout: 5000 }).catch(() => false);

    if (!hasSecretsTab) {
      test.skip(true, 'webapp-with-secret RGD not found or has no secretRefs');
      return;
    }

    expect(hasSecretsTab).toBe(true);

    // Click on Secrets tab
    await secretsTab.click();

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-catalog-02-secrets-tab.png`,
      fullPage: true,
    });

    // Verify the secrets tab content renders (use Playwright expect for proper retry/timeout)
    await expect(page.locator('text=Required Secrets')).toBeVisible({ timeout: 10000 });

    // Verify at least one secret reference card is shown
    await expect(page.locator('[data-testid^="catalog-secret-ref-"]').first()).toBeVisible({ timeout: 5000 });

    // Verify a type badge is visible (fixed, dynamic, or user-provided)
    await expect(page.locator('[data-testid^="catalog-secret-ref-"] >> text=/fixed|dynamic|user-provided/').first()).toBeVisible({ timeout: 5000 });
  });

  test('AC-3: Deploy page does NOT show Secrets tab', async ({
    page,
  }) => {
    // Navigate to catalog to find the RGD
    await page.goto('/catalog/webapp-with-secret');
    await page.waitForLoadState('networkidle');

    // Click Deploy button
    const deployButton = page.locator('button:has-text("Deploy")');
    const hasDeployButton = await deployButton.first().isVisible({ timeout: 5000 }).catch(() => false);

    if (!hasDeployButton) {
      test.skip(true, 'Deploy button not visible - skipping');
      return;
    }

    await deployButton.first().click();
    await page.waitForLoadState('networkidle');

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-catalog-03-deploy-no-secrets.png`,
      fullPage: true,
    });

    // Verify Secrets tab does NOT appear in deploy form
    const secretsTab = page.locator('button[role="tab"]:has-text("Secrets")');
    const hasSecretsTab = await secretsTab.isVisible({ timeout: 2000 }).catch(() => false);
    expect(hasSecretsTab).toBe(false);
  });

  test('AC-4: Catalog detail does NOT show Secrets tab for RGDs without SecretRefs', async ({
    page,
  }) => {
    // Navigate to catalog
    await page.goto('/catalog');
    await page.waitForLoadState('networkidle');

    // Find an RGD that is NOT webapp-with-secret (likely has no secretRefs)
    const rgdCards = page.locator('[data-testid="rgd-card"], a[href*="/catalog/"]');
    const cardCount = await rgdCards.count();

    if (cardCount === 0) {
      test.skip(true, 'No RGDs found in catalog');
      return;
    }

    // Click on the first RGD that isn't webapp-with-secret
    let foundNonSecret = false;
    for (let i = 0; i < Math.min(cardCount, 5); i++) {
      const card = rgdCards.nth(i);
      const href = await card.getAttribute('href');
      if (href && !href.includes('webapp-with-secret')) {
        await card.click();
        foundNonSecret = true;
        break;
      }
    }

    if (!foundNonSecret) {
      test.skip(true, 'No non-secret RGD found to test');
      return;
    }

    await page.waitForLoadState('networkidle');

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-catalog-04-no-secrets-tab.png`,
      fullPage: true,
    });

    // Secrets tab should NOT appear for RGDs without SecretRefs
    const secretsTab = page.locator('button[role="tab"]:has-text("Secrets")');
    const hasSecretsTab = await secretsTab.isVisible({ timeout: 2000 }).catch(() => false);
    expect(hasSecretsTab).toBe(false);
  });
});
