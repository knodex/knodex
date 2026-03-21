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
  // Skip entire suite if not enterprise build - secrets is enterprise-only
  test.skip(!process.env.ENTERPRISE_BUILD, 'Secrets features require ENTERPRISE_BUILD=true');

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
    await auth.setupAs(TestUserRole.ORG_VIEWER);
    await page.goto('/secrets');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-rbac-02-viewer-secrets-page.png`,
      fullPage: true,
    });

    // Create button must be hidden for viewers
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
    expect(createResp.status).toBe(201);

    // Try to access it via a different project — should return 404
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
    expect(crossProjectResp.status).toBe(404);

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
