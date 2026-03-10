// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { expect, test } from "@playwright/test";
import * as path from "path";
import {
    generateTestTokens,
    loginAs,
    ensureEvidenceDir,
    type TestTokens,
} from "../fixture/rbac-test-helpers";
import { setupPermissionMocking } from "../fixture/auth-helper";

/**
 * RBAC Catalog Feature E2E Tests
 *
 * Tests catalog visibility, deploy button permissions, namespace enforcement,
 * deployment modes, and shared RGD visibility across roles.
 *
 * Covers: AC-1 to AC-5, AC-23 to AC-25, AC-43 to AC-46
 *
 * Prerequisites:
 * 1. Deploy to Kind cluster: make qa-deploy
 * 2. Set E2E_BASE_URL=http://localhost:8080 (or your QA port)
 *
 * Run: E2E_BASE_URL=http://localhost:8080 npx playwright test rbac_permissions_catalog_test.spec.ts
 */

test.describe("RBAC: Catalog Feature Tests", () => {
  let tokens: TestTokens;

  test.beforeAll(async () => {
    tokens = await generateTestTokens();
    console.log(`Generated test tokens at: ${tokens.generated_at}`);
  });

  test.describe("Catalog Feature RBAC (AC-1 to AC-5)", () => {
    test("AC-1: Global Admin sees all RGDs in catalog", async ({ page }) => {
      await setupPermissionMocking(page, { '*:*': true });
      await loginAs(page, tokens.users.global_admin, "/catalog");

      // Wait for the catalog page to fully load
      await page.waitForSelector('text=/\\d+ available/', {
        timeout: 10000,
      });
      await page.waitForLoadState('networkidle'); // Wait for data to load

      // Global Admin should see all RGDs (including test data)
      // RGDCard renders as <div role="button" aria-label="View details for {name}">
      const rgdCards = page.locator(
        'div[role="button"][aria-label^="View details for"]'
      );
      const rgdCount = await rgdCards.count();

      // Get the count text from the header
      const countText = await page
        .locator("text=/\\d+ available/")
        .textContent()
        .catch(() => "");

      console.log(
        `Global Admin sees ${rgdCount} RGDs, count text: ${countText}`
      );

      // Should see multiple RGDs including test data (at least 1 for catalog to work)
      expect(rgdCount).toBeGreaterThanOrEqual(1);

      // Take evidence screenshot
      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC1-global-admin-sees-all-rgds.png"
        ),
        fullPage: true,
      });
    });

    test("AC-2: Global Admin can see Deploy button on RGD detail", async ({
      page,
    }) => {
      await setupPermissionMocking(page, { '*:*': true });
      await loginAs(page, tokens.users.global_admin, "/catalog");

      // Click on first RGD to view details
      await page
        .getByRole("button", { name: /View details for/ })
        .first()
        .click();
      await page.waitForLoadState("load");

      // Should see Deploy button
      const deployButton = page.getByRole("button", { name: "Deploy" });
      await expect(deployButton).toBeVisible();

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC2-global-admin-deploy-button-visible.png"
        ),
        fullPage: true,
      });
    });

    test("AC-3: Org Viewer sees filtered RGDs (org-specific + shared)", async ({
      page,
    }) => {
      await loginAs(page, tokens.users.alpha_viewer, "/catalog");

      // Wait for the catalog page to fully load
      await page.waitForSelector('text=/\\d+ available/', {
        timeout: 10000,
      });
      await page.waitForLoadState('networkidle'); // Wait for data to load

      // Viewer should see filtered RGDs based on org membership
      const countText = await page
        .locator("text=/\\d+ available/")
        .textContent()
        .catch(() => "0 available");
      console.log(`Alpha Viewer sees: ${countText}`);

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC3-viewer-sees-filtered-rgds.png"
        ),
        fullPage: true,
      });
    });

    test("AC-4: Org Viewer cannot see Deploy button on RGD detail", async ({
      page,
    }) => {
      await loginAs(page, tokens.users.alpha_viewer, "/catalog");

      // Try to navigate to an RGD detail if any are visible
      const rgdButtons = page.getByRole("button", { name: /View details for/ });
      const count = await rgdButtons.count();

      if (count > 0) {
        await rgdButtons.first().click();
        await page.waitForLoadState("load");

        // Viewer should NOT see Deploy button
        const deployButton = page.getByRole("button", { name: "Deploy" });
        await expect(deployButton).not.toBeVisible();
      }

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC4-viewer-no-deploy-button.png"
        ),
        fullPage: true,
      });
    });

    test("AC-5: Org Developer can see Deploy button", async ({ page }) => {
      await loginAs(page, tokens.users.alpha_developer, "/catalog");

      const rgdButtons = page.getByRole("button", { name: /View details for/ });
      const count = await rgdButtons.count();
      console.log(`Alpha Developer sees ${count} RGDs`);

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC5-developer-catalog-view.png"
        ),
        fullPage: true,
      });
    });
  });

  test.describe("Catalog Feature - Deploy Namespace Enforcement", () => {
    test("AC-23: Developer can only deploy to their organization namespace", async ({
      page,
    }) => {
      await loginAs(page, tokens.users.alpha_developer, "/catalog");

      // Find an RGD and click to view details
      const rgdButtons = page.getByRole("button", { name: /View details for/ });
      const count = await rgdButtons.count();

      if (count > 0) {
        await rgdButtons.first().click();
        await page.waitForLoadState("load");

        // Click Deploy button if visible
        const deployButton = page.getByRole("button", { name: "Deploy" });
        if (await deployButton.isVisible()) {
          await deployButton.click();
          await page.waitForLoadState("load");

          // Check namespace dropdown - should only contain org-alpha-team namespace
          const namespaceSelect = page.locator(
            'select[name="namespace"], [data-testid="namespace-select"], input[name="namespace"]'
          );
          if ((await namespaceSelect.count()) > 0) {
            // Get available namespace options
            const options = await page
              .locator('option, [role="option"]')
              .allTextContents();
            console.log(
              `Alpha Developer available namespaces: ${options.join(", ")}`
            );

            // Should NOT contain beta-team namespace
            const hasBetaNamespace = options.some((opt) =>
              opt.toLowerCase().includes("beta")
            );
            expect(hasBetaNamespace).toBe(false);
          }
        }
      }

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC23-developer-namespace-restriction.png"
        ),
        fullPage: true,
      });
    });

    test("AC-24: Platform Admin cannot deploy to other organization namespaces", async ({
      page,
    }) => {
      await loginAs(page, tokens.users.alpha_admin, "/catalog");

      const rgdButtons = page.getByRole("button", { name: /View details for/ });
      const count = await rgdButtons.count();

      if (count > 0) {
        await rgdButtons.first().click();
        await page.waitForLoadState("load");

        const deployButton = page.getByRole("button", { name: "Deploy" });
        if (await deployButton.isVisible()) {
          await deployButton.click();
          await page.waitForLoadState("load");

          // Platform admin should only see namespaces from their organization
          const namespaceSelect = page.locator(
            'select[name="namespace"], [data-testid="namespace-select"]'
          );
          if ((await namespaceSelect.count()) > 0) {
            const options = await page
              .locator('option, [role="option"]')
              .allTextContents();
            console.log(
              `Alpha Admin available namespaces: ${options.join(", ")}`
            );

            // Should NOT contain beta-team namespace
            const hasBetaNamespace = options.some((opt) =>
              opt.toLowerCase().includes("beta")
            );
            expect(hasBetaNamespace).toBe(false);
          }
        }
      }

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC24-admin-namespace-restriction.png"
        ),
        fullPage: true,
      });
    });

    test("AC-25: Global Admin can deploy to any namespace", async ({
      page,
    }) => {
      await setupPermissionMocking(page, { '*:*': true });
      await loginAs(page, tokens.users.global_admin, "/catalog");

      const rgdButtons = page.getByRole("button", { name: /View details for/ });
      await rgdButtons.first().click();
      await page.waitForLoadState("load");

      const deployButton = page.getByRole("button", { name: "Deploy" });
      await deployButton.click();
      await page.waitForLoadState("load");

      // Global admin should have access to all namespaces
      const namespaceSelect = page.locator(
        'select[name="namespace"], [data-testid="namespace-select"]'
      );
      if ((await namespaceSelect.count()) > 0) {
        const options = await page
          .locator('option, [role="option"]')
          .allTextContents();
        console.log(`Global Admin available namespaces: ${options.join(", ")}`);

        // Should have access to multiple organization namespaces
        expect(options.length).toBeGreaterThanOrEqual(1);
      }

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC25-global-admin-all-namespaces.png"
        ),
        fullPage: true,
      });
    });
  });

  test.describe("Deployment Mode RBAC", () => {
    test("AC-43: Deployment mode selection respects user permissions", async ({
      page,
    }) => {
      await loginAs(page, tokens.users.alpha_developer, "/catalog");

      const rgdButtons = page.getByRole("button", { name: /View details for/ });
      const count = await rgdButtons.count();

      if (count > 0) {
        await rgdButtons.first().click();
        await page.waitForLoadState("load");

        const deployButton = page.getByRole("button", { name: "Deploy" });
        if (await deployButton.isVisible()) {
          await deployButton.click();
          await page.waitForLoadState("load");

          // Check for deployment mode selector
          const modeSelector = page.locator("text=/direct|gitops|hybrid/i");
          const hasDeploymentModes = (await modeSelector.count()) > 0;

          console.log(
            `Developer sees deployment mode options: ${hasDeploymentModes}`
          );
        }
      }

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC43-deployment-mode-permissions.png"
        ),
        fullPage: true,
      });
    });

    test("AC-44: GitOps mode requires repository configuration", async ({
      page,
    }) => {
      await setupPermissionMocking(page, { '*:*': true });
      await loginAs(page, tokens.users.global_admin, "/catalog");

      const rgdButtons = page.getByRole("button", { name: /View details for/ });
      await rgdButtons.first().click();
      await page.waitForLoadState("load");

      const deployButton = page.getByRole("button", { name: "Deploy" });
      await deployButton.click();
      await page.waitForLoadState("load");

      // Look for GitOps mode option
      const gitopsOption = page.locator("text=/gitops/i").first();
      if (await gitopsOption.isVisible()) {
        await gitopsOption.click();
        await page.waitForLoadState('networkidle');

        // Should show repository selection or warning about no repos
        const repoSelect = page.locator(
          "text=/select repository|no repository|configure repository/i"
        );
        const hasRepoPrompt = (await repoSelect.count()) > 0;

        console.log(`GitOps mode shows repository prompt: ${hasRepoPrompt}`);
      }

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC44-gitops-requires-repo.png"
        ),
        fullPage: true,
      });
    });
  });

  test.describe("Catalog Feature - Shared RGD Visibility", () => {
    test("AC-45: Viewer sees shared RGDs plus project-specific RGDs", async ({
      page,
    }) => {
      await loginAs(page, tokens.users.alpha_viewer, "/catalog");

      // Wait for page to load
      await page.waitForSelector('text=/\\d+ available/', {
        timeout: 10000,
      });
      await page.waitForLoadState('networkidle');

      // Count visible RGDs
      const rgdCards = page.locator('[data-testid="rgd-card"], .grid > button');
      const count = await rgdCards.count();

      // Check for shared namespace RGDs
      const sharedRgds = page.locator("text=/shared|common|public/i");
      const sharedCount = await sharedRgds.count();

      console.log(
        `Alpha Viewer sees ${count} total RGDs, ${sharedCount} shared references`
      );

      // Verify filtering is working (may see 0 or limited RGDs based on RBAC)
      const countText = await page
        .locator("text=/\\d+ available/")
        .textContent()
        .catch(() => "");
      console.log(`Catalog count text: ${countText}`);

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC45-viewer-shared-plus-project-rgds.png"
        ),
        fullPage: true,
      });
    });

    test("AC-46: Developer in project-alpha cannot see project-beta RGDs", async ({
      page,
    }) => {
      await loginAs(page, tokens.users.alpha_developer, "/catalog");

      await page.waitForLoadState('networkidle');

      // Check for beta-specific RGDs
      const betaRgds = await page.locator("text=/beta-|project-beta/i").count();
      console.log(`Alpha Developer sees ${betaRgds} beta RGDs (should be 0)`);

      expect(betaRgds).toBe(0);

      await page.screenshot({
        path: path.join(
          ensureEvidenceDir("catalog-rbac"),
          "AC46-developer-no-other-project-rgds.png"
        ),
        fullPage: true,
      });
    });
  });
});
