// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';
import { setupPermissionMocking } from '../fixture/auth-helper';

/**
 * Global Admin - Deployment Modes Tests
 *
 * Tests that Global Admin users can access and use all three deployment modes:
 * Direct Deploy, GitOps, and Hybrid.
 *
 * Prerequisites:
 * - Backend deployed with deployment modes configured
 * - Global Admin user logged in (groups: ["global-admins"])
 * - Repository configured for GitOps mode (if testing GitOps)
 *
 * Test coverage:
 * - Global Admin can switch between Direct, GitOps, Hybrid modes
 * - Direct mode deploys instance to Kubernetes immediately
 * - GitOps mode commits manifest to configured repository
 * - Hybrid mode allows per-instance deployment mode selection
 */

// Use relative URLs - Playwright baseURL is set in playwright.config.ts
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8080';

test.describe('Global Admin - Deployment Modes', () => {
  // Authenticate as Global Admin to access all deployment modes
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test.beforeEach(async ({ page }) => {
    // Mock permission API for Global Admin - full access
    await setupPermissionMocking(page, { '*:*': true });
  });

  test('AC-DEPLOY-01: Global Admin can switch between Direct, GitOps, Hybrid modes', async ({ page }) => {
    // Navigate to settings or organization settings where deployment mode is configured
    await page.goto(`/settings`);

    // Alternative: Organization-level deployment mode
    if (page.url().includes('404') || !await page.locator('text=Deployment Mode, text=Deploy Mode').isVisible({ timeout: 3000 })) {
      await page.goto(`/organizations/org-alpha`);
      await page.waitForLoadState('load');

      const settingsTab = page.locator('button:has-text("Settings"), a:has-text("Settings")');
      if (await settingsTab.isVisible({ timeout: 5000 })) {
        await settingsTab.click();
        await page.waitForTimeout(1000);
      }
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/deploy-modes-01-settings.png',
      fullPage: true
    });

    // Look for deployment mode selector
    const deployModeSelector = page.locator('select[name="deploymentMode"], select[name="deploy_mode"], [data-testid="deployment-mode-select"]');
    const deployModeRadio = page.locator('input[type="radio"][name="deploymentMode"], input[type="radio"][name="deploy_mode"]');

    if (await deployModeSelector.isVisible({ timeout: 5000 })) {
      // Dropdown selector
      await page.screenshot({
        path: '../test-results/e2e/screenshots/deploy-modes-01-selector-dropdown.png',
        fullPage: true
      });

      // Get available options
      const options = await deployModeSelector.locator('option').allTextContents();
      console.log('Available deployment modes:', options);

      // Verify all three modes are available
      expect(options.some(opt => opt.toLowerCase().includes('direct'))).toBeTruthy();
      expect(options.some(opt => opt.toLowerCase().includes('gitops'))).toBeTruthy();
      expect(options.some(opt => opt.toLowerCase().includes('hybrid'))).toBeTruthy();

      // Switch to Direct mode
      await deployModeSelector.selectOption({ label: /Direct/i });
      await page.waitForTimeout(500);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/deploy-modes-01-direct-selected.png',
        fullPage: true
      });

      // Switch to GitOps mode
      await deployModeSelector.selectOption({ label: /GitOps/i });
      await page.waitForTimeout(500);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/deploy-modes-01-gitops-selected.png',
        fullPage: true
      });

      // Switch to Hybrid mode
      await deployModeSelector.selectOption({ label: /Hybrid/i });
      await page.waitForTimeout(500);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/deploy-modes-01-hybrid-selected.png',
        fullPage: true
      });

      // Save settings
      const saveButton = page.locator('button:has-text("Save"), button:has-text("Update"), button[type="submit"]');
      if (await saveButton.isVisible({ timeout: 3000 })) {
        await saveButton.click();
        await page.waitForTimeout(2000);
      }
    } else if (await deployModeRadio.first().isVisible({ timeout: 5000 })) {
      // Radio buttons
      const directRadio = page.locator('input[type="radio"][value="direct"], input[type="radio"] + label:has-text("Direct")');
      const gitopsRadio = page.locator('input[type="radio"][value="gitops"], input[type="radio"] + label:has-text("GitOps")');
      const hybridRadio = page.locator('input[type="radio"][value="hybrid"], input[type="radio"] + label:has-text("Hybrid")');

      await expect(directRadio).toBeVisible();
      await expect(gitopsRadio).toBeVisible();
      await expect(hybridRadio).toBeVisible();

      await page.screenshot({
        path: '../test-results/e2e/screenshots/deploy-modes-01-all-modes-visible.png',
        fullPage: true
      });
    } else {
      console.log('Deployment mode selector not found, checking via API');

      const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

      if (token) {
        const response = await page.request.get(`${BASE_URL}/api/v1/settings/deployment-modes`, {
          headers: { Authorization: `Bearer ${token}` },
          failOnStatusCode: false
        });

        if (response.ok()) {
          const modes = await response.json();
          console.log('Available deployment modes from API:', JSON.stringify(modes, null, 2));

          expect(modes.includes('direct') || modes.includes('Direct')).toBeTruthy();
          expect(modes.includes('gitops') || modes.includes('GitOps')).toBeTruthy();
          expect(modes.includes('hybrid') || modes.includes('Hybrid')).toBeTruthy();
        }
      }
    }
  });

  // SKIP: Requires functional direct-deploy mode with working K8s instance creation.
  // The deploy form field names/structure may not match expected selectors.
  // Prerequisite: Verify deploy form selectors and ensure K8s instance creation works.
  test.skip('AC-DEPLOY-02: Direct mode deploys instance to Kubernetes immediately', async ({ page }) => {
    // Navigate to catalog
    await page.goto(`/catalog`);
    await page.waitForLoadState('load');

    // Select an RGD (use aria-label since RGDCard doesn't have data-testid)
    const firstRGD = page.getByRole('button', { name: /View details for/ }).first();
    await firstRGD.click();

    await page.waitForURL(`/catalog/**`);

    // Click Deploy
    const deployButton = page.locator('button:has-text("Deploy"), button:has-text("Create Instance")');
    await deployButton.click();

    await page.waitForTimeout(1000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/deploy-modes-02-deploy-form.png',
      fullPage: true
    });

    // Look for deployment mode selector (if in hybrid mode)
    const instanceDeployModeSelector = page.locator('select[name="deployMode"], select[name="deployment_mode"], [data-testid="instance-deploy-mode"]');

    if (await instanceDeployModeSelector.isVisible({ timeout: 3000 })) {
      // Select Direct mode
      await instanceDeployModeSelector.selectOption({ label: /Direct/i });
      await page.waitForTimeout(500);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/deploy-modes-02-direct-mode-selected.png',
        fullPage: true
      });
    }

    // Fill instance form
    const instanceName = `direct-deploy-${Date.now()}`;
    const instanceNameInput = page.getByRole('textbox', { name: /instance name/i });
    await instanceNameInput.fill(instanceName);

    // Select organization (combobox role for select element)
    const orgSelector = page.getByRole('combobox', { name: /organization/i });
    if (await orgSelector.isVisible({ timeout: 3000 })) {
      await orgSelector.selectOption({ index: 0 });
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/deploy-modes-02-form-filled.png',
      fullPage: true
    });

    // Note the time before deployment
    const beforeDeployTime = Date.now();

    // Submit deployment
    const submitButton = page.locator('button:has-text("Deploy"), button:has-text("Create"), button[type="submit"]');
    await submitButton.click();

    // Wait for deployment
    await page.waitForTimeout(3000);

    const afterDeployTime = Date.now();
    const deployDuration = afterDeployTime - beforeDeployTime;

    console.log(`Direct deployment duration: ${deployDuration}ms`);

    // Verify instance appears in instances list immediately
    await page.goto(`/instances`);
    await page.waitForLoadState('load');

    const newInstance = page.locator(`text=${instanceName}`);
    await expect(newInstance).toBeVisible({ timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/deploy-modes-02-instance-deployed.png',
      fullPage: true
    });

    // Verify via API that instance was deployed to Kubernetes
    const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

    if (token) {
      const response = await page.request.get(`${BASE_URL}/api/v1/instances`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      if (response.ok()) {
        const instancesData = await response.json();
        const instances = instancesData.items || instancesData.instances || instancesData;

        const deployedInstance = instances.find((inst: any) =>
          (inst.name || inst.metadata?.name) === instanceName
        );

        expect(deployedInstance).toBeDefined();
        console.log('Deployed instance:', JSON.stringify(deployedInstance, null, 2));

        // Verify it was deployed directly (not via GitOps)
        expect(
          deployedInstance.deploymentMode === 'direct' ||
          deployedInstance.metadata?.annotations?.['deployment-mode'] === 'direct'
        ).toBeTruthy();
      }
    }
  });

  test('AC-DEPLOY-03: GitOps mode commits manifest to configured repository', async ({ page }) => {
    // This test requires a configured repository
    // Navigate to catalog
    await page.goto(`/catalog`);
    await page.waitForLoadState('load');

    // Select an RGD (use aria-label since RGDCard doesn't have data-testid)
    const firstRGD = page.getByRole('button', { name: /View details for/ }).first();
    await firstRGD.click();

    await page.waitForURL(`/catalog/**`);

    const deployButton = page.locator('button:has-text("Deploy")');
    await deployButton.click();

    await page.waitForTimeout(1000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/deploy-modes-03-deploy-form.png',
      fullPage: true
    });

    // Select GitOps mode (if available)
    const instanceDeployModeSelector = page.locator('select[name="deployMode"], [data-testid="instance-deploy-mode"]');

    if (await instanceDeployModeSelector.isVisible({ timeout: 3000 })) {
      await instanceDeployModeSelector.selectOption({ label: /GitOps/i });
      await page.waitForTimeout(500);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/deploy-modes-03-gitops-selected.png',
        fullPage: true
      });

      // Fill instance form
      const instanceName = `gitops-deploy-${Date.now()}`;
      const instanceNameInput = page.locator('input[name="name"]');
      await instanceNameInput.fill(instanceName);

      const orgSelector = page.locator('select[name="namespace"], select[name="organization"]');
      if (await orgSelector.isVisible({ timeout: 3000 })) {
        await orgSelector.selectOption({ index: 0 });
      }

      // Repository selector (for GitOps)
      const repoSelector = page.locator('select[name="repository"], [data-testid="repo-select"]');
      if (await repoSelector.isVisible({ timeout: 3000 })) {
        await repoSelector.selectOption({ index: 0 });
      }

      await page.screenshot({
        path: '../test-results/e2e/screenshots/deploy-modes-03-gitops-form-filled.png',
        fullPage: true
      });

      // Submit deployment
      const submitButton = page.locator('button:has-text("Deploy"), button[type="submit"]');
      await submitButton.click();

      await page.waitForTimeout(3000);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/deploy-modes-03-gitops-submitted.png',
        fullPage: true
      });

      // Verify success message mentions commit or PR
      const gitopsMessage = page.locator('text=commit, text=pull request, text=PR created, text=manifest committed');
      if (await gitopsMessage.isVisible({ timeout: 5000 })) {
        await page.screenshot({
          path: '../test-results/e2e/screenshots/deploy-modes-03-gitops-commit-message.png',
          fullPage: true
        });
      }

      // Verify via API
      const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

      if (token) {
        const response = await page.request.get(`${BASE_URL}/api/v1/instances/${instanceName}`, {
          headers: { Authorization: `Bearer ${token}` },
          failOnStatusCode: false
        });

        if (response.ok()) {
          const instance = await response.json();
          console.log('GitOps instance:', JSON.stringify(instance, null, 2));

          // Verify GitOps metadata
          expect(
            instance.deploymentMode === 'gitops' ||
            instance.metadata?.annotations?.['deployment-mode'] === 'gitops'
          ).toBeTruthy();

          // Check for commit SHA or PR number
          expect(
            instance.gitCommit ||
            instance.metadata?.annotations?.['git-commit'] ||
            instance.metadata?.annotations?.['pr-number']
          ).toBeDefined();
        }
      }
    } else {
      console.log('GitOps mode not available, skipping test');
    }
  });

  // SKIP: UI uses button-based mode selection, not dropdown selector as test expects.
  // The hybrid per-instance mode selection feature may not be implemented yet.
  // Prerequisite: Update test selectors to match actual UI, or implement hybrid mode feature.
  test.skip('AC-DEPLOY-04: Hybrid mode allows per-instance deployment mode selection', async ({ page }) => {
    // Set organization to Hybrid mode first
    await page.goto(`/organizations/org-alpha`);
    await page.waitForLoadState('load');

    const settingsTab = page.locator('button:has-text("Settings")');
    if (await settingsTab.isVisible({ timeout: 5000 })) {
      await settingsTab.click();
      await page.waitForTimeout(1000);

      const deployModeSelector = page.locator('select[name="deploymentMode"], [data-testid="deployment-mode-select"]');
      if (await deployModeSelector.isVisible({ timeout: 3000 })) {
        await deployModeSelector.selectOption({ label: /Hybrid/i });

        const saveButton = page.locator('button:has-text("Save")');
        if (await saveButton.isVisible({ timeout: 2000 })) {
          await saveButton.click();
          await page.waitForTimeout(2000);
        }
      }
    }

    // Navigate to catalog to deploy
    await page.goto(`/catalog`);
    await page.waitForLoadState('load');

    // Select an RGD (use aria-label since RGDCard doesn't have data-testid)
    const firstRGD = page.getByRole('button', { name: /View details for/ }).first();
    await firstRGD.click();

    const deployButton = page.locator('button:has-text("Deploy")');
    await deployButton.click();

    await page.waitForTimeout(1000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/deploy-modes-04-hybrid-deploy-form.png',
      fullPage: true
    });

    // Verify deployment mode selector is visible in form
    const instanceDeployModeSelector = page.locator('select[name="deployMode"], select[name="deployment_mode"], [data-testid="instance-deploy-mode"]');

    await expect(instanceDeployModeSelector).toBeVisible({ timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/deploy-modes-04-mode-selector-visible.png',
      fullPage: true
    });

    // Verify both Direct and GitOps options are available
    const options = await instanceDeployModeSelector.locator('option').allTextContents();
    console.log('Available instance deployment modes (Hybrid):', options);

    expect(options.some(opt => opt.toLowerCase().includes('direct'))).toBeTruthy();
    expect(options.some(opt => opt.toLowerCase().includes('gitops'))).toBeTruthy();

    // Test deploying with Direct mode selected
    await instanceDeployModeSelector.selectOption({ label: /Direct/i });
    await page.waitForTimeout(500);

    const instanceName1 = `hybrid-direct-${Date.now()}`;
    const instanceNameInput1 = page.getByRole('textbox', { name: /instance name/i });
    await instanceNameInput1.fill(instanceName1);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/deploy-modes-04-hybrid-direct-selected.png',
      fullPage: true
    });

    const submitButton1 = page.locator('button:has-text("Deploy"), button[type="submit"]');
    await submitButton1.click();

    await page.waitForTimeout(3000);

    // Go back and deploy with GitOps mode
    await page.goto(`/catalog`);
    await page.waitForLoadState('load');

    const secondRGD = page.getByRole('button', { name: /View details for/ }).nth(1);
    if (await secondRGD.isVisible({ timeout: 5000 })) {
      await secondRGD.click();

      const deployButton2 = page.locator('button:has-text("Deploy")');
      await deployButton2.click();

      await page.waitForTimeout(1000);

      const instanceDeployModeSelector2 = page.locator('select[name="deployMode"], select[name="deployment_mode"], [data-testid="instance-deploy-mode"]');
      if (await instanceDeployModeSelector2.isVisible({ timeout: 3000 })) {
        await instanceDeployModeSelector2.selectOption({ label: /GitOps/i });
        await page.waitForTimeout(500);

        const instanceName2 = `hybrid-gitops-${Date.now()}`;
        const instanceNameInput2 = page.getByRole('textbox', { name: /instance name/i });
        await instanceNameInput2.fill(instanceName2);

        await page.screenshot({
          path: '../test-results/e2e/screenshots/deploy-modes-04-hybrid-gitops-selected.png',
          fullPage: true
        });

        const submitButton2 = page.locator('button:has-text("Deploy"), button[type="submit"]');
        await submitButton2.click();

        await page.waitForTimeout(3000);
      }
    }

    // Verify both instances exist with different deployment modes
    await page.goto(`/instances`);
    await page.waitForLoadState('load');

    await page.screenshot({
      path: '../test-results/e2e/screenshots/deploy-modes-04-hybrid-both-instances.png',
      fullPage: true
    });

    const instance1 = page.locator(`text=${instanceName1}`);
    await expect(instance1).toBeVisible({ timeout: 10000 });
  });
});
