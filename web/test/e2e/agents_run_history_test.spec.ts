// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Agent Run/Conversation History & Live Status E2E Tests (Stories 49.4 / 53.2)
 *
 * Re-pointed for the 53.x workspace collapse: the old `/agents` hub run-history
 * table is gone. Conversation history now lives under the Agents tab
 * (`/agents/list`) as PastConversationsSection, and the live in-flight
 * indicator rides on the agent cards. The run-history TABLE's columns, filters
 * and pagination are exercised directly by RunHistorySection.test.tsx (unit) —
 * not duplicated here. All agents endpoints are route-mocked (49.x strategy:
 * the QA cluster has no kagent installed).
 */

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'
import type { Page } from '@playwright/test'

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

type SessionSummaryPayload = {
  id: string
  agentType: string
  agentNamespace: string
  firstPrompt: string
  startedAt: string
  lastActivityAt: string
  runCount: number
  status: 'running' | 'completed' | 'failed'
}

type AgentRunPayload = {
  id: string
  actor: string
  agentType: string
  agentNamespace: string
  contextRef: string
  kagentSessionId: string
  inputSummary: string
  recommendationSummary: string
  actionTaken: string
  timestamp: string
  completedAt?: string
  status: 'running' | 'completed' | 'failed'
  triggerType: string
}

const readyStatusPayload: AgentsStatusPayload = {
  status: 'ready',
  crdPresent: true,
  controllerHealthy: true,
  message: 'kagent is installed and healthy',
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

async function mockAgents(page: Page, agents: AgentPayload[]) {
  await page.route('**/api/v1/agents', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ agents }),
    })
  })
}

async function mockSessions(page: Page, items: SessionSummaryPayload[]) {
  await page.route('**/api/v1/agents/sessions**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items, total: items.length, page: 1, pageSize: 20 }),
    })
  })
}

/**
 * Mocks GET /api/v1/agents/runs honoring the status filter (the card
 * indicator polls status=running). Accepts a thunk for live-convergence.
 */
async function mockAgentRuns(
  page: Page,
  runsSource: AgentRunPayload[] | (() => AgentRunPayload[])
) {
  const currentRuns = typeof runsSource === 'function' ? runsSource : () => runsSource
  await page.route('**/api/v1/agents/runs**', async (route) => {
    const runs = currentRuns()
    const url = new URL(route.request().url())
    const status = url.searchParams.get('status') ?? ''
    const filtered = runs.filter((r) => !status || r.status === status)
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: filtered,
        total: filtered.length,
        page: 1,
        pageSize: 100,
      }),
    })
  })
}

function makeRun(overrides: Partial<AgentRunPayload> & { id: string }): AgentRunPayload {
  return {
    actor: 'admin@e2e-test.local',
    agentType: 'alpha-helper',
    agentNamespace: 'alpha-apps',
    contextRef: '',
    kagentSessionId: '',
    inputSummary: 'do the thing',
    recommendationSummary: '',
    actionTaken: '',
    timestamp: '2026-06-06T10:00:00Z',
    status: 'completed',
    triggerType: 'on_demand',
    ...overrides,
  }
}

test.describe('Agent Conversation History (Stories 49.4 / 53.2)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
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
    await setupPermissionMocking(page, { '*:*': true })
    await mockAgentsStatus(page, readyStatusPayload)
  })

  test('past conversations render under the Agents tab', async ({ page }) => {
    await mockAgents(page, [])
    await mockAgentRuns(page, [])
    await mockSessions(page, [
      {
        id: 'sess-1',
        agentType: 'alpha-helper',
        agentNamespace: 'alpha-apps',
        firstPrompt: 'scale the webapp',
        startedAt: '2026-06-06T09:00:00Z',
        lastActivityAt: '2026-06-06T10:00:00Z',
        runCount: 3,
        status: 'completed',
      },
    ])

    await page.goto('/agents/list')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByTestId('past-conversations-section')).toBeVisible()
    const rows = page.getByTestId('past-conversation-row')
    await expect(rows).toHaveCount(1)
    await expect(rows.first()).toContainText('scale the webapp')
  })

  test('empty conversation history shows the empty state, not an error', async ({
    page,
  }) => {
    await mockAgents(page, [])
    await mockAgentRuns(page, [])
    await mockSessions(page, [])

    await page.goto('/agents/list')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByTestId('past-conversations-empty')).toBeVisible()
    await expect(page.getByText('No conversations yet')).toBeVisible()
    await expect(page.getByTestId('past-conversations-error')).not.toBeVisible()
  })

  test('a running run shows the live indicator on ONLY the matching agent card (UX-DR6)', async ({
    page,
  }) => {
    await mockSessions(page, [])
    await mockAgents(page, [
      {
        name: 'alpha-helper',
        namespace: 'alpha-apps',
        description: 'Helps with alpha things',
        createdAt: '2026-06-01T10:00:00Z',
      },
      {
        name: 'beta-helper',
        namespace: 'beta-apps',
        description: 'Helps with beta things',
        createdAt: '2026-06-02T10:00:00Z',
      },
    ])
    await mockAgentRuns(page, [
      makeRun({
        id: 'run-live',
        agentType: 'alpha-helper',
        agentNamespace: 'alpha-apps',
        status: 'running',
      }),
    ])

    await page.goto('/agents/list')
    await page.waitForLoadState('domcontentloaded')

    const indicators = page.getByTestId('agent-running-indicator')
    await expect(indicators).toHaveCount(1)
    // Default list view: one row per agent; only alpha's row is in-flight.
    const alphaRow = page.getByTestId('agent-row').filter({ hasText: 'alpha-helper' })
    await expect(alphaRow.getByTestId('agent-running-indicator')).toBeVisible()
    const betaRow = page.getByTestId('agent-row').filter({ hasText: 'beta-helper' })
    await expect(betaRow.getByTestId('agent-running-indicator')).toHaveCount(0)
  })

  test('an in-flight run converges to idle WITHOUT a page refresh (UX-DR6)', async ({
    page,
  }) => {
    await mockSessions(page, [])
    await mockAgents(page, [
      {
        name: 'alpha-helper',
        namespace: 'alpha-apps',
        description: 'Helps with alpha things',
        createdAt: '2026-06-01T10:00:00Z',
      },
    ])

    // Mutable backing state: the run starts in-flight; flipping `completed`
    // empties the running-runs page so the card indicator clears via the
    // conditional 5s poll — no reload/navigation (UX-DR6).
    let completed = false
    await mockAgentRuns(page, () =>
      completed
        ? []
        : [
            makeRun({
              id: 'run-live',
              agentType: 'alpha-helper',
              agentNamespace: 'alpha-apps',
              status: 'running',
            }),
          ]
    )

    await page.goto('/agents/list')
    await page.waitForLoadState('domcontentloaded')

    await expect(page.getByTestId('agent-running-indicator')).toHaveCount(1)

    // The run finishes server-side; the poll fires within 5s.
    completed = true
    await expect(page.getByTestId('agent-running-indicator')).toHaveCount(0, {
      timeout: 15_000,
    })
  })
})
