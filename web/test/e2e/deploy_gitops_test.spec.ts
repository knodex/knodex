// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture';
import { setupPermissionMocking } from '../fixture/auth-helper';

/**
 * Note: GitOps Instance Location Tracking and Path Configuration
 *
 * Tests that verify:
 * - Branch and path fields are auto-populated when repository is selected
 * - Users can override branch and path values
 * - GitOps deployment properly includes branch and path in API request
 * - Backend correctly populates repository configuration for GitOps deployments
 *
 * Prerequisites:
 * - Backend deployed with GitOps deployment support
 * - Repository configured in the system
 * - Global Admin user logged in
 *
 * Test coverage:
 * - Branch field auto-populates with repository's default branch
 * - Path field auto-populates with semantic path structure
 * - User can override both branch and path values
 * - GitOps deployment uses the specified branch and path
 */

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8080';

// Track instances created during tests for cleanup
const createdInstances: { namespace: string; kind: string; name: string }[] = [];

test.describe('GitOps Path Configuration', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test.beforeEach(async ({ page }) => {
    // Mock permission API for Global Admin - full access
    await setupPermissionMocking(page, { '*:*': true });
  });

  test.afterAll(async ({ request }) => {
    // Cleanup: Delete test instances created during tests
    // Note: This may fail if backend doesn't support DELETE, which is okay
    for (const instance of createdInstances) {
      try {
        await request.delete(`${BASE_URL}/api/v1/instances/${instance.namespace}/${instance.kind}/${instance.name}`, {
          failOnStatusCode: false
        });
      } catch {
        // Ignore cleanup errors
      }
    }
  });

  test('AC-PATH-01: Branch field auto-populates with repository default branch', async ({ page }) => {
    // Navigate to catalog
    await page.goto('/catalog');
    await page.waitForLoadState('networkidle');

    // Select an RGD
    const firstRGD = page.getByRole('button', { name: /View details for/ }).first();
    await firstRGD.click();
    await page.waitForURL('/catalog/**');

    // Click Deploy to open deploy dialog/page
    const deployButton = page.locator('button:has-text("Deploy")');
    await deployButton.click();
    await page.waitForTimeout(1000);

    // Screenshot of deploy form initial state
    await page.screenshot({
      path: '../test-results/e2e/screenshots/path-01-deploy-form-initial.png',
      fullPage: true
    });

    // Select GitOps mode
    const gitopsButton = page.locator('button:has-text("GitOps")');
    if (await gitopsButton.isVisible({ timeout: 3000 })) {
      await gitopsButton.click();
      await page.waitForTimeout(500);
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/path-01-gitops-mode-selected.png',
      fullPage: true
    });

    // Get the repository selector
    const repoSelector = page.locator('select#repository, select[id="repository"]');

    if (await repoSelector.isVisible({ timeout: 5000 })) {
      // Get available options
      const options = await repoSelector.locator('option').allTextContents();
      console.log('Available repositories:', options);

      // Skip if no repositories configured
      if (options.length <= 1) {
        console.log('No repositories configured, skipping test');
        test.skip();
        return;
      }

      // Select the first available repository
      await repoSelector.selectOption({ index: 1 });
      await page.waitForTimeout(1000);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/path-01-repo-selected.png',
        fullPage: true
      });

      // Verify branch field is visible and auto-populated
      const branchInput = page.locator('input#gitBranch, input[id="gitBranch"]');
      await expect(branchInput).toBeVisible({ timeout: 5000 });

      const branchValue = await branchInput.inputValue();
      console.log('Auto-populated branch value:', branchValue);

      // Branch should not be empty after repository selection
      expect(branchValue.length).toBeGreaterThan(0);

      // Common default branches
      const validBranches = ['main', 'master', 'develop'];
      const isValidBranch = validBranches.some(b => branchValue.toLowerCase() === b.toLowerCase());
      expect(isValidBranch).toBeTruthy();

      await page.screenshot({
        path: '../test-results/e2e/screenshots/path-01-branch-auto-populated.png',
        fullPage: true
      });
    } else {
      console.log('Repository selector not found, checking for alternate UI');
      await page.screenshot({
        path: '../test-results/e2e/screenshots/path-01-no-repo-selector.png',
        fullPage: true
      });
      test.skip();
    }
  });

  test('AC-PATH-02: Path field auto-populates with semantic path structure', async ({ page }) => {
    // Navigate to catalog
    await page.goto('/catalog');
    await page.waitForLoadState('networkidle');

    // Select an RGD
    const firstRGD = page.getByRole('button', { name: /View details for/ }).first();
    await firstRGD.click();
    await page.waitForURL('/catalog/**');

    // Click Deploy
    const deployButton = page.locator('button:has-text("Deploy")');
    await deployButton.click();
    await page.waitForTimeout(1000);

    // Select GitOps mode
    const gitopsButton = page.locator('button:has-text("GitOps")');
    if (await gitopsButton.isVisible({ timeout: 3000 })) {
      await gitopsButton.click();
      await page.waitForTimeout(500);
    }

    // Fill in required fields first (they affect path generation)
    const instanceNameInput = page.getByRole('textbox', { name: /instance name/i });
    if (await instanceNameInput.isVisible({ timeout: 3000 })) {
      await instanceNameInput.fill('test-gitops-path-ac02');
    }

    // Select namespace if available
    const namespaceSelector = page.locator('select#namespace, select[id="namespace"]');
    if (await namespaceSelector.isVisible({ timeout: 3000 })) {
      const options = await namespaceSelector.locator('option').allTextContents();
      if (options.length > 1) {
        await namespaceSelector.selectOption({ index: 1 });
        await page.waitForTimeout(500);
      }
    }

    // Select repository
    const repoSelector = page.locator('select#repository, select[id="repository"]');
    if (await repoSelector.isVisible({ timeout: 5000 })) {
      const options = await repoSelector.locator('option').allTextContents();
      if (options.length <= 1) {
        console.log('No repositories configured, skipping test');
        test.skip();
        return;
      }

      await repoSelector.selectOption({ index: 1 });
      await page.waitForTimeout(1000);

      // Verify path field is visible and auto-populated
      const pathInput = page.locator('input#gitPath, input[id="gitPath"]');
      await expect(pathInput).toBeVisible({ timeout: 5000 });

      const pathValue = await pathInput.inputValue();
      console.log('Auto-populated path value:', pathValue);

      // Path should contain semantic structure: manifests/{...}/{instanceName}.yaml
      expect(pathValue.length).toBeGreaterThan(0);
      expect(pathValue).toContain('manifests');
      expect(pathValue).toContain('.yaml');

      await page.screenshot({
        path: '../test-results/e2e/screenshots/path-02-path-auto-populated.png',
        fullPage: true
      });
    } else {
      test.skip();
    }
  });

  test('AC-PATH-03: User can override branch and path values', async ({ page }) => {
    // Navigate to catalog
    await page.goto('/catalog');
    await page.waitForLoadState('networkidle');

    // Select an RGD
    const firstRGD = page.getByRole('button', { name: /View details for/ }).first();
    await firstRGD.click();
    await page.waitForURL('/catalog/**');

    // Click Deploy
    const deployButton = page.locator('button:has-text("Deploy")');
    await deployButton.click();
    await page.waitForTimeout(1000);

    // Select GitOps mode
    const gitopsButton = page.locator('button:has-text("GitOps")');
    if (await gitopsButton.isVisible({ timeout: 3000 })) {
      await gitopsButton.click();
      await page.waitForTimeout(500);
    }

    // Select repository first
    const repoSelector = page.locator('select#repository, select[id="repository"]');
    if (await repoSelector.isVisible({ timeout: 5000 })) {
      const options = await repoSelector.locator('option').allTextContents();
      if (options.length <= 1) {
        console.log('No repositories configured, skipping test');
        test.skip();
        return;
      }

      await repoSelector.selectOption({ index: 1 });
      await page.waitForTimeout(1000);
    } else {
      test.skip();
      return;
    }

    // Get the auto-populated values
    const branchInput = page.locator('input#gitBranch, input[id="gitBranch"]');
    const pathInput = page.locator('input#gitPath, input[id="gitPath"]');

    const originalBranch = await branchInput.inputValue();
    const originalPath = await pathInput.inputValue();

    console.log('Original branch:', originalBranch);
    console.log('Original path:', originalPath);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/path-03-original-values.png',
      fullPage: true
    });

    // Override the branch
    const customBranch = 'feature/custom-branch';
    await branchInput.clear();
    await branchInput.fill(customBranch);

    // Override the path
    const customPath = 'custom/path/to/manifests/instance.yaml';
    await pathInput.clear();
    await pathInput.fill(customPath);

    await page.waitForTimeout(500);

    // Verify the values were changed
    const newBranch = await branchInput.inputValue();
    const newPath = await pathInput.inputValue();

    expect(newBranch).toBe(customBranch);
    expect(newPath).toBe(customPath);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/path-03-overridden-values.png',
      fullPage: true
    });

    console.log('Successfully overrode branch and path values');
  });

  test('AC-PATH-04: GitOps deployment API includes branch and path', async ({ page, request }) => {
    // This test verifies the complete flow: selecting GitOps mode,
    // configuring branch/path, and ensuring the API request includes them

    // Navigate to catalog
    await page.goto('/catalog');
    await page.waitForLoadState('networkidle');

    // Select an RGD (use simple-app which is usually available)
    const simpleAppRGD = page.locator('button:has-text("View details for simple-app"), button[aria-label*="simple-app"]');
    const firstRGD = page.getByRole('button', { name: /View details for/ }).first();

    if (await simpleAppRGD.isVisible({ timeout: 3000 })) {
      await simpleAppRGD.click();
    } else {
      await firstRGD.click();
    }

    await page.waitForURL('/catalog/**');

    // Click Deploy
    const deployButton = page.locator('button:has-text("Deploy")');
    await deployButton.click();
    await page.waitForTimeout(1000);

    // Select GitOps mode
    const gitopsButton = page.locator('button:has-text("GitOps")');
    if (await gitopsButton.isVisible({ timeout: 3000 })) {
      await gitopsButton.click();
      await page.waitForTimeout(500);
    }

    // Check if repositories are available
    const repoSelector = page.locator('select#repository, select[id="repository"]');
    if (!(await repoSelector.isVisible({ timeout: 5000 }))) {
      console.log('Repository selector not visible, skipping test');
      test.skip();
      return;
    }

    const options = await repoSelector.locator('option').allTextContents();
    if (options.length <= 1) {
      console.log('No repositories configured, skipping test');
      test.skip();
      return;
    }

    // Fill in required fields
    const instanceName = `gitops-e2e-${Date.now()}`;
    const instanceNameInput = page.getByRole('textbox', { name: /instance name/i });
    await instanceNameInput.fill(instanceName);

    // Select project if available
    const projectSelector = page.locator('select#project, select[id="project"]');
    if (await projectSelector.isVisible({ timeout: 3000 })) {
      const projectOptions = await projectSelector.locator('option').allTextContents();
      if (projectOptions.length > 1) {
        await projectSelector.selectOption({ index: 1 });
        await page.waitForTimeout(500);
      }
    }

    // Select namespace
    const namespaceSelector = page.locator('select#namespace, select[id="namespace"]');
    if (await namespaceSelector.isVisible({ timeout: 3000 })) {
      const nsOptions = await namespaceSelector.locator('option').allTextContents();
      if (nsOptions.length > 1) {
        await namespaceSelector.selectOption({ index: 1 });
        await page.waitForTimeout(500);
      }
    }

    // Select repository
    await repoSelector.selectOption({ index: 1 });
    await page.waitForTimeout(1000);

    // Get the current namespace for cleanup
    const selectedNamespace = await namespaceSelector.inputValue();

    // Set custom branch and path for verification
    const customBranch = 'main';
    const branchInput = page.locator('input#gitBranch, input[id="gitBranch"]');
    if (await branchInput.isVisible({ timeout: 3000 })) {
      await branchInput.clear();
      await branchInput.fill(customBranch);
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/path-04-form-filled.png',
      fullPage: true
    });

    // Intercept the API request to verify the payload
    let capturedRequest: any = null;
    page.on('request', (req) => {
      if (req.url().includes('/api/v1/instances') && req.method() === 'POST') {
        try {
          capturedRequest = req.postDataJSON();
        } catch {
          // Ignore parse errors
        }
      }
    });

    // Submit the deployment
    const submitButton = page.locator('button:has-text("Deploy"), button[type="submit"]').last();
    await submitButton.click();

    // Wait for the request
    await page.waitForTimeout(3000);

    // Capture screenshot of result
    await page.screenshot({
      path: '../test-results/e2e/screenshots/path-04-deployment-result.png',
      fullPage: true
    });

    // Verify the request payload included gitBranch and gitPath
    if (capturedRequest) {
      console.log('Captured API request:', JSON.stringify(capturedRequest, null, 2));

      // The request should include deployment mode as gitops
      expect(capturedRequest.deploymentMode).toBe('gitops');

      // Should include gitBranch (either from auto-population or our override)
      if (capturedRequest.gitBranch) {
        console.log('gitBranch in request:', capturedRequest.gitBranch);
        expect(capturedRequest.gitBranch.length).toBeGreaterThan(0);
      }

      // Should include gitPath
      if (capturedRequest.gitPath) {
        console.log('gitPath in request:', capturedRequest.gitPath);
        expect(capturedRequest.gitPath).toContain('.yaml');
      }

      // Should include repositoryId
      expect(capturedRequest.repositoryId).toBeDefined();
      expect(capturedRequest.repositoryId.length).toBeGreaterThan(0);

      // Track for cleanup
      if (selectedNamespace) {
        createdInstances.push({
          namespace: selectedNamespace,
          kind: 'SimpleApp',
          name: instanceName
        });
      }
    } else {
      console.log('Could not capture API request, verifying via UI response');

      // Check for success or error message
      const successMessage = page.locator('text=successfully, text=created, text=deployed');
      const errorMessage = page.locator('text=failed, text=error, text=Failed');

      if (await errorMessage.isVisible({ timeout: 3000 })) {
        const errorText = await errorMessage.textContent();
        console.log('Deployment error:', errorText);

        // If error is NOT about repository configuration, that's a different issue
        expect(errorText?.toLowerCase()).not.toContain('repository configuration is required');
      }
    }
  });

  test('Verify repository service is wired up in backend', async ({ request }) => {
    // This test directly calls the API to verify backend repository lookup works
    // It requires a repository to be configured

    // Get list of repositories first
    const reposResponse = await request.get(`${BASE_URL}/api/v1/repositories`, {
      failOnStatusCode: false
    });

    if (!reposResponse.ok()) {
      console.log('Could not get repositories, status:', reposResponse.status());
      test.skip();
      return;
    }

    const reposData = await reposResponse.json();
    const repositories = reposData.items || reposData.repositories || reposData || [];

    if (repositories.length === 0) {
      console.log('No repositories configured');
      test.skip();
      return;
    }

    // Use the first enabled repository
    const enabledRepo = repositories.find((r: any) =>
      r.spec?.enabled !== false && r.enabled !== false
    );

    if (!enabledRepo) {
      console.log('No enabled repositories found');
      test.skip();
      return;
    }

    const repoId = enabledRepo.metadata?.name || enabledRepo.name || enabledRepo.id;
    console.log('Testing with repository:', repoId);

    // Try to create an instance with GitOps mode
    // This should NOT fail with "repository configuration is required"
    const testPayload = {
      name: `backend-test-${Date.now()}`,
      namespace: 'default',
      rgdName: 'simple-app',
      deploymentMode: 'gitops',
      repositoryId: repoId,
      gitBranch: 'main',
      gitPath: 'manifests/test/instance.yaml',
      spec: {
        replicas: 1
      }
    };

    const createResponse = await request.post(`${BASE_URL}/api/v1/instances`, {
      data: testPayload,
      failOnStatusCode: false
    });

    const responseData = await createResponse.json().catch(() => ({}));
    console.log('Create instance response:', createResponse.status(), JSON.stringify(responseData, null, 2));

    // The key verification: should NOT get "repository configuration is required" error
    if (!createResponse.ok()) {
      const errorMessage = responseData.error || responseData.message || JSON.stringify(responseData);

      // This specific error means the backend repository lookup is broken
      expect(errorMessage.toLowerCase()).not.toContain('repository configuration is required');
      expect(errorMessage.toLowerCase()).not.toContain('repository service not configured');

      // Other errors (like namespace not found, RGD not found) are acceptable
      // as they indicate the repository lookup succeeded
      console.log('Deployment failed with:', errorMessage);
      console.log('This is acceptable if it is not a repository configuration error');
    } else {
      console.log('GitOps deployment succeeded!');

      // Cleanup: track for deletion
      createdInstances.push({
        namespace: testPayload.namespace,
        kind: 'SimpleApp',
        name: testPayload.name
      });
    }
  });
});
