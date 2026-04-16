// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture'
import {
  mockRGDListResponse,
  mockRGDs,
  mockInstances,
  API_PATHS,
} from '../fixture/mock-data'
import type { Instance, ChildResourceResponse } from '../../src/types/rgd'
import type { Project, ProjectListResponse } from '../../src/types/project'

/**
 * Instance Child Resources Tab — E2E Tests
 *
 * Tests the "Resources" tab on the instance detail view which displays
 * child Kubernetes resources discovered via KRO labels, grouped by node-id.
 */

const detailInstance: Instance = {
  ...mockInstances[0],
  name: 'demo-app',
  namespace: 'default',
  rgdName: 'test-pod-pair',
  rgdNamespace: 'default',
  kind: 'TestPodPair',
  health: 'Healthy',
  deploymentMode: 'direct',
  conditions: [
    { type: 'Ready', status: 'True', reason: 'AllReady', message: 'All resources ready' },
  ],
  spec: { name: 'demo' },
  status: { state: 'ACTIVE' },
  createdAt: '2026-03-20T10:00:00Z',
  updatedAt: '2026-03-27T14:00:00Z',
}

const mockChildrenResponse: ChildResourceResponse = {
  instanceName: 'demo-app',
  instanceNamespace: 'default',
  instanceKind: 'TestPodPair',
  totalCount: 3,
  groups: [
    {
      nodeId: 'frontend',
      kind: 'Pod',
      apiVersion: 'v1',
      count: 2,
      readyCount: 2,
      health: 'Healthy',
      resources: [
        {
          name: 'frontend-pod-1',
          namespace: 'default',
          kind: 'Pod',
          apiVersion: 'v1',
          nodeId: 'frontend',
          health: 'Healthy',
          phase: 'Running',
          createdAt: '2026-03-20T10:00:00Z',
        },
        {
          name: 'frontend-pod-2',
          namespace: 'default',
          kind: 'Pod',
          apiVersion: 'v1',
          nodeId: 'frontend',
          health: 'Healthy',
          phase: 'Running',
          createdAt: '2026-03-20T10:00:00Z',
        },
      ],
    },
    {
      nodeId: 'backend',
      kind: 'Pod',
      apiVersion: 'v1',
      count: 1,
      readyCount: 0,
      health: 'Unhealthy',
      resources: [
        {
          name: 'backend-pod-1',
          namespace: 'default',
          kind: 'Pod',
          apiVersion: 'v1',
          nodeId: 'backend',
          health: 'Unhealthy',
          phase: 'Failed',
          createdAt: '2026-03-20T10:00:00Z',
        },
      ],
    },
  ],
}

const mockProjectsResponse: ProjectListResponse = {
  items: [
    { name: 'default', namespace: 'default', displayName: 'Default', destinations: [{ namespace: 'default' }], roles: [] } as unknown as Project,
  ],
  totalCount: 1,
}

test.describe('Instance Child Resources Tab', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test.beforeEach(async ({ page }) => {
    // Mock account/info so session restore succeeds (prevents "Connection Error" state)
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

    // Mock API routes
    await page.route(`**/api${API_PATHS.instances}?*`, route =>
      route.fulfill({ json: { items: [detailInstance], totalCount: 1, page: 1, pageSize: 20 } })
    )
    await page.route(`**/api/v1/namespaces/default/instances/TestPodPair/demo-app`, route =>
      route.fulfill({ json: detailInstance })
    )
    await page.route(`**/api/v1/namespaces/default/instances/TestPodPair/demo-app/children`, route =>
      route.fulfill({ json: mockChildrenResponse })
    )
    await page.route(`**/api${API_PATHS.rgds}?*`, route =>
      route.fulfill({ json: mockRGDListResponse })
    )
    await page.route(`**/api/v1/rgds/test-pod-pair?*`, route =>
      route.fulfill({ json: mockRGDs[0] })
    )
    await page.route(`**/api${API_PATHS.projects}`, route =>
      route.fulfill({ json: mockProjectsResponse })
    )
    await page.route('**/api/v1/account/can-i', route =>
      route.fulfill({ json: { allowed: true } })
    )
    await page.route('**/api/v1/namespaces/default/instances/TestPodPair/demo-app/history?*', route =>
      route.fulfill({ json: { items: [], totalCount: 0, page: 1, pageSize: 20 } })
    )
    await page.route('**/api/v1/namespaces/default/instances/TestPodPair/demo-app/timeline?*', route =>
      route.fulfill({ json: { events: [] } })
    )
    await page.route(`**/api${API_PATHS.instanceCount}`, route =>
      route.fulfill({ json: { count: 1 } })
    )
  })

  test('shows Resources tab on instance detail page', async ({ page }) => {
    await page.goto('/instances/default/TestPodPair/demo-app')
    await expect(page.getByRole('tab', { name: /Resources/ })).toBeVisible()
  })

  test('displays child resource groups when Resources tab clicked', async ({ page }) => {
    await page.goto('/instances/default/TestPodPair/demo-app')

    // Click Resources tab
    await page.getByRole('tab', { name: /Resources/ }).click()

    // Verify group headers (node IDs from the mock response)
    await expect(page.getByText('frontend')).toBeVisible()
    await expect(page.getByText('backend')).toBeVisible()
  })

  test('shows ready count per group', async ({ page }) => {
    await page.goto('/instances/default/TestPodPair/demo-app')
    await page.getByRole('tab', { name: /Resources/ }).click()

    await expect(page.getByText('2/2 ready')).toBeVisible()
    await expect(page.getByText('0/1 ready')).toBeVisible()
  })

  test('expands group to show individual resources', async ({ page }) => {
    await page.goto('/instances/default/TestPodPair/demo-app')
    await page.getByRole('tab', { name: /Resources/ }).click()

    // Click frontend group to expand
    await page.getByText('frontend').click()

    // Verify individual resources visible
    await expect(page.getByText('frontend-pod-1')).toBeVisible()
    await expect(page.getByText('frontend-pod-2')).toBeVisible()
  })

  test('shows empty state when no children', async ({ page }) => {
    // Override children route with empty response
    await page.route(`**/api/v1/namespaces/default/instances/TestPodPair/demo-app/children`, route =>
      route.fulfill({ json: { ...mockChildrenResponse, totalCount: 0, groups: [] } })
    )

    await page.goto('/instances/default/TestPodPair/demo-app')
    await page.getByRole('tab', { name: /Resources/ }).click()

    await expect(page.getByText('No child resources found for this instance.')).toBeVisible()
  })
})
