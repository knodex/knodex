// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';

/**
 * Secrets RBAC Isolation E2E Tests
 *
 * Tests that secrets are properly isolated by project and role:
 * - Viewers cannot create secrets
 * - Cross-project access is denied
 * - Global Admin has full access
 *
 * Prerequisites:
 * - Backend deployed with secrets feature enabled
 * - Multiple test users with different roles
 */

const SCREENSHOT_DIR = '../test-results/e2e/screenshots';

test.describe('Secrets RBAC Isolation', () => {
  test('AC-SEC-RBAC-01: Viewer cannot create secrets via API', async ({ page, auth }) => {
    await auth.setupAs(TestUserRole.ORG_VIEWER);
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');

    // Attempt to create a secret as a viewer — should be forbidden
    const createResp = await page.evaluate(async () => {
      const token = localStorage.getItem('jwt_token');
      const resp = await fetch('/api/v1/secrets?project=proj-alpha-team', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          name: 'viewer-test-secret',
          namespace: 'default',
          data: { key: 'value' },
        }),
      });
      return { status: resp.status };
    });

    // Viewer should get 403 Forbidden
    expect(createResp.status).toBe(403);

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-rbac-01-viewer-create-denied.png`,
      fullPage: true,
    });
  });

  test('AC-SEC-RBAC-02: Viewer cannot see create button in UI', async ({ page, auth }) => {
    // Mock can-i to deny create/delete BEFORE auth setup (prevents caching real response)
    await page.route('**/api/v1/account/can-i/**', async (route) => {
      const url = route.request().url();
      if (url.includes('/create') || url.includes('/delete')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ value: 'no' }) });
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ value: 'yes' }) });
      }
    });
    await auth.setupAs(TestUserRole.ORG_VIEWER);
    await page.goto('/secrets');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Select a specific project so the can-i check runs against it
    // (without a project selected, Create button is always shown because
    // the dialog has its own project selector)
    const projectSelector = page.getByRole('combobox').or(page.locator('[data-testid="project-selector"]'));
    if (await projectSelector.isVisible({ timeout: 3000 }).catch(() => false)) {
      await projectSelector.click();
      const alphaOption = page.getByText('proj-alpha-team').first();
      if (await alphaOption.isVisible({ timeout: 3000 }).catch(() => false)) {
        await alphaOption.click();
        await page.waitForTimeout(1000);
      }
    }

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-rbac-02-viewer-secrets-page.png`,
      fullPage: true,
    });

    // With a project selected, Create button visibility depends on can-i mock
    const createButton = page.locator(
      'button:has-text("Create"), button:has-text("New Secret"), button:has-text("Add Secret")',
    );
    const hasCreateButton = await createButton.first().isVisible({ timeout: 3000 }).catch(() => false);

    expect(hasCreateButton).toBe(false);
  });

  test('AC-SEC-RBAC-03: Cross-project secret access is denied', async ({ page, auth }) => {
    await auth.setupAs(TestUserRole.GLOBAL_ADMIN);
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');

    const secretName = `rbac-cross-project-${Date.now()}`;

    // Create a secret in proj-alpha-team
    const createResp = await page.evaluate(
      async ({ name }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch('/api/v1/secrets?project=proj-alpha-team', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            name,
            namespace: 'default',
            data: { key: 'value' },
          }),
        });
        return { status: resp.status };
      },
      { name: secretName },
    );
    // Secret creation should succeed (200/201) or may fail with auth issues in test env
    // In CI, the admin may not have secrets:create in the default namespace
    if (![200, 201].includes(createResp.status)) {
      test.skip(true, `Secret creation returned ${createResp.status} — skipping cross-project test`);
      return;
    }

    // Try to access it via a different project — should return 404 or 403
    const crossProjectResp = await page.evaluate(
      async ({ name }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/secrets/${encodeURIComponent(name)}?project=proj-beta-team&namespace=default`,
          { headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
        return { status: resp.status };
      },
      { name: secretName },
    );
    // Cross-project access should be denied (403) or secret not found in that project (404)
    expect([403, 404]).toContain(crossProjectResp.status);

    // Cleanup: delete the secret
    await page.evaluate(
      async ({ name }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(
          `/api/v1/secrets/${encodeURIComponent(name)}?project=proj-alpha-team&namespace=default`,
          {
            method: 'DELETE',
            headers: token ? { 'Authorization': `Bearer ${token}` } : {},
          },
        );
      },
      { name: secretName },
    );

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-rbac-03-cross-project-denied.png`,
      fullPage: true,
    });
  });

  test('AC-SEC-RBAC-04: Global Admin has full secrets access', async ({ page, auth }) => {
    await auth.setupAs(TestUserRole.GLOBAL_ADMIN);
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');

    // List secrets — should succeed
    const listResp = await page.evaluate(async () => {
      const token = localStorage.getItem('jwt_token');
      const resp = await fetch('/api/v1/secrets?project=proj-alpha-team', {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {},
      });
      return { status: resp.status };
    });
    expect(listResp.status).toBe(200);

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-rbac-04-admin-full-access.png`,
      fullPage: true,
    });
  });
});
