// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture';
import type { Page } from '@playwright/test';

/**
 * Project Access Tab E2E — team-centric project authorization.
 *
 * Replaces the former role-editor spec (project_roles_test.spec.ts). The
 * project detail "Access" tab (formerly "Roles") now presents one row per Team
 * — its OIDC-group provenance, namespace scope, and a role dropdown — derived
 * from `roles[].teams[]`. Edits translate back into `roles[]` and persist via
 * `PUT /api/v1/projects/{name}`.
 *
 * These specs route-mock the teams + project APIs so the flow runs in the
 * standard Playwright harness (dev server, no live cluster). The live end-to-end
 * (a bound team's member gaining access through the single Casbin enforcer) is
 * exercised by `make qa`.
 *
 * Run: npx playwright test project_access_test.spec.ts
 */

const TEST_PROJECT = process.env.E2E_TEST_PROJECT || 'proj-alpha-team';

interface MockRole {
  name: string;
  policies?: string[];
  destinations?: string[];
  teams?: string[];
}
interface MockProject {
  name: string;
  type: string;
  description?: string;
  destinations?: { namespace: string }[];
  roles: MockRole[];
  resourceVersion: string;
  createdAt: string;
}

const TEAMS = [
  { name: 'platform-eng', description: 'Platform Engineering', oidcGroups: ['alpha-admins', 'alpha-ops'] },
  { name: 'payments', description: 'Payments', oidcGroups: ['alpha-developers'] },
  { name: 'identity', description: 'Identity & Security', oidcGroups: ['sec-a', 'sec-b'] },
  { name: 'unbound-team', description: 'Not yet bound', oidcGroups: ['unbound-grp'] },
];

function baseProject(): MockProject {
  return {
    name: TEST_PROJECT,
    type: 'app',
    description: 'Access tab E2E project',
    destinations: [{ namespace: 'payments-system' }, { namespace: 'payments-dev' }],
    resourceVersion: '100',
    createdAt: '2026-01-01T00:00:00Z',
    roles: [
      { name: 'admin', policies: ['instances/*, *, allow'], teams: ['platform-eng'], destinations: [] },
      { name: 'developer', policies: ['instances/*, get, allow'], teams: ['payments'], destinations: ['payments-system'] },
      { name: 'readonly', policies: ['instances/*, get, allow'], teams: [], destinations: [] },
    ],
  };
}

/**
 * Stateful mocks for teams + the project. PUTs merge `roles` into the stored
 * project, bump resourceVersion, and echo the full project so React Query's
 * cache reflects edits across re-renders. Returns the captured PUT bodies.
 */
async function mockBackend(page: Page, project: MockProject = baseProject()) {
  let proj: MockProject = JSON.parse(JSON.stringify(project));
  const puts: Array<{ roles?: MockRole[]; resourceVersion?: string }> = [];

  await page.route('**/api/v1/teams', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: TEAMS, totalCount: TEAMS.length }),
      });
      return;
    }
    await route.continue();
  });

  await page.route('**/api/v1/instances**', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], totalCount: 0 }),
      });
      return;
    }
    await route.continue();
  });

  await page.route(`**/api/v1/projects/${TEST_PROJECT}`, async (route) => {
    const method = route.request().method();
    if (method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(proj) });
      return;
    }
    if (method === 'PUT') {
      const body = route.request().postDataJSON();
      puts.push(body);
      proj = {
        ...proj,
        ...body,
        resourceVersion: String(Number(proj.resourceVersion) + 1),
      };
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(proj) });
      return;
    }
    await route.continue();
  });

  return { puts, getProject: () => proj };
}

async function openAccessTab(page: Page) {
  await page.goto(`/projects/${TEST_PROJECT}`);
  await page.waitForLoadState('networkidle', { timeout: 15000 });
  const accessTab = page.locator('button[role="tab"]').filter({ hasText: 'Access' });
  await expect(accessTab).toBeVisible({ timeout: 10000 });
  await accessTab.click();
  // The Access panel renders one of two roots: the populated table
  // (project-access-tab) or the no-teams empty state (project-access-empty).
  await expect(
    page.getByTestId('project-access-tab').or(page.getByTestId('project-access-empty'))
  ).toBeVisible({ timeout: 10000 });
}

/** Find the role of the first PUT whose roles[] contains `team`. */
function roleBindingFor(puts: Array<{ roles?: MockRole[] }>, team: string): string | undefined {
  const last = puts[puts.length - 1];
  return last?.roles?.find((r) => (r.teams || []).includes(team))?.name;
}

test.describe('Project Access Tab — team-centric (admin)', () => {
  test.describe.configure({ mode: 'serial' });
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test.beforeEach(async ({ page }) => {
    await setupPermissionMocking(page, { '*:*': true });
  });

  test('renders one row per bound team with OIDC-group provenance and scope', async ({ page }) => {
    await mockBackend(page);
    await openAccessTab(page);

    // Rows for the two bound teams (identity + unbound-team are not bound).
    await expect(page.getByTestId('team-access-row-platform-eng')).toBeVisible();
    await expect(page.getByTestId('team-access-row-payments')).toBeVisible();
    await expect(page.getByTestId('team-access-row-identity')).toHaveCount(0);

    // Provenance: platform-eng has 2 OIDC groups, payments has 1.
    await expect(
      page.getByTestId('team-access-row-platform-eng').getByText('via 2 OIDC groups')
    ).toBeVisible();
    await expect(
      page.getByTestId('team-access-row-payments').getByText('via 1 OIDC group')
    ).toBeVisible();

    // Scope: admin = All namespaces; developer = payments-system.
    await expect(
      page.getByTestId('team-access-row-platform-eng').getByText('All namespaces')
    ).toBeVisible();
    await expect(
      page.getByTestId('team-access-row-payments').getByText('payments-system')
    ).toBeVisible();

    // Role pills show each team's role.
    await expect(page.getByTestId('team-role-platform-eng')).toContainText('admin');
    await expect(page.getByTestId('team-role-payments')).toContainText('developer');

    await expect(page.getByTestId('add-team-access-button')).toBeVisible();
    console.log('[PASS] Access tab renders team rows with provenance, scope, and roles');
  });

  test("changing a team's role re-binds it via PUT roles[].teams", async ({ page }) => {
    const { puts } = await mockBackend(page);
    await openAccessTab(page);

    // payments is currently 'developer'; switch it to 'admin'.
    await page.getByTestId('team-role-payments').click();
    await page.getByTestId('role-option-payments-admin').click();

    await expect.poll(() => puts.length).toBeGreaterThan(0);
    const last = puts[puts.length - 1];
    const admin = last.roles?.find((r) => r.name === 'admin');
    const developer = last.roles?.find((r) => r.name === 'developer');
    expect(admin?.teams).toContain('payments');
    expect(developer?.teams || []).not.toContain('payments');
    console.log('[PASS] Role change moves the team between roles[].teams');
  });

  test('removing a team strips it from all roles via PUT', async ({ page }) => {
    const { puts } = await mockBackend(page);
    await openAccessTab(page);

    await page.getByTestId('team-access-menu-platform-eng').click();
    await page.getByTestId('remove-team-platform-eng').click();

    await expect.poll(() => puts.length).toBeGreaterThan(0);
    const last = puts[puts.length - 1];
    for (const r of last.roles || []) {
      expect(r.teams || []).not.toContain('platform-eng');
    }
    console.log('[PASS] Remove strips the team from every role');
  });

  test('adding a team binds it to the chosen role via PUT', async ({ page }) => {
    const { puts } = await mockBackend(page);
    await openAccessTab(page);

    await page.getByTestId('add-team-access-button').click();
    await expect(page.getByTestId('add-team-access-dialog')).toBeVisible();

    // Pick the unbound team.
    await page.locator('#add-team-team').click();
    await page.getByTestId('team-option-unbound-team').click();
    // Pick the readonly role.
    await page.locator('#add-team-role').click();
    await page.getByTestId('role-select-option-readonly').click();

    await page.getByTestId('add-team-submit').click();

    await expect.poll(() => puts.length).toBeGreaterThan(0);
    expect(roleBindingFor(puts, 'unbound-team')).toBe('readonly');
    console.log('[PASS] Add team binds the team to the selected role');
  });

  test('shows an empty state with Add team when no teams are bound', async ({ page }) => {
    const empty = baseProject();
    empty.roles = empty.roles.map((r) => ({ ...r, teams: [] }));
    await mockBackend(page, empty);
    await openAccessTab(page);

    await expect(page.getByTestId('project-access-empty')).toBeVisible();
    await expect(page.getByText('No teams have access')).toBeVisible();
    await expect(page.getByTestId('add-team-access-button')).toBeVisible();
    console.log('[PASS] Empty state renders with Add team CTA');
  });
});

test.describe('Project Access Tab — viewer cannot manage', () => {
  test.use({ authenticateAs: TestUserRole.ORG_VIEWER });

  test('viewer sees rows but no management controls', async ({ page }) => {
    await setupPermissionMocking(page, { 'projects:get': true, 'projects:update': false });
    await mockBackend(page);
    await openAccessTab(page);

    // Rows render (read-only).
    await expect(page.getByTestId('team-access-row-platform-eng')).toBeVisible();
    // The role is shown as a static label, not an editable pill button.
    await expect(page.getByTestId('team-role-platform-eng')).toHaveCount(0);
    // No Add team, no overflow menu.
    await expect(page.getByTestId('add-team-access-button')).toHaveCount(0);
    await expect(page.getByTestId('team-access-menu-platform-eng')).toHaveCount(0);
    // Role text still visible somewhere in the row.
    await expect(page.getByTestId('team-access-row-platform-eng').getByText('admin')).toBeVisible();
    console.log('[PASS] Viewer sees read-only access rows without management controls');
  });
});
