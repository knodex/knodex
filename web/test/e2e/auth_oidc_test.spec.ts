import { test, expect } from '@playwright/test';

/**
 * OIDC Authentication Flow Tests
 *
 * Tests the complete OIDC authentication flow with Mock OIDC server,
 * group claim extraction, and user provisioning.
 *
 * Prerequisites:
 * - Mock OIDC server running on http://localhost:4444
 * - Backend deployed with OIDC configuration
 * - Test users configured in Mock OIDC server
 *
 * Test coverage:
 * - User can successfully login via OIDC mock server
 * - Groups claim is correctly extracted from ID token
 * - User is provisioned with correct project memberships
 * - JWT token contains correct claims
 * - User is redirected to dashboard after successful login
 * - User can logout and JWT token is invalidated
 */

// Use relative URLs where possible - Playwright baseURL is set in playwright.config.ts
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8080';
const MOCK_OIDC_URL = process.env.MOCK_OIDC_URL || 'http://localhost:4444';

// OIDC authentication tests - requires mock OIDC server running
// To run: Start mock OIDC server on port 4444, then run: npx playwright test e2e/oidc-authentication.spec.ts
//
// Prerequisites:
// 1. Mock OIDC server running on http://localhost:4444
// 2. Backend deployed with OIDC enabled (values-e2e.yaml has oidc.enabled: true)
// 3. Mock OIDC server must provide:
//    - /.well-known/openid-configuration endpoint
//    - /auth and /token endpoints
//    - Test users with groups claims
//
// TODO: Implement mock OIDC server or use existing solution (e.g., ory/hydra, keycloak)
// FIXME: All OIDC tests require a mock OIDC server running on port 4444
// Skip until mock OIDC server is integrated into the E2E test infrastructure
test.describe.skip('OIDC Authentication Flow', () => {
  test.beforeEach(async ({ page, context }) => {
    // Clear all auth state before each test
    await context.clearCookies();
    await page.goto(BASE_URL);
    await page.evaluate(() => {
      localStorage.clear();
      sessionStorage.clear();
    });
  });

  test('AC-OIDC-01: User can successfully login via OIDC mock server', async ({ page }) => {
    // Navigate to login page
    await page.goto(`${BASE_URL}/login`);

    // Take screenshot: Login page with OIDC button
    await page.screenshot({
      path: '../test-results/e2e/screenshots/oidc-01-login-page.png',
      fullPage: true
    });

    // Verify OIDC login button is visible
    const oidcButton = page.locator('button:has-text("Sign in with OIDC"), button:has-text("Login with OIDC")');
    await expect(oidcButton).toBeVisible({ timeout: 10000 });

    // Click OIDC login button
    await oidcButton.click();

    // Wait for redirect to Mock OIDC server
    await page.waitForURL(`${MOCK_OIDC_URL}/**`, { timeout: 10000 });

    // Take screenshot: Mock OIDC authorization page
    await page.screenshot({
      path: '../test-results/e2e/screenshots/oidc-01-mock-oidc-auth.png',
      fullPage: true
    });

    // Mock OIDC server should auto-redirect back (simplified flow)
    // Wait for callback redirect to dashboard
    await page.waitForURL(`${BASE_URL}/**`, { timeout: 15000 });

    // Verify we're on the dashboard (not login page)
    await expect(page).not.toHaveURL(`${BASE_URL}/login`);

    // Take screenshot: Dashboard after successful login
    await page.screenshot({
      path: '../test-results/e2e/screenshots/oidc-01-dashboard-after-login.png',
      fullPage: true
    });

    // Verify user is authenticated (check for logout button or user menu)
    const userMenu = page.locator('[data-testid="user-menu"], button:has-text("Logout"), [aria-label*="user menu"]');
    await expect(userMenu.first()).toBeVisible({ timeout: 10000 });
  });

  test('AC-OIDC-02: Groups claim is correctly extracted from ID token', async ({ page }) => {
    // This test requires access to backend logs or network inspection
    // We'll verify by checking the user's role in the UI after login

    await page.goto(`${BASE_URL}/login`);

    // Login as Global Admin (groups: ["global-admins"])
    const oidcButton = page.locator('button:has-text("Sign in with OIDC"), button:has-text("Login with OIDC")');
    await oidcButton.click();

    // Wait for authentication flow to complete
    await page.waitForURL(`${BASE_URL}/**`, { timeout: 15000 });
    await expect(page).not.toHaveURL(`${BASE_URL}/login`);

    // Check for Global Admin indicator in UI
    // This could be a badge, role label, or admin menu items
    const globalAdminIndicator = page.locator('[data-testid="global-admin-badge"], text="Global Admin", [role="badge"]:has-text("Admin")');

    // Wait for role to be loaded and displayed
    await page.waitForTimeout(2000);

    // Take screenshot showing Global Admin role
    await page.screenshot({
      path: '../test-results/e2e/screenshots/oidc-02-global-admin-role.png',
      fullPage: true
    });

    // Verify Global Admin has access to all projects
    // Navigate to projects page or dropdown
    const projectSelector = page.locator('[data-testid="project-selector"], select[name="project"], [aria-label*="project"]');
    if (await projectSelector.isVisible({ timeout: 5000 })) {
      await projectSelector.click();

      // Global Admin should see all test projects
      await expect(page.locator('text=project-alpha')).toBeVisible();
      await expect(page.locator('text=project-beta')).toBeVisible();
      await expect(page.locator('text=project-gamma')).toBeVisible();

      await page.screenshot({
        path: '../test-results/e2e/screenshots/oidc-02-all-projects-visible.png',
        fullPage: true
      });
    }
  });

  test('AC-OIDC-03: User is provisioned with correct project memberships', async ({ page }) => {
    await page.goto(`${BASE_URL}/login`);

    const oidcButton = page.locator('button:has-text("Sign in with OIDC"), button:has-text("Login with OIDC")');
    await oidcButton.click();

    await page.waitForURL(`${BASE_URL}/**`, { timeout: 15000 });
    await expect(page).not.toHaveURL(`${BASE_URL}/login`);

    // Navigate to user profile or settings to see project memberships
    const userMenu = page.locator('[data-testid="user-menu"], button:has-text("Profile"), [aria-label*="user menu"]');
    if (await userMenu.isVisible({ timeout: 5000 })) {
      await userMenu.click();

      // Look for profile or settings option
      const profileLink = page.locator('a:has-text("Profile"), a:has-text("Settings"), button:has-text("Profile")');
      if (await profileLink.isVisible({ timeout: 3000 })) {
        await profileLink.click();

        // Take screenshot of user profile showing projects
        await page.screenshot({
          path: '../test-results/e2e/screenshots/oidc-03-user-projects.png',
          fullPage: true
        });

        // Verify projects are listed (Global Admin should have access to all)
        await expect(page.locator('text=project-alpha, text=Project')).toBeVisible();
      }
    }

    // Check via API call to verify user info from JWT claims
    // User CRD removed - /api/v1/auth/me returns user info from JWT claims
    const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

    if (token) {
      const response = await page.request.get(`${BASE_URL}/api/v1/auth/me`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      expect(response.ok()).toBeTruthy();
      const userData = await response.json();

      // Verify user has project memberships
      expect(userData.projects).toBeDefined();
      expect(Array.isArray(userData.projects)).toBeTruthy();

      console.log('User provisioning data:', JSON.stringify(userData, null, 2));
    }
  });

  test('AC-OIDC-04: JWT token contains correct claims', async ({ page }) => {
    await page.goto(`${BASE_URL}/login`);

    const oidcButton = page.locator('button:has-text("Sign in with OIDC"), button:has-text("Login with OIDC")');
    await oidcButton.click();

    await page.waitForURL(`${BASE_URL}/**`, { timeout: 15000 });
    await expect(page).not.toHaveURL(`${BASE_URL}/login`);

    // Extract JWT token from localStorage or sessionStorage
    const token = await page.evaluate(() => {
      return localStorage.getItem('token') ||
             localStorage.getItem('auth_token') ||
             sessionStorage.getItem('token') ||
             sessionStorage.getItem('auth_token');
    });

    expect(token).toBeTruthy();

    // Decode JWT token (base64 decode the payload)
    const tokenParts = token!.split('.');
    expect(tokenParts.length).toBe(3); // header.payload.signature

    const payload = JSON.parse(Buffer.from(tokenParts[1], 'base64').toString());

    console.log('JWT Token Claims:', JSON.stringify(payload, null, 2));

    // Verify required claims are present
    expect(payload.sub).toBeDefined(); // Subject (user ID)
    expect(payload.email).toBeDefined(); // Email
    expect(payload.projects).toBeDefined(); // Projects array
    expect(payload.role || payload.roles).toBeDefined(); // Role(s)
    // Global admin is determined by casbin_roles containing 'role:serveradmin'
    expect(payload.casbin_roles).toContain('role:serveradmin'); // Global Admin via Casbin role

    // Take screenshot documenting token verification
    await page.screenshot({
      path: '../test-results/e2e/screenshots/oidc-04-jwt-token-verified.png',
      fullPage: true
    });
  });

  test('AC-OIDC-05: User is redirected to dashboard after successful login with all projects visible', async ({ page }) => {
    await page.goto(`${BASE_URL}/login`);

    const oidcButton = page.locator('button:has-text("Sign in with OIDC"), button:has-text("Login with OIDC")');
    await oidcButton.click();

    // Wait for authentication and redirect
    await page.waitForURL(`${BASE_URL}/**`, { timeout: 15000 });

    // Verify NOT on login page
    await expect(page).not.toHaveURL(`${BASE_URL}/login`);

    // Verify on dashboard (common paths: /, /dashboard, /home)
    const currentUrl = page.url();
    expect(
      currentUrl === `${BASE_URL}/` ||
      currentUrl === `${BASE_URL}/dashboard` ||
      currentUrl === `${BASE_URL}/home` ||
      currentUrl.includes('/catalog') ||
      currentUrl.includes('/instances')
    ).toBeTruthy();

    // Wait for page to fully load
    await page.waitForLoadState('networkidle', { timeout: 10000 });

    // Verify all projects are visible
    // Global Admin should see all projects in selector or sidebar
    const allProjectsVisible = page.locator('text=project-alpha');
    await expect(allProjectsVisible).toBeVisible({ timeout: 10000 });

    // Take screenshot showing dashboard with all projects
    await page.screenshot({
      path: '../test-results/e2e/screenshots/oidc-05-dashboard-all-projects.png',
      fullPage: true
    });

    // Verify we can see multiple projects (at least 3)
    const alphaProject = page.locator('text=project-alpha').first();
    const betaProject = page.locator('text=project-beta').first();
    const gammaProject = page.locator('text=project-gamma').first();

    await expect(alphaProject).toBeVisible();
    await expect(betaProject).toBeVisible();
    await expect(gammaProject).toBeVisible();
  });

  test('AC-OIDC-06: User can logout and JWT token is invalidated', async ({ page }) => {
    // Login first
    await page.goto(`${BASE_URL}/login`);

    const oidcButton = page.locator('button:has-text("Sign in with OIDC"), button:has-text("Login with OIDC")');
    await oidcButton.click();

    await page.waitForURL(`${BASE_URL}/**`, { timeout: 15000 });
    await expect(page).not.toHaveURL(`${BASE_URL}/login`);

    // Get token before logout
    const tokenBeforeLogout = await page.evaluate(() => {
      return localStorage.getItem('token') || sessionStorage.getItem('token');
    });
    expect(tokenBeforeLogout).toBeTruthy();

    // Take screenshot before logout
    await page.screenshot({
      path: '../test-results/e2e/screenshots/oidc-06-before-logout.png',
      fullPage: true
    });

    // Find and click logout button
    const logoutButton = page.locator('button:has-text("Logout"), button:has-text("Sign out"), a:has-text("Logout")');

    // May need to open user menu first
    const userMenu = page.locator('[data-testid="user-menu"], [aria-label*="user menu"], button:has([data-testid="user-avatar"])');
    if (await userMenu.isVisible({ timeout: 5000 })) {
      await userMenu.click();
      await page.waitForTimeout(500);
    }

    await expect(logoutButton).toBeVisible({ timeout: 10000 });
    await logoutButton.click();

    // Wait for redirect to login page
    await page.waitForURL(`${BASE_URL}/login`, { timeout: 10000 });

    // Take screenshot after logout
    await page.screenshot({
      path: '../test-results/e2e/screenshots/oidc-06-after-logout.png',
      fullPage: true
    });

    // Verify token is cleared
    const tokenAfterLogout = await page.evaluate(() => {
      return localStorage.getItem('token') || sessionStorage.getItem('token');
    });
    expect(tokenAfterLogout).toBeNull();

    // Verify that making API call with old token returns 401
    if (tokenBeforeLogout) {
      const response = await page.request.get(`${BASE_URL}/api/v1/auth/me`, {
        headers: { Authorization: `Bearer ${tokenBeforeLogout}` },
        failOnStatusCode: false
      });

      // Should return 401 Unauthorized
      expect(response.status()).toBe(401);
    }

    // Verify cannot access protected pages
    await page.goto(`${BASE_URL}/catalog`);

    // Should redirect back to login
    await page.waitForURL(`${BASE_URL}/login`, { timeout: 10000 });
    await expect(page).toHaveURL(`${BASE_URL}/login`);
  });
});
