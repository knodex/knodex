// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';

/**
 * Secrets RBAC Isolation E2E Tests
 *
 * Tests that secrets are properly isolated under the namespace-keyed
 * authorization model:
 *  - Viewers cannot create secrets
 *  - Cross-namespace access without a destinations binding is denied
 *  - Global Admin has full access in every namespace
 *  - A shared namespace can be read from multiple project bindings
 *
 * Prerequisites:
 *  - Backend deployed with secrets feature enabled
 *  - Multiple test users with different roles
 */

const SCREENSHOT_DIR = '../test-results/e2e/screenshots';

test.describe('Secrets RBAC Isolation (namespace-keyed)', () => {
  test('AC-SEC-RBAC-01: Viewer cannot create secrets via API', async ({ page, auth }) => {
    await auth.setupAs(TestUserRole.ORG_VIEWER);
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');

    // Attempt to create a secret as a viewer — should be forbidden by the
    // route-level Casbin middleware (secrets/{ns}/{name}, create).
    const createResp = await page.evaluate(async () => {
      const token = localStorage.getItem('jwt_token');
      const resp = await fetch('/api/v1/namespaces/default/secrets', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          name: 'viewer-test-secret',
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
    // Mock can-i to deny create/delete BEFORE auth setup so the page never
    // sees a real "yes" response cached.
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

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-rbac-02-viewer-secrets-page.png`,
      fullPage: true,
    });

    // Create button visibility depends entirely on the can-i mock above.
    const createButton = page.locator(
      'button:has-text("Create"), button:has-text("New Secret"), button:has-text("Add Secret")',
    );
    const hasCreateButton = await createButton.first().isVisible({ timeout: 3000 }).catch(() => false);

    expect(hasCreateButton).toBe(false);
  });

  // SKIP — coverage debt, not a missing assertion. AC9 (cross-namespace
  // denial) IS pinned at the unit/integration layer:
  //   - server/internal/rbac/policy_enforcer_destinations_test.go
  //       TestPolicyEnforcer_NamespaceScopedPolicies — verifies a developer
  //       without destinations for xxx-infra cannot access secrets there.
  //   - server/internal/api/handlers/secrets_handler_test.go
  //       TestSecretsHandler_GetSecret_AC9_CrossNamespaceDenied — 404 (not
  //       leaking existence) for unauthorized namespace.
  //   - server/test/e2e/secrets_namespace_authz_test.go
  //       TestSecretsAuthz_AC9_CrossNamespaceDenied — same at the live API.
  //
  // This UI-layer test is skipped because the Playwright fixture set does
  // not yet include a project role whose destinations EXCLUDE 'default'.
  // Re-enable once CI seeds such a user; tracked as follow-up to STORY-437.
  test.skip('AC-SEC-RBAC-03: Cross-namespace access without destinations binding is denied', async ({ page, auth }) => {
    await auth.setupAs(TestUserRole.GLOBAL_ADMIN);
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');

    const secretName = `rbac-cross-ns-${Date.now()}`;
    const seedNamespace = 'default';

    // Seed a secret in the default namespace (admin can write anywhere).
    const createResp = await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(`/api/v1/namespaces/${namespace}/secrets`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({ name, data: { key: 'value' } }),
        });
        return { status: resp.status };
      },
      { name: secretName, namespace: seedNamespace },
    );
    if (![200, 201].includes(createResp.status)) {
      test.skip(true, `Secret creation returned ${createResp.status} — skipping`);
      return;
    }

    // Switch to a viewer who has NO destinations covering 'default'.
    await auth.setupAs(TestUserRole.ORG_VIEWER);

    // Read attempt should be denied — but the handler returns 404 (not
    // 403) to avoid leaking the secret's existence to an unauthorized
    // caller. This matches Instances' not-leaking-existence behavior.
    const crossNamespaceResp = await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
        return { status: resp.status };
      },
      { name: secretName, namespace: seedNamespace },
    );
    expect([403, 404]).toContain(crossNamespaceResp.status);

    // Cleanup as admin
    await auth.setupAs(TestUserRole.GLOBAL_ADMIN);
    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { method: 'DELETE', headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
      },
      { name: secretName, namespace: seedNamespace },
    );

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-rbac-03-cross-namespace-denied.png`,
      fullPage: true,
    });
  });

  test('AC-SEC-RBAC-04: Global Admin has full secrets access', async ({ page, auth }) => {
    await auth.setupAs(TestUserRole.GLOBAL_ADMIN);
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');

    // List secrets — namespace-agnostic, server-filtered. Admin gets the
    // full set.
    const listResp = await page.evaluate(async () => {
      const token = localStorage.getItem('jwt_token');
      const resp = await fetch('/api/v1/secrets', {
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

  // SKIP — coverage debt, not a missing assertion. AC2 (shared-namespace
  // cross-project read) IS pinned at the unit layer:
  //   - server/internal/rbac/policy_enforcer_project_test.go
  //       TestPolicyEnforcer_SecretsAccess_SharedNamespace — verifies two
  //       distinct project roles (alpha, beta) that both list xxx-shared
  //       in destinations BOTH gain access to the same secret.
  //   - server/internal/api/handlers/secrets_handler_test.go
  //       TestSecretsHandler_GetSecret_AC2_SharedNamespaceRead — handler
  //       returns 200 for both audit lenses on the same secret.
  //
  // This UI-layer test is skipped because the Playwright fixture set
  // doesn't yet expose two project roles with overlapping shared-namespace
  // destinations (the TODO references PROJECT_ALPHA_DEVELOPER and
  // PROJECT_BETA_DEVELOPER fixtures that don't exist yet). Re-enable when
  // those CI fixtures land; tracked as follow-up to STORY-437.
  test.skip('AC-SEC-RBAC-05: Shared namespace allows cross-project read (AC2)', async ({ page, auth }) => {
    const secretName = `rbac-shared-${Date.now()}`;
    const sharedNamespace = 'xxx-shared';

    // Seed as admin so the secret definitely exists.
    await auth.setupAs(TestUserRole.GLOBAL_ADMIN);
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');

    const createResp = await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(`/api/v1/namespaces/${namespace}/secrets`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({ name, data: { sharedKey: 'sharedValue' } }),
        });
        return { status: resp.status };
      },
      { name: secretName, namespace: sharedNamespace },
    );
    if (![200, 201].includes(createResp.status)) {
      test.skip(true, `Secret seed returned ${createResp.status} — fixture not ready`);
      return;
    }

    // Read as the alpha developer (project alpha, destinations includes shared).
    // NOTE: this references a fixture role (PROJECT_ALPHA_DEVELOPER) that
    // does not yet exist in TestUserRole. The whole test is currently
    // skipped; the role name documents the fixture this test will need
    // once it is wired up.
    await auth.setupAs(TestUserRole.ORG_DEVELOPER); // TODO: PROJECT_ALPHA_DEVELOPER
    const alphaResp = await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
        return { status: resp.status };
      },
      { name: secretName, namespace: sharedNamespace },
    );
    expect(alphaResp.status).toBe(200);

    // Read as the beta developer (different project, destinations also covers shared).
    await auth.setupAs(TestUserRole.ORG_DEVELOPER); // TODO: PROJECT_BETA_DEVELOPER
    const betaResp = await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        const resp = await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
        return { status: resp.status };
      },
      { name: secretName, namespace: sharedNamespace },
    );
    expect(betaResp.status).toBe(200);

    // Cleanup as admin
    await auth.setupAs(TestUserRole.GLOBAL_ADMIN);
    await page.evaluate(
      async ({ name, namespace }) => {
        const token = localStorage.getItem('jwt_token');
        await fetch(
          `/api/v1/namespaces/${namespace}/secrets/${encodeURIComponent(name)}`,
          { method: 'DELETE', headers: token ? { 'Authorization': `Bearer ${token}` } : {} },
        );
      },
      { name: secretName, namespace: sharedNamespace },
    );
  });
});
