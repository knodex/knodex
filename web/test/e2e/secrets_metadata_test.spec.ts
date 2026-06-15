// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';

/**
 * Secret Metadata E2E Tests
 *
 * Covers the operational metadata layer added to Secrets:
 *  - `knodex.io/rotation` label   (manual | auto)
 *  - `knodex.io/docs-url` annotation
 *  - `knodex.io/expires-at` annotation (RFC3339)
 *
 * These fields are purely human-facing reminders. There is no automatic
 * rotation engine — the values exist so operators can scan the Secrets
 * list and see at a glance which credentials need attention.
 *
 * Prerequisites:
 *  - Backend deployed with secrets feature enabled.
 *  - Global Admin user logged in via the test auth fixture.
 *  - A real K8s namespace (`default`) in the cluster the backend talks to.
 *
 * API-driven assertions match the skip convention of secrets_crud_test.spec.ts:
 * Casbin policies for secrets RBAC are not yet aligned in CI fixtures, so
 * the round-trip checks are written but disabled until the test env is
 * updated. Unskip in the same change that unskips AC-SEC-03.
 */

const SCREENSHOT_DIR = '../test-results/e2e/screenshots';

/**
 * Build an end-of-day-UTC RFC3339 timestamp `daysFromNow` days from now.
 * Mirrors the conversion the Create dialog does for the date input.
 */
function expiresAtFromNow(daysFromNow: number): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + daysFromNow);
  return `${d.toISOString().slice(0, 10)}T23:59:59Z`;
}

test.describe('Secret operational metadata', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  // Secrets are namespace-keyed under the unified Casbin model — these tests
  // exercise /api/v1/namespaces/{namespace}/secrets. The default namespace
  // is used because the GLOBAL_ADMIN fixture has access to every namespace.
  const namespace = 'default';
  const runId = Date.now();

  // ─── UI: the new columns render once at least one secret exists ─────────

  test('AC-META-03: Secrets list shows Rotation and Status columns', async ({ page }) => {
    // Mock at least one secret so the table renders. The production page
    // shows a centered "No secrets yet" empty state with no <th> elements
    // when the list is empty, so this assertion fails on a fresh cluster.
    await page.route('**/api/v1/secrets**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [{
            name: 'e2e-meta-fixture',
            namespace,
            keys: ['key'],
            updatedAt: new Date().toISOString(),
            metadata: { rotation: 'manual' },
          }],
        }),
      });
    });

    await page.goto('/secrets');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Page should not 403/error. Mirrors AC-SEC-01 / AC-SEC-02 defensive pattern.
    const accessDenied = page.locator('text=Access Denied');
    const isDenied = await accessDenied.isVisible({ timeout: 2000 }).catch(() => false);
    expect(isDenied).toBe(false);

    // Both new sortable headers must be present in the table for the new
    // metadata surface to be considered live.
    const rotationHeader = page.locator('th button:has-text("Rotation"), th:has-text("Rotation")').first();
    const statusHeader = page.locator('th button:has-text("Status"), th:has-text("Status")').first();
    await expect(rotationHeader).toBeVisible({ timeout: 5000 });
    await expect(statusHeader).toBeVisible({ timeout: 5000 });

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-metadata-03-columns-visible.png`,
      fullPage: true,
    });
  });

  // ─── API roundtrip: create → list/get reflects the typed metadata ───────

  // SKIP: Casbin policies for secrets RBAC not yet aligned in CI fixtures
  // (same reason as AC-SEC-03 in secrets_crud_test.spec.ts). Re-enable in
  // the same change that updates the test env policies.
  test.skip('AC-META-01: Create stamps rotation label + docs-url + expires-at annotations', async ({ page }) => {
    const name = `e2e-meta-full-${runId}`;
    const expiresAt = expiresAtFromNow(60); // Outside the 30-day window → "active"

    await page.goto('/');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    const createResp = await page.evaluate(
      async ({ name, namespace, expiresAt }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(`/api/v1/namespaces/${namespace}/secrets`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            name,
            data: { password: 'do-not-leak' },
            metadata: {
              rotation: 'auto',
              docsUrl: 'https://wiki.example.com/secrets/' + name,
              expiresAt,
            },
          }),
        });
        return { status: resp.status, body: await resp.json() };
      },
      { name, namespace, expiresAt },
    );

    expect(createResp.status).toBe(201);
    expect(createResp.body.metadata).toMatchObject({
      rotation: 'auto',
      docsUrl: `https://wiki.example.com/secrets/${name}`,
      expiresAt,
    });
    expect(createResp.body.status).toBe('active');
    // Plaintext value MUST NOT come back.
    expect(JSON.stringify(createResp.body)).not.toContain('do-not-leak');

    // GET surfaces the same metadata
    const getResp = await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
        return { status: resp.status, body: await resp.json() };
      },
      { name, namespace },
    );
    expect(getResp.status).toBe(200);
    expect(getResp.body.metadata?.rotation).toBe('auto');
    expect(getResp.body.metadata?.docsUrl).toBe(`https://wiki.example.com/secrets/${name}`);
    expect(getResp.body.status).toBe('active');

    // Cleanup
    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { method: 'DELETE', headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
      },
      { name, namespace },
    );
  });

  test.skip('AC-META-02: Create without metadata writes no metadata keys', async ({ page }) => {
    const name = `e2e-meta-none-${runId}`;

    await page.goto('/');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    const createResp = await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(`/api/v1/namespaces/${namespace}/secrets`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            name,
            data: { key: 'value' },
          }),
        });
        return { status: resp.status, body: await resp.json() };
      },
      { name, namespace },
    );

    expect(createResp.status).toBe(201);
    // metadata is omitted from the wire shape when no fields are set
    expect(createResp.body.metadata).toBeUndefined();
    expect(createResp.body.status).toBeFalsy();
    // ManagedBy label is always present. AC5: under the namespace-keyed
    // model, knodex.io/project is intentionally NOT stamped on the K8s
    // object — the namespace is the access boundary, not the project.
    expect(createResp.body.labels?.['knodex.io/managed-by']).toBe('knodex');
    expect(createResp.body.labels?.['knodex.io/project']).toBeUndefined();
    // Metadata-specific keys are NOT in the flat label map either
    expect(createResp.body.labels?.['knodex.io/rotation']).toBeUndefined();

    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { method: 'DELETE', headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
      },
      { name, namespace },
    );
  });

  test.skip('AC-META-04: Metadata-only update succeeds (no data value change required)', async ({ page }) => {
    const name = `e2e-meta-only-update-${runId}`;

    await page.goto('/');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Seed
    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(`/api/v1/namespaces/${namespace}/secrets`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({ name, data: { k: 'v' } }),
        });
      },
      { name, namespace },
    );

    // Update with metadata only (empty data MUST still be rejected by the
    // server's data-required check — we send the existing key to satisfy
    // it without leaking new values).
    const updateResp = await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          {
            method: 'PUT',
            headers: {
              'Content-Type': 'application/json',
              ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
            },
            body: JSON.stringify({
              data: { k: 'v' },
              metadata: {
                rotation: 'manual',
                docsUrl: 'https://docs.example.com/' + name,
              },
            }),
          },
        );
        return { status: resp.status, body: await resp.json() };
      },
      { name, namespace },
    );
    expect(updateResp.status).toBe(200);
    expect(updateResp.body.metadata?.rotation).toBe('manual');
    expect(updateResp.body.metadata?.docsUrl).toBe(`https://docs.example.com/${name}`);
    // ManagedBy survives a metadata update; ProjectLabel is intentionally
    // never stamped on the K8s object under the namespace-keyed model.
    expect(updateResp.body.labels?.['knodex.io/managed-by']).toBe('knodex');

    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { method: 'DELETE', headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
      },
      { name, namespace },
    );
  });

  test.skip('AC-META-05: Update without metadata field preserves existing metadata', async ({ page }) => {
    const name = `e2e-meta-preserve-${runId}`;

    await page.goto('/');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Seed with metadata
    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(`/api/v1/namespaces/${namespace}/secrets`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            name,
            data: { k: 'v' },
            metadata: { rotation: 'auto', docsUrl: 'https://stays.example.com' },
          }),
        });
      },
      { name, namespace },
    );

    // Update WITHOUT passing the metadata field at all — server must leave
    // the existing labels/annotations intact.
    const updateResp = await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          {
            method: 'PUT',
            headers: {
              'Content-Type': 'application/json',
              ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
            },
            body: JSON.stringify({ data: { k: 'v2' } }),
          },
        );
        return { status: resp.status, body: await resp.json() };
      },
      { name, namespace },
    );
    expect(updateResp.status).toBe(200);
    expect(updateResp.body.metadata?.rotation).toBe('auto');
    expect(updateResp.body.metadata?.docsUrl).toBe('https://stays.example.com');

    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { method: 'DELETE', headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
      },
      { name, namespace },
    );
  });

  test.skip('AC-META-06: Past expiresAt renders the Expired status badge in the UI', async ({ page }) => {
    const name = `e2e-meta-expired-${runId}`;

    // Seed via API so the row exists before we render the page.
    await page.goto('/');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(`/api/v1/namespaces/${namespace}/secrets`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            name,
            data: { k: 'v' },
            metadata: {
              rotation: 'manual',
              // 7 days in the past → server computes status="expired"
              expiresAt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
            },
          }),
        });
      },
      { name, namespace },
    );

    await page.goto(`/secrets`);
    await page.waitForLoadState('domcontentloaded');

    // Find our row's status cell via the data-testid we wired in
    const statusCell = page.locator(`[data-testid="secret-status-${name}"]`);
    await expect(statusCell).toBeVisible({ timeout: 10000 });
    await expect(statusCell).toContainText('Expired');

    // The rotation chip should read "Manual"
    const rotationCell = page.locator(`[data-testid="secret-rotation-${name}"]`);
    await expect(rotationCell).toContainText('Manual');

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-metadata-06-expired-badge.png`,
      fullPage: true,
    });

    // Cleanup
    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { method: 'DELETE', headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
      },
      { name, namespace },
    );
  });
});
