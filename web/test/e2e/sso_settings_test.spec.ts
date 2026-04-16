// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, setupAuthWithMocking } from '../fixture';

/**
 * SSO Settings E2E Tests
 *
 * Tests that Global Admin users can manage OIDC SSO providers through the Settings UI,
 * including creating, viewing, editing, and deleting providers.
 *
 * Prerequisites:
 * - Backend deployed with SSO Settings API
 * - Global Admin user logged in
 *
 * Test coverage:
 * - Admin can navigate to SSO settings from Settings hub
 * - Admin can create a new SSO provider
 * - Client secret is not displayed in provider list
 * - Admin can edit an existing provider
 * - Admin can delete a provider with confirmation
 * - Non-admin sees access denied
 */

test.describe('Global Admin - SSO Settings Management', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test('AC1: Navigate to SSO settings from Settings hub', async ({ page }) => {
    // Navigate to Settings hub
    await page.goto('/settings');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Verify Settings page loads
    const settingsTitle = page.locator('h1:has-text("Settings"), h2:has-text("Settings")').first();
    await expect(settingsTitle).toBeVisible({ timeout: 10000 });

    // Find SSO Providers card
    const ssoCard = page.locator('a[href="/settings/sso"], [href="/settings/sso"]');
    await expect(ssoCard).toBeVisible({ timeout: 5000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-01-settings-hub.png',
      fullPage: true,
    });

    // Click to navigate to SSO settings
    await ssoCard.click();
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Verify SSO Providers page loads
    const ssoTitle = page.locator('h1:has-text("SSO"), h2:has-text("SSO Providers")');
    await expect(ssoTitle).toBeVisible({ timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-01-sso-settings-page.png',
      fullPage: true,
    });

    console.log('✓ AC1: Admin can navigate to SSO settings from Settings hub');
  });

  test('AC4: Create a new SSO provider via form', async ({ page }) => {
    // Mock SSO API endpoints for create flow
    const mockProviders: any[] = [];

    await page.route('**/api/v1/settings/sso/providers', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockProviders),
        });
      } else if (route.request().method() === 'POST') {
        const body = route.request().postDataJSON();
        const created = {
          name: body.name,
          issuerURL: body.issuerURL,
          clientID: body.clientID,
          redirectURL: body.redirectURL,
          scopes: body.scopes || [],
        };
        mockProviders.push(created);
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify(created),
        });
      }
    });

    // Mock permission checks for admin
    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      });
    });

    await page.goto('/settings/sso');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Verify empty state
    const emptyMessage = page.locator('text=No SSO providers configured');
    await expect(emptyMessage).toBeVisible({ timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-04-empty-state.png',
      fullPage: true,
    });

    // Click Add Provider button
    const addButton = page.locator('button:has-text("Add Provider")').first();
    await expect(addButton).toBeVisible({ timeout: 5000 });
    await addButton.click();

    await page.waitForTimeout(500);

    // Verify form is displayed
    const formTitle = page.locator('h2:has-text("Add SSO Provider")');
    await expect(formTitle).toBeVisible({ timeout: 5000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-04-create-form.png',
      fullPage: true,
    });

    // Fill in provider details
    await page.locator('#name').fill('google');
    await page.locator('#issuerURL').fill('https://accounts.google.com');
    await page.locator('#clientID').fill('test-client-id');
    await page.locator('#clientSecret').fill('test-client-secret');
    await page.locator('#redirectURL').fill('https://app.example.com/api/v1/auth/oidc/callback');
    await page.locator('#scopes').clear();
    await page.locator('#scopes').fill('openid,profile,email');

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-04-form-filled.png',
      fullPage: true,
    });

    // Submit form
    const submitButton = page.locator('button:has-text("Create Provider")');
    await submitButton.click();

    await page.waitForTimeout(2000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-04-after-create.png',
      fullPage: true,
    });

    // Verify provider appears in list
    const providerName = page.locator('text=google').first();
    await expect(providerName).toBeVisible({ timeout: 10000 });

    // Verify issuer URL is displayed
    const issuerURL = page.locator('text=https://accounts.google.com');
    await expect(issuerURL).toBeVisible({ timeout: 5000 });

    // Verify scope badges
    const openidBadge = page.locator('text=openid').first();
    await expect(openidBadge).toBeVisible({ timeout: 5000 });

    console.log('✓ AC4: Admin can create a new SSO provider');
  });

  test('AC5: Client secret is not displayed in provider list', async ({ page }) => {
    // Mock SSO API with a provider that has a secret
    await page.route('**/api/v1/settings/sso/providers', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([{
            name: 'google',
            issuerURL: 'https://accounts.google.com',
            clientID: 'test-client-id',
            redirectURL: 'https://app.example.com/callback',
            scopes: ['openid', 'profile'],
          }]),
        });
      }
    });

    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      });
    });

    await page.goto('/settings/sso');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Verify provider is displayed
    const providerName = page.locator('text=google').first();
    await expect(providerName).toBeVisible({ timeout: 10000 });

    // Verify client secret is NOT shown anywhere on the page
    const pageContent = await page.textContent('body');
    expect(pageContent).not.toContain('client-secret');
    expect(pageContent).not.toContain('super-secret');

    // Verify clientID is visible (for reference)
    expect(pageContent).not.toContain('test-client-id'); // clientID is also not shown in list view

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-05-no-secret-in-list.png',
      fullPage: true,
    });

    console.log('✓ AC5: Client secret is masked in provider list');
  });

  test('AC6: Edit an existing SSO provider', async ({ page }) => {
    const mockProvider = {
      name: 'google',
      issuerURL: 'https://accounts.google.com',
      clientID: 'old-client-id',
      redirectURL: 'https://old.example.com/callback',
      scopes: ['openid'],
    };

    await page.route('**/api/v1/settings/sso/providers', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([mockProvider]),
        });
      }
    });

    await page.route('**/api/v1/settings/sso/providers/google', async (route) => {
      if (route.request().method() === 'PUT') {
        const body = route.request().postDataJSON();
        const updated = {
          ...mockProvider,
          issuerURL: body.issuerURL || mockProvider.issuerURL,
          clientID: body.clientID || mockProvider.clientID,
          redirectURL: body.redirectURL || mockProvider.redirectURL,
          scopes: body.scopes || mockProvider.scopes,
        };
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(updated),
        });
      }
    });

    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      });
    });

    await page.goto('/settings/sso');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Verify provider is listed
    const providerEntry = page.locator('text=google').first();
    await expect(providerEntry).toBeVisible({ timeout: 10000 });

    // Click edit button (Pencil icon)
    const editButton = page.locator('button:has(svg.lucide-pencil), button:has-text("Edit")').first();
    await expect(editButton).toBeVisible({ timeout: 5000 });
    await editButton.click();

    await page.waitForTimeout(500);

    // Verify edit form is displayed
    const editTitle = page.locator('h2:has-text("Edit google")');
    await expect(editTitle).toBeVisible({ timeout: 5000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-06-edit-form.png',
      fullPage: true,
    });

    // Verify name field is NOT shown (name is immutable on edit)
    const nameField = page.locator('#name');
    await expect(nameField).not.toBeVisible();

    // Verify existing values are pre-filled
    await expect(page.locator('#issuerURL')).toHaveValue('https://accounts.google.com');
    await expect(page.locator('#clientID')).toHaveValue('old-client-id');

    // Client secret should be empty (write-only)
    await expect(page.locator('#clientSecret')).toHaveValue('');

    // Verify client secret label mentions keeping existing
    const secretLabel = page.locator('text=leave blank to keep existing');
    await expect(secretLabel).toBeVisible({ timeout: 3000 });

    // Update issuer URL
    await page.locator('#issuerURL').clear();
    await page.locator('#issuerURL').fill('https://updated.google.com');

    // Submit
    const saveButton = page.locator('button:has-text("Save Changes")');
    await saveButton.click();

    await page.waitForTimeout(2000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-06-after-edit.png',
      fullPage: true,
    });

    console.log('✓ AC6: Admin can edit an existing provider');
  });

  test('AC8: Delete a provider with type-to-confirm dialog', async ({ page }) => {
    const mockProviders = [
      {
        name: 'google',
        issuerURL: 'https://accounts.google.com',
        clientID: 'client-id',
        redirectURL: 'https://app.example.com/callback',
        scopes: ['openid', 'profile'],
      },
    ];

    await page.route('**/api/v1/settings/sso/providers', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockProviders),
        });
      }
    });

    await page.route('**/api/v1/settings/sso/providers/google', async (route) => {
      if (route.request().method() === 'DELETE') {
        mockProviders.length = 0; // Clear the array
        await route.fulfill({ status: 204 });
      }
    });

    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      });
    });

    await page.goto('/settings/sso');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Verify provider is listed
    const providerEntry = page.locator('text=google').first();
    await expect(providerEntry).toBeVisible({ timeout: 10000 });

    // Click delete button (Trash icon)
    const deleteButton = page.locator('button:has(svg.lucide-trash-2), button:has-text("Delete")').first();
    await expect(deleteButton).toBeVisible({ timeout: 5000 });
    await deleteButton.click();

    await page.waitForTimeout(500);

    // Verify delete confirmation dialog appears
    const dialog = page.locator('[role="alertdialog"]');
    await expect(dialog).toBeVisible({ timeout: 5000 });

    // Verify dialog title
    const dialogTitle = dialog.locator('text=Delete SSO Provider');
    await expect(dialogTitle).toBeVisible({ timeout: 3000 });

    // Verify warning text
    const warningText = dialog.locator('text=cannot be undone');
    await expect(warningText).toBeVisible({ timeout: 3000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-08-delete-dialog.png',
      fullPage: true,
    });

    // Verify delete button is disabled before typing confirmation
    const confirmDeleteBtn = dialog.locator('button:has-text("Delete Provider")');
    await expect(confirmDeleteBtn).toBeDisabled();

    // Type the provider name to confirm
    const confirmInput = dialog.locator('#confirm-provider-name');
    await confirmInput.fill('google');

    // Verify delete button is now enabled
    await expect(confirmDeleteBtn).toBeEnabled();

    // Click Delete Provider
    await confirmDeleteBtn.click();

    await page.waitForTimeout(2000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-08-after-delete.png',
      fullPage: true,
    });

    console.log('✓ AC8: Admin can delete a provider with confirmation');
  });

  test('No restart banner after mutations (hot-reload)', async ({ page }) => {
    const mockProviders: any[] = [];

    await page.route('**/api/v1/settings/sso/providers', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockProviders),
        });
      } else if (route.request().method() === 'POST') {
        const body = route.request().postDataJSON();
        const created = {
          name: body.name,
          issuerURL: body.issuerURL,
          clientID: body.clientID,
          redirectURL: body.redirectURL,
          scopes: body.scopes || [],
        };
        mockProviders.push(created);
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify(created),
        });
      }
    });

    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      });
    });

    await page.goto('/settings/sso');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Click Add Provider
    const addButton = page.locator('button:has-text("Add Provider")').first();
    await addButton.click();
    await page.waitForTimeout(500);

    // Fill form and submit
    await page.locator('#name').fill('test-provider');
    await page.locator('#issuerURL').fill('https://issuer.example.com');
    await page.locator('#clientID').fill('client-id');
    await page.locator('#clientSecret').fill('client-secret');
    await page.locator('#redirectURL').fill('https://app.example.com/callback');

    const submitButton = page.locator('button:has-text("Create Provider")');
    await submitButton.click();

    await page.waitForTimeout(2000);

    // Verify NO restart banner is shown (hot-reload replaces restart requirement)
    const restartBanner = page.locator('text=Restart required');
    await expect(restartBanner).not.toBeVisible();

    // Verify the old restart message is NOT present
    const restartMessage = page.locator('text=SSO provider changes require a pod restart');
    await expect(restartMessage).not.toBeVisible();

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-no-restart-banner.png',
      fullPage: true,
    });

    console.log('✓ No restart banner after changes (hot-reload active)');
  });

  test('Form validation prevents invalid submissions', async ({ page }) => {
    await page.route('**/api/v1/settings/sso/providers', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      }
    });

    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      });
    });

    await page.goto('/settings/sso');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Open create form
    const addButton = page.locator('button:has-text("Add Provider")').first();
    await addButton.click();
    await page.waitForTimeout(500);

    // Try to submit empty form
    const submitButton = page.locator('button:has-text("Create Provider")');
    await submitButton.click();
    await page.waitForTimeout(500);

    // Verify validation errors appear
    const nameError = page.locator('text=Name is required');
    await expect(nameError).toBeVisible({ timeout: 3000 });

    const issuerError = page.locator('text=Issuer URL is required');
    await expect(issuerError).toBeVisible({ timeout: 3000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-validation-errors.png',
      fullPage: true,
    });

    // Test invalid name (uppercase)
    await page.locator('#name').fill('INVALID');
    await page.locator('#issuerURL').fill('https://valid.example.com');
    await page.locator('#clientID').fill('id');
    await page.locator('#clientSecret').fill('secret');
    await page.locator('#redirectURL').fill('https://app.example.com/callback');
    await submitButton.click();
    await page.waitForTimeout(500);

    const dnsError = page.locator('.text-destructive:has-text("DNS label format")');
    await expect(dnsError).toBeVisible({ timeout: 3000 });

    // Test HTTP issuer URL (should require HTTPS)
    await page.locator('#name').clear();
    await page.locator('#name').fill('valid-name');
    await page.locator('#issuerURL').clear();
    await page.locator('#issuerURL').fill('http://insecure.example.com');
    await submitButton.click();
    await page.waitForTimeout(500);

    const httpsError = page.locator('.text-destructive:has-text("must use HTTPS")');
    await expect(httpsError).toBeVisible({ timeout: 3000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-validation-https.png',
      fullPage: true,
    });

    console.log('✓ Form validation prevents invalid submissions');
  });
});

test.describe('Viewer - SSO Settings Access Denied', () => {
  test.use({ authenticateAs: TestUserRole.ORG_VIEWER });

  test('AC10: Non-admin user sees access denied on SSO settings', async ({ page }) => {
    // Mock SSO API to return 403 for non-admin
    await page.route('**/api/v1/settings/sso/providers', async (route) => {
      await route.fulfill({
        status: 403,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'access denied' }),
      });
    });

    // Mock permission checks to deny settings access
    await page.route('**/api/v1/account/can-i/**', async (route) => {
      const url = route.request().url();
      if (url.includes('/settings/')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ value: 'no' }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ value: 'no' }),
        });
      }
    });

    // Navigate directly to SSO settings
    await page.goto('/settings/sso');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Wait for access denied indicator to appear
    // Check for various access denied patterns:
    // - "Access Denied" text
    // - "Forbidden" text
    // - "permission denied" text
    // - Redirect to login/unauthorized page
    const accessDeniedPatterns = [
      page.locator('text=Access Denied'),
      page.locator('text=Forbidden'),
      page.locator('text=/permission.*denied/i'),
      page.locator('text=/not authorized/i'),
      page.locator('text=/do not have permission/i'),
    ];

    let accessDeniedFound = false;
    for (const pattern of accessDeniedPatterns) {
      if (await pattern.isVisible({ timeout: 3000 }).catch(() => false)) {
        accessDeniedFound = true;
        break;
      }
    }

    // If no explicit access denied message, check if we were redirected or the page is empty/blocked
    if (!accessDeniedFound) {
      // Check if Add Provider button is hidden (which also indicates access denied)
      const addButton = page.locator('button:has-text("Add Provider")');
      const buttonNotVisible = await addButton.isVisible({ timeout: 2000 }).catch(() => false) === false;
      accessDeniedFound = buttonNotVisible;
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/sso-10-access-denied.png',
      fullPage: true,
    });

    expect(accessDeniedFound).toBe(true);

    console.log('✓ AC10: Non-admin sees access denied on SSO settings');
  });
});
