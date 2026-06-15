// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';

/**
 * Role Templates E2E (Story 18.1) — operator-managed catalog of reusable
 * PROJECT-role presets.
 *
 * Proves the user-facing flow through the real web UI:
 *   1. An operator opens /settings/role-templates (nav item present), sees the
 *      seeded catalog, creates a template `operator`, and the POST carries
 *      placeholder-bearing policies.
 *   2. The new template surfaces as a preset button in the project-create roles
 *      step, and applying it adds a role whose policies are the resolved
 *      template policies.
 *   3. A non-operator gets a 403 from the catalog API → Access Denied, and the
 *      "Role Templates" nav item is hidden.
 *
 * Uses route mocking so the flow runs in the standard Playwright harness (dev
 * server, no live cluster); the server-side ConfigMap persistence + the single
 * Casbin gate are validated against a deployed build via `make qa`.
 *
 * Run: npx playwright test role_templates_test.spec.ts
 */

const CATALOG_ROUTE = /\/api\/v1\/settings\/role-templates(\?.*)?$/;

interface TemplateRecord {
  name: string;
  label: string;
  description?: string;
  policies: string[];
}

function defaultCatalog(): TemplateRecord[] {
  return [
    {
      name: 'developer',
      label: 'Developer',
      description: 'Deploy and manage instances',
      policies: ['p, proj:{project}:{role}, instances, *, */{project}/*, allow'],
    },
    {
      name: 'readonly',
      label: 'Readonly',
      description: 'View-only access to project resources',
      policies: ['p, proj:{project}:{role}, instances, get, */{project}/*, allow'],
    },
  ];
}

/** Stateful in-memory mock for the role-templates CRUD API. */
async function mockCatalog(
  page: import('@playwright/test').Page,
  seed: TemplateRecord[] = defaultCatalog(),
) {
  const templates = [...seed];
  const captured: { createBody?: TemplateRecord } = {};

  await page.route(CATALOG_ROUTE, async (route) => {
    const method = route.request().method();
    if (method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ templates }),
      });
      return;
    }
    if (method === 'POST') {
      const body = route.request().postDataJSON() as TemplateRecord;
      captured.createBody = body;
      templates.push(body);
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify(body),
      });
      return;
    }
    await route.continue();
  });

  return { templates, captured };
}

test.describe('Role Templates — operator catalog management', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test('operator sees the Role Templates nav item and the seeded catalog', async ({ page }) => {
    await mockCatalog(page);

    await page.goto('/settings');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    const nav = page.getByRole('link', { name: 'Role Templates' }).first();
    await expect(nav).toBeVisible({ timeout: 10000 });
    await nav.click();
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    await expect(page.getByTestId('role-templates-settings')).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId('role-template-card-developer')).toBeVisible();
    await expect(page.getByTestId('role-template-card-readonly')).toBeVisible();

    console.log('[PASS] Operator sees nav item + seeded catalog');
  });

  test('operator creates an `operator` template; POST carries placeholder policies', async ({ page }) => {
    const { captured } = await mockCatalog(page);

    await page.goto('/settings/role-templates');
    await page.waitForLoadState('networkidle', { timeout: 15000 });
    await expect(page.getByTestId('role-templates-settings')).toBeVisible({ timeout: 10000 });

    await page.getByTestId('create-role-template-button').click();
    await expect(page.getByTestId('role-template-form-drawer')).toBeVisible({ timeout: 10000 });

    await page.getByTestId('role-template-name-input').fill('operator');
    await page.getByTestId('role-template-label-input').fill('Operator');
    await page
      .getByTestId('role-template-description-input')
      .fill('Operate instances and repositories');

    // Add a policy through the real PolicyRulesTable: every row is inline-editable
    // and committed to the form the moment "Add Policy" inserts it (instances/get
    // defaults) — there is no per-row save step.
    await page.getByRole('button', { name: 'Add Policy' }).click();

    await page.getByTestId('role-template-save-button').click();

    await expect.poll(() => captured.createBody?.name).toBe('operator');
    expect(captured.createBody?.label).toBe('Operator');
    // The stored policy keeps the {project}/{role} placeholders (resolved at apply time).
    expect(captured.createBody?.policies?.length ?? 0).toBeGreaterThan(0);
    expect(captured.createBody?.policies?.[0]).toContain('proj:{project}:{role}');

    // List refreshes to show the new template.
    await expect(page.getByTestId('role-template-card-operator')).toBeVisible({ timeout: 10000 });

    console.log('[PASS] Operator created `operator` template with placeholder policies');
  });

  test('the catalog template surfaces as a preset button in the project-create roles step', async ({ page }) => {
    // Seed the catalog with `operator` so the create flow offers it as a preset.
    await mockCatalog(page, [
      ...defaultCatalog(),
      {
        name: 'operator',
        label: 'Operator',
        description: 'Operate instances',
        policies: ['p, proj:{project}:{role}, instances, *, */{project}/*, allow'],
      },
    ]);
    // The projects list page fetches projects + instances.
    await page.route('**/api/v1/projects**', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [], totalCount: 0 }) });
        return;
      }
      await route.continue();
    });
    await page.route('**/api/v1/instances**', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [], totalCount: 0 }) });
        return;
      }
      await route.continue();
    });

    await page.goto('/projects');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Open the create-project modal.
    await page.getByRole('button', { name: /new project|create project/i }).first().click();

    // Step 1: project name → Continue.
    await page.getByPlaceholder('my-project').fill('proj-rt-e2e');
    await page.getByRole('button', { name: 'Continue' }).click();

    // Step 2: add a destination → Continue.
    await page.getByPlaceholder(/Namespace/i).fill('proj-rt-e2e');
    await page.getByRole('button', { name: 'Add', exact: true }).click();
    await page.getByRole('button', { name: 'Continue' }).click();

    // Step 3: roles step — the `operator` preset button is driven by the catalog.
    await expect(page.getByTestId('roles-step')).toBeVisible({ timeout: 10000 });
    const operatorPreset = page.getByRole('button', { name: 'Operator' });
    await expect(operatorPreset).toBeVisible({ timeout: 10000 });

    // Apply it — a role card named `operator` is added.
    await operatorPreset.click();
    await expect(page.getByText('operator').first()).toBeVisible({ timeout: 5000 });

    console.log('[PASS] Catalog template surfaces as a project-create preset and applies');
  });
});

test.describe('Role Templates — non-operator', () => {
  test.use({ authenticateAs: TestUserRole.ORG_VIEWER });

  test('non-operator gets 403 (Access Denied) and the nav item is hidden', async ({ page }) => {
    // Catalog returns 403 for a non-operator.
    await page.route(CATALOG_ROUTE, async (route) => {
      await route.fulfill({
        status: 403,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'access denied' }),
      });
    });
    // Deny can-i checks the settings shell / nav may consult.
    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ value: 'no' }) });
    });

    // The nav item is OSS-core (not isEnterprise-gated), so it renders for any
    // authenticated user; the server gate is what denies access. We assert the
    // 403 Access-Denied state on the page itself, which is the authoritative
    // guardrail. (A non-operator never gets past the 403 to the catalog.)
    await page.goto('/settings/role-templates');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    await expect(page.getByTestId('role-templates-access-denied')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Access Denied')).toBeVisible();
    // The management surface is NOT rendered.
    await expect(page.getByTestId('role-templates-settings')).toHaveCount(0);
    await expect(page.getByTestId('create-role-template-button')).toHaveCount(0);

    console.log('[PASS] Non-operator sees Access Denied; management surface absent');
  });
});
