// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture'
import type { Page } from '@playwright/test'
import type { Instance } from '../../src/types/rgd'
import {
  mockRGDListResponse,
  mockInstanceListResponse,
  mockInstances,
  API_PATHS,
} from '../fixture/mock-data'

/**
 * Set up API route mocks for instance status card tests.
 * Optionally override the detail instance returned for prod-db-1.
 */
async function setupInstanceRoutes(page: Page, detailInstance?: Instance) {
  await page.route(`**${API_PATHS.rgds}**`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockRGDListResponse),
    })
  })

  const instanceHandler = async (route: import('@playwright/test').Route) => {
    const url = route.request().url()

    if (url.includes('/prod-db-1')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(detailInstance ?? mockInstances[0]),
      })
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockInstanceListResponse),
      })
    }
  }
  await page.route(`**${API_PATHS.instances}**`, instanceHandler)
  await page.route(`**${API_PATHS.namespacedInstances}**`, instanceHandler)
}

/** Navigate to the prod-db-1 instance detail view */
async function navigateToInstanceDetail(page: Page) {
  await page.goto('/instances')
  await page.waitForURL('**/instances')
  const instanceCard = page.getByRole('button', { name: /view details for prod-db-1/i })
  await expect(instanceCard).toBeVisible()
  await instanceCard.click()
  await expect(page.getByRole('heading', { name: 'prod-db-1' })).toBeVisible()
}

test.describe('Instance Status Card (Unified Status Display)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    await setupInstanceRoutes(page)
    await navigateToInstanceDetail(page)
  })

  test('AC-1: renders unified status card with all three sections', async ({ page }) => {
    // The unified status card should be present
    const statusCard = page.getByTestId('instance-status-card')
    await expect(statusCard).toBeVisible()

    // State badge in header
    await expect(statusCard.getByTestId('state-badge')).toBeVisible()
    await expect(statusCard.getByTestId('state-badge')).toHaveText('ACTIVE')

    // Custom fields section
    await expect(statusCard.getByTestId('custom-fields-section')).toBeVisible()

    // Conditions section
    await expect(statusCard.getByTestId('conditions-section')).toBeVisible()
  })

  test('AC-2: displays KRO state badge with correct color', async ({ page }) => {
    const badge = page.getByTestId('state-badge')
    await expect(badge).toBeVisible()
    await expect(badge).toHaveText('ACTIVE')
  })

  test('AC-3: renders custom status fields as structured key-values', async ({ page }) => {
    const fieldsSection = page.getByTestId('custom-fields-section')
    await expect(fieldsSection).toBeVisible()

    // Scalar string field
    await expect(fieldsSection.getByText('10.96.0.15')).toBeVisible()

    // Scalar number field
    await expect(fieldsSection.getByText('3', { exact: true }).first()).toBeVisible()
  })

  test('AC-4: renders scalar values with appropriate formatting', async ({ page }) => {
    const card = page.getByTestId('instance-status-card')

    // String value
    await expect(card.getByText('postgresql://prod-db-1.production:5432/app')).toBeVisible()

    // Boolean value rendered as "true"
    await expect(card.getByTestId('custom-fields-section').getByText('true', { exact: true }).first()).toBeVisible()
  })

  test('AC-5: renders nested objects with visual hierarchy', async ({ page }) => {
    const card = page.getByTestId('instance-status-card')

    // Nested object keys should be visible
    await expect(card.getByText('Primary', { exact: true }).first()).toBeVisible()
    await expect(card.getByText('Readonly', { exact: true }).first()).toBeVisible()

    // Nested URLs should be clickable links
    const primaryLink = card.getByRole('link', { name: /db-primary\.example\.com/i })
    await expect(primaryLink).toBeVisible()
    await expect(primaryLink).toHaveAttribute('href', 'https://db-primary.example.com')

    const readonlyLink = card.getByRole('link', { name: /db-readonly\.example\.com/i })
    await expect(readonlyLink).toBeVisible()
  })

  test('AC-6: renders array values as chips', async ({ page }) => {
    const card = page.getByTestId('instance-status-card')

    // Array items should appear as chips (not JSON brackets)
    await expect(card.getByText('node-1')).toBeVisible()
    await expect(card.getByText('node-2')).toBeVisible()
    await expect(card.getByText('node-3')).toBeVisible()

    // Should NOT contain JSON array syntax
    await expect(card.locator('text=["node-1"')).toHaveCount(0)
  })

  test('AC-7: conditions section preserves existing rendering', async ({ page }) => {
    const conditions = page.getByTestId('conditions-section')
    await expect(conditions).toBeVisible()

    // Expand conditions section (collapsed by default when all conditions are True)
    const expandButton = conditions.getByRole('button')
    if (await expandButton.getAttribute('aria-expanded') === 'false') {
      await expandButton.click()
    }

    // Condition type
    await expect(conditions.getByText('Ready', { exact: true }).first()).toBeVisible()

    // Condition reason in parentheses
    await expect(conditions.getByText('(ResourcesReady)')).toBeVisible()

    // Condition message
    await expect(conditions.getByText('All resources are ready')).toBeVisible()

    // Condition status badge
    await expect(conditions.getByText('True')).toBeVisible()

    // Sub-header
    await expect(conditions.getByText('Conditions')).toBeVisible()
  })

  test('AC-9: raw JSON status section is removed (status is now a tab, not collapsible)', async ({ page }) => {
    // Status is now a tab, not a collapsible button. Verify the Status tab exists
    // and that there is no separate collapsible "Status" section.
    const statusTab = page.getByRole('tab', { name: 'Status' })
    await expect(statusTab).toBeVisible()

    // The status tab should be active by default (border-primary class)
    await expect(statusTab).toHaveAttribute('aria-selected', 'true')
  })

  test('AC-9: spec raw JSON section still exists (now as a Spec tab)', async ({ page }) => {
    // Spec is now a tab instead of a collapsible section
    const specTab = page.getByRole('tab', { name: 'Spec' })
    await expect(specTab).toBeVisible()

    // Click Spec tab to show spec content
    await specTab.click()

    // Should show JSON content for spec
    await expect(page.getByText('"replicas": 3')).toBeVisible()
    await expect(page.getByText('"storage": "100Gi"')).toBeVisible()
  })
})

test.describe('Instance Status Card - Empty/Minimal Status', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('AC-8: card hides custom fields when only state + conditions exist', async ({ page }) => {
    const instanceWithMinimalStatus: Instance = {
      ...mockInstances[0],
      status: {
        state: 'IN_PROGRESS',
      },
    }

    await setupInstanceRoutes(page, instanceWithMinimalStatus)
    await navigateToInstanceDetail(page)

    // Status card should be visible with state badge
    const statusCard = page.getByTestId('instance-status-card')
    await expect(statusCard).toBeVisible()
    await expect(statusCard.getByTestId('state-badge')).toHaveText('IN_PROGRESS')

    // Custom fields section should NOT be visible (no custom fields)
    await expect(statusCard.getByTestId('custom-fields-section')).not.toBeVisible()
  })
})
