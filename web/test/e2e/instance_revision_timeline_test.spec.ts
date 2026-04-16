// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture'
import {
  mockRGDs,
  mockRGDListResponse,
  mockInstances,
  API_PATHS,
} from '../fixture/mock-data'
import type { Instance } from '../../src/types/rgd'
import type { Project, ProjectListResponse } from '../../src/types/project'

/**
 * Revision Timeline Markers — E2E Tests (STORY-402)
 *
 * Tests query-time merge of GraphRevision markers into the Deployment History tab:
 * - RevisionChanged markers appear in the timeline with GitBranch icon label
 * - First revision shows "(initial)" label; subsequent show "N → N+1" format
 * - Markers are interleaved with deployment events in timestamp order
 * - When no revisions exist the timeline shows only deployment events
 * - Timeline still renders correctly when history API returns 404 (deleted instance)
 */

const testRGD = {
  ...mockRGDs[0],
  name: 'postgres-database',
  lastIssuedRevision: 3,
}

const testInstance = {
  ...mockInstances[0],
  name: 'my-database',
  namespace: 'team-alpha',
  rgdName: 'postgres-database',
  rgdNamespace: 'databases',
  kind: 'PostgresDatabase',
  health: 'Healthy',
  deploymentMode: 'direct',
  conditions: [{ type: 'Ready', status: 'True', reason: 'AllReady', message: 'All resources ready' }],
  spec: { replicas: 1 },
  status: { state: 'ACTIVE' },
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
}

const testProjects: Project[] = [
  {
    name: 'alpha-team',
    type: 'app',
    description: 'Alpha team',
    destinations: [{ namespace: 'team-alpha' }],
    roles: [],
    resourceVersion: '1',
    createdAt: '2026-01-01T00:00:00Z',
    createdBy: 'admin',
  },
]

const testProjectList: ProjectListResponse = {
  items: testProjects,
  totalCount: testProjects.length,
}

// Timeline with a Created event + two RevisionChanged markers
const timelineWithRevisions = {
  namespace: testInstance.namespace,
  kind: testInstance.kind,
  name: testInstance.name,
  timeline: [
    {
      timestamp: '2026-01-01T00:00:00Z',
      eventType: 'Created',
      status: 'Pending',
      user: 'admin@test.local',
      message: 'Instance created',
      isCompleted: true,
      isCurrent: false,
    },
    {
      timestamp: '2026-01-01T01:00:00Z',
      eventType: 'RevisionChanged',
      status: '',
      user: 'system',
      message: 'RGD Revision 1 (initial)',
      isCompleted: true,
      isCurrent: false,
      revisionNumber: 1,
      previousRevision: 0,
    },
    {
      timestamp: '2026-01-01T02:00:00Z',
      eventType: 'RevisionChanged',
      status: '',
      user: 'system',
      message: 'RGD Revision 1 → 2',
      isCompleted: true,
      isCurrent: false,
      revisionNumber: 2,
      previousRevision: 1,
    },
    {
      timestamp: '2026-01-01T03:00:00Z',
      eventType: 'Ready',
      status: 'Ready',
      isCompleted: true,
      isCurrent: true,
    },
  ],
}

// Timeline with no revision markers (feature disabled / nil provider)
const timelineWithoutRevisions = {
  namespace: testInstance.namespace,
  kind: testInstance.kind,
  name: testInstance.name,
  timeline: [
    {
      timestamp: '2026-01-01T00:00:00Z',
      eventType: 'Created',
      status: 'Pending',
      user: 'admin@test.local',
      isCompleted: true,
      isCurrent: false,
    },
    {
      timestamp: '2026-01-01T03:00:00Z',
      eventType: 'Ready',
      status: 'Ready',
      isCompleted: true,
      isCurrent: true,
    },
  ],
}

const instanceUrl = `/instances/${testInstance.namespace}/${testInstance.kind}/${testInstance.name}`

async function setupBaseMocks(
  page: import('@playwright/test').Page,
  opts: { timeline?: object; timelineStatus?: number } = {},
) {
  const { timeline = timelineWithRevisions, timelineStatus = 200 } = opts

  return Promise.all([
    // Mock account/info so session restore succeeds (prevents "Connection Error" state)
    page.route('**/api/v1/account/info', async (route) => {
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
    }),
    page.route(/\/api\/v1\/rgds/, async (route) => {
      const url = route.request().url()
      if (url.includes('/postgres-database')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(testRGD) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockRGDListResponse) })
      }
    }),
    page.route(`**${API_PATHS.instances}**`, async (route) => {
      const url = route.request().url()
      if (url.includes(`/${testInstance.name}`)) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(testInstance) })
      } else if (url.includes('/count')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ count: 1 }) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [testInstance], totalCount: 1, page: 1, pageSize: 10 }) })
      }
    }),
    page.route(`**${API_PATHS.namespacedInstances}**`, async (route) => {
      const url = route.request().url()
      if (url.includes(`/${testInstance.name}`)) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(testInstance) })
      } else if (url.includes('/count')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ count: 1 }) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [testInstance], totalCount: 1, page: 1, pageSize: 10 }) })
      }
    }),
    page.route(`**${API_PATHS.projects}**`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(testProjectList) })
    }),
    page.route(`**${API_PATHS.canI}**`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ value: 'yes' }) })
    }),
    page.route('**/instances/**/timeline', async (route) => {
      await route.fulfill({ status: timelineStatus, contentType: 'application/json', body: JSON.stringify(timeline) })
    }),
    page.route('**/history**', async (route) => {
      if (!route.request().url().includes('/timeline')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ events: [] }) })
      }
    }),
  ])
}

test.describe('Revision Timeline Markers', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('shows RevisionChanged markers with "Revision Changed" label in the deployment timeline', async ({ page }) => {
    await setupBaseMocks(page)
    await page.goto(instanceUrl)

    // Click Deployment History tab to show timeline content
    const deploymentHistoryTab = page.getByRole('tab', { name: /Deployment History/i })
    await expect(deploymentHistoryTab).toBeVisible({ timeout: 10000 })
    await deploymentHistoryTab.click()

    // Two RevisionChanged entries should appear
    const revisionLabels = page.getByText('Revision Changed')
    await expect(revisionLabels.first()).toBeVisible()
    const count = await revisionLabels.count()
    expect(count).toBe(2)
  })

  test('shows correct revision messages: initial and N → N+1 format', async ({ page }) => {
    await setupBaseMocks(page)
    await page.goto(instanceUrl)

    const dhTab = page.getByRole('tab', { name: /Deployment History/i })
    await expect(dhTab).toBeVisible({ timeout: 10000 })
    await dhTab.click()

    // RevisionChanged messages are shown in the detail card when the node is selected.
    // Click the first "Revision Changed" timeline node to select it.
    const revisionNodes = page.getByRole('button', { name: /Revision Changed/i })
    await expect(revisionNodes.first()).toBeVisible()
    await revisionNodes.first().click()
    await expect(page.getByText('RGD Revision 1 (initial)')).toBeVisible()

    // Click the second revision node
    await revisionNodes.nth(1).click()
    await expect(page.getByText('RGD Revision 1 → 2')).toBeVisible()
  })

  test('shows deployment events alongside revision markers in the same timeline', async ({ page }) => {
    await setupBaseMocks(page)
    await page.goto(instanceUrl)

    const dhTab = page.getByRole('tab', { name: /Deployment History/i })
    await expect(dhTab).toBeVisible({ timeout: 10000 })
    await dhTab.click()

    // Deployment events — use .first() to handle multiple occurrences in DOM
    await expect(page.getByText('Created').first()).toBeVisible()
    await expect(page.getByText('Ready').first()).toBeVisible()

    // Revision markers
    await expect(page.getByText('Revision Changed').first()).toBeVisible()

    // Event count badge: 4 events total (1 Created + 2 RevisionChanged + 1 Ready)
    await expect(page.getByText('4 events')).toBeVisible()
  })

  test('timeline shows only deployment events when no revision markers present', async ({ page }) => {
    await setupBaseMocks(page, { timeline: timelineWithoutRevisions })
    await page.goto(instanceUrl)

    const dhTab = page.getByRole('tab', { name: /Deployment History/i })
    await expect(dhTab).toBeVisible({ timeout: 10000 })
    await dhTab.click()

    await expect(page.getByText('Created').first()).toBeVisible()
    await expect(page.getByText('Ready').first()).toBeVisible()
    await expect(page.getByText('Revision Changed')).not.toBeVisible()
    await expect(page.getByText('2 events')).toBeVisible()
  })

  test('timeline renders correctly when history returns 404 (deleted instance)', async ({ page }) => {
    await setupBaseMocks(page, { timeline: { code: 'NOT_FOUND', message: 'Timeline not found' }, timelineStatus: 404 })
    await page.goto(instanceUrl)

    const dhTab = page.getByRole('tab', { name: /Deployment History/i })
    await expect(dhTab).toBeVisible({ timeout: 10000 })
    await dhTab.click()

    // Wait for loading spinner to disappear (timeline fetch completes)
    await expect(page.locator('.animate-spin').first()).not.toBeVisible({ timeout: 10000 })

    // Should show error state or empty — not crash
    const failedText = page.getByText('Failed to load deployment history')
    const emptyText = page.getByText('No deployment history available')
    const isFailedOrEmpty = (await failedText.isVisible()) || (await emptyText.isVisible())
    expect(isFailedOrEmpty).toBe(true)
  })
})
