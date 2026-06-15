// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Agents Workspace E2E Tests (Story 53.6)
 *
 * Covers the unified Casbin-scoped Agents workspace that replaced the old
 * Built-in/Installed hub (Stories 53.1–53.5): the Overview tab's onboarding
 * and ready states, the single unified agents list, the Create Agent → Deploy
 * drawer hand-off, and the Create Model dialog → POST → list-refresh flow.
 *
 * Everything is route-mocked — the QA cluster has no kagent installed (the
 * 49.x strategy), so the live status/list responses would always be
 * not_installed/empty. Server-side Casbin filtering and the invoke lifecycle
 * are proven by the Go tests, not here.
 */

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'
import type { Page } from '@playwright/test'
import { selectShadcnOption } from './fixtures/select'

type AgentsStatusPayload = {
  status: 'ready' | 'not_installed' | 'degraded'
  crdPresent: boolean | null
  controllerHealthy: boolean | null
  message: string
}

type AgentPayload = {
  name: string
  namespace: string
  description: string
  createdAt: string
}

type ModelPayload = {
  name: string
  namespace: string
  provider: string
  model: string
}

const readyStatusPayload: AgentsStatusPayload = {
  status: 'ready',
  crdPresent: true,
  controllerHealthy: true,
  message: 'kagent is installed and healthy',
}

async function mockAccountInfo(page: Page) {
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
}

async function mockAgentsStatus(page: Page, payload: AgentsStatusPayload) {
  await page.route('**/api/v1/agents/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(payload),
    })
  })
}

// GET /api/v1/agents — the single Casbin-scoped list (no hub/installed split).
async function mockAgents(page: Page, agents: AgentPayload[]) {
  await page.route('**/api/v1/agents', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ agents }),
    })
  })
}

// GET /api/v1/agents/models — accepts a thunk so the Create Model flow can flip
// the list after the POST.
async function mockModels(
  page: Page,
  modelsSource: ModelPayload[] | (() => ModelPayload[])
) {
  const current = typeof modelsSource === 'function' ? modelsSource : () => modelsSource
  await page.route('**/api/v1/agents/models', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ models: current() }),
      })
      return
    }
    await route.continue()
  })
}

// The Agents tab also fires the live-running runs query + the sessions list;
// keep them empty so those sub-views render their empty/idle states.
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

test.describe('Agents Workspace (Story 53.6)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    await mockAccountInfo(page)
    await setupPermissionMocking(page, { '*:*': true })
  })

  // ── Overview onboarding ─────────────────────────────────────────────────────

  test('Overview not_installed renders the install snippet + docs link', async ({
    page,
  }) => {
    await mockAgentsStatus(page, {
      status: 'not_installed',
      crdPresent: false,
      controllerHealthy: null,
      message: 'kagent Agent CRD (agents.kagent.dev) not found in cluster',
    })

    await page.goto('/agents')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByTestId('agents-onboarding')).toBeVisible()
    await expect(page.getByText(/helm install kagent-crds/)).toBeVisible()
    await expect(
      page.getByRole('link', { name: /view kagent docs/i })
    ).toHaveAttribute('href', 'https://kagent.dev/docs')

    // The ready-state counts/quickstart must NOT fire on a kagent-less cluster.
    await expect(page.getByTestId('agents-overview-ready')).not.toBeVisible()
  })

  test('Overview ready renders counts + both quickstart links (Model before Agent)', async ({
    page,
  }) => {
    await mockAgentsStatus(page, readyStatusPayload)
    await mockAgents(page, [
      { name: 'alpha-helper', namespace: 'alpha-apps', description: '', createdAt: '' },
      { name: 'beta-helper', namespace: 'beta-apps', description: '', createdAt: '' },
    ])
    await mockModels(page, [
      { name: 'gpt', namespace: 'alpha-apps', provider: 'openai', model: 'gpt-4o' },
    ])

    await page.goto('/agents')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByTestId('agents-overview-ready')).toBeVisible()
    await expect(page.getByTestId('agents-overview-agent-count')).toContainText('2')
    await expect(page.getByTestId('agents-overview-model-count')).toContainText('1')

    const modelLink = page.getByTestId('agents-overview-quickstart-model')
    const agentLink = page.getByTestId('agents-overview-quickstart-agent')
    await expect(modelLink).toHaveAttribute('href', '/agents/models')
    await expect(agentLink).toHaveAttribute('href', '/agents/templates')

    // The Model quickstart appears before the Agent one (first-run sequencing).
    const modelBox = await modelLink.boundingBox()
    const agentBox = await agentLink.boundingBox()
    expect(modelBox).not.toBeNull()
    expect(agentBox).not.toBeNull()
    expect(modelBox!.y).toBeLessThan(agentBox!.y)
  })

  // ── Unified list scoping ────────────────────────────────────────────────────

  test('a multi-namespace payload renders one unified list with no hub split', async ({
    page,
  }) => {
    await mockAgentsStatus(page, readyStatusPayload)
    await mockAgentsTabSideQueries(page)
    await mockAgents(page, [
      {
        name: 'alpha-helper',
        namespace: 'alpha-apps',
        description: 'Helps with alpha',
        createdAt: '2026-06-01T10:00:00Z',
      },
      {
        name: 'beta-helper',
        namespace: 'beta-apps',
        description: 'Helps with beta',
        createdAt: '2026-06-02T10:00:00Z',
      },
    ])

    await page.goto('/agents/list')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByTestId('agents-list')).toBeVisible()
    await expect(page.getByText('alpha-helper')).toBeVisible()
    await expect(page.getByText('beta-helper')).toBeVisible()

    // No hub/installed split survives the collapse.
    await expect(page.getByTestId('agents-hub')).toHaveCount(0)
    await expect(
      page.getByRole('heading', { name: 'Installed Agents', exact: true })
    ).toHaveCount(0)
  })

  test('an empty agents payload shows the empty state with a Catalog link (no Create Agent action)', async ({
    page,
  }) => {
    await mockAgentsStatus(page, readyStatusPayload)
    await mockAgentsTabSideQueries(page)
    await mockAgents(page, [])

    await page.goto('/agents/list')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByText('No agents available')).toBeVisible()
    // Agents arrive via the Catalog — the empty state points there, and the
    // former Create Agent action no longer exists on this page.
    await expect(
      page.getByRole('link', { name: /browse the full catalog/i })
    ).toBeVisible()
    await expect(page.getByTestId('create-agent-button-empty')).toHaveCount(0)
    await expect(page.getByTestId('create-agent-button')).toHaveCount(0)
  })

  // ── Create Model happy path ─────────────────────────────────────────────────

  test('Create Model submits a valid form and the new model appears in the list', async ({
    page,
  }) => {
    await mockAgentsStatus(page, readyStatusPayload)

    // Projects + namespaces for the dialog's cascading selects.
    await page.route('**/api/v1/projects', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [{ name: 'alpha', destinations: [{ namespace: 'alpha-apps' }] }],
          totalCount: 1,
        }),
      })
    })
    await page.route('**/api/v1/projects/alpha/namespaces', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ namespaces: ['alpha-apps'] }),
      })
    })

    // Single handler for both verbs: the POST flips `created`, and the
    // subsequent GET (the post-mutation refetch) then carries the new row.
    const newModel = {
      name: 'my-model',
      namespace: 'alpha-apps',
      provider: 'openai',
      model: 'gpt-4o',
    }
    let created = false
    await page.route('**/api/v1/agents/models', async (route) => {
      if (route.request().method() === 'POST') {
        created = true
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify(newModel),
        })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ models: created ? [newModel] : [] }),
      })
    })

    await page.goto('/agents/models')
    await page.waitForLoadState('domcontentloaded')

    // Zero models initially, so the empty-state owns the Create CTA — the
    // toolbar button only mounts once the list is non-empty.
    await page.getByTestId('create-model-button-empty').click()

    // Cascading selects, then the scalar fields (provider/model default to
    // openai/gpt-4o).
    await selectShadcnOption(page.locator('#model-project'), 'alpha')
    const nsTrigger = page.locator('#model-namespace')
    await expect(nsTrigger).toBeEnabled()
    await selectShadcnOption(nsTrigger, 'alpha-apps')
    await page.locator('#model-name').fill('my-model')
    await page.locator('#model-apikey').fill('sk-test-key')

    await page.getByRole('button', { name: 'Create', exact: true }).click()

    await expect(page.getByText('Model "my-model" created successfully')).toBeVisible({
      timeout: 10000,
    })

    // The dialog closed and the refetched list now carries the row.
    await expect(page.getByTestId('agents-models-table')).toBeVisible()
    const row = page.getByTestId('agents-models-row').filter({ hasText: 'my-model' })
    await expect(row).toBeVisible()
    await expect(row).toContainText('alpha-apps')
  })
})
