// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * My Access self-view E2E (story 17.1) — the "member" baseline of the two-axis
 * application-role model.
 *
 * Asserts the core promise of the story: a freshly authenticated user who is NOT
 * yet a member of any project (the canonical `no-groups@test.local` shape — no
 * group mappings ⇒ no project bindings ⇒ `applicationRole === 'member'`) lands on
 * a coherent self-scoped "My Access" view with an honest empty-state, NOT the old
 * broken-empty 403-walled `/instances` shell.
 *
 * Like auth_session_restore_test.spec.ts and settings_users_test.spec.ts, this
 * drives the flow via route mocking of GET /api/v1/account/info so it exercises
 * the real landing-gate + self-view wiring without depending on a specific
 * cluster-provisioned user. (Live mock-OIDC `no-groups@test.local` coverage is
 * exercised by the batched pre-merge cluster run.)
 */

import { test, expect, Page } from '@playwright/test';
import { generateTestToken } from '../fixture/auth-helper';

// A bare member: authenticated, but with zero global roles and zero projects.
const MEMBER_USER = {
  sub: 'user-bare-member',
  email: 'no-groups@e2e-test.local',
  displayName: 'No Groups Member',
  casbinRoles: [] as string[],
  projects: [] as string[],
  roles: {} as Record<string, string>,
  groups: [] as string[],
};

// Server-authoritative account/info for the bare member. applicationRole is the
// story 17.1 derived value: 'member' because role:serveradmin ∉ casbinRoles.
const MEMBER_ACCOUNT_INFO = {
  userID: MEMBER_USER.sub,
  email: MEMBER_USER.email,
  displayName: MEMBER_USER.displayName,
  groups: [],
  casbinRoles: [],
  projects: [],
  roles: {},
  issuer: 'https://idp.e2e-test.local',
  tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
  tokenIssuedAt: Math.floor(Date.now() / 1000) - 60,
  isOrgAdmin: false,
  applicationRole: 'member',
};

/** Mock GET /api/v1/account/info to return the bare-member payload. */
async function mockMemberAccountInfo(page: Page) {
  await page.route('**/api/v1/account/info', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(MEMBER_ACCOUNT_INFO),
    });
  });
}

/**
 * Mock the surfaces a member would otherwise be funneled into. instances 403s a
 * bare member on a live server; we 403 it here so a regression that drops the
 * landing gate (sending the member to /instances) fails loudly instead of
 * silently rendering an empty list.
 */
async function mockPrivilegedSurfaces(page: Page) {
  await page.route('**/api/v1/instances*', async (route) => {
    await route.fulfill({
      status: 403,
      contentType: 'application/json',
      body: JSON.stringify({ error: { code: 'FORBIDDEN', message: 'permission denied' } }),
    });
  });
  await page.route('**/api/v1/account/can-i/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ value: 'no' }),
    });
  });
}

/** Inject an authenticated session for the bare member (matches auth-helper). */
async function injectMemberSession(page: Page) {
  await page.goto('/login', { waitUntil: 'domcontentloaded' });
  await page.evaluate(() => {
    localStorage.clear();
    sessionStorage.clear();
  });
  const token = await generateTestToken(MEMBER_USER);
  await page.evaluate((token) => {
    localStorage.setItem('jwt_token', token);
    localStorage.setItem(
      'user-storage',
      JSON.stringify({ state: { hasSession: true, currentProject: null }, version: 0 })
    );
  }, token);
}

test.describe('My Access self-view (story 17.1)', () => {
  test('member with no projects lands on My Access, not a 403 wall', async ({ page }) => {
    await mockMemberAccountInfo(page);
    await mockPrivilegedSurfaces(page);
    await injectMemberSession(page);

    // Land on the protected index route — the landing gate should redirect a bare
    // member to the self-view rather than the 403-walled /instances shell.
    await page.goto('/', { waitUntil: 'domcontentloaded' });

    await expect(page).toHaveURL(/\/user-info$/);

    // Self-view renders with identity + honest empty-state.
    await expect(page.getByTestId('my-access-view')).toBeVisible();
    await expect(
      page.getByRole('heading', { name: 'My Access' })
    ).toBeVisible();
    await expect(page.getByText(MEMBER_USER.email)).toBeVisible();

    // Application role badge reads "member".
    await expect(page.getByTestId('application-role')).toHaveText('member');

    // Honest empty-state instead of a 403 wall / empty-instances shell.
    await expect(page.getByTestId('my-access-empty')).toBeVisible();
    await expect(
      page.getByText("You're signed in but not yet a member of any project")
    ).toBeVisible();
  });

  test('account-menu entry navigates to the My Access view', async ({ page }) => {
    await mockMemberAccountInfo(page);
    await mockPrivilegedSurfaces(page);
    await injectMemberSession(page);

    // Start somewhere other than the self-view, then reach it via the account menu.
    await page.goto('/user-info', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('my-access-view')).toBeVisible();

    // Open the user menu and confirm the entry exposes a discoverable "My Access"
    // label wired to the self-view (data-testid kept stable for existing tests).
    await page.getByTestId('user-menu-trigger').click();
    const profileEntry = page.getByTestId('user-menu-profile');
    await expect(profileEntry).toBeVisible();
    await expect(profileEntry).toContainText('My Access');

    await profileEntry.click();
    await expect(page).toHaveURL(/\/user-info$/);
    await expect(page.getByTestId('my-access-view')).toBeVisible();
  });
});
