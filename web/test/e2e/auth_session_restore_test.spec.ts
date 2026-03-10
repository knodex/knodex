// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * E2E tests for server-validated session restore on page refresh.
 *
 * Verifies the two-phase session restore flow:
 * Phase 1: Sync localStorage check (hasPersistedSession) — renders dashboard chrome immediately
 * Phase 2: Async server validation via GET /api/v1/account/info — populates user state
 *
 * These tests use route mocking to control the /api/v1/account/info response,
 * simulating both successful and failed session restore scenarios.
 */

import { test, expect, Page } from '@playwright/test';
import { setupAuth, setupAuthAndNavigate, TEST_USERS, TestUserRole } from '../fixture/auth-helper';

// Mock account info response matching AccountInfoResponse type
const MOCK_ACCOUNT_INFO = {
  userID: TEST_USERS[TestUserRole.GLOBAL_ADMIN].sub,
  email: TEST_USERS[TestUserRole.GLOBAL_ADMIN].email,
  displayName: TEST_USERS[TestUserRole.GLOBAL_ADMIN].displayName,
  groups: [],
  casbinRoles: TEST_USERS[TestUserRole.GLOBAL_ADMIN].casbinRoles,
  projects: TEST_USERS[TestUserRole.GLOBAL_ADMIN].projects,
  roles: {},
  issuer: 'knodex',
  tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
  tokenIssuedAt: Math.floor(Date.now() / 1000) - 60,
};

/** Mock the common dashboard API endpoints with empty responses */
async function mockDashboardAPIs(page: Page) {
  await page.route('**/api/v1/rgds*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [] }),
    });
  });
  await page.route('**/api/v1/projects*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [] }),
    });
  });
  await page.route('**/api/v1/instances*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [] }),
    });
  });
  await page.route('**/api/v1/account/can-i/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ value: 'yes' }),
    });
  });
}

/** Mock account info endpoint to return success with MOCK_ACCOUNT_INFO */
async function mockAccountInfoSuccess(page: Page) {
  await page.route('**/api/v1/account/info', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(MOCK_ACCOUNT_INFO),
    });
  });
}

test.describe('Session Restore on Page Refresh', () => {
  test.beforeEach(async ({ page, context }) => {
    await context.clearCookies();
    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => {
      localStorage.clear();
      sessionStorage.clear();
    });
  });

  test('authenticated user stays on dashboard after page refresh', async ({ page }) => {
    await setupAuth(page, TestUserRole.GLOBAL_ADMIN);
    await mockAccountInfoSuccess(page);
    await mockDashboardAPIs(page);

    // Navigate to catalog (simulates initial page load after login)
    await page.goto('/catalog', { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle');

    // Verify we're on the catalog page, not redirected to login
    await expect(page).toHaveURL('/catalog');

    // Simulate page refresh — the critical scenario this feature fixes
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle');

    // After refresh, should still be on catalog (session restored via API)
    await expect(page).toHaveURL('/catalog');
  });

  test('user with expired server session is redirected to login on refresh', async ({ page }) => {
    await setupAuth(page, TestUserRole.GLOBAL_ADMIN);

    // Mock account info to return 401 (session expired on server)
    await page.route('**/api/v1/account/info', async (route) => {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'UNAUTHORIZED', message: 'session expired' }),
      });
    });

    // Navigate to a protected route
    await page.goto('/catalog', { waitUntil: 'domcontentloaded' });

    // Should be redirected to login because server returned 401
    await page.waitForURL('**/login', { timeout: 10000 });
    await expect(page).toHaveURL('/login');
  });

  test('user with no localStorage session marker is immediately redirected', async ({ page }) => {
    // Don't set any auth — localStorage is clean from beforeEach

    // Try to access a protected route
    await page.goto('/catalog', { waitUntil: 'domcontentloaded' });

    // Should immediately redirect to login (Phase 1: sync check, no API call needed)
    await page.waitForURL('**/login', { timeout: 5000 });
    await expect(page).toHaveURL('/login');
  });

  test('shows connection error when server is unreachable during restore', async ({ page }) => {
    await setupAuth(page, TestUserRole.GLOBAL_ADMIN);

    // Mock account info to return network error (500)
    await page.route('**/api/v1/account/info', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'INTERNAL_ERROR', message: 'internal server error' }),
      });
    });

    // Navigate to a protected route
    await page.goto('/catalog', { waitUntil: 'domcontentloaded' });

    // Should show the connection error state in the content area
    // DashboardLayout renders "Connection Error" for sessionStatus === 'error'
    await expect(page.getByText('Connection Error')).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole('button', { name: /retry/i })).toBeVisible();
  });

  test('dashboard chrome renders immediately while session validates', async ({ page }) => {
    await setupAuth(page, TestUserRole.GLOBAL_ADMIN);

    // Mock account info with a slow response to observe the loading state
    await page.route('**/api/v1/account/info', async (route) => {
      // Delay response to keep us in 'validating' state
      await new Promise((resolve) => setTimeout(resolve, 2000));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_ACCOUNT_INFO),
      });
    });

    // Navigate to a protected route
    await page.goto('/catalog', { waitUntil: 'domcontentloaded' });

    // Phase 1 passed (hasPersistedSession = true), so dashboard chrome should render
    // The sidebar and top bar should be visible even during validation
    // Content area should show a loading spinner (Loader2 component)
    await expect(page.locator('#main-content')).toBeVisible({ timeout: 5000 });

    // We should NOT be on the login page
    await expect(page).not.toHaveURL('/login');
  });

  test('retry button re-triggers session restore after error', async ({ page }) => {
    await setupAuth(page, TestUserRole.GLOBAL_ADMIN);

    let callCount = 0;

    // First call fails, second succeeds (after retry)
    await page.route('**/api/v1/account/info', async (route) => {
      callCount++;
      if (callCount === 1) {
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ code: 'INTERNAL_ERROR', message: 'temporary failure' }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_ACCOUNT_INFO),
        });
      }
    });

    await mockDashboardAPIs(page);

    // Navigate to a protected route
    await page.goto('/catalog', { waitUntil: 'domcontentloaded' });

    // Wait for error state
    await expect(page.getByText('Connection Error')).toBeVisible({ timeout: 10000 });

    // Click retry
    await page.getByRole('button', { name: /retry/i }).click();

    // After retry, the session restore should succeed and render the outlet
    // The error message should disappear
    await expect(page.getByText('Connection Error')).not.toBeVisible({ timeout: 10000 });
  });

  test('clearing localStorage after login forces redirect on refresh', async ({ page }) => {
    await mockAccountInfoSuccess(page);
    await mockDashboardAPIs(page);

    await setupAuthAndNavigate(page, TestUserRole.GLOBAL_ADMIN, '/catalog');
    await expect(page).toHaveURL('/catalog');

    // Clear localStorage (simulates user clearing browser data)
    await page.evaluate(() => {
      localStorage.clear();
    });

    // Refresh the page
    await page.reload({ waitUntil: 'domcontentloaded' });

    // Should redirect to login (hasPersistedSession() returns false)
    await page.waitForURL('**/login', { timeout: 5000 });
    await expect(page).toHaveURL('/login');
  });
});
