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
import type { GraphRevision, RevisionDiff } from '../../src/types/rgd'

/**
 * Revision Diff Drawer — E2E Tests (STORY-401)
 *
 * Tests the slide-over drawer that opens when clicking the "Rev N" badge
 * on the instance detail page:
 * - Badge renders and is clickable
 * - Drawer opens and shows structured diff between current and previous revision
 * - Initial revision (rev 1) shows info notice and full snapshot YAML
 * - "Open in Revision Explorer" link navigates to the correct URL
 * - Loading and error states are handled gracefully
 * - Drawer is hidden when RBAC denies RGD access
 */

// RGD with revision 3 (to test diff between rev 2 → 3)
const rgdWithRevision = {
  ...mockRGDs[0],
  name: 'postgres-database',
  lastIssuedRevision: 3,
}

// RGD with revision 1 (to test initial revision path)
const rgdWithRevisionOne = {
  ...mockRGDs[0],
  name: 'postgres-database',
  lastIssuedRevision: 1,
}

const testInstance: Instance = {
  ...mockInstances[0],
  name: 'my-database',
  namespace: 'team-alpha',
  rgdName: 'postgres-database',
  rgdNamespace: 'databases',
  kind: 'PostgresDatabase',
  health: 'Healthy',
  deploymentMode: 'direct',
  conditions: [{ type: 'Ready', status: 'True', reason: 'AllReady', message: 'All resources ready' }],
  spec: { replicas: 3, storage: '100Gi' },
  status: { state: 'ACTIVE' },
  createdAt: '2026-03-20T10:00:00Z',
  updatedAt: '2026-03-27T14:00:00Z',
}

const testProjects: Project[] = [
  {
    name: 'alpha-team',
    description: 'Alpha team deployments',
    destinations: [{ namespace: 'team-alpha' }],
    roles: [],
    resourceVersion: '1',
    createdAt: '2026-01-01T00:00:00Z',
    createdBy: 'admin',
  },
]
const testProjectListResponse: ProjectListResponse = {
  items: testProjects,
  totalCount: testProjects.length,
}

// Structured diff: rev 2 → rev 3
const mockDiff: RevisionDiff = {
  rgdName: 'postgres-database',
  rev1: 2,
  rev2: 3,
  added: [{ path: 'spec.resources[1].apiVersion', newValue: 'apps/v1' }],
  removed: [],
  modified: [{ path: 'spec.schema.spec.replicas.default', oldValue: 2, newValue: 3 }],
  identical: false,
}

// Revision snapshot for rev 3 (for YAML diff rendering)
const mockRevision3: GraphRevision = {
  revisionNumber: 3,
  rgdName: 'postgres-database',
  namespace: 'databases',
  conditions: [{ type: 'GraphVerified', status: 'True', reason: 'Verified' }],
  contentHash: 'abc123',
  createdAt: '2026-03-27T14:00:00Z',
  snapshot: {
    apiVersion: 'kro.run/v1alpha1',
    kind: 'ResourceGraphDefinition',
    metadata: { name: 'postgres-database' },
    spec: {
      schema: { spec: { replicas: { default: 3 } } },
      resources: [{ id: 'deploy', apiVersion: 'apps/v1' }],
    },
  },
}

// Revision snapshot for rev 2
const mockRevision2: GraphRevision = {
  revisionNumber: 2,
  rgdName: 'postgres-database',
  namespace: 'databases',
  conditions: [{ type: 'GraphVerified', status: 'True', reason: 'Verified' }],
  contentHash: 'def456',
  createdAt: '2026-03-20T10:00:00Z',
  snapshot: {
    apiVersion: 'kro.run/v1alpha1',
    kind: 'ResourceGraphDefinition',
    metadata: { name: 'postgres-database' },
    spec: {
      schema: { spec: { replicas: { default: 2 } } },
      resources: [],
    },
  },
}

// Revision snapshot for rev 1 (initial)
const mockRevision1: GraphRevision = {
  revisionNumber: 1,
  rgdName: 'postgres-database',
  namespace: 'databases',
  conditions: [{ type: 'GraphVerified', status: 'True', reason: 'Verified' }],
  contentHash: 'ghi789',
  createdAt: '2026-03-15T08:00:00Z',
  snapshot: {
    apiVersion: 'kro.run/v1alpha1',
    kind: 'ResourceGraphDefinition',
    metadata: { name: 'postgres-database' },
    spec: { schema: { spec: { replicas: { default: 1 } } }, resources: [] },
  },
}

test.describe('Revision Diff Drawer', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  function setupMocks(
    page: import('@playwright/test').Page,
    opts: {
      rgd?: typeof rgdWithRevision
      diffResponse?: RevisionDiff | null
      rev2?: GraphRevision
      rev3?: GraphRevision
      canReadRGD?: boolean
    } = {},
  ) {
    const {
      rgd = rgdWithRevision,
      diffResponse = mockDiff,
      rev2 = mockRevision2,
      rev3 = mockRevision3,
      canReadRGD = true,
    } = opts

    return Promise.all([
      page.route(/\/api\/v1\/rgds/, async (route) => {
        const url = route.request().url()
        if (url.match(/\/revisions\/\d+\/diff\/\d+/)) {
          if (diffResponse === null) {
            await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'internal error' }) })
          } else {
            await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(diffResponse) })
          }
        } else if (url.match(/\/revisions\/3/)) {
          await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(rev3) })
        } else if (url.match(/\/revisions\/2/)) {
          await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(rev2) })
        } else if (url.match(/\/revisions\/1/)) {
          await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockRevision1) })
        } else if (url.match(/\/revisions/)) {
          await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [rev3, rev2], totalCount: 2 }) })
        } else if (url.includes('/postgres-database')) {
          await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(rgd) })
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
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(testProjectListResponse) })
      }),
      page.route(`**${API_PATHS.canI}**`, async (route) => {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ value: canReadRGD ? 'yes' : 'no' }) })
      }),
      page.route('**/history**', async (route) => {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ timeline: [], events: [] }) })
      }),
    ])
  }

  const instanceUrl = `/instances/${testInstance.namespace}/${testInstance.kind}/${testInstance.name}`

  test('debug: network requests for rgd and can-i', async ({ page }) => {
    const allRequests: Array<{ url: string; status?: number; body?: string }> = []
    page.on('request', req => allRequests.push({ url: req.url() }))
    page.on('response', async res => {
      const entry = allRequests.find(r => r.url === res.url() && r.status === undefined)
      if (entry) {
        entry.status = res.status()
        if (res.url().includes('rgds/postgres-database') && !res.url().includes('resources') && !res.url().includes('schema') && !res.url().includes('revisions')) {
          try { entry.body = await res.text() } catch { /* text() may throw if response is detached */ }
        }
      }
    })
    await setupMocks(page)
    await page.goto(instanceUrl)
    await page.waitForTimeout(3000)
    console.log('ALL API requests:', JSON.stringify(allRequests.filter(r => r.url.includes('/api/')).map(r => ({ url: r.url.replace('http://localhost:8080', ''), status: r.status }))))
    // Check what React Query state has for the RGD
    const rgdState = await page.evaluate(() => {
      // @ts-expect-error - accessing internal React Query client
      const qc = (window as any).__queryClient
      if (!qc) return 'no query client found'
      try {
        return JSON.stringify(qc.getQueryData(['rgd', 'postgres-database', 'databases']))
      } catch { return 'error' }
    })
    console.log('React Query RGD state:', rgdState)
  })

  test.describe('Rev badge', () => {
    test('shows Rev N badge on instance header', async ({ page }) => {
      await setupMocks(page)
      await page.goto(instanceUrl)

      await expect(page.getByRole('button', { name: /View changes for revision 3/i })).toBeVisible()
      await expect(page.getByText('Rev 3')).toBeVisible()
    })

    test('badge is not shown when RGD access is denied', async ({ page }) => {
      await setupMocks(page, { canReadRGD: false })
      await page.goto(instanceUrl)

      await expect(page.getByRole('button', { name: /View changes for revision/i })).not.toBeVisible()
    })
  })

  test.describe('Drawer open/close', () => {
    test('clicking the badge opens the drawer', async ({ page }) => {
      await setupMocks(page)
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 3/i }).click()

      await expect(page.getByRole('dialog')).toBeVisible()
      await expect(page.getByText('Revision Changes')).toBeVisible()
      await expect(page.getByText('Rev 2 → Rev 3')).toBeVisible()
    })

    test('pressing Escape closes the drawer', async ({ page }) => {
      await setupMocks(page)
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 3/i }).click()
      await expect(page.getByRole('dialog')).toBeVisible()

      await page.keyboard.press('Escape')
      await expect(page.getByRole('dialog')).not.toBeVisible()
    })
  })

  test.describe('Diff content', () => {
    test('shows diff summary with added and modified counts', async ({ page }) => {
      await setupMocks(page)
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 3/i }).click()

      // Summary badges
      await expect(page.getByText('1 added')).toBeVisible()
      await expect(page.getByText('1 modified')).toBeVisible()
    })

    test('shows YAML changes section with unified diff', async ({ page }) => {
      await setupMocks(page)
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 3/i }).click()

      await expect(page.getByText('YAML Changes')).toBeVisible()
    })

    test('shows "No differences" when revisions are identical', async ({ page }) => {
      const identicalDiff: RevisionDiff = { ...mockDiff, added: [], removed: [], modified: [], identical: true }
      await setupMocks(page, { diffResponse: identicalDiff })
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 3/i }).click()

      await expect(page.getByText('No differences')).toBeVisible()
    })

    test('shows error state when diff fetch fails', async ({ page }) => {
      await setupMocks(page, { diffResponse: null })
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 3/i }).click()

      await expect(page.getByTestId('diff-error')).toBeVisible()
      await expect(page.getByText(/Failed to load diff/i)).toBeVisible()
    })
  })

  test.describe('Initial revision (rev 1)', () => {
    test('shows initial revision notice instead of diff', async ({ page }) => {
      await setupMocks(page, { rgd: rgdWithRevisionOne })
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 1/i }).click()

      await expect(page.getByTestId('initial-revision-notice')).toBeVisible()
      await expect(page.getByText(/no previous revision to compare/i)).toBeVisible()
    })

    test('shows full snapshot YAML for initial revision', async ({ page }) => {
      await setupMocks(page, { rgd: rgdWithRevisionOne })
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 1/i }).click()

      await expect(page.getByTestId('snapshot-yaml')).toBeVisible()
      // YAML dump of the snapshot should contain the RGD name
      await expect(page.getByTestId('snapshot-yaml')).toContainText('postgres-database')
    })

    test('header shows "Rev 1 (initial)" description', async ({ page }) => {
      await setupMocks(page, { rgd: rgdWithRevisionOne })
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 1/i }).click()

      await expect(page.getByText('Rev 1 (initial)')).toBeVisible()
    })
  })

  test.describe('Open in Revision Explorer link', () => {
    test('link navigates to the RGD revisions tab', async ({ page }) => {
      await setupMocks(page)
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 3/i }).click()

      const link = page.getByTestId('revision-explorer-link')
      await expect(link).toBeVisible()
      await expect(link).toHaveAttribute('href', '/catalog/postgres-database?tab=revisions')
    })

    test('clicking the link closes the drawer and navigates', async ({ page }) => {
      await setupMocks(page)
      await page.goto(instanceUrl)

      await page.getByRole('button', { name: /View changes for revision 3/i }).click()
      await expect(page.getByRole('dialog')).toBeVisible()

      await page.getByTestId('revision-explorer-link').click()

      // Navigated away — drawer is no longer visible
      await expect(page.getByRole('dialog')).not.toBeVisible()
      await page.waitForURL('**/catalog/postgres-database**')
    })
  })
})
