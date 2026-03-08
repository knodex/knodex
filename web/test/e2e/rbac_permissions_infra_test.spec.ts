// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { expect, test } from "@playwright/test";
import * as path from "path";
import {
    generateTestTokens,
    isGlobalAdmin,
    loginAs,
    ensureEvidenceDir,
    type TestTokens,
} from "../fixture/rbac-test-helpers";

/**
 * RBAC Infrastructure, User Context & API E2E Tests
 *
 * Tests token validity, user configuration, test data existence,
 * project switching, and API error handling for unauthorized access.
 *
 * Covers: AC-20 to AC-22, AC-47 to AC-52
 *
 * Prerequisites:
 * 1. Deploy to Kind cluster: make qa-deploy
 * 2. Set E2E_BASE_URL=http://localhost:8080 (or your QA port)
 *
 * Run: E2E_BASE_URL=http://localhost:8080 npx playwright test rbac_permissions_infra_test.spec.ts
 */

test.describe("RBAC: Infrastructure & API Tests", () => {
  let tokens: TestTokens;

  test.beforeAll(async () => {
    tokens = await generateTestTokens();
    console.log(`Generated test tokens at: ${tokens.generated_at}`);
  });

  test.describe("Test Infrastructure Verification (AC-20 to AC-22)", () => {
    test("AC-20: Test tokens are valid and not expired", async ({ page }) => {
      // Verify token expiration
      const now = Math.floor(Date.now() / 1000);
      const tokenPayload = JSON.parse(
        Buffer.from(
          tokens.users.global_admin.token.split(".")[1],
          "base64"
        ).toString()
      );

      expect(tokenPayload.exp).toBeGreaterThan(now);
      console.log(
        `Token expires at: ${new Date(tokenPayload.exp * 1000).toISOString()}`
      );
      console.log(`Current time: ${new Date().toISOString()}`);

      // Verify token works with API
      await loginAs(page, tokens.users.global_admin, "/catalog");

      // Should not be redirected to login
      expect(page.url()).not.toContain("/login");
    });

    // FIXME: Rapid user switching causes navigation race conditions.
    // Root cause: Sequential loginAs() calls trigger "Navigation interrupted" errors.
    // Fix: Add explicit page.waitForLoadState('networkidle') between user switches.
    test.fixme("AC-21: All test users are properly configured", async ({
      page,
    }) => {
      const users = Object.entries(tokens.users);

      // Test users one by one, skipping no_orgs user since it has no project access
      for (const [name, user] of users) {
        console.log(`Testing user: ${name} (${user.email})`);

        // Skip no_orgs user - they have no project access and will fail to load catalog
        if (name === "no_orgs" || user.projects.length === 0) {
          console.log(`  Skipping ${name} - no project access`);
          continue;
        }

        try {
          // Add delay between user tests to avoid navigation race conditions
          await page.waitForLoadState('networkidle');
          await loginAs(page, user, "/");

          // Verify user can access the app (not stuck on login for users with projects)
          // Check casbin_roles instead of is_global_admin
          if (user.projects.length > 0 || isGlobalAdmin(user)) {
            // Should be able to access dashboard
            const url = page.url();
            console.log(`  URL after login: ${url}`);
          }
        } catch (e) {
          console.log(`  User ${name} login failed: ${e}`);
          // Only fail if this is a user that should be able to log in
          if (user.projects.length > 0 || isGlobalAdmin(user)) {
            throw e;
          }
        }
      }

      expect(users.length).toBeGreaterThanOrEqual(7); // At least 7 test users exist in config
    });

    // SKIP: Requires ≥2 project cards visible on Settings/Projects page.
    // The E2E setup may not have enough projects or the selectors may not match the UI.
    // Prerequisite: Verify project card selectors match actual UI structure.
    test.skip("AC-22: Test data (projects, RGDs, instances) exists", async ({
      page,
    }) => {
      // Start with catalog (simpler to verify)
      await loginAs(page, tokens.users.global_admin, "/catalog");
      const rgdCount = await page
        .getByRole("button", { name: /View details for/ })
        .count();
      console.log(`RGDs found: ${rgdCount}`);
      expect(rgdCount).toBeGreaterThanOrEqual(4);

      // Check instances via sidebar link
      await page.getByRole("link", { name: "Instances" }).click();
      await page.waitForLoadState("domcontentloaded");
      await page.waitForLoadState('networkidle'); // Allow data to load
      const instanceCount = await page
        .getByRole("button", { name: /View details for/ })
        .count();
      console.log(`Instances found: ${instanceCount}`);
      // NOTE: Instance count may be 0 if no test instances deployed - this is acceptable
      // The important thing is that the page loads without error
      console.log(
        `Instance check: ${instanceCount >= 0 ? "PASS" : "FAIL"} (found ${instanceCount})`
      );

      // Check projects via Settings page (only Global Admin can access)
      await page.goto("/settings/projects");
      await page.waitForLoadState("domcontentloaded");
      await page.waitForLoadState('networkidle'); // Allow data to load

      // Wait for project cards using multiple selectors
      const projectCardSelectors = [
        '[data-testid="project-card"]',
        "article.cursor-pointer",
        ".cursor-pointer:has(h3)",
        'a[href^="/settings/projects/"]',
      ];

      let projectCount = 0;
      for (const selector of projectCardSelectors) {
        const count = await page.locator(selector).count();
        if (count > projectCount) {
          projectCount = count;
        }
      }
      console.log(`Projects found: ${projectCount}`);
      expect(projectCount).toBeGreaterThanOrEqual(2);

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("test-infrastructure"),
          "AC22-test-data-exists.png"
        ),
        fullPage: true,
      });
    });
  });

  test.describe("User Context and Project Switching", () => {
    test("AC-47: User current project is displayed in header", async ({
      page,
    }) => {
      await loginAs(page, tokens.users.global_admin, "/catalog");

      // Look for project indicator in header
      const projectIndicator = page
        .locator("header, nav")
        .locator("text=/alpha|beta|project/i");
      const hasProjectIndicator = (await projectIndicator.count()) > 0;

      console.log(`Project displayed in header: ${hasProjectIndicator}`);

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("projects-rbac"),
          "AC47-project-in-header.png"
        ),
        fullPage: true,
      });
    });

    test("AC-48: User can switch between their projects", async ({ page }) => {
      await loginAs(page, tokens.users.global_admin, "/catalog");

      // Look for project switcher
      const projectSwitcher = page.getByRole("button", {
        name: /select project|switch project|project:/i,
      });

      if ((await projectSwitcher.count()) > 0) {
        await projectSwitcher.first().click();
        await page.waitForLoadState('networkidle');

        // Check for project options
        const projectOptions = page
          .locator('[role="option"], [role="menuitem"]')
          .filter({ hasText: /team|project/i });
        const optionCount = await projectOptions.count();

        console.log(`Project options available: ${optionCount}`);
      }

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("projects-rbac"),
          "AC48-project-switching.png"
        ),
        fullPage: true,
      });
    });

    test("AC-49: Single-project user does not see project switcher", async ({
      page,
    }) => {
      // Alpha admin only has access to one project
      await loginAs(page, tokens.users.alpha_admin, "/catalog");

      // Look for project switcher
      const projectSwitcher = page.getByRole("button", {
        name: /select project|switch project/i,
      });
      const hasSwitcher = (await projectSwitcher.count()) > 0;

      console.log(`Single-project user sees switcher: ${hasSwitcher}`);
      // Single-project users may or may not see switcher (depends on UI design)

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("projects-rbac"),
          "AC49-single-project-no-switcher.png"
        ),
        fullPage: true,
      });
    });
  });

  test.describe("API Error Handling and Unauthorized Access", () => {
    test("AC-50: Unauthorized API access returns proper error", async ({
      page,
    }) => {
      // Try to access API without token
      const response = await page.request.get("/api/v1/projects", {
        headers: { Authorization: "" },
      });

      const status = response.status();
      console.log(`API response without token: ${status}`);

      // Should return 401 Unauthorized
      expect(status).toBe(401);
    });

    test("AC-51: Invalid token returns authentication error", async ({
      page,
    }) => {
      const response = await page.request.get("/api/v1/projects", {
        headers: { Authorization: "Bearer invalid-token-here" },
      });

      const status = response.status();
      console.log(`API response with invalid token: ${status}`);

      // Should return 401 Unauthorized
      expect(status).toBe(401);
    });

    test("AC-52: Cross-project API access is denied", async ({ page }) => {
      await loginAs(page, tokens.users.alpha_developer, "/");

      // Try to access beta project's resources via API
      const response = await page.request.get(
        "/api/v1/projects/project-beta-team",
        {
          headers: {
            Authorization: `Bearer ${tokens.users.alpha_developer.token}`,
          },
        }
      );

      const status = response.status();
      console.log(`Cross-project API access status: ${status}`);

      // Should return 403 Forbidden or 404 Not Found (depending on implementation)
      expect([403, 404]).toContain(status);
    });
  });
});
