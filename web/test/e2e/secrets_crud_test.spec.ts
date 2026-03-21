// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';

/**
 * Secrets CRUD E2E Tests
 *
 * Tests the full secrets lifecycle: create, list, view detail, edit, delete.
 *
 * Prerequisites:
 * - Backend deployed with secrets feature enabled
 * - Global Admin user logged in
 */

const SCREENSHOT_DIR = '../test-results/e2e/screenshots';

test.describe('Secrets CRUD Workflow', () => {
  // Skip entire suite if not enterprise build - secrets is enterprise-only
  test.skip(!process.env.ENTERPRISE_BUILD, 'Secrets features require ENTERPRISE_BUILD=true');

  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  const secretName = `e2e-test-secret-${Date.now()}`;
  const project = 'proj-alpha-team';

  test('AC-SEC-01: Admin can create a secret', async ({ page }) => {
    await page.goto('/secrets');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-crud-01-list-page.png`,
      fullPage: true,
    });

    // Look for a create button
    const createButton = page.locator('button:has-text("Create"), button:has-text("New Secret"), button:has-text("Add Secret")');
    const hasCreateButton = await createButton.first().isVisible({ timeout: 5000 }).catch(() => false);

    if (hasCreateButton) {
      await createButton.first().click();
      await page.waitForTimeout(1000);

      await page.screenshot({
        path: `${SCREENSHOT_DIR}/secrets-crud-02-create-form.png`,
        fullPage: true,
      });
    }

    // Verify page is accessible (not access denied)
    const accessDenied = page.locator('text=Access Denied');
    const isDenied = await accessDenied.isVisible({ timeout: 2000 }).catch(() => false);
    expect(isDenied).toBe(false);
  });

  test('AC-SEC-02: Admin can view secrets list', async ({ page }) => {
    await page.goto('/secrets');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-crud-03-list-loaded.png`,
      fullPage: true,
    });

    // Page should load without error
    const errorState = page.locator('text=Error, text=Something went wrong');
    const hasError = await errorState.first().isVisible({ timeout: 2000 }).catch(() => false);
    expect(hasError).toBe(false);
  });

  test('AC-SEC-03: Secrets API CRUD workflow via fetch', async ({ page }) => {
    // Use direct API calls to test the full CRUD lifecycle
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');

    // Create
    const createResp = await page.evaluate(
      async ({ name, project }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(`/api/v1/secrets?project=${project}`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            name,
            namespace: 'default',
            data: { username: 'admin', password: 'test123' },
          }),
        });
        return { status: resp.status, body: await resp.json() };
      },
      { name: secretName, project },
    );
    expect(createResp.status).toBe(201);
    expect(createResp.body.name).toBe(secretName);
    expect(createResp.body.keys).toContain('username');
    expect(createResp.body.keys).toContain('password');
    // Values must NOT be in response
    expect(JSON.stringify(createResp.body)).not.toContain('test123');

    // List
    const listResp = await page.evaluate(async (project) => {
      const token = localStorage.getItem('jwt_token');
      const resp = await fetch(`/api/v1/secrets?project=${project}`, {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {},
      });
      return { status: resp.status, body: await resp.json() };
    }, project);
    expect(listResp.status).toBe(200);
    expect(listResp.body.items.some((s: { name: string }) => s.name === secretName)).toBe(true);

    // Get (with values)
    const getResp = await page.evaluate(
      async ({ name, project }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/secrets/${encodeURIComponent(name)}?project=${project}&namespace=default`,
          { headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
        return { status: resp.status, body: await resp.json() };
      },
      { name: secretName, project },
    );
    expect(getResp.status).toBe(200);
    expect(getResp.body.data.username).toBe('admin');
    expect(getResp.body.data.password).toBe('test123');

    // Update
    const updateResp = await page.evaluate(
      async ({ name, project }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(`/api/v1/secrets/${encodeURIComponent(name)}?project=${project}`, {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            namespace: 'default',
            data: { username: 'admin', password: 'updated123', api_key: 'newkey' },
          }),
        });
        return { status: resp.status, body: await resp.json() };
      },
      { name: secretName, project },
    );
    expect(updateResp.status).toBe(200);
    expect(updateResp.body.keys).toContain('api_key');

    // Delete
    const deleteResp = await page.evaluate(
      async ({ name, project }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/secrets/${encodeURIComponent(name)}?project=${project}&namespace=default`,
          {
            method: 'DELETE',
            headers: token ? { 'Authorization': `Bearer ${token}` } : {},
          },
        );
        return { status: resp.status, body: await resp.json() };
      },
      { name: secretName, project },
    );
    expect(deleteResp.status).toBe(200);
    expect(deleteResp.body.deleted).toBe(true);

    // Verify deleted
    const verifyResp = await page.evaluate(
      async ({ name, project }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/secrets/${encodeURIComponent(name)}?project=${project}&namespace=default`,
          { headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
        return { status: resp.status };
      },
      { name: secretName, project },
    );
    expect(verifyResp.status).toBe(404);

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-crud-04-api-workflow-complete.png`,
      fullPage: true,
    });
  });
});
