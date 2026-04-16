// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture'
import type { Page } from '@playwright/test'
import {
  mockMicroservicesPlatformRGD,
  mockMicroservicesPlatformSchema,
  API_PATHS,
} from '../fixture/mock-data'

/**
 * Minimal API mocks needed to reach the Review step of the deploy wizard.
 * Compliance always passes; preflight is overridden per test.
 */
async function setupBaseMocks(page: Page) {
  await page.route('**/api/v1/account/info', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        userID: 'user-global-admin',
        email: 'admin@e2e-test.local',
        displayName: 'Global Administrator',
        groups: [],
        casbinRoles: ['role:serveradmin'],
        projects: [],
        roles: {},
        issuer: 'knodex',
        tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
        tokenIssuedAt: Math.floor(Date.now() / 1000) - 60,
      }),
    })
  })

  await page.route(`**${API_PATHS.rgds}**`, async (route) => {
    const url = route.request().url()
    if (url.includes('/microservices-platform')) {
      if (url.includes('/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockMicroservicesPlatformSchema),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockMicroservicesPlatformRGD),
        })
      }
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [mockMicroservicesPlatformRGD],
          totalCount: 1,
          page: 1,
          pageSize: 10,
        }),
      })
    }
  })

  await page.route('**/api/v1/account/can-i/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ value: 'yes' }),
    })
  })

  await page.route('**/api/v1/dependencies/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        node: null,
        upstream: [],
        downstream: [],
        deploymentOrder: ['microservices-platform'],
        hasCycle: false,
      }),
    })
  })

  await page.route('**/api/v1/projects**', async (route) => {
    const url = route.request().url()
    if (url.includes('/namespaces')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ namespaces: ['default'] }),
      })
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [{ name: 'default-project', destinations: [{ namespace: 'default' }] }],
          totalCount: 1,
        }),
      })
    }
  })

  await page.route('**/api/v1/repositories**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], totalCount: 0 }),
    })
  })

  // Compliance always passes in these tests
  await page.route('**/api/v1/compliance/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ result: 'pass', violations: [] }),
    })
  })

  // Mock resources endpoint (externalRef selector fetches resources)
  await page.route('**/api/v1/resources**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], count: 0 }),
    })
  })

  // NOTE: No default preflight mock here — each test registers its own via page.route()
}

/**
 * Navigate through Target + Configure steps and land on the Review step.
 * Requires base mocks and a preflight mock to be set up first.
 */
async function navigateToReviewStep(page: Page) {
  await page.goto('/catalog')
  await page.waitForLoadState('networkidle')

  const rgdCard = page.getByRole('button', { name: /view details for/i }).first()
  await expect(rgdCard).toBeVisible({ timeout: 15000 })
  await rgdCard.click()

  await page.waitForURL(/\/catalog\//, { timeout: 10000 })
  await page.waitForLoadState('networkidle')

  const deployButton = page.getByRole('button', { name: /deploy/i }).first()
  await expect(deployButton).toBeVisible({ timeout: 15000 })
  await deployButton.click()

  // Step 1: Target
  await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })
  await page.getByPlaceholder('my-instance').fill('test-preflight')

  const nsSelect = page.getByTestId('namespace-select')
  await expect(nsSelect).toBeEnabled({ timeout: 5000 })
  await nsSelect.click()
  await page.getByRole('option', { name: 'default' }).click()

  await page.getByRole('button', { name: /continue/i }).click()

  // Step 2: Configure
  await expect(page.getByTestId('configure-step')).toBeVisible({ timeout: 15000 })
  await page.getByTestId('input-platformName').fill('my-platform')
  // Blur the field to trigger onBlur validation before clicking Continue
  await page.getByTestId('configure-step').click({ position: { x: 1, y: 1 } })

  // Wait for form validation to enable the Continue button, then advance to Review
  const continueBtn = page.getByRole('button', { name: /continue/i })
  await expect(continueBtn).toBeEnabled({ timeout: 10000 })
  await continueBtn.click()

  await expect(page.getByTestId('review-step')).toBeVisible({ timeout: 15000 })
}

test.describe('Deploy Preflight Check', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('shows red blocking banner when preflight returns blocked', async ({ page }) => {
    await setupBaseMocks(page)

    // Mock preflight to return a block
    await page.route('**/instances/**/preflight', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          valid: false,
          message: 'Blocked by Gatekeeper policy "require-owner-annotation-simpleapp": Missing required annotations: {"owner"}',
        }),
      })
    })

    await navigateToReviewStep(page)

    // Red preflight alert banner must be visible
    const banner = page.getByTestId('preflight-alert')
    await expect(banner).toBeVisible({ timeout: 10000 })
    await expect(banner).toContainText('Deployment blocked by admission policy')
    await expect(banner).toContainText('require-owner-annotation-simpleapp')
    await expect(banner).toContainText('Missing required annotations')
  })

  test('Deploy button is disabled when preflight is blocked', async ({ page }) => {
    await setupBaseMocks(page)

    await page.route('**/instances/**/preflight', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          valid: false,
          message: 'Blocked by Gatekeeper policy "my-constraint": violation message',
        }),
      })
    })

    await navigateToReviewStep(page)

    // Deploy button must be disabled
    const deployBtn = page.getByRole('button', { name: /deploy/i }).last()
    await expect(deployBtn).toBeDisabled({ timeout: 10000 })
  })

  test('no banner and Deploy button enabled when preflight passes', async ({ page }) => {
    await setupBaseMocks(page)

    await page.route('**/instances/**/preflight', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ valid: true }),
      })
    })

    await navigateToReviewStep(page)

    // No preflight alert banner
    await expect(page.getByTestId('preflight-alert')).not.toBeVisible()

    // Deploy button must be enabled
    const deployBtn = page.getByRole('button', { name: /deploy/i }).last()
    await expect(deployBtn).toBeEnabled({ timeout: 10000 })
  })

  test('banner shows the exact friendly message from the preflight response', async ({ page }) => {
    await setupBaseMocks(page)

    const friendlyMessage = 'Blocked by Gatekeeper policy "no-default-namespace": Deployments must not target the default namespace'

    await page.route('**/instances/**/preflight', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ valid: false, message: friendlyMessage }),
      })
    })

    await navigateToReviewStep(page)

    const banner = page.getByTestId('preflight-alert')
    await expect(banner).toBeVisible({ timeout: 10000 })
    await expect(banner).toContainText(friendlyMessage)
  })
})

test.describe('Condition Message Formatting in Status Tab', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('displays friendly Gatekeeper error instead of raw admission webhook text', async ({ page }) => {
    const gatekeeperBlockedInstance = {
      name: 'blocked-app',
      namespace: 'production',
      rgdName: 'simple-app',
      rgdNamespace: 'default',
      apiVersion: 'kro.run/v1alpha1',
      kind: 'SimpleApp',
      health: 'Unhealthy',
      conditions: [
        {
          type: 'ResourcesReady',
          status: 'False',
          reason: 'NotReady',
          message:
            'resource reconciliation failed: apply results contain errors: ' +
            'admission webhook "validation.gatekeeper.sh" denied the request: ' +
            '[require-team-label-deployments] Missing required labels: {"team"}',
          lastTransitionTime: '2026-04-07T10:00:00Z',
        },
      ],
      spec: { appName: 'blocked-app', image: 'nginx:latest', port: 80 },
      status: { state: 'ERROR' },
      createdAt: '2026-04-07T10:00:00Z',
      updatedAt: '2026-04-07T10:00:00Z',
    }

    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [],
          totalCount: 0,
          page: 1,
          pageSize: 10,
        }),
      })
    })

    const instanceHandler = async (route: import('@playwright/test').Route) => {
      const url = route.request().url()
      if (url.includes('/blocked-app')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(gatekeeperBlockedInstance),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            items: [gatekeeperBlockedInstance],
            totalCount: 1,
            page: 1,
            pageSize: 10,
          }),
        })
      }
    }
    await page.route(`**${API_PATHS.instances}**`, instanceHandler)
    await page.route(`**${API_PATHS.namespacedInstances}**`, instanceHandler)

    await page.goto('/instances')
    await page.waitForURL('**/instances')

    const instanceCard = page.getByRole('button', { name: /view details for blocked-app/i })
    await expect(instanceCard).toBeVisible({ timeout: 15000 })
    await instanceCard.click()

    await expect(page.getByRole('heading', { name: 'blocked-app' })).toBeVisible()

    // Friendly message must be shown
    await expect(
      page.getByText('Blocked by Gatekeeper policy "require-team-label-deployments": Missing required labels: {"team"}')
    ).toBeVisible()

    // Raw admission webhook text must NOT appear
    await expect(
      page.getByText(/admission webhook "validation\.gatekeeper\.sh" denied/)
    ).not.toBeVisible()
  })
})
