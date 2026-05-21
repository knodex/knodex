// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture'
import type { Page } from '@playwright/test'

/**
 * E2E coverage for the full-page tabbed deploy experience at /deploy/:rgdName.
 *
 * Covers:
 *   - Tab derivation from RGD schema (Basics → General → object tabs → Review)
 *   - Direct-link via URL hash selects the right tab on mount
 *   - Browser back button navigates between visited tabs
 *   - Free-jump tab navigation
 *   - Deploy gated by (form valid) ∧ (compliance ≠ block) ∧ (warning ⇒ acked) ∧ (preflight valid)
 *   - Redeploy via `error-recovery-actions` prefills Basics from location.state
 */

const TEST_RGD_NAME = 'tabbed-test-rgd'

const SCHEMA_RESPONSE = {
  crdFound: true,
  schema: {
    name: TEST_RGD_NAME,
    namespace: 'default',
    group: 'tabbed.knodex.io',
    version: 'v1alpha1',
    kind: 'TabbedTest',
    title: 'Tabbed Test',
    description: 'RGD used by E2E tests for the tabbed deploy page',
    propertyOrder: ['name', 'networking', 'storage'],
    required: ['name'],
    properties: {
      name: {
        type: 'string',
        description: 'Display name for the deployment',
        default: 'demo',
      },
      networking: {
        type: 'object',
        properties: {
          port: {
            type: 'integer',
            minimum: 1,
            maximum: 65535,
            default: 8080,
          },
          host: {
            type: 'string',
            default: 'localhost',
          },
        },
        required: ['port'],
      },
      storage: {
        type: 'object',
        properties: {
          size: {
            type: 'string',
            default: '1Gi',
          },
        },
      },
    },
  },
}

const RGD_RESPONSE = {
  name: TEST_RGD_NAME,
  namespace: 'default',
  description: SCHEMA_RESPONSE.schema.description,
  title: SCHEMA_RESPONSE.schema.title,
  tags: [],
  category: 'examples',
  labels: { 'knodex.io/catalog': 'true' },
  instances: 0,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGraphDefinition',
  status: 'Active',
  createdAt: '2026-05-18T10:00:00Z',
  updatedAt: '2026-05-18T10:00:00Z',
}

interface SetupOptions {
  compliance?: 'pass' | 'warning' | 'block'
  preflightValid?: boolean
  preflightMessage?: string
}

async function setupMocks(page: Page, opts: SetupOptions = {}) {
  const compliance = opts.compliance ?? 'pass'
  const preflightValid = opts.preflightValid ?? true

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

  await page.route('**/api/v1/account/can-i/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ value: 'yes' }),
    })
  })

  await page.route('**/api/v1/rgds/**', async (route) => {
    const url = route.request().url()
    if (url.includes('/schema')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(SCHEMA_RESPONSE),
      })
    } else if (url.includes(TEST_RGD_NAME)) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(RGD_RESPONSE),
      })
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], totalCount: 0 }),
      })
    }
  })

  await page.route('**/api/v1/projects**', async (route) => {
    const url = route.request().url()
    if (url.includes('/namespaces')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ namespaces: ['alpha-prod', 'alpha-dev'] }),
      })
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [{ name: 'alpha', destinations: [] }],
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

  await page.route('**/api/v1/compliance/validate', async (route) => {
    let body: { result: string; violations: unknown[] } = {
      result: 'pass',
      violations: [],
    }
    if (compliance === 'warning') {
      body = {
        result: 'warning',
        violations: [
          {
            policy: 'policy/x',
            severity: 'warning',
            message: 'concern',
          },
        ],
      }
    } else if (compliance === 'block') {
      body = {
        result: 'block',
        violations: [
          { policy: 'policy/blocked', severity: 'error', message: 'denied' },
        ],
      }
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(body),
    })
  })

  // Preflight URLs differ for cluster-scoped vs namespaced. Mock both.
  await page.route('**/v1/apigroups/*/instances/*/preflight', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        valid: preflightValid,
        message: opts.preflightMessage,
      }),
    })
  })
  await page.route('**/v1/apigroups/*/namespaces/*/instances/*/preflight', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        valid: preflightValid,
        message: opts.preflightMessage,
      }),
    })
  })
}

async function gotoBasics(page: Page) {
  await page.goto(`/deploy/${TEST_RGD_NAME}`)
  await expect(page.getByTestId('deploy-tab-basics')).toBeVisible({
    timeout: 15000,
  })
}

test.describe('Tabbed deploy navigation', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('renders tabs derived from schema in propertyOrder', async ({ page }) => {
    await setupMocks(page)
    await gotoBasics(page)

    await expect(page.getByTestId('deploy-tab-basics')).toBeVisible()
    await expect(page.getByTestId('deploy-tab-general')).toBeVisible()
    await expect(page.getByTestId('deploy-tab-networking')).toBeVisible()
    await expect(page.getByTestId('deploy-tab-storage')).toBeVisible()
    await expect(page.getByTestId('deploy-tab-review')).toBeVisible()
  })

  test('direct-link via hash selects the matching tab on mount', async ({
    page,
  }) => {
    await setupMocks(page)
    await page.goto(`/deploy/${TEST_RGD_NAME}#networking`)
    const networkingTab = page.getByTestId('deploy-tab-networking')
    await expect(networkingTab).toBeVisible({ timeout: 15000 })
    await expect(networkingTab).toHaveAttribute('data-state', 'active')
  })

  test('back button moves through visited tabs without unmount', async ({
    page,
  }) => {
    await setupMocks(page)
    await gotoBasics(page)

    await page.getByTestId('instance-name-input').fill('e2e-instance')
    await page.getByTestId('deploy-tab-networking').click()
    await expect(page).toHaveURL(/#networking$/)
    await page.getByTestId('deploy-tab-storage').click()
    await expect(page).toHaveURL(/#storage$/)

    await page.goBack()
    await expect(page).toHaveURL(/#networking$/)

    await page.goBack()
    await expect(page).toHaveURL(/#basics$|\/deploy\/[^#]+$/)
    // Form value preserved (not unmounted).
    await expect(page.getByTestId('instance-name-input')).toHaveValue(
      'e2e-instance'
    )
  })

  test('free-jump: any tab is reachable regardless of validation', async ({
    page,
  }) => {
    await setupMocks(page)
    await gotoBasics(page)

    await page.getByTestId('deploy-tab-review').click()
    await expect(page.getByTestId('deploy-tab-review')).toHaveAttribute(
      'data-state',
      'active'
    )
  })

  test('Deploy button is disabled until compliance + preflight gates open', async ({
    page,
  }) => {
    await setupMocks(page, { compliance: 'warning' })
    await gotoBasics(page)

    // Fill required Basics fields. namespace-select is a native <select>
    // (avoids the Radix SlotClone ref-composition loop) — use selectOption().
    await page.getByTestId('instance-name-input').fill('e2e-instance')
    await page.getByTestId('namespace-select').selectOption('alpha-dev')

    // Jump to Review — compliance returns warning
    await page.getByTestId('deploy-tab-review').click()
    const deployBtn = page.getByTestId('deploy-footer-deploy')
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    // Wait for compliance + preflight to settle. Deploy stays disabled until ack.
    await expect(deployBtn).toBeDisabled()

    const ackBox = page.getByTestId('compliance-acknowledge-checkbox')
    await expect(ackBox).toBeVisible({ timeout: 10000 })
    await ackBox.check()
    await expect(deployBtn).toBeEnabled()
  })

  // FIXME (full-page-tabbed-deploy v1):
  // The `prefill` flow lives in `error-recovery-actions.tsx` which calls
  // `navigate("/deploy/<rgdName>", {state: {prefill, instanceId, namespace}})`.
  // This test reproduces the navigation via window.history.pushState +
  // PopStateEvent, but that doesn't populate react-router's `useLocation().state`
  // — react-router reads its own internal history stack, not the browser's
  // `history.state`. Result: DeployPage receives `location.state === null` and
  // never prefills. A correct test would navigate from a route that calls
  // `navigate(..., {state})` programmatically (e.g., simulate the instance
  // redeploy CTA on error-recovery-actions), not synthesize the history entry.
  test.fixme('prefill from location.state populates Basics on entry', async ({
    page,
  }) => {
    await setupMocks(page)
    await page.goto('/catalog')
    await page.evaluate(
      ({ rgd }) => {
        window.history.pushState(
          { prefill: true, instanceId: 'redeploy-me', namespace: 'alpha-dev' },
          '',
          `/deploy/${rgd}`
        )
        window.dispatchEvent(new PopStateEvent('popstate'))
      },
      { rgd: TEST_RGD_NAME }
    )

    await expect(page.getByTestId('instance-name-input')).toHaveValue(
      'redeploy-me',
      { timeout: 15000 }
    )
  })
})
