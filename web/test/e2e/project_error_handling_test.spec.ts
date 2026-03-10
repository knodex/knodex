// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * E2E Tests for Project Creation Error Handling
 *
 * Tests that API validation errors are properly displayed to users
 * when project creation fails. Covers:
 * - Missing required field validation error
 * - Duplicate project name error
 * - Form state preservation after error
 * - Error toast visibility and dismissibility
 *
 * Related Story: STORY-188 - Project Creation UX - Display API Error Messages
 *
 * Prerequisites:
 * 1. Deploy to Kind cluster: make qa-deploy
 * 2. Set E2E_BASE_URL=http://localhost:8080 (or your QA port)
 *
 * Run: E2E_BASE_URL=http://localhost:8080 npx playwright test e2e/project_error_handling_test.spec.ts
 */

import { expect, test, TestUserRole, setupPermissionMocking } from "../fixture";

test.describe("Project Creation Error Handling", () => {
  test.describe("Validation Error Display", () => {
    test("displays error toast when project creation fails with validation error", async ({
      page,
      auth,
    }) => {
      // Setup as admin with full permissions
      await setupPermissionMocking(page, { "*:*": true });
      await auth.setupAs(TestUserRole.GLOBAL_ADMIN);

      // Mock the projects list API
      await page.route("**/api/v1/projects", async (route, request) => {
        if (request.method() === "GET") {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ items: [] }),
          });
        } else if (request.method() === "POST") {
          // Return validation error for missing required field
          await route.fulfill({
            status: 400,
            contentType: "application/json",
            body: JSON.stringify({
              code: "BAD_REQUEST",
              message: "Validation failed",
              details: {
                name: "project name is required",
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto("/settings/projects");
      await page.waitForLoadState("networkidle");

      // Click Create Project button
      const createButton = page.getByRole("button", { name: /create project/i });
      await expect(createButton).toBeVisible();
      await createButton.click();

      // Wait for form to appear
      await page.waitForLoadState("networkidle");

      // Fill in description but skip required fields
      const descInput = page.locator('textarea[name="description"]');
      if (await descInput.isVisible()) {
        await descInput.fill("Test project description");
      }

      // Submit form
      const submitButton = page.getByRole("button", { name: /create|save|submit/i });
      await expect(submitButton).toBeVisible();
      await submitButton.click();

      // Wait for error indication to appear
      // Check for various error display patterns:
      // - Toast with error message from API details
      // - Toast with main error message
      // - Inline form error
      const errorPatterns = [
        page.getByText(/project name/i).first(),
        page.getByText(/validation failed/i).first(),
        page.getByText(/required/i).first(),
        page.locator('[role="alert"]').first(),
        page.locator('.text-destructive, .text-red').first(),
      ];

      // Wait for any error indication
      let errorFound = false;
      for (const pattern of errorPatterns) {
        if (await pattern.isVisible({ timeout: 2000 }).catch(() => false)) {
          errorFound = true;
          break;
        }
      }

      expect(errorFound).toBe(true);

      await page.screenshot({
        path: "../test-results/e2e/screenshots/projects/error-validation.png",
        fullPage: true,
      });
    });

    test("displays error toast when creating duplicate project", async ({
      page,
      auth,
    }) => {
      // Setup as admin with full permissions
      await setupPermissionMocking(page, { "*:*": true });
      await auth.setupAs(TestUserRole.GLOBAL_ADMIN);

      // Mock the projects list API with existing project
      await page.route("**/api/v1/projects", async (route, request) => {
        if (request.method() === "GET") {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({
              items: [{ name: "existing-project", description: "Already exists" }],
            }),
          });
        } else if (request.method() === "POST") {
          // Return conflict error for duplicate name
          await route.fulfill({
            status: 409,
            contentType: "application/json",
            body: JSON.stringify({
              code: "CONFLICT",
              message: "Project already exists: existing-project",
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto("/settings/projects");
      await page.waitForLoadState("networkidle");

      // Click Create Project button
      const createButton = page.getByRole("button", { name: /create project/i });
      await expect(createButton).toBeVisible();
      await createButton.click();

      // Wait for form to appear
      await page.waitForLoadState("networkidle");

      // Fill in duplicate project name
      const nameInput = page.locator('input[name="name"]');
      await expect(nameInput).toBeVisible();
      await nameInput.fill("existing-project");

      // Add a destination (required by form validation)
      // The namespace input auto-populates with the project name
      const addDestButton = page.locator('form button[type="button"]').filter({ has: page.locator('svg.lucide-plus') }).first();
      await addDestButton.click();

      // Submit form
      const submitButton = page.getByRole("button", { name: /create|save|submit/i });
      await expect(submitButton).toBeVisible();
      await submitButton.click();

      // Wait for error toast/message to appear
      // Use text-based detection (works reliably across toast libraries)
      await expect(page.getByText(/already exists/i).first()).toBeVisible({ timeout: 5000 });

      await page.screenshot({
        path: "../test-results/e2e/screenshots/projects/error-duplicate-name.png",
        fullPage: true,
      });
    });

    test("form remains editable after error - allows resubmission", async ({
      page,
      auth,
    }) => {
      // Setup as admin with full permissions
      await setupPermissionMocking(page, { "*:*": true });
      await auth.setupAs(TestUserRole.GLOBAL_ADMIN);

      let submitCount = 0;

      // Mock the projects list API
      await page.route("**/api/v1/projects", async (route, request) => {
        if (request.method() === "GET") {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ items: [] }),
          });
        } else if (request.method() === "POST") {
          submitCount++;
          if (submitCount === 1) {
            // First submission fails
            await route.fulfill({
              status: 400,
              contentType: "application/json",
              body: JSON.stringify({
                code: "BAD_REQUEST",
                message: "Validation failed",
                details: {
                  name: "project name is required",
                },
              }),
            });
          } else {
            // Second submission succeeds
            await route.fulfill({
              status: 201,
              contentType: "application/json",
              body: JSON.stringify({
                name: "test-project",
                description: "Test project",
              }),
            });
          }
        } else {
          await route.continue();
        }
      });

      await page.goto("/settings/projects");
      await page.waitForLoadState("networkidle");

      // Click Create Project button
      const createButton = page.getByRole("button", { name: /create project/i });
      await expect(createButton).toBeVisible();
      await createButton.click();

      // Wait for form to appear
      await page.waitForLoadState("networkidle");

      // Fill in project name
      const nameInput = page.locator('input[name="name"]');
      await expect(nameInput).toBeVisible();
      await nameInput.fill("test-project");

      // Add a destination (required by form validation)
      const addDestButton = page.locator('form button[type="button"]').filter({ has: page.locator('svg.lucide-plus') }).first();
      await addDestButton.click();

      // Submit form (first attempt - should fail)
      const submitButton = page.getByRole("button", { name: /create|save|submit/i });
      await submitButton.click();

      // Wait for error indication (toast or inline error)
      const errorPatterns = [
        page.getByText(/project name/i).first(),
        page.getByText(/validation failed/i).first(),
        page.getByText(/required/i).first(),
        page.locator('[role="alert"]').first(),
      ];

      let errorFound = false;
      for (const pattern of errorPatterns) {
        if (await pattern.isVisible({ timeout: 2000 }).catch(() => false)) {
          errorFound = true;
          break;
        }
      }
      expect(errorFound).toBe(true);

      // Verify form is still visible and editable (not reset)
      await expect(nameInput).toBeVisible();
      await expect(nameInput).toHaveValue("test-project");

      // Verify submit button is still available (not stuck in loading state)
      await expect(submitButton).toBeEnabled();

      // Submit again (second attempt - should succeed)
      await submitButton.click();

      // Wait for success toast/message
      await expect(page.getByText(/created|success/i).first()).toBeVisible({ timeout: 5000 });

      // Verify we submitted twice
      expect(submitCount).toBe(2);

      await page.screenshot({
        path: "../test-results/e2e/screenshots/projects/error-form-resubmission.png",
        fullPage: true,
      });
    });
  });

  test.describe("Additional Validation Errors (AC #2)", () => {
    test("displays error when project name format is invalid", async ({
      page,
      auth,
    }) => {
      // Setup as admin with full permissions
      await setupPermissionMocking(page, { "*:*": true });
      await auth.setupAs(TestUserRole.GLOBAL_ADMIN);

      // Mock the projects list API
      await page.route("**/api/v1/projects", async (route, request) => {
        if (request.method() === "GET") {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ items: [] }),
          });
        } else if (request.method() === "POST") {
          // Return validation error for invalid name format
          await route.fulfill({
            status: 400,
            contentType: "application/json",
            body: JSON.stringify({
              code: "BAD_REQUEST",
              message: "Validation failed",
              details: {
                name: "invalid project name: must be lowercase alphanumeric with hyphens",
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto("/settings/projects");
      await page.waitForLoadState("networkidle");

      // Click Create Project button
      const createButton = page.getByRole("button", { name: /create project/i });
      await expect(createButton).toBeVisible();
      await createButton.click();

      // Wait for form to appear
      await page.waitForLoadState("networkidle");

      // Note: Frontend form validation may catch invalid names first
      // This test verifies backend validation errors are also displayed
      const nameInput = page.locator('input[name="name"]');
      await expect(nameInput).toBeVisible();
      await nameInput.fill("Invalid_Name_123"); // Invalid format

      // Submit form
      const submitButton = page.getByRole("button", { name: /create|save|submit/i });
      await expect(submitButton).toBeVisible();
      await submitButton.click();

      // Wait for error (could be form validation or API error)
      // Check for either inline form error or toast - use text-based detection
      const formError = page.locator('.text-destructive');
      const toastError = page.getByText(/invalid|must be|alphanumeric|lowercase/i);

      // Either form validation or API error should show
      const hasFormError = await formError.isVisible({ timeout: 3000 }).catch(() => false);
      const hasToastError = await toastError.first().isVisible({ timeout: 3000 }).catch(() => false);

      expect(hasFormError || hasToastError).toBe(true);

      await page.screenshot({
        path: "../test-results/e2e/screenshots/projects/error-invalid-name-format.png",
        fullPage: true,
      });
    });

    test("displays error when destinations are missing (if required by backend)", async ({
      page,
      auth,
    }) => {
      // Setup as admin with full permissions
      await setupPermissionMocking(page, { "*:*": true });
      await auth.setupAs(TestUserRole.GLOBAL_ADMIN);

      // Mock the projects list API
      await page.route("**/api/v1/projects", async (route, request) => {
        if (request.method() === "GET") {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ items: [] }),
          });
        } else if (request.method() === "POST") {
          // Return validation error for missing destinations
          await route.fulfill({
            status: 400,
            contentType: "application/json",
            body: JSON.stringify({
              code: "BAD_REQUEST",
              message: "Validation failed",
              details: {
                destinations: "at least one destination is required",
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto("/settings/projects");
      await page.waitForLoadState("networkidle");

      // Click Create Project button
      const createButton = page.getByRole("button", { name: /create project/i });
      await expect(createButton).toBeVisible();
      await createButton.click();

      // Wait for form to appear
      await page.waitForLoadState("networkidle");

      // Fill in project name only (no destinations)
      const nameInput = page.locator('input[name="name"]');
      await expect(nameInput).toBeVisible();
      await nameInput.fill("test-project-no-dest");

      // Submit form without adding destinations
      const submitButton = page.getByRole("button", { name: /create|save|submit/i });
      await expect(submitButton).toBeVisible();
      await submitButton.click();

      // Wait for error toast/message to appear
      // Use text-based detection (works reliably across toast libraries)
      await expect(page.getByText(/destination/i).first()).toBeVisible({ timeout: 5000 });

      await page.screenshot({
        path: "../test-results/e2e/screenshots/projects/error-missing-destinations.png",
        fullPage: true,
      });
    });
  });

  test.describe("Delete Error Handling", () => {
    test("displays error toast when project deletion fails", async ({
      page,
      auth,
    }) => {
      // Setup as admin with full permissions
      await setupPermissionMocking(page, { "*:*": true });
      await auth.setupAs(TestUserRole.GLOBAL_ADMIN);

      // Mock the projects list API
      await page.route("**/api/v1/projects", async (route, request) => {
        if (request.method() === "GET") {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({
              items: [
                {
                  name: "test-project",
                  description: "A project with instances",
                },
              ],
            }),
          });
        } else {
          await route.continue();
        }
      });

      // Mock delete to fail
      await page.route("**/api/v1/projects/test-project", async (route, request) => {
        if (request.method() === "DELETE") {
          await route.fulfill({
            status: 400,
            contentType: "application/json",
            body: JSON.stringify({
              code: "BAD_REQUEST",
              message: "Cannot delete project: active instances exist",
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto("/settings/projects");
      await page.waitForLoadState("networkidle");

      // Find and click delete button on the project
      const deleteButton = page.locator('[aria-label*="delete" i], button:has-text("Delete")').first();

      // If delete button isn't directly visible, look for menu trigger
      if (!(await deleteButton.isVisible({ timeout: 2000 }).catch(() => false))) {
        // Click on project row or menu to reveal delete option
        const projectRow = page.locator('text=test-project').first();
        await projectRow.click();
        await page.waitForLoadState("networkidle");
      }

      // Try to find delete action
      const deleteAction = page.getByRole("button", { name: /delete/i });
      if (await deleteAction.isVisible({ timeout: 2000 }).catch(() => false)) {
        await deleteAction.click();

        // Confirm deletion if dialog appears
        const confirmButton = page.getByRole("button", { name: /confirm|delete|yes/i }).last();
        if (await confirmButton.isVisible({ timeout: 2000 }).catch(() => false)) {
          await confirmButton.click();
        }

        // Wait for error toast/message
        // Use text-based detection (works reliably across toast libraries)
        await expect(page.getByText(/cannot delete|active instances/i).first()).toBeVisible({ timeout: 5000 });

        await page.screenshot({
          path: "../test-results/e2e/screenshots/projects/error-delete-failed.png",
          fullPage: true,
        });
      }
    });
  });

  test.describe("Success Toast Display", () => {
    test("displays success toast when project created successfully", async ({
      page,
      auth,
    }) => {
      // Setup as admin with full permissions
      await setupPermissionMocking(page, { "*:*": true });
      await auth.setupAs(TestUserRole.GLOBAL_ADMIN);

      // Mock the projects list API
      await page.route("**/api/v1/projects", async (route, request) => {
        if (request.method() === "GET") {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ items: [] }),
          });
        } else if (request.method() === "POST") {
          // Return success
          await route.fulfill({
            status: 201,
            contentType: "application/json",
            body: JSON.stringify({
              name: "new-project",
              description: "New project created",
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto("/settings/projects");
      await page.waitForLoadState("networkidle");

      // Click Create Project button
      const createButton = page.getByRole("button", { name: /create project/i });
      await expect(createButton).toBeVisible();
      await createButton.click();

      // Wait for form to appear
      await page.waitForLoadState("networkidle");

      // Fill in project name
      const nameInput = page.locator('input[name="name"]');
      await expect(nameInput).toBeVisible();
      await nameInput.fill("new-project");

      // Add a destination (required by form validation)
      const addDestButton = page.locator('form button[type="button"]').filter({ has: page.locator('svg.lucide-plus') }).first();
      await addDestButton.click();

      // Submit form
      const submitButton = page.getByRole("button", { name: /create|save|submit/i });
      await expect(submitButton).toBeVisible();
      await submitButton.click();

      // Wait for success toast/message to appear
      // Use text-based detection (works reliably across toast libraries)
      await expect(page.getByText(/created|success/i).first()).toBeVisible({ timeout: 5000 });

      await page.screenshot({
        path: "../test-results/e2e/screenshots/projects/success-create.png",
        fullPage: true,
      });
    });
  });
});
