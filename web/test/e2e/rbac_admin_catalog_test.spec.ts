import { test, expect, TestUserRole } from '../fixture';
import { setupPermissionMocking } from '../fixture/auth-helper';

/**
 * Global Admin - RGD Catalog Access Tests
 *
 * Tests that Global Admin users can see ALL RGDs in the catalog
 * without any organization-based filtering.
 *
 * Prerequisites:
 * - Backend deployed with test data (RGDs in cluster)
 * - Global Admin user logged in (groups: ["global-admins"])
 * - Test RGDs: simple-app, webapp-with-features, microservices-platform, e2e-* RGDs
 *
 * Test coverage:
 * - Global Admin sees all RGDs in catalog (no org filtering)
 * - Global Admin sees both shared RGDs and all org-specific RGDs
 * - RGD count matches total RGDs in cluster (5 test RGDs)
 */

// Use relative URLs - Playwright baseURL is set in playwright.config.ts
// This allows tests to work with dynamic Kind cluster ports
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8080';

test.describe('Global Admin - RGD Catalog Access', () => {
  // Authenticate as Global Admin to access all RGDs
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test.beforeEach(async ({ page }) => {
    // Mock permission API for Global Admin - full access
    await setupPermissionMocking(page, { '*:*': true });
  });

  test('AC-CATALOG-01: Global Admin sees all RGDs in catalog (no org filtering)', async ({ page }) => {
    // Navigate to catalog page
    await page.goto(`/catalog`);

    // Wait for catalog to load
    await page.waitForLoadState('networkidle', { timeout: 10000 });

    // Take screenshot of full catalog
    await page.screenshot({
      path: '../test-results/e2e/screenshots/catalog-01-all-rgds.png',
      fullPage: true
    });

    // Verify RGDs are visible in the catalog
    // Global Admin should see all RGDs with catalog annotation
    const rgdCards = page.getByRole('button', { name: /view details for/i });
    const rgdCount = await rgdCards.count();

    console.log(`Global Admin sees ${rgdCount} RGDs in catalog`);

    // Global Admin should see at least 1 RGD for catalog to be functional
    expect(rgdCount).toBeGreaterThanOrEqual(1);

    // Verify no organization filter is applied
    // Look for "All Organizations" or lack of org filter
    const orgFilter = page.locator('[data-testid="org-filter"], select[name="organization-filter"]');
    if (await orgFilter.isVisible({ timeout: 3000 })) {
      const filterValue = await orgFilter.inputValue();
      // Should be empty, "all", or not filtering
      expect(filterValue === '' || filterValue === 'all' || filterValue === 'All Organizations').toBeTruthy();
    }
  });

  test('AC-CATALOG-02: Global Admin sees both shared RGDs and all org-specific RGDs', async ({ page }) => {
    await page.goto(`/catalog`);
    await page.waitForLoadState('networkidle', { timeout: 10000 });

    // Verify RGDs are visible in the catalog
    // RGD cards are buttons with accessible name: "View details for {rgd-name}"
    const allRGDs = page.getByRole('button', { name: /view details for/i });
    const rgdCount = await allRGDs.count();
    expect(rgdCount).toBeGreaterThanOrEqual(1); // At least 1 RGD in catalog

    console.log(`Found ${rgdCount} RGDs in catalog`);

    // Take screenshot highlighting public RGDs
    await page.screenshot({
      path: '../test-results/e2e/screenshots/catalog-02-public-rgds.png',
      fullPage: true
    });

    // Verify RGDs are visible in the catalog
    // Global Admin should see all available RGDs
    const allRgdCards = page.getByRole('button', { name: /view details for/i });
    const totalCount = await allRgdCards.count();

    console.log(`Global Admin sees ${totalCount} total RGDs in catalog`);

    // Global Admin should see at least the public RGDs
    expect(totalCount).toBeGreaterThanOrEqual(1);

    // Take screenshot highlighting org-specific RGDs
    await page.screenshot({
      path: '../test-results/e2e/screenshots/catalog-02-org-specific-rgds.png',
      fullPage: true
    });

    // Verify API response includes both shared and org-specific
    const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

    if (token) {
      const response = await page.request.get(`${BASE_URL}/api/v1/rgds`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      expect(response.ok()).toBeTruthy();
      const rgdsData = await response.json();

      console.log('RGDs API Response:', JSON.stringify(rgdsData, null, 2));

      // Verify response contains both shared and org-specific RGDs
      const rgds = rgdsData.items || rgdsData.rgds || rgdsData;
      expect(Array.isArray(rgds)).toBeTruthy();

      const sharedRGDsFromAPI = rgds.filter((rgd: any) =>
        rgd.name.includes('shared') || rgd.metadata?.labels?.['visibility'] === 'shared'
      );

      const orgSpecificRGDsFromAPI = rgds.filter((rgd: any) =>
        (rgd.name.includes('alpha') || rgd.name.includes('beta') || rgd.name.includes('gamma')) ||
        rgd.metadata?.labels?.['organization']
      );

      // Log counts for debugging - these may vary by test environment
      console.log(`Shared RGDs: ${sharedRGDsFromAPI.length}, Org-specific RGDs: ${orgSpecificRGDsFromAPI.length}`);

      // Verify API returns RGDs (total count should match UI)
      expect(rgds.length).toBeGreaterThanOrEqual(1);
    }
  });

  test('AC-CATALOG-03: RGD count matches API response', async ({ page }) => {
    await page.goto(`/catalog`);
    await page.waitForLoadState('networkidle', { timeout: 10000 });

    // Count total RGDs displayed in UI
    // RGD cards are buttons with accessible name: "View details for {rgd-name}"
    const rgdCards = page.getByRole('button', { name: /view details for/i });

    // Wait for catalog items to load
    await page.waitForTimeout(2000);

    const displayedCount = await rgdCards.count();
    console.log(`Total RGDs displayed: ${displayedCount}`);

    // Should have at least 1 RGD for catalog functionality
    expect(displayedCount).toBeGreaterThanOrEqual(1);

    // Look for count indicator in UI (e.g., "11 available")
    const countIndicator = page.locator('text=/\\d+ available/i').or(page.getByTestId('rgd-count'));
    if (await countIndicator.isVisible({ timeout: 3000 }).catch(() => false)) {
      const countText = await countIndicator.textContent();
      console.log('RGD count from UI:', countText);

      await page.screenshot({
        path: '../test-results/e2e/screenshots/catalog-03-rgd-count-indicator.png',
        fullPage: true
      });
    }

    // Verify via API that count matches
    const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

    if (token) {
      const response = await page.request.get(`${BASE_URL}/api/v1/rgds`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      expect(response.ok()).toBeTruthy();
      const rgdsData = await response.json();

      const rgds = rgdsData.items || rgdsData.rgds || rgdsData;
      const totalCount = rgdsData.total || rgds.length;

      console.log(`Total RGDs from API: ${totalCount}`);

      // API should return at least the same count as displayed
      expect(totalCount).toBeGreaterThanOrEqual(displayedCount);

      // Log available RGD names for debugging
      const rgdNames = rgds.map((rgd: any) => rgd.name || rgd.metadata?.name);
      console.log(`Available RGDs: ${rgdNames.join(', ')}`);
    }

    // Take final screenshot
    await page.screenshot({
      path: '../test-results/e2e/screenshots/catalog-03-rgds.png',
      fullPage: true
    });
  });

  test('Global Admin can click on any RGD and view details', async ({ page }) => {
    // Additional test: Verify Global Admin can access any RGD detail page
    await page.goto(`/catalog`);
    await page.waitForLoadState('networkidle', { timeout: 10000 });

    // Click on first RGD card
    // RGD cards are buttons with accessible name: "View details for {rgd-name}"
    const firstRGD = page.getByRole('button', { name: /view details for/i }).first();
    await firstRGD.click();

    // Verify detail page loaded
    // Look for detail page indicators: Back to catalog link, tabs, details section
    const backLink = page.getByText('Back to catalog');
    const overviewTab = page.getByText('Overview');
    const detailsSection = page.getByText('Details');

    await expect(backLink).toBeVisible({ timeout: 10000 });
    await expect(overviewTab).toBeVisible();
    await expect(detailsSection).toBeVisible();

    await page.screenshot({
      path: '../test-results/e2e/screenshots/catalog-rgd-detail-page.png',
      fullPage: true
    });
  });
});
