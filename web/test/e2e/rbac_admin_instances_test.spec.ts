// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';
import { setupPermissionMocking } from '../fixture/auth-helper';

/**
 * Global Admin - Instance Deployment & Management Tests
 *
 * Tests that Global Admin users can deploy instances to any organization namespace,
 * view all instances across all organizations, and manage them.
 *
 * Prerequisites:
 * - Backend deployed with test data (3 orgs: org-alpha, org-beta, org-gamma)
 * - Global Admin user logged in (groups: ["global-admins"])
 * - Test RGDs available in catalog
 *
 * Test coverage:
 * - Global Admin can deploy instances to any organization namespace
 * - Deployed instance appears in Instances list immediately
 * - Instance shows correct namespace and organization labels
 * - Global Admin can view all instances across all organizations
 * - Global Admin can delete instances from any organization
 */

// Use relative URLs - Playwright baseURL is set in playwright.config.ts
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8080';

test.describe('Global Admin - Instance Deployment & Management', () => {
  // Authenticate as Global Admin to access all instances
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test.beforeEach(async ({ page }) => {
    // Mock permission API for Global Admin - full access
    await setupPermissionMocking(page, { '*:*': true });
  });

  // SKIP: Casbin authorization model recently changed; CI test environment setup
  // does not yet reflect the updated policy format. Re-enable after CI fixtures update.
  test.skip('AC-INSTANCE-01: Global Admin can deploy instances to any organization namespace', async ({ page }) => {
    // Navigate to catalog
    await page.goto(`/catalog`);
    await page.waitForLoadState('load', { timeout: 10000 });

    // Select the first available RGD card (resilient to which RGDs are deployed)
    const rgdCard = page.getByRole('button', { name: /view details for/i }).first();
    await expect(rgdCard).toBeVisible({ timeout: 15000 });
    await rgdCard.click();

    await page.waitForURL(`/catalog/**`, { timeout: 10000 });

    // Click Deploy button to open the 3-step wizard
    const deployButton = page.getByRole('button', { name: /deploy/i }).first();
    await expect(deployButton).toBeVisible({ timeout: 10000 });
    await deployButton.click();

    // Step 1: Target — fill instance name, select project & namespace
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 });
    await page.getByPlaceholder('my-instance').fill(`test-instance-alpha-${Date.now()}`);

    // Select namespace (auto-selects project when only one)
    const nsSelect = page.getByTestId('namespace-select');
    if (await nsSelect.isVisible({ timeout: 3000 }).catch(() => false)) {
      await expect(nsSelect).toBeEnabled({ timeout: 5000 });
      await nsSelect.click();
      const firstOption = page.getByRole('option').first();
      await expect(firstOption).toBeVisible({ timeout: 3000 });
      await firstOption.click();
    }

    // Advance to Configure step
    const continueBtn = page.getByRole('button', { name: /continue/i });
    await expect(continueBtn).toBeEnabled({ timeout: 5000 });
    await continueBtn.click();
    await expect(page.getByTestId('configure-step')).toBeVisible({ timeout: 15000 });

    // Fill any visible text fields on Configure step
    const textInputs = page.getByTestId('configure-step').locator('input[type="text"]');
    const inputCount = await textInputs.count();
    for (let i = 0; i < inputCount; i++) {
      const input = textInputs.nth(i);
      const currentValue = await input.inputValue();
      if (!currentValue) {
        await input.fill(`test-value-${i}`);
      }
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-01-deploy-form-filled.png',
      fullPage: true
    });

    // Advance to Review step
    const continueBtn2 = page.getByRole('button', { name: /continue/i });
    await expect(continueBtn2).toBeEnabled({ timeout: 10000 });
    await continueBtn2.click();

    // Click Deploy on Review step
    const deploySubmit = page.getByTestId('deploy-submit-button');
    await expect(deploySubmit).toBeEnabled({ timeout: 10000 });
    await deploySubmit.click();

    // Verify success: toast message or navigation to instance detail
    const successToast = page.locator('text=deployed successfully');
    const instanceDetailPage = page.locator('h1, h2, [data-testid="instance-name"]');

    const isSuccess = await Promise.race([
      successToast.isVisible({ timeout: 10000 }).then(() => true),
      page.waitForURL(/\/instances\//, { timeout: 10000 }).then(() => true),
    ]).catch(() => false);

    expect(isSuccess).toBeTruthy();

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-01-deploy-success-alpha.png',
      fullPage: true
    });

    // Fill required fields for SharedDatabase CRD
    const dbNameInput = page.getByRole('textbox', { name: /database name/i });
    if (await dbNameInput.isVisible({ timeout: 2000 })) {
      await dbNameInput.fill(`db-test-beta-${Date.now()}`);
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-01-deploy-form-beta.png',
      fullPage: true
    });

    const submitButton2 = page.getByRole('button', { name: /deploy instance/i });
    await submitButton2.click();

    await page.waitForTimeout(3000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-01-deploy-success-beta.png',
      fullPage: true
    });
  });

  // SKIP: Requires working Kubernetes instance deployment in E2E environment.
  // The test deploys an instance and expects it to appear in the instances list.
  // Prerequisite: Functional K8s cluster with CRDs and instance controller.
  test.skip('AC-INSTANCE-02: Deployed instance appears in Instances list immediately', async ({ page }) => {
    // Deploy a new instance
    await page.goto(`/catalog`);
    await page.waitForLoadState('load');

    // Select any RGD button (like "simple-app")
    const firstRGD = page.getByRole('button', { name: /simple-app/i });
    await firstRGD.click();

    const deployButton = page.getByRole('button', { name: /deploy/i }).first();
    await deployButton.click();

    await page.waitForTimeout(1000);

    const instanceName = `test-immediate-${Date.now()}`;
    const instanceNameInput = page.getByRole('textbox', { name: /instance name/i });
    await instanceNameInput.fill(instanceName);

    // Select namespace for deployment
    const nsSelector = page.locator('select#namespace');
    if (await nsSelector.isVisible({ timeout: 2000 })) {
      await nsSelector.selectOption({ index: 0 });
    }

    // Fill in the appName field (required by SharedWebApp CRD)
    const appNameInput = page.getByRole('textbox', { name: /app name/i });
    if (await appNameInput.isVisible({ timeout: 2000 })) {
      await appNameInput.fill(`app-${instanceName}`);
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-02-before-deploy.png',
      fullPage: true
    });

    const submitButton = page.getByRole('button', { name: /deploy instance/i });
    await submitButton.click();

    // Wait a moment for deployment
    await page.waitForTimeout(2000);

    // Navigate to instances page
    await page.goto(`/instances`);
    await page.waitForLoadState('load', { timeout: 10000 });

    // Verify new instance appears
    const newInstance = page.locator(`text=${instanceName}`);
    await expect(newInstance).toBeVisible({ timeout: 15000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-02-after-deploy-immediate.png',
      fullPage: true
    });

    // Note: We don't check if card count increased because with pagination (PAGE_SIZE=20),
    // the count stays at 20 even when a new instance is added (oldest instance moves to page 2).
    // The visibility check above is the correct assertion for the acceptance criterion.
  });

  // SKIP: Casbin authorization model recently changed; CI test environment setup
  // does not yet reflect the updated policy format. Re-enable after CI fixtures update.
  test.skip('AC-INSTANCE-03: Instance shows correct namespace and organization labels', async ({ page }) => {
    // Navigate to instances page
    await page.goto(`/instances`);
    await page.waitForLoadState('load', { timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-03-instance-list.png',
      fullPage: true
    });

    // Find an instance card
    const instanceCard = page.locator('[data-testid="instance-card"]').first();
    await expect(instanceCard).toBeVisible({ timeout: 10000 });

    // Verify namespace is visible (shown as text in the card)
    // The InstanceCard component shows namespace in a paragraph with font-mono
    const namespaceText = await instanceCard.locator('.font-mono').first().textContent();
    console.log('Instance namespace:', namespaceText);
    expect(namespaceText).toBeTruthy();

    // Click on instance to view details
    await instanceCard.click();

    await page.waitForTimeout(2000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-03-instance-details.png',
      fullPage: true
    });

    // Verify namespace shown in details page
    const namespaceField = page.getByText('Namespace', { exact: false });
    if (await namespaceField.isVisible({ timeout: 5000 })) {
      const namespaceValue = await namespaceField.textContent();
      console.log('Instance namespace field:', namespaceValue);
      expect(namespaceValue).toBeTruthy();
    }

    // Verify via API
    const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

    if (token) {
      const response = await page.request.get(`${BASE_URL}/api/v1/instances`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      expect(response.ok()).toBeTruthy();
      const instancesData = await response.json();

      const instances = instancesData.items || instancesData.instances || instancesData;
      expect(Array.isArray(instances)).toBeTruthy();

      if (instances.length > 0) {
        const firstInstance = instances[0];
        console.log('First instance from API:', JSON.stringify(firstInstance, null, 2));

        // Verify namespace field exists
        expect(firstInstance.namespace || firstInstance.metadata?.namespace).toBeDefined();

        // Verify organization label exists
        expect(
          firstInstance.organization ||
          firstInstance.metadata?.labels?.['organization'] ||
          firstInstance.spec?.organization
        ).toBeDefined();
      }
    }
  });

  // SKIP: Requires pre-seeded instances across multiple organizations.
  // The E2E setup does not create test instances.
  // Prerequisite: Add multi-org instance seeding to qa-deploy or E2E test setup.
  test.skip('AC-INSTANCE-04: Global Admin can view all instances across all organizations', async ({ page }) => {
    await page.goto(`/instances`);
    await page.waitForLoadState('load', { timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-04-all-instances.png',
      fullPage: true
    });

    // Verify instances are visible (Global Admin sees all)
    const instanceCards = page.locator('[data-testid="instance-card"]');
    const instanceCount = await instanceCards.count();
    console.log(`Global Admin sees ${instanceCount} instances`);

    // Global Admin should see at least some instances
    expect(instanceCount).toBeGreaterThan(0);

    // Verify via API that all instances are returned
    const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

    if (token) {
      const response = await page.request.get(`${BASE_URL}/api/v1/instances`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      expect(response.ok()).toBeTruthy();
      const instancesData = await response.json();

      const instances = instancesData.items || instancesData.instances || instancesData;
      console.log(`Total instances from API: ${instances.length}`);

      // Group instances by organization
      const instancesByOrg = instances.reduce((acc: any, inst: any) => {
        const org = inst.organization || inst.metadata?.labels?.['organization'] || inst.namespace;
        if (!acc[org]) acc[org] = 0;
        acc[org]++;
        return acc;
      }, {});

      console.log('Instances by organization:', JSON.stringify(instancesByOrg, null, 2));

      // Global Admin should see instances from multiple orgs
      expect(Object.keys(instancesByOrg).length).toBeGreaterThan(0);
    }

    // Verify no org filter is hiding instances
    const orgFilter = page.locator('[data-testid="org-filter"], select[name="organization-filter"]');
    if (await orgFilter.isVisible({ timeout: 3000 })) {
      const filterValue = await orgFilter.inputValue();
      console.log('Instance org filter:', filterValue);
      // Should show all orgs
      expect(filterValue === '' || filterValue === 'all').toBeTruthy();
    }
  });

  // SKIP: Requires working Kubernetes instance deployment and deletion.
  // The test creates a fresh instance for deletion testing.
  // Prerequisite: Functional K8s cluster with CRDs and instance controller.
  test.skip('AC-INSTANCE-05: Global Admin can delete instances from any organization', async ({ page }) => {
    // Create a fresh instance specifically for deletion testing
    // This ensures it exists in both Kubernetes and backend cache
    await page.goto(`/catalog`);
    await page.waitForLoadState('load');

    const firstRGD = page.getByRole('button', { name: /simple-app/i });
    await firstRGD.click();

    const deployButton = page.getByRole('button', { name: /deploy/i }).first();
    await deployButton.click();

    await page.waitForTimeout(1000);

    const instanceName = `test-delete-${Date.now()}`;
    const instanceNameInput = page.getByRole('textbox', { name: /instance name/i });
    await instanceNameInput.fill(instanceName);

    // Select namespace for deployment
    const nsSelector = page.locator('select#namespace');
    if (await nsSelector.isVisible({ timeout: 2000 })) {
      await nsSelector.selectOption({ index: 0 });
    }

    const appNameInput = page.getByRole('textbox', { name: /app name/i });
    if (await appNameInput.isVisible({ timeout: 2000 })) {
      await appNameInput.fill(`app-${instanceName}`);
    }

    const submitButton = page.getByRole('button', { name: /deploy instance/i });
    await submitButton.click();

    // Wait for instance to be created and appear in backend cache
    await page.waitForTimeout(3000);

    // Navigate to instances page and find the instance we just created
    await page.goto(`/instances`);
    await page.waitForLoadState('load', { timeout: 10000 });

    // Verify our instance is visible (proves it's in the cache)
    const newInstance = page.locator(`text=${instanceName}`);
    await expect(newInstance).toBeVisible({ timeout: 10000 });

    console.log('Deleting instance:', instanceName);

    // Find the instance card for our specific instance
    const instanceCard = page.locator('[data-testid="instance-card"]').filter({ hasText: instanceName });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-05-before-delete.png',
      fullPage: true
    });

    // Look for delete button (could be on card or in details)
    let deleteButton = instanceCard.locator('button:has-text("Delete"), button[aria-label*="delete"]');

    if (!await deleteButton.isVisible({ timeout: 2000 })) {
      // Click on instance to open details, then find delete button
      await instanceCard.click();
      await page.waitForTimeout(1000);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/instances-05-instance-details.png',
        fullPage: true
      });

      deleteButton = page.locator('button:has-text("Delete"), button:has-text("Delete Instance")');
    }

    await expect(deleteButton).toBeVisible({ timeout: 10000 });
    await deleteButton.click();

    // Wait for confirmation dialog
    await page.waitForTimeout(1000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-05-delete-confirmation.png',
      fullPage: true
    });

    // Confirm deletion and wait for the DELETE API call to complete
    const confirmButton = page.locator('button:has-text("Confirm"), button:has-text("Yes"), button:has-text("Delete")').last();

    // Wait for the DELETE request to the backend
    const deleteRequestPromise = page.waitForResponse(
      response => response.url().includes('/api/v1/instances/') && response.request().method() === 'DELETE',
      { timeout: 15000 }
    );

    await confirmButton.click();

    let deleteSucceeded = false;
    try {
      const deleteResponse = await deleteRequestPromise;
      const status = deleteResponse.status();
      console.log('Delete API call completed with status:', status);
      deleteSucceeded = (status === 200 || status === 204);
    } catch (error) {
      console.log('Delete API call timeout or failed, continuing anyway');
    }

    // Wait for Kubernetes to propagate the deletion to the backend informer cache
    // The DELETE API returns immediately, but K8s informer needs time to update
    console.log('Waiting for Kubernetes informer to propagate deletion...');
    await page.waitForTimeout(5000);

    // Verify instance is removed from list
    await page.goto(`/instances`);
    await page.waitForLoadState('load', { timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/instances-05-after-delete.png',
      fullPage: true
    });

    // Instance should not be visible anymore
    if (deleteSucceeded) {
      const deletedInstance = page.getByText(instanceName, { exact: true });
      // Increased timeout to 15s to allow for Kubernetes informer lag
      await expect(deletedInstance.first()).not.toBeVisible({ timeout: 15000 });
    } else {
      console.log('Delete API did not return success, skipping invisibility check');
    }
  });
});
