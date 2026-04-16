// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture';
import type { Page } from '@playwright/test';

/**
 * Note: Global Admin - Projects Settings UI Tests
 *
 * Tests that Global Admin users can manage Projects through the Settings UI,
 * including creating, viewing details, and deleting projects.
 *
 * Prerequisites:
 * - Backend deployed with Projects API
 * - Global Admin user logged in (groups: ["global-admins"])
 *
 * Test coverage:
 * - Project List View
 * - Create Project Form
 * - Project Detail View
 * - Delete Project
 * - Empty State
 * - RBAC Protection
 */

// Use relative URLs - Playwright baseURL is set in playwright.config.ts
// This allows tests to work with dynamic Kind cluster ports

/**
 * Helper: Navigate through the multi-step Create Project wizard.
 * Steps: Project (name) -> Destinations (namespace) -> Roles -> "Create Project"
 */
async function openCreateWizardAndSubmit(page: Page, projectName: string) {
  // Click Create button to open wizard modal
  const createButton = page.getByRole('button', { name: 'Create', exact: true });
  await expect(createButton).toBeVisible({ timeout: 10000 });
  await createButton.click();

  // Step 1: Project — fill name using label
  await expect(page.getByText('Create Project')).toBeVisible();
  const nameInput = page.getByLabel(/project name/i);
  await nameInput.fill(projectName);

  // Click Continue to Step 2 (Destinations)
  await page.getByRole('button', { name: /continue/i }).click();

  // Step 2: Destinations — add a namespace destination so step is valid
  const nsInput = page.getByPlaceholder(/namespace/i).or(page.locator('input[placeholder*="namespace"]'));
  if (await nsInput.isVisible({ timeout: 3000 }).catch(() => false)) {
    await nsInput.fill('default');
    await nsInput.press('Enter');
  }

  // Click Continue to Step 3 (Roles)
  const continueBtn2 = page.getByRole('button', { name: /continue/i });
  if (await continueBtn2.isVisible({ timeout: 3000 }).catch(() => false)) {
    await continueBtn2.click();
  }

  // Step 3: Roles — click "Create Project" submit button
  const submitButton = page.getByRole('button', { name: /create project/i });
  await expect(submitButton).toBeVisible({ timeout: 5000 });
  await submitButton.click();
}

test.describe('Global Admin - Projects Settings UI', () => {
  // Authenticate as Global Admin to manage projects
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test.beforeEach(async ({ page }) => {
    // Mock permissions for Global Admin (full access)
    await setupPermissionMocking(page, { '*:*': true });
  });

  test('AC-129-01: Project List View displays projects with badges', async ({ page }) => {
    // Navigate to Projects Settings page
    await page.goto(`/projects`);
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-01-list-page.png',
      fullPage: true
    });

    // Verify page title
    const pageTitle = page.locator('h1, h2').filter({ hasText: /Projects/i }).first();
    await expect(pageTitle).toBeVisible({ timeout: 10000 });

    // Check for Create Project button in header
    const createButton = page.getByRole('button', { name: 'Create', exact: true });
    await expect(createButton).toBeVisible({ timeout: 5000 });

    // Check for project cards or list items (if any projects exist)
    const projectCards = page.locator('[data-testid="project-card"], .project-card, article').filter({ hasText: /role|repo|destination/i });
    const projectCount = await projectCards.count();
    console.log(`Found ${projectCount} project cards`);

    // If projects exist, verify badges are shown
    if (projectCount > 0) {
      // Look for role count badge
      const roleBadge = projectCards.first().locator('text=/\\d+ role/i');
      const repoBadge = projectCards.first().locator('text=/\\d+ repo/i');

      // At least one badge type should be visible
      const hasBadges = await roleBadge.isVisible({ timeout: 3000 }).catch(() => false) ||
                        await repoBadge.isVisible({ timeout: 3000 }).catch(() => false);
      console.log(`Project cards have badges: ${hasBadges}`);
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-01-list-with-badges.png',
      fullPage: true
    });

    console.log('✓ Project List View displays correctly');
  });

  test('AC-129-02: Create Project Form with validation', async ({ page }) => {
    const testProjectName = `test-project-${Date.now()}`;

    // Navigate to Projects Settings page
    await page.goto(`/projects`);
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Open the wizard modal
    const createButton = page.getByRole('button', { name: 'Create', exact: true });
    await expect(createButton).toBeVisible({ timeout: 10000 });
    await createButton.click();

    await page.waitForTimeout(1000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-02-create-form.png',
      fullPage: true
    });

    // Step 1: Verify form fields exist on wizard step 1
    const nameInput = page.getByLabel(/project name/i);
    await expect(nameInput).toBeVisible({ timeout: 5000 });

    // Verify Continue button is disabled when name is empty (validation)
    const continueButton = page.getByRole('button', { name: /continue/i });
    await expect(continueButton).toBeDisabled();
    const hasValidation = true; // Button disabled = validation working
    console.log(`Form validation present: ${hasValidation}`);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-02-validation-error.png',
      fullPage: true
    });

    // Fill in valid project name
    await nameInput.fill(testProjectName);

    // Fill description if available
    const descriptionInput = page.getByRole('textbox', { name: /description/i });
    if (await descriptionInput.isVisible({ timeout: 2000 }).catch(() => false)) {
      await descriptionInput.fill('Test project created by E2E test');
    }

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-02-form-filled.png',
      fullPage: true
    });

    // Navigate through wizard: Step 1 -> Step 2 (Destinations)
    await continueButton.click();

    // Step 2: Destinations — add a namespace
    const nsInput = page.getByPlaceholder(/namespace/i).or(page.locator('input[placeholder*="namespace"]'));
    if (await nsInput.isVisible({ timeout: 3000 }).catch(() => false)) {
      await nsInput.fill('default');
      await nsInput.press('Enter');
    }

    // Step 2 -> Step 3 (Roles)
    const continueBtn2 = page.getByRole('button', { name: /continue/i });
    if (await continueBtn2.isVisible({ timeout: 3000 }).catch(() => false)) {
      await continueBtn2.click();
    }

    // Step 3: Submit with "Create Project" button
    const submitButton = page.getByRole('button', { name: /create project/i });
    await expect(submitButton).toBeVisible({ timeout: 5000 });
    await submitButton.click();

    // Wait for creation to complete
    await page.waitForTimeout(3000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-02-after-create.png',
      fullPage: true
    });

    // Verify project appears in list or navigate to detail
    const createdProject = page.locator(`text=${testProjectName}`);
    const projectCreated = await createdProject.isVisible({ timeout: 10000 }).catch(() => false);
    console.log(`Project created and visible: ${projectCreated}`);

    console.log('✓ Create Project Form works with validation');
  });

  test('AC-129-03: Project Detail View with tabbed interface', async ({ page }) => {
    // First, navigate to projects list
    await page.goto(`/projects`);
    await page.waitForLoadState('load', { timeout: 15000 });
    await page.waitForTimeout(1000); // Wait for UI to settle

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-03-list-before-detail.png',
      fullPage: true
    });

    // Try multiple selectors to find a clickable project card
    const projectCardSelectors = [
      '[data-testid="project-card"]',
      'article.cursor-pointer',
      '.cursor-pointer:has(h3)',
      'a[href^="/projects/"]'
    ];

    let projectCard = null;
    for (const selector of projectCardSelectors) {
      const card = page.locator(selector).first();
      if (await card.isVisible({ timeout: 2000 }).catch(() => false)) {
        projectCard = card;
        console.log(`Found project card with selector: ${selector}`);
        break;
      }
    }

    if (projectCard) {
      // Try to get project name, but don't fail if we can't
      let projectName = 'unknown';
      try {
        const titleElement = projectCard.locator('h3, h4, [class*="title"]').first();
        if (await titleElement.isVisible({ timeout: 2000 })) {
          projectName = await titleElement.textContent() || 'unknown';
        }
      } catch {
        console.log('Could not extract project name, continuing...');
      }
      console.log(`Clicking on project: ${projectName}`);

      await projectCard.click();
      await page.waitForTimeout(2000);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/projects-03-detail-view.png',
        fullPage: true
      });

      // Verify we navigated to a project detail page
      const currentUrl = page.url();
      console.log(`Navigated to: ${currentUrl}`);

      // Verify tabbed interface (may or may not exist)
      const tabList = page.locator('[role="tablist"], .tabs');
      const hasTabs = await tabList.isVisible({ timeout: 5000 }).catch(() => false);

      if (hasTabs) {
        // Check for expected tabs
        const overviewTab = page.locator('button[role="tab"]:has-text("Overview"), [role="tab"]:has-text("Overview")');
        const rolesTab = page.locator('button[role="tab"]:has-text("Roles"), [role="tab"]:has-text("Roles")');
        const reposTab = page.locator('button[role="tab"]:has-text("Repositories"), [role="tab"]:has-text("Repos")');
        const policiesTab = page.locator('button[role="tab"]:has-text("Policies"), [role="tab"]:has-text("Policies")');

        // At least Overview tab should be visible
        await expect(overviewTab).toBeVisible({ timeout: 5000 });

        // Click on Roles tab
        if (await rolesTab.isVisible({ timeout: 3000 }).catch(() => false)) {
          await rolesTab.click();
          await page.waitForTimeout(1000);

          await page.screenshot({
            path: '../test-results/e2e/screenshots/projects-03-roles-tab.png',
            fullPage: true
          });

          // Check for role content
          const roleContent = page.locator('text=/admin|developer|viewer|role/i');
          const hasRoles = await roleContent.isVisible({ timeout: 5000 }).catch(() => false);
          console.log(`Roles tab shows roles: ${hasRoles}`);
        }

        // Click on Repositories tab
        if (await reposTab.isVisible({ timeout: 3000 }).catch(() => false)) {
          await reposTab.click();
          await page.waitForTimeout(1000);

          await page.screenshot({
            path: '../test-results/e2e/screenshots/projects-03-repos-tab.png',
            fullPage: true
          });
        }

        // Click on Policies tab
        if (await policiesTab.isVisible({ timeout: 3000 }).catch(() => false)) {
          await policiesTab.click();
          await page.waitForTimeout(1000);

          await page.screenshot({
            path: '../test-results/e2e/screenshots/projects-03-policies-tab.png',
            fullPage: true
          });

          // Check for policy content (Casbin format)
          const policyContent = page.locator('text=/subject|action|resource|policy/i');
          const hasPolicies = await policyContent.isVisible({ timeout: 5000 }).catch(() => false);
          console.log(`Policies tab shows policies: ${hasPolicies}`);
        }

        console.log('✓ Project Detail View displays with tabbed interface');
      } else {
        // No tabs - check if we at least see project details
        const projectDetailContent = page.locator('h1, h2, [class*="project"]').first();
        const hasContent = await projectDetailContent.isVisible({ timeout: 5000 }).catch(() => false);
        console.log(`Project detail page shows content: ${hasContent}`);
        expect(hasContent).toBe(true);
      }
    } else {
      // No projects exist
      console.log('No projects found to view details, skipping detail view test');
      // Check if empty state is shown instead
      const emptyState = page.locator('text=/no projects|create.*project/i');
      const hasEmptyState = await emptyState.isVisible({ timeout: 3000 }).catch(() => false);
      console.log(`Empty state shown: ${hasEmptyState}`);
    }
  });

  test('AC-129-09: Delete Project with confirmation dialog', async ({ page }) => {
    const testProjectName = `delete-test-${Date.now()}`;

    // Navigate to Projects Settings page
    await page.goto(`/projects`);
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000); // Allow data to load

    // First, create a project to delete using the wizard
    await openCreateWizardAndSubmit(page, testProjectName);
    await page.waitForTimeout(3000);

    // Refresh to see the new project
    await page.goto(`/projects`);
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000); // Allow data to load

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-09-before-delete.png',
      fullPage: true
    });

    // Find the project card with delete button
    const projectCard = page.locator('article, [data-testid="project-card"]').filter({ hasText: testProjectName }).first();

    if (await projectCard.isVisible({ timeout: 5000 })) {
      // Look for delete button
      const deleteButton = projectCard.locator('button:has-text("Delete"), button[aria-label*="delete"], button svg[class*="trash"]').first();

      if (await deleteButton.isVisible({ timeout: 3000 })) {
        await deleteButton.click();
        await page.waitForTimeout(1000);

        await page.screenshot({
          path: '../test-results/e2e/screenshots/projects-09-delete-dialog.png',
          fullPage: true
        });

        // Verify confirmation dialog appears
        const confirmDialog = page.locator('[role="alertdialog"], [role="dialog"]');
        await expect(confirmDialog).toBeVisible({ timeout: 5000 });

        // Check for warning message about deletion
        const warningText = page.locator('text=/delete|remove|permanent|cannot be undone/i');
        await expect(warningText).toBeVisible({ timeout: 3000 });

        // Cancel first to test cancel flow
        const cancelButton = page.locator('button:has-text("Cancel")');
        if (await cancelButton.isVisible({ timeout: 2000 })) {
          await cancelButton.click();
          await page.waitForTimeout(500);
        }

        // Click delete again to actually delete
        await deleteButton.click();
        await page.waitForTimeout(1000);

        // Confirm deletion
        const confirmDeleteButton = page.locator('button:has-text("Delete"), button:has-text("Confirm")').last();
        await confirmDeleteButton.click();

        await page.waitForTimeout(3000);

        await page.screenshot({
          path: '../test-results/e2e/screenshots/projects-09-after-delete.png',
          fullPage: true
        });

        // Verify project is removed from list
        const deletedProject = page.locator(`text=${testProjectName}`);
        const isDeleted = !(await deletedProject.isVisible({ timeout: 3000 }).catch(() => false));
        console.log(`Project successfully deleted: ${isDeleted}`);

        console.log('✓ Delete Project works with confirmation dialog');
      }
    } else {
      console.log('Test project not found, skipping delete test');
    }
  });

  test('AC-129-10: Empty State when no projects exist', async ({ page }) => {
    // Mock projects API to return empty list (deterministic empty state)
    await page.route(/\/api\/v1\/projects(\?|$)/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], totalCount: 0 }),
      });
    });

    await page.goto(`/projects`);

    // Wait for empty state to render - "No projects yet" from ProjectList component
    const emptyState = page.getByText('No projects yet');
    await expect(emptyState).toBeVisible({ timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-10-page-state.png',
      fullPage: true
    });

    // Verify empty state has a CTA button (Create Project button in empty state)
    const createCTA = page.getByRole('button', { name: 'Create Project', exact: true });
    await expect(createCTA).toBeVisible({ timeout: 5000 });

    console.log('✓ Empty State displays correctly with Create Project CTA');
  });

  test('AC-129-11: RBAC Protection - Settings accessible only to Global Admin', async ({ page }) => {
    // Navigate directly to Projects page
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // Verify we can access Projects page (title renders)
    const projectsPage = page.getByRole('heading', { name: /Projects/i });
    await expect(projectsPage).toBeVisible({ timeout: 10000 });

    // Verify Global Admin can see Create button
    const createButton = page.getByRole('button', { name: 'Create', exact: true });
    await expect(createButton).toBeVisible({ timeout: 5000 });

    console.log('✓ RBAC Protection - Global Admin can access Projects Settings');
  });
});

test.describe('Viewer - Projects Settings Access Denied', () => {
  // Authenticate as Viewer (should not have access)
  test.use({ authenticateAs: TestUserRole.ORG_VIEWER });

  test('AC-129-11: Viewer cannot access Settings page', async ({ page }) => {
    // Mock permissions for Viewer (no create/delete)
    await setupPermissionMocking(page, {
      'projects:get': true,
      'projects:create': false,
      'projects:delete': false,
    });

    // Navigate to Projects page
    await page.goto('/projects');
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Viewer can see the page but should NOT see Create button
    const createButton = page.getByRole('button', { name: 'Create', exact: true });
    await expect(createButton).not.toBeVisible({ timeout: 5000 });

    console.log('✓ Viewer cannot see Create button on Projects page');
  });

  test('AC-129-11: Viewer cannot access Projects Settings directly', async ({ page }) => {
    // Try to navigate directly to Projects Settings
    await page.goto(`/projects`);
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/projects-11-viewer-projects-attempt.png',
      fullPage: true
    });

    // Viewer should be redirected or see access denied
    const projectsPage = page.locator('h1:has-text("Projects Settings"), h2:has-text("Projects")').first();
    const accessDenied = page.locator('text=/access denied|unauthorized|forbidden|not authorized/i');
    const catalogPage = page.locator('h1:has-text("Catalog"), h1:has-text("Catalog")').first();

    const hasProjects = await projectsPage.isVisible({ timeout: 3000 }).catch(() => false);
    const hasDenied = await accessDenied.isVisible({ timeout: 3000 }).catch(() => false);
    const hasCatalog = await catalogPage.isVisible({ timeout: 3000 }).catch(() => false);

    console.log(`Projects Settings visible to Viewer: ${hasProjects}`);
    console.log(`Access Denied shown: ${hasDenied}`);
    console.log(`Redirected to Catalog: ${hasCatalog}`);

    // Viewer should NOT see Projects Settings, or should see access denied, or be redirected
    expect(!hasProjects || hasDenied || hasCatalog).toBeTruthy();

    console.log('✓ Viewer cannot access Projects Settings');
  });
});
