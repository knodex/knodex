// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect } from '@playwright/test';

// Backend returns provider names array, frontend transforms them
const MOCK_PROVIDERS_RESPONSE = { providers: ['google', 'keycloak'] };
const MOCK_PROVIDERS_EMPTY = { providers: [] };

// JWT payload with casbin_roles instead of is_global_admin
// JWT payload: {"sub":"admin","email":"admin@example.com","name":"Administrator","projects":["project-alpha","project-beta"],"default_project":"project-alpha","casbin_roles":["role:serveradmin"],"exp":9999999999,"iat":1700000000,"iss":"knodex","aud":"kro"}
const MOCK_JWT_TOKEN =
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiIsImVtYWlsIjoiYWRtaW5AZXhhbXBsZS5jb20iLCJuYW1lIjoiQWRtaW5pc3RyYXRvciIsInByb2plY3RzIjpbInByb2plY3QtYWxwaGEiLCJwcm9qZWN0LWJldGEiXSwiZGVmYXVsdF9wcm9qZWN0IjoicHJvamVjdC1hbHBoYSIsImNhc2Jpbl9yb2xlcyI6WyJyb2xlOmdsb2JhbC1hZG1pbiJdLCJleHAiOjk5OTk5OTk5OTksImlhdCI6MTcwMDAwMDAwMCwiaXNzIjoia3JvLWRhc2hib2FyZCIsImF1ZCI6Imtyby1kYXNoYm9hcmQifQ.mock-signature';

test.describe('Authentication Flow', () => {
  test.beforeEach(async ({ page, context }) => {
    // Clear all storage and cookies before each test
    await context.clearCookies();

    // Navigate to app domain first and clear storage once
    // This ensures we start with clean auth state
    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => {
      localStorage.clear();
      sessionStorage.clear();
    });
  });

  test.describe('Login Page', () => {
    test('redirects unauthenticated users to login', async ({ page }) => {
      await page.goto('/', { waitUntil: 'domcontentloaded' });
      await page.waitForURL('**/login', { timeout: 5000 });
      await expect(page).toHaveURL('/login');
      await expect(page.getByText('Knodex')).toBeVisible();
      await expect(page.getByText('Kubernetes Native Self Service Platform')).toBeVisible();
    });

    test('displays local admin login form', async ({ page }) => {
      // Mock OIDC providers endpoint (no providers configured)
      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_EMPTY),
        });
      });

      await page.goto('/login', { waitUntil: 'domcontentloaded' });

      await expect(page.getByText('Administrator Login')).toBeVisible();
      await expect(page.getByLabel(/username/i)).toBeVisible();
      await expect(page.getByLabel(/password/i)).toBeVisible();
      await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible();
    });

    test('displays OIDC provider buttons when available', async ({ page }) => {
      // Mock OIDC providers endpoint (with providers)
      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_RESPONSE),
        });
      });

      await page.goto('/login');

      await expect(page.getByText('Single Sign-On')).toBeVisible();
      await expect(page.getByRole('button', { name: /continue with google/i })).toBeVisible();
      await expect(page.getByRole('button', { name: /continue with keycloak/i })).toBeVisible();
      await expect(page.getByText('Or', { exact: true })).toBeVisible();
    });

    test('shows validation errors for empty form', async ({ page }) => {
      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_EMPTY),
        });
      });

      await page.goto('/login');

      // Submit empty form
      await page.getByRole('button', { name: /sign in/i }).click();

      // Check for validation errors
      await expect(page.getByText(/username is required/i)).toBeVisible();
      await expect(page.getByText(/password is required/i)).toBeVisible();
    });
  });

  test.describe('Local Admin Login', () => {
    // FIXME: Flaky when running against deployed backend with mocked routes.
    // Root cause: page.route() mocks may not intercept requests fired during initial page load.
    // Fix: Convert to component test or use fixture-based auth with real backend login API.
    test.fixme('successfully logs in with valid credentials', async ({ page }) => {
      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_EMPTY),
        });
      });

      // Mock successful login
      await page.route('**/api/v1/auth/local/login', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ token: MOCK_JWT_TOKEN }),
        });
      });

      // Mock RGD list endpoint (dashboard data)
      await page.route('**/api/v1/rgds*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      // Mock projects endpoint (for namespace selector)
      await page.route('**/api/v1/projects*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [{ name: 'project-alpha', display_name: 'Project Alpha' }] }),
        });
      });

      // Mock instances endpoint
      await page.route('**/api/v1/instances*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      await page.goto('/login');

      // Fill in credentials
      await page.getByLabel(/username/i).fill('admin');
      await page.getByLabel(/password/i).fill('password123');

      // Submit form
      await page.getByRole('button', { name: /sign in/i }).click();

      // Should redirect to catalog (root "/" redirects to "/catalog")
      await expect(page).toHaveURL('/catalog');
      await expect(page.getByText('Resource Definitions')).toBeVisible();

      // Should show username in header
      await expect(page.getByText('admin')).toBeVisible();
    });

    test('displays error on invalid credentials', async ({ page }) => {
      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_EMPTY),
        });
      });

      // Mock failed login with standardized API error format
      await page.route('**/api/v1/auth/local/login', async (route) => {
        await route.fulfill({
          status: 401,
          contentType: 'application/json',
          body: JSON.stringify({ code: 'UNAUTHORIZED', message: 'invalid credentials' }),
        });
      });

      await page.goto('/login');

      // Fill in credentials
      await page.getByLabel(/username/i).fill('admin');
      await page.getByLabel(/password/i).fill('wrongpassword');

      // Submit form
      await page.getByRole('button', { name: /sign in/i }).click();

      // Should show error message from API response
      await expect(page.getByText(/invalid credentials/i)).toBeVisible();
      await expect(page).toHaveURL('/login');
    });

    // FIXME: Mocked login response timing - loading state transitions before assertion.
    // Fix: Convert to component test or use page.waitForResponse() pattern.
    test.fixme('disables form during submission', async ({ page }) => {
      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_EMPTY),
        });
      });

      // Mock slow login response
      await page.route('**/api/v1/auth/local/login', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 200));
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ token: MOCK_JWT_TOKEN }),
        });
      });

      await page.goto('/login');

      // Fill in credentials
      await page.getByLabel(/username/i).fill('admin');
      await page.getByLabel(/password/i).fill('password123');

      // Submit form
      const submitButton = page.getByRole('button', { name: /sign in/i });
      await submitButton.click();

      // Form should be disabled
      await expect(page.getByLabel(/username/i)).toBeDisabled();
      await expect(page.getByLabel(/password/i)).toBeDisabled();
      await expect(submitButton).toBeDisabled();
      await expect(submitButton).toHaveText(/signing in/i);
    });
  });

  test.describe('OIDC Authentication', () => {
    test('initiates OIDC login when provider button clicked', async ({ page }) => {
      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_RESPONSE),
        });
      });

      await page.goto('/login');

      // Click Google provider button
      const googleButton = page.getByRole('button', { name: /continue with google/i });

      // Mock the OIDC login redirect
      await page.route('**/api/v1/auth/oidc/login**', async (route) => {
        // Simulate redirect to auth callback with token
        await route.fulfill({
          status: 302,
          headers: {
            'Location': `/auth/callback?token=${MOCK_JWT_TOKEN}`,
          },
        });
      });

      await googleButton.click();
    });
  });

  test.describe('Auth Callback', () => {
    // FIXME: Mocked OIDC callback response timing - Zustand state not updated before assertion.
    // Fix: Convert to component test or inject auth state directly via localStorage.
    test.fixme('handles successful OIDC callback', async ({ page }) => {
      // Mock RGD list endpoint
      await page.route('**/api/v1/rgds*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      // Mock projects endpoint (for namespace selector)
      await page.route('**/api/v1/projects*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [{ name: 'project-alpha', display_name: 'Project Alpha' }] }),
        });
      });

      // Mock instances endpoint
      await page.route('**/api/v1/instances*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      // Navigate directly to callback with token
      await page.goto(`/auth/callback?token=${MOCK_JWT_TOKEN}`);

      // Should redirect to catalog (root "/" redirects to "/catalog")
      await expect(page).toHaveURL('/catalog');
      await expect(page.getByText('Resource Definitions')).toBeVisible();
      await expect(page.getByText('admin')).toBeVisible();
    });

    test('handles OIDC callback error', async ({ page }) => {
      const errorMessage = 'Authentication failed: invalid provider';

      // Navigate to callback with error
      await page.goto(`/auth/callback?error=${encodeURIComponent(errorMessage)}`);

      // Should show error message
      await expect(page.getByRole('heading', { name: 'Authentication Failed' })).toBeVisible();
      await expect(page.getByText(errorMessage)).toBeVisible();
      await expect(page.getByText('Redirecting to login page...')).toBeVisible();

      // Should redirect to login after timeout
      await page.waitForURL('/login', { timeout: 5000 });
    });

    test('handles callback without token', async ({ page }) => {
      // Navigate to callback without token
      await page.goto('/auth/callback');

      // Should show error
      await expect(page.getByRole('heading', { name: 'Authentication Failed' })).toBeVisible();
      await expect(
        page.getByText('No authorization code received from authentication provider')
      ).toBeVisible();

      // Should redirect to login
      await page.waitForURL('/login', { timeout: 5000 });
    });
  });

  test.describe('Protected Routes', () => {
    test('redirects to login when accessing protected route unauthenticated', async ({ page }) => {
      await page.goto('/');
      await expect(page).toHaveURL('/login');
    });

    // FIXME: Mocked routes don't intercept reliably against deployed backend.
    // Fix: Convert to component test or use fixture-based auth.
    test.fixme('allows access to dashboard when authenticated', async ({ page }) => {
      // Mock login
      await page.route('**/api/v1/auth/local/login', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ token: MOCK_JWT_TOKEN }),
        });
      });

      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_EMPTY),
        });
      });

      await page.route('**/api/v1/rgds*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      // Mock projects endpoint (for namespace selector)
      await page.route('**/api/v1/projects*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [{ name: 'project-alpha', display_name: 'Project Alpha' }] }),
        });
      });

      // Mock instances endpoint
      await page.route('**/api/v1/instances*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      await page.goto('/login');

      // Login
      await page.getByLabel(/username/i).fill('admin');
      await page.getByLabel(/password/i).fill('password123');
      await page.getByRole('button', { name: /sign in/i }).click();

      // Should be on catalog (root "/" redirects to "/catalog")
      await expect(page).toHaveURL('/catalog');
      await expect(page.getByText('admin')).toBeVisible();
    });
  });

  test.describe('Logout', () => {
    // FIXME: Mocked login + logout flow has timing issues with Zustand state transitions.
    // Fix: Convert to component test or use fixture-based auth for the login step.
    test.fixme('logs out user and redirects to login', async ({ page }) => {
      // Setup authentication
      await page.route('**/api/v1/auth/local/login', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ token: MOCK_JWT_TOKEN }),
        });
      });

      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_EMPTY),
        });
      });

      await page.route('**/api/v1/rgds*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      // Mock projects endpoint (for namespace selector)
      await page.route('**/api/v1/projects*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [{ name: 'project-alpha', display_name: 'Project Alpha' }] }),
        });
      });

      // Mock instances endpoint
      await page.route('**/api/v1/instances*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      await page.goto('/login');

      // Login
      await page.getByLabel(/username/i).fill('admin');
      await page.getByLabel(/password/i).fill('password123');
      await page.getByRole('button', { name: /sign in/i }).click();

      // Wait for catalog (root "/" redirects to "/catalog")
      await expect(page).toHaveURL('/catalog');

      // Click logout
      await page.getByRole('button', { name: /logout/i }).click();

      // Should redirect to login
      await expect(page).toHaveURL('/login');

      // User should not be able to access dashboard
      await page.goto('/');
      await expect(page).toHaveURL('/login');
    });
  });

  test.describe('Token Persistence', () => {
    // FIXME: Mocked routes don't intercept reliably against deployed backend.
    // Fix: Convert to component test or use fixture-based auth.
    test.fixme('persists authentication across page reloads', async ({ page }) => {
      // Mock login
      await page.route('**/api/v1/auth/local/login', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ token: MOCK_JWT_TOKEN }),
        });
      });

      await page.route('**/api/v1/auth/oidc/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PROVIDERS_EMPTY),
        });
      });

      await page.route('**/api/v1/rgds*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      // Mock projects endpoint (for namespace selector)
      await page.route('**/api/v1/projects*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [{ name: 'project-alpha', display_name: 'Project Alpha' }] }),
        });
      });

      // Mock instances endpoint
      await page.route('**/api/v1/instances*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [] }),
        });
      });

      await page.goto('/login');

      // Login
      await page.getByLabel(/username/i).fill('admin');
      await page.getByLabel(/password/i).fill('password123');
      await page.getByRole('button', { name: /sign in/i }).click();

      await expect(page).toHaveURL('/catalog');

      // Reload page
      await page.reload();

      // Should still be authenticated (root "/" redirects to "/catalog")
      await expect(page).toHaveURL('/catalog');
      await expect(page.getByText('admin')).toBeVisible();
    });
  });
});
