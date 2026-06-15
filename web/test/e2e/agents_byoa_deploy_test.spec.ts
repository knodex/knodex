// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * BYOA Agent Deployment E2E Tests (Stories 49.3 / 53.3)
 *
 * A kagent agent is deployed as an ORDINARY RGD instance through the GVK-aware
 * instance routes and the Casbin instances/create gate. Re-pointed for the 53.x
 * collapse: 53.3 dropped `knodex.io/catalog: "true"` from the agent RGD, so it
 * no longer surfaces in the Catalog — the entry point is the Agents tab's
 * Create Agent button (→ /deploy/kagent-agent). The deployed agent lands in the
 * single Casbin-scoped Agents list (no Built-in/Installed hub).
 *
 * These tests prove the deploy/allowed/denied/removed UX; the Casbin
 * enforcement itself is proven by the Go middleware tests
 * (deployment_validator_test.go). Everything is route-mocked — the QA cluster
 * has no kagent installed (49.x strategy).
 */

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'
import type { Page, Route } from '@playwright/test'
import { selectShadcnOption } from './fixtures/select'

// ── Mock data ────────────────────────────────────────────────────────────────

// 53.3 removed the catalog annotation — the RGD is fetched by its known name
// for the Deploy drawer, never listed in the Catalog.
const kagentAgentRGD = {
  name: 'kagent-agent',
  namespace: 'default',
  description: 'Deploy a kagent AI agent into your project namespace',
  tags: ['ai', 'agent', 'kagent'],
  category: 'ai-agents',
  labels: {},
  instances: 0,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGraphDefinition',
  status: 'Active',
  producesKinds: [{ group: 'kagent.dev', version: 'v1alpha2', kind: 'Agent' }],
  createdAt: '2026-06-01T10:00:00Z',
  updatedAt: '2026-06-01T10:00:00Z',
}

const kagentAgentSchema = {
  crdFound: true,
  schema: {
    group: 'kro.run',
    version: 'v1alpha1',
    kind: 'KagentAgent',
    description: 'Deploy a kagent AI agent into your project namespace',
    properties: {
      agentName: { type: 'string', description: 'Name of the agent' },
      description: { type: 'string', description: 'What the agent does' },
      systemMessage: {
        type: 'string',
        description: 'System prompt for the agent',
      },
      modelConfig: {
        type: 'string',
        description: 'kagent ModelConfig to use',
        default: 'default-model-config',
      },
    },
    propertyOrder: ['agentName', 'description', 'systemMessage', 'modelConfig'],
    required: ['agentName'],
  },
}

const deployedInstance = {
  name: 'my-agent',
  namespace: 'alpha-apps',
  rgdName: 'kagent-agent',
  rgdNamespace: 'default',
  apiVersion: 'kro.run/v1alpha1',
  kind: 'KagentAgent',
  health: 'Healthy',
  conditions: [],
  spec: {
    agentName: 'my-agent',
    description: 'Helps with alpha things',
    systemMessage: 'You are a helpful agent',
    modelConfig: 'default-model-config',
  },
  status: { state: 'ACTIVE' },
  createdAt: '2026-06-06T10:00:00Z',
  updatedAt: '2026-06-06T10:00:00Z',
}

const installedAgentPayload = {
  name: 'my-agent',
  namespace: 'alpha-apps',
  description: 'Helps with alpha things',
  createdAt: '2026-06-06T10:00:00Z',
}

const permissionDenied403 = {
  code: 'PERMISSION_DENIED',
  message: "You don't have permission to deploy instances",
  details: "Required permission: 'create' on instances for project 'alpha'",
}

// ── Mock harness ─────────────────────────────────────────────────────────────

async function mockAgentsStatus(page: Page) {
  await page.route('**/api/v1/agents/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        status: 'ready',
        crdPresent: true,
        controllerHealthy: true,
        message: 'kagent is installed and healthy',
      }),
    })
  })
}

// GET /api/v1/agents — the single Casbin-scoped list (no hub bucket).
async function mockAgents(page: Page, agents: Array<typeof installedAgentPayload>) {
  await page.route('**/api/v1/agents', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ agents }),
    })
  })
}

// The Agents tab also fires the running-runs + sessions queries — keep idle.
async function mockAgentsTabSideQueries(page: Page) {
  await page.route('**/api/v1/agents/runs**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], total: 0, page: 1, pageSize: 100 }),
    })
  })
  await page.route('**/api/v1/agents/sessions**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], total: 0, page: 1, pageSize: 20 }),
    })
  })
}

async function setupByoaMocks(page: Page) {
  // Session restore
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

  // RGD detail/schema/validate fetched by NAME for the Deploy drawer (the RGD
  // is no longer in the catalog list).
  await page.route('**/api/v1/rgds**', async (route) => {
    const url = route.request().url()

    if (url.includes('/kagent-agent')) {
      if (url.includes('/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(kagentAgentSchema),
        })
      } else if (url.includes('/validate-deployment')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ valid: true, errors: [] }),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(kagentAgentRGD),
        })
      }
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], totalCount: 0, page: 1, pageSize: 10 }),
      })
    }
  })

  await page.route('**/api/v1/dependencies/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        node: null,
        upstream: [],
        downstream: [],
        deploymentOrder: ['kagent-agent'],
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
        body: JSON.stringify({ namespaces: ['alpha-apps'] }),
      })
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [{ name: 'alpha', destinations: [{ namespace: 'alpha-apps' }] }],
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

  // Compliance validate + preflight dry-run (called advancing to Review)
  await page.route('**/api/v1/compliance/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ result: 'pass', violations: [] }),
    })
  })
  await page.route('**/instances/**/preflight', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ valid: true }),
    })
  })

  // Global instances list (post-delete navigation target)
  await page.route('**/api/v1/instances**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], totalCount: 0, page: 1, pageSize: 20 }),
    })
  })

  await mockAgentsStatus(page)
  await mockAgentsTabSideQueries(page)
  await setupPermissionMocking(page, { '*:*': true })
}

/**
 * Mocks the GVK-aware instance routes for KagentAgent:
 *   POST   .../apigroups/kro.run/namespaces/{ns}/instances/KagentAgent          → create
 *   GET    .../apigroups/kro.run/namespaces/{ns}/instances/KagentAgent/{name}   → detail
 *   GET    .../events                                                           → empty
 *   DELETE .../apigroups/kro.run/namespaces/{ns}/instances/KagentAgent/{name}   → 200
 */
async function mockKagentInstanceRoutes(
  page: Page,
  opts: { createStatus?: number } = {}
) {
  const createStatus = opts.createStatus ?? 201

  await page.route('**/api/v1/apigroups/**', async (route: Route) => {
    const url = route.request().url()
    const method = route.request().method()

    if (url.endsWith('/preflight')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ valid: true }),
      })
      return
    }

    if (method === 'POST') {
      if (createStatus === 201) {
        const body = JSON.parse(route.request().postData() || '{}')
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            name: body.name || 'my-agent',
            namespace: body.namespace || 'alpha-apps',
            rgdName: 'kagent-agent',
            apiGroup: 'kro.run',
            kind: 'KagentAgent',
            version: 'v1alpha1',
            status: 'created',
            createdAt: new Date().toISOString(),
          }),
        })
      } else {
        // The real DeploymentValidator 403 body shape (AC 3)
        await route.fulfill({
          status: createStatus,
          contentType: 'application/json',
          body: JSON.stringify(permissionDenied403),
        })
      }
      return
    }

    if (method === 'GET' && url.includes('/events')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ events: [] }),
      })
      return
    }

    if (method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(deployedInstance),
      })
      return
    }

    if (method === 'DELETE') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({}),
      })
      return
    }

    await route.continue()
  })
}

/** Deploy drawer bound to the kagent-agent RGD → fill the General tab. */
async function navigateToKagentDeployForm(page: Page) {
  await mockAgents(page, [])

  // The agent-wrapping RGD deploys via the Catalog's Deploy route (the Agents
  // page no longer surfaces a Create Agent shortcut).
  await page.goto('/deploy/kagent-agent')
  await page.waitForURL(/\/deploy\/kagent-agent/, { timeout: 10000 })

  await expect(page.getByTestId('deploy-tab-general')).toBeVisible({
    timeout: 15000,
  })
  await page.getByTestId('instance-name-input').fill('my-agent')

  const nsSelect = page.getByTestId('namespace-select')
  await expect(nsSelect).toBeEnabled({ timeout: 5000 })
  await selectShadcnOption(nsSelect, 'alpha-apps')

  // Schema-driven scalar fields on the General tab (generic deploy form).
  await page.getByTestId('input-agentName').fill('my-agent')
  await page.getByTestId('input-description').fill('Helps with alpha things')
  await page.getByTestId('input-systemMessage').fill('You are a helpful agent')
}

/** Advance to Review and submit. */
async function submitDeploy(page: Page) {
  const reviewBtn = page.getByTestId('deploy-footer-review')
  await expect(reviewBtn).toBeEnabled({ timeout: 10000 })
  await reviewBtn.click()

  const deploySubmit = page.getByTestId('deploy-footer-deploy')
  await expect(deploySubmit).toBeEnabled({ timeout: 10000 })
  await deploySubmit.click()
}

// ── Tests ────────────────────────────────────────────────────────────────────

test.describe('BYOA Agent Deployment (Stories 49.3 / 53.3)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    await setupByoaMocks(page)
  })

  test('deploying via Create Agent lands the agent in the unified Agents list (AC 2)', async ({
    page,
  }) => {
    await mockKagentInstanceRoutes(page)

    await navigateToKagentDeployForm(page)
    await submitDeploy(page)

    // Success toast + redirect to the instance detail route — the EXISTING
    // instance deploy flow, no agent-specific path.
    await expect(
      page.getByText('"my-agent" deployed successfully')
    ).toBeVisible({ timeout: 10000 })
    await expect(page).toHaveURL(
      /\/instances\/kro\.run\/v1alpha1\/alpha-apps\/KagentAgent\/my-agent/,
      { timeout: 10000 }
    )

    // The agent now shows in the single Casbin-scoped Agents list — no hub.
    await mockAgents(page, [installedAgentPayload])
    await page.goto('/agents/list')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByTestId('agents-list')).toBeVisible()
    await expect(page.getByTestId('agents-hub')).toHaveCount(0)
    // Default list view: the agent shows as a row (name + namespace).
    const agentRow = page.getByTestId('agent-row').filter({ hasText: 'my-agent' })
    await expect(agentRow).toHaveCount(1)
    await expect(agentRow).toContainText('alpha-apps')
  })

  test('unauthorized deploy is blocked with a clear authorization error (AC 3)', async ({
    page,
  }) => {
    await mockKagentInstanceRoutes(page, { createStatus: 403 })

    await navigateToKagentDeployForm(page)
    await submitDeploy(page)

    // The server's 403 PERMISSION_DENIED message surfaces as an error toast …
    await expect(
      page.getByText("You don't have permission to deploy instances")
    ).toBeVisible({ timeout: 10000 })

    // … and no success navigation happens — we stay on the deploy form.
    await expect(page).toHaveURL(/\/deploy\//)
    await expect(
      page.getByText('"my-agent" deployed successfully')
    ).not.toBeVisible()
  })

  test('deleting the agent instance removes it from the Agents list (AC 4)', async ({
    page,
  }) => {
    await mockKagentInstanceRoutes(page)
    // After deletion + KRO garbage collection the live LIST returns nothing.
    await mockAgents(page, [])

    // Instance detail for the deployed agent (existing instance delete flow).
    await page.goto(
      '/instances/kro.run/v1alpha1/alpha-apps/KagentAgent/my-agent'
    )
    await page.waitForLoadState('networkidle')

    const deleteButton = page.getByRole('button', { name: 'Delete', exact: true })
    await expect(deleteButton).toBeVisible({ timeout: 15000 })
    await deleteButton.click()

    // Type-to-confirm dialog (DeleteInstanceDialog).
    const confirmInput = page.getByTestId('confirm-name-input')
    await expect(confirmInput).toBeVisible()
    await confirmInput.fill('my-agent')
    await page.getByTestId('confirm-delete-button').click()

    // Deletion succeeded via the existing flow → back to the instances list.
    await expect(page.getByText('"my-agent" deleted')).toBeVisible({
      timeout: 10000,
    })
    await expect(page).toHaveURL(/\/instances$/, { timeout: 10000 })

    // The Agents list no longer shows the agent — empty state.
    await page.goto('/agents/list')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByText('No agents available')).toBeVisible()
    await expect(page.getByTestId('agent-card')).toHaveCount(0)
  })
})
