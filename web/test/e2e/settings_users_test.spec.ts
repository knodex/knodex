// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';

/**
 * Settings → Users (roster) E2E Tests — Story 16.1 / UM-1.
 *
 * Read-only roster page consuming the frozen 15.8 Users API
 * (GET /api/v1/users). Mirrors sso_settings_test.spec.ts:
 *  - Global Admin (operator) navigates from the Settings hub and sees the list.
 *  - Org Viewer (non-operator) gets a 403 and sees Access Denied, not the table.
 */

// Matches /api/v1/users with or without a query string (the React Query hook
// appends ?limit=...), but not /api/v1/users/{id}.
const USERS_LIST_ROUTE = /\/api\/v1\/users(\?.*)?$/;

const MOCK_ROSTER = {
  users: [
    {
      id: 'u-alice',
      email: 'alice@e2e-test.local',
      displayName: 'Alice Admin',
      state: 'active',
      isInactive: false,
      applicationRole: 'serveradmin',
      firstSeenAt: '2026-01-15T10:00:00Z',
      lastSeenAt: '2026-06-01T10:00:00Z',
      federatedIdentities: [
        {
          issuer: 'https://idp.e2e-test.local',
          sub: 'alice-sub',
          providerKind: 'oidc',
          sourceKind: 'oidc_jit',
          createdAt: '2026-01-15T10:00:00Z',
          updatedAt: '2026-06-01T10:00:00Z',
        },
      ],
    },
    {
      id: 'u-bob',
      email: 'bob@e2e-test.local',
      displayName: 'Bob Builder',
      state: 'removed',
      isInactive: false,
      applicationRole: 'member',
      firstSeenAt: '2026-02-01T10:00:00Z',
      lastSeenAt: '2026-05-01T10:00:00Z',
      federatedIdentities: [
        {
          issuer: 'https://idp.e2e-test.local',
          sub: 'bob-sub',
          providerKind: 'oidc',
          sourceKind: 'oidc_jit',
          createdAt: '2026-02-01T10:00:00Z',
          updatedAt: '2026-05-01T10:00:00Z',
        },
      ],
    },
  ],
};

test.describe('Global Admin - Settings Users roster', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test('AC1/AC2: Operator navigates to Users from the Settings hub and sees the roster', async ({ page }) => {
    await page.route(USERS_LIST_ROUTE, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_ROSTER),
      });
    });

    // Settings hub
    await page.goto('/settings');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    await expect(page.getByTestId('topbar-breadcrumb-leaf')).toHaveText('Settings', {
      timeout: 10000,
    });

    // Users nav link in the SettingsLayout sidebar
    const usersNav = page.getByRole('link', { name: 'Users' }).first();
    await expect(usersNav).toBeVisible({ timeout: 5000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-01-settings-hub.png',
      fullPage: true,
    });

    await usersNav.click();
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Breadcrumb leaf identifies the page
    await expect(page.getByTestId('topbar-breadcrumb-leaf')).toHaveText('Users', {
      timeout: 10000,
    });

    // Roster rows render
    await expect(page.getByText('alice@e2e-test.local')).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('bob@e2e-test.local')).toBeVisible();
    await expect(page.getByText('Alice Admin')).toBeVisible();

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-01-users-list.png',
      fullPage: true,
    });

    console.log('✓ AC1/AC2: Operator can navigate to Users and see the roster');
  });

  // ─── Story 17.3: read-only Application role column (Path A) ───
  test('17.3 AC4/AC6: Application role column shows serveradmin vs member badges with NO mutate control', async ({
    page,
  }) => {
    await page.route(USERS_LIST_ROUTE, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_ROSTER),
      });
    });

    await page.goto('/settings/users');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Users are rendered as a list (not a table), so there is no column header.
    // Verify badges are present inline in each user row.

    // Alice (serveradmin) and Bob (member) carry the correct read-only badges.
    const aliceRow = page.getByTestId('user-row-u-alice');
    await expect(aliceRow.getByTestId('user-app-role-badge')).toHaveText(
      'Server admin',
    );
    const bobRow = page.getByTestId('user-row-u-bob');
    await expect(bobRow.getByTestId('user-app-role-badge')).toHaveText('Member');

    // Path A: there is NO control to change a user's application role anywhere
    // on the page — only the read-only badge.
    await expect(page.getByTestId('user-app-role-select')).toHaveCount(0);
    await expect(
      aliceRow.getByRole('combobox', { name: /role/i }),
    ).toHaveCount(0);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-17-3-app-role-column.png',
      fullPage: true,
    });

    console.log('✓ 17.3 AC4/AC6: read-only Application role column, no mutate affordance');
  });
});

// Story 16.3 / UM-3: a roster mixing isInactive true/false and state
// active/removed, so the inactive badge + search + state/inactive filters all
// have something to narrow. Carol is the only inactive user; Dave is removed.
const MOCK_ROSTER_16_3 = {
  users: [
    {
      id: 'u-alice',
      email: 'alice@e2e-test.local',
      displayName: 'Alice Admin',
      state: 'active',
      isInactive: false,
      firstSeenAt: '2026-01-15T10:00:00Z',
      lastSeenAt: '2026-06-01T10:00:00Z',
      federatedIdentities: [
        {
          issuer: 'https://idp.e2e-test.local',
          sub: 'alice-sub',
          providerKind: 'oidc',
          sourceKind: 'oidc_jit',
          createdAt: '2026-01-15T10:00:00Z',
          updatedAt: '2026-06-01T10:00:00Z',
        },
      ],
      applicationRole: 'serveradmin',
    },
    {
      id: 'u-carol',
      email: 'carol@e2e-test.local',
      displayName: 'Carol Idle',
      state: 'active',
      isInactive: true,
      applicationRole: 'member',
      firstSeenAt: '2026-01-10T10:00:00Z',
      lastSeenAt: '2026-03-01T10:00:00Z',
      federatedIdentities: [
        {
          issuer: 'https://idp.e2e-test.local',
          sub: 'carol-sub',
          providerKind: 'oidc',
          sourceKind: 'oidc_jit',
          createdAt: '2026-01-10T10:00:00Z',
          updatedAt: '2026-03-01T10:00:00Z',
        },
      ],
    },
    {
      id: 'u-dave',
      email: 'dave@e2e-test.local',
      displayName: 'Dave Removed',
      state: 'removed',
      isInactive: false,
      applicationRole: 'member',
      firstSeenAt: '2026-02-01T10:00:00Z',
      lastSeenAt: '2026-05-01T10:00:00Z',
      federatedIdentities: [
        {
          issuer: 'https://idp.e2e-test.local',
          sub: 'dave-sub',
          providerKind: 'oidc',
          sourceKind: 'oidc_jit',
          createdAt: '2026-02-01T10:00:00Z',
          updatedAt: '2026-05-01T10:00:00Z',
        },
      ],
    },
  ],
};

test.describe('Global Admin - Settings Users inactive badge + filters (Story 16.3)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test.beforeEach(async ({ page }) => {
    await page.route(USERS_LIST_ROUTE, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_ROSTER_16_3),
      });
    });
  });

  test('AC1: an inactive user shows the Inactive badge; active users do not', async ({ page }) => {
    await page.goto('/settings/users');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // All three rows render.
    await expect(page.getByTestId('user-row-u-alice')).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId('user-row-u-carol')).toBeVisible();
    await expect(page.getByTestId('user-row-u-dave')).toBeVisible();

    // The Inactive badge appears on Carol's row only.
    await expect(
      page.getByTestId('user-row-u-carol').getByTestId('user-inactive-badge'),
    ).toBeVisible();
    await expect(
      page.getByTestId('user-row-u-alice').getByTestId('user-inactive-badge'),
    ).toHaveCount(0);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-16-3-inactive-badge.png',
      fullPage: true,
    });

    console.log('✓ AC1: inactive badge renders for the idle user only');
  });

  test('AC2/AC4: search narrows the visible rows and updates the ListFooter count', async ({ page }) => {
    await page.goto('/settings/users');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    const footer = page.getByTestId('users-list-footer');
    await expect(footer).toBeVisible({ timeout: 10000 });
    // Unfiltered: 3 users loaded.
    await expect(footer).toContainText('3 users');

    // Search for "carol" — narrows to one row.
    await page.getByTestId('users-search').fill('carol');

    await expect(page.getByTestId('user-row-u-carol')).toBeVisible();
    await expect(page.getByTestId('user-row-u-alice')).toHaveCount(0);
    await expect(page.getByTestId('user-row-u-dave')).toHaveCount(0);

    // Footer reflects the FILTERED set, not the loaded set.
    await expect(footer).toContainText('1 users');
    await expect(footer).not.toContainText('3 users');

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-16-3-search-narrows.png',
      fullPage: true,
    });

    console.log('✓ AC2/AC4: search narrows rows and the footer count follows');
  });

  test('AC3/AC4: the inactive-only filter narrows rows and the footer follows', async ({ page }) => {
    await page.goto('/settings/users');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    const footer = page.getByTestId('users-list-footer');
    await expect(footer).toBeVisible({ timeout: 10000 });
    await expect(footer).toContainText('3 users');

    // Open the Filters dropdown and engage the inactive-only toggle.
    await page.getByRole('button', { name: 'Filters' }).click();
    await page.getByTestId('users-inactive-filter').click();

    // Only the inactive user (Carol) remains.
    await expect(page.getByTestId('user-row-u-carol')).toBeVisible();
    await expect(page.getByTestId('user-row-u-alice')).toHaveCount(0);
    await expect(page.getByTestId('user-row-u-dave')).toHaveCount(0);

    // The dropdown count badge reflects one engaged filter.
    await expect(page.getByTestId('filters-active-count')).toHaveText('1');
    // Footer reflects the filtered set.
    await expect(footer).toContainText('1 users');

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-16-3-inactive-filter.png',
      fullPage: true,
    });

    console.log('✓ AC3/AC4: inactive-only filter narrows rows and the footer follows');
  });
});

// Story 16.2 / UM-2: seat-usage widget + per-row reclaim. A roster with two
// ACTIVE users so reclaiming one drops the active count (and the seat widget's
// `used`) by exactly one. The license mock carries a live seat snapshot.
//
// NOTE: the seat widget is EE-only (the license query is enabled: isEnterprise()
// and OSS omits `seats`). Per the sibling stories' pattern the LIVE run is
// batched to epic pre-merge QA on an ENTERPRISE_BUILD=true deployment; this
// dev-story delivers a parsing, --list-discovered spec.
const RECLAIM_ROSTER_ACTIVE = {
  users: [
    {
      id: 'u-alice',
      email: 'alice@e2e-test.local',
      displayName: 'Alice Admin',
      state: 'active',
      isInactive: false,
      applicationRole: 'serveradmin',
      firstSeenAt: '2026-01-15T10:00:00Z',
      lastSeenAt: '2026-06-01T10:00:00Z',
      federatedIdentities: [
        {
          issuer: 'https://idp.e2e-test.local',
          sub: 'alice-sub',
          providerKind: 'oidc',
          sourceKind: 'oidc_jit',
          createdAt: '2026-01-15T10:00:00Z',
          updatedAt: '2026-06-01T10:00:00Z',
        },
      ],
    },
    {
      id: 'u-bob',
      email: 'bob@e2e-test.local',
      displayName: 'Bob Builder',
      state: 'active',
      isInactive: false,
      applicationRole: 'member',
      firstSeenAt: '2026-02-01T10:00:00Z',
      lastSeenAt: '2026-05-01T10:00:00Z',
      federatedIdentities: [
        {
          issuer: 'https://idp.e2e-test.local',
          sub: 'bob-sub',
          providerKind: 'oidc',
          sourceKind: 'oidc_jit',
          createdAt: '2026-02-01T10:00:00Z',
          updatedAt: '2026-05-01T10:00:00Z',
        },
      ],
    },
  ],
};

// The same roster after Alice's seat is reclaimed: she flips to `removed`
// (still listed under the default "all" filter), leaving one active user.
const RECLAIM_ROSTER_AFTER = {
  users: [
    { ...RECLAIM_ROSTER_ACTIVE.users[0], state: 'removed' },
    RECLAIM_ROSTER_ACTIVE.users[1],
  ],
};

const seatStatus = (used: number) => ({
  licensed: true,
  enterprise: true,
  status: 'valid',
  message: '',
  license: {
    licenseId: 'lic-e2e',
    customer: 'E2E Test Co',
    edition: 'enterprise',
    features: [],
    maxUsers: 10,
    issuedAt: '2026-01-01T00:00:00Z',
    expiresAt: '2027-01-01T00:00:00Z',
  },
  seats: {
    used,
    allowed: 10,
    windowDays: 30,
    percent: used / 10,
    threshold: 'ok',
    lastUpdated: '2026-06-01T10:00:00Z',
    advisoryOnly: false,
  },
});

test.describe('Global Admin - Settings Users seat widget + reclaim (Story 16.2)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test('AC1/AC3/AC4: seat widget shows used/allowed; reclaiming a user drops the active count and seat usage by one', async ({
    page,
  }) => {
    // Mutable server state: the roster + seat count flip once the reclaim lands.
    let reclaimed = false;

    await page.route(USERS_LIST_ROUTE, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(reclaimed ? RECLAIM_ROSTER_AFTER : RECLAIM_ROSTER_ACTIVE),
      });
    });

    await page.route('**/api/v1/license', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(seatStatus(reclaimed ? 1 : 2)),
      });
    });

    // DELETE /api/v1/users/{id} → 200 with the verbatim reclaim note; flips the
    // mutable state so the subsequent roster + license refetch reflect it.
    await page.route('**/api/v1/users/*', async (route) => {
      if (route.request().method() === 'DELETE') {
        reclaimed = true;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            id: 'u-alice',
            state: 'removed',
            note: 'Seat reclaimed. Permanent exclusion requires IdP-side revocation.',
          }),
        });
        return;
      }
      await route.fallback();
    });

    await page.goto('/settings/users');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Seat widget renders used/allowed.
    const seatWidget = page.getByTestId('users-seat-usage');
    await expect(seatWidget).toBeVisible({ timeout: 10000 });
    await expect(seatWidget).toContainText('2 / 10');

    // Footer shows two active users to start.
    const footer = page.getByTestId('users-list-footer');
    await expect(footer).toContainText('2 active');

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-16-2-seat-widget.png',
      fullPage: true,
    });

    // Trigger reclaim on Alice's (active) row.
    await page.getByTestId('reclaim-seat-u-alice').click();

    // Dialog copy carries the verbatim revocation note and NO hard-delete wording.
    const dialog = page.getByTestId('reclaim-seat-dialog');
    await expect(dialog).toBeVisible({ timeout: 5000 });
    await expect(dialog).toContainText('IdP-side revocation');
    await expect(dialog).not.toContainText(/delete/i);
    await expect(dialog).not.toContainText(/permanently/i);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-16-2-reclaim-dialog.png',
      fullPage: true,
    });

    // Confirm → roster + seat query refetch.
    await page.getByTestId('reclaim-seat-confirm').click();

    // Active count drops by one (footer) AND the seat widget `used` drops by one.
    await expect(footer).toContainText('1 active', { timeout: 10000 });
    await expect(seatWidget).toContainText('1 / 10');
    // Alice is now listed as removed (no reclaim action on her row).
    await expect(page.getByTestId('reclaim-seat-u-alice')).toHaveCount(0);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-16-2-after-reclaim.png',
      fullPage: true,
    });

    console.log('✓ AC1/AC3/AC4: reclaim drops the active count and seat usage by one');
  });
});

test.describe('Viewer - Settings Users Access Denied', () => {
  test.use({ authenticateAs: TestUserRole.ORG_VIEWER });

  test('AC4: Non-operator sees Access Denied, not the table', async ({ page }) => {
    // Roster API returns 403 for a non-operator
    await page.route(USERS_LIST_ROUTE, async (route) => {
      await route.fulfill({
        status: 403,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'access denied' }),
      });
    });

    // Deny settings permission checks (if the page pre-checks)
    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'no' }),
      });
    });

    await page.goto('/settings/users');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Access Denied is shown
    await expect(page.getByText('Access Denied')).toBeVisible({ timeout: 5000 });

    // The roster table is NOT rendered
    await expect(page.getByTestId('users-settings')).toHaveCount(0);
    await expect(page.getByTestId('user-state-badge')).toHaveCount(0);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/users-04-access-denied.png',
      fullPage: true,
    });

    console.log('✓ AC4: Non-operator sees Access Denied on Settings Users');
  });
});
