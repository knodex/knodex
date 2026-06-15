// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Agents → Templates E2E Tests
 *
 * Covers the Templates tab: published agent-template RGDs (catalog
 * annotation, routed here by schema.kind == KnodexAgentTemplate) via
 * GET /api/v1/agents/templates, rendered list/empty/error states, and the
 * per-row Deploy hand-off to the standard /deploy/{name} flow.
 *
 * Everything is route-mocked — the QA cluster has no kagent installed (the
 * 49.x strategy), so the live templates response would always be empty.
 * Server-side discovery-by-Kind and Casbin visibility are proven by the Go
 * tests, not here.
 */

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'
import type { Page } from '@playwright/test'

type AgentsStatusPayload = {
  status: 'ready' | 'not_installed' | 'degraded'
  crdPresent: boolean | null
  controllerHealthy: boolean | null
  message: string
}

// A subset of CatalogRGD — only the fields the Templates page reads, plus the
// envelope-required shape returned by GET /api/v1/agents/templates.
type TemplatePayload = {
  name: string
  title?: string
  namespace: string
  description: string
  tags: string[]
  category: string
  labels: Record<string, string>
  instances: number
  kind: string
  createdAt: string
  updatedAt: string
}

const readyStatusPayload: AgentsStatusPayload = {
  status: 'ready',
  crdPresent: true,
  controllerHealthy: true,
  message: 'kagent is installed and healthy',
}

function template(partial: Partial<TemplatePayload>): TemplatePayload {
  return {
    name: 'kagent-rgd-builder-agent',
    namespace: '',
    description: 'Deploy the Knodex RGD Builder agent',
    tags: ['ai', 'agent'],
    category: 'ai-agents',
    labels: {},
    instances: 0,
    kind: 'KnodexAgentTemplate',
    createdAt: '2026-06-10T00:00:00Z',
    updatedAt: '2026-06-10T00:00:00Z',
    ...partial,
  }
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

// The Agents list tab (the entry point for the sub-nav reachability test) fires
// the unified list + the live-runs/sessions side queries. Keep them empty so
// the page settles without touching a real backend.
async function mockAgentsListTab(page: Page) {
  await page.route('**/api/v1/agents', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ agents: [] }),
    })
  })
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

// GET /api/v1/agents/templates — the discovered-by-Kind RGD list. `status`
// lets a test exercise the error branch; otherwise a 200 envelope is served.
async function mockTemplates(
  page: Page,
  items: TemplatePayload[],
  opts: { status?: number } = {}
) {
  await page.route('**/api/v1/agents/templates', async (route) => {
    if (opts.status && opts.status >= 400) {
      await route.fulfill({
        status: opts.status,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'INTERNAL', message: 'boom' } }),
      })
      return
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items,
        totalCount: items.length,
        page: 1,
        pageSize: 100,
      }),
    })
  })
}

test.describe('Agents → Templates', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    await mockAccountInfo(page)
    await setupPermissionMocking(page, { '*:*': true })
    await mockAgentsStatus(page, readyStatusPayload)
  })

  test('lists one row per discovered template with name, description and instance count', async ({
    page,
  }) => {
    await mockTemplates(page, [
      template({ name: 'kagent-rgd-builder-agent', title: 'RGD Builder', instances: 2 }),
      template({
        name: 'custom-template',
        title: 'Custom Agent',
        description: 'A second agent template',
        instances: 0,
      }),
    ])

    await page.goto('/agents/templates')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByTestId('agents-templates-table')).toBeVisible()
    const rows = page.getByTestId('agents-templates-row')
    await expect(rows).toHaveCount(2)

    const builder = rows.filter({ hasText: 'RGD Builder' })
    await expect(builder).toBeVisible()
    await expect(builder).toContainText('Deploy the Knodex RGD Builder agent')
    await expect(builder).toContainText('2')

    await expect(rows.filter({ hasText: 'Custom Agent' })).toBeVisible()
  })

  test('shows an empty state explaining discovery-by-Kind when no templates exist', async ({
    page,
  }) => {
    await mockTemplates(page, [])

    await page.goto('/agents/templates')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByText('No agent templates')).toBeVisible()
    await expect(page.getByText(/kind KnodexAgentTemplate/i)).toBeVisible()
    await expect(page.getByTestId('agents-templates-table')).not.toBeVisible()
  })

  test('Deploy hands off to the standard /deploy/{name} flow', async ({ page }) => {
    await mockTemplates(page, [
      template({ name: 'kagent-rgd-builder-agent', title: 'RGD Builder' }),
    ])

    await page.goto('/agents/templates')
    await page.waitForLoadState('domcontentloaded')

    await page.getByTestId('deploy-template-button').click()

    await expect(page).toHaveURL(/\/deploy\/kagent-rgd-builder-agent/)
  })

  test('renders a retryable error when the templates fetch fails', async ({ page }) => {
    await mockTemplates(page, [], { status: 500 })

    await page.goto('/agents/templates')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByTestId('agents-templates-error')).toBeVisible()
    await expect(page.getByRole('button', { name: /retry/i })).toBeVisible()
  })

  test('the Templates tab is reachable from the Agents sub-navigation', async ({ page }) => {
    await mockTemplates(page, [template({ title: 'RGD Builder' })])
    await mockAgentsListTab(page)

    await page.goto('/agents/list')
    await page.waitForLoadState('domcontentloaded')

    // Sidebar sub-nav exposes a Templates link → lands on the templates page.
    await page.getByRole('link', { name: 'Templates' }).click()
    await expect(page).toHaveURL(/\/agents\/templates/)
    await expect(page.getByTestId('agents-templates-page')).toBeVisible()
  })
})
