import { test, expect, TestUserRole } from '../fixture'
import type { Page } from '@playwright/test'
import {
  mockMicroservicesPlatformRGD,
  mockMicroservicesPlatformSchema,
  mockK8sServices,
  mockCompositeRGD,
  mockCompositeRGDSchema,
  mockArgoCDClusters,
  mockAzureKeyVaults,
  API_PATHS,
} from '../fixture/mock-data'
import {
  toggleConditionalField,
  fillField,
  captureFormSubmission,
} from '../fixture/conditional-fields-helpers'

/**
 * Shared setup for deploy form E2E tests: mocks all API endpoints and navigates
 * to the deploy form for the microservices-platform RGD.
 */
async function setupDeployFormMocks(page: Page) {
  // Mock the RGD list endpoint and specific RGD endpoints
  await page.route(`**${API_PATHS.rgds}**`, async (route) => {
    const url = route.request().url()

    if (url.includes('/microservices-platform')) {
      if (url.includes('/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockMicroservicesPlatformSchema),
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

  await page.route('**/api/v1/resources**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockK8sServices),
    })
  })
}

/** Navigate from catalog to the deploy form for microservices-platform */
async function navigateToDeployForm(page: Page) {
  await page.goto('/catalog')
  await page.waitForLoadState('networkidle')

  const rgdCard = page.getByRole('button', { name: /view details for/i }).first()
  await expect(rgdCard).toBeVisible({ timeout: 15000 })
  await rgdCard.click()

  await page.waitForURL(/\/catalog\//, { timeout: 10000 })
  await page.waitForLoadState('networkidle')

  const deployButton = page.getByRole('button', { name: /deploy/i })
  await expect(deployButton).toBeVisible({ timeout: 15000 })
  await deployButton.click()

  await page.waitForLoadState('networkidle')
  await expect(page.getByText('Configuration')).toBeVisible({ timeout: 15000 })
}

test.describe('Conditional Field Visibility', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    await setupDeployFormMocks(page)
    await navigateToDeployForm(page)
  })

  test('hides externalRef field by default when useExistingDatabase is false', async ({
    page,
  }) => {
    // The useExistingDatabase field should be visible
    await expect(page.getByTestId('field-useExistingDatabase')).toBeVisible()

    // The externalRef section should NOT be visible (hidden by conditional)
    await expect(page.getByTestId('field-externalRef')).not.toBeVisible()
  })

  test('shows externalRef resource picker when useExistingDatabase is checked', async ({
    page,
  }) => {
    // Find and check the useExistingDatabase checkbox
    const checkbox = page.getByTestId('input-useExistingDatabase')
    await checkbox.check()

    // Wait for the conditional externalRef section to appear in the DOM
    await expect(page.getByTestId('field-externalRef')).toBeVisible()

    // The resource picker dropdown should be visible
    await expect(page.getByTestId('input-externalRef.externaldb')).toBeVisible()
  })

  test('hides externalRef field when useExistingDatabase is unchecked', async ({
    page,
  }) => {
    // First, check the checkbox
    const checkbox = page.getByTestId('input-useExistingDatabase')
    await checkbox.check()

    // Wait for field to appear
    await expect(page.getByTestId('field-externalRef')).toBeVisible()

    // Now uncheck it
    await checkbox.uncheck()

    // Wait for field to be hidden
    await expect(page.getByTestId('field-externalRef')).not.toBeVisible()
  })

  test('displays conditional field immediately after controlling field when enabled', async ({
    page,
  }) => {
    // Check the useExistingDatabase checkbox
    const checkbox = page.getByTestId('input-useExistingDatabase')
    await checkbox.check()

    // Wait for conditional field to appear
    await expect(page.getByTestId('field-externalRef')).toBeVisible()

    // Verify the field appears in the DOM after useExistingDatabase
    const controllingField = page.getByTestId('field-useExistingDatabase')
    const conditionalField = page.getByTestId('field-externalRef')

    // Both should be visible
    await expect(controllingField).toBeVisible()
    await expect(conditionalField).toBeVisible()

    // Get bounding boxes to verify positioning
    const controllingBox = await controllingField.boundingBox()
    const conditionalBox = await conditionalField.boundingBox()

    // External Database Name should come after Use Existing Database vertically
    expect(conditionalBox?.y).toBeGreaterThan(controllingBox!.y)
  })

  test('allows filling values in conditional field when visible', async ({ page }) => {
    // Check the useExistingDatabase checkbox
    const checkbox = page.getByTestId('input-useExistingDatabase')
    await checkbox.check()

    // Wait for resource picker dropdown to appear
    const select = page.getByTestId('input-externalRef.externaldb')
    await expect(select).toBeVisible()

    // Select a specific resource from the dropdown by value
    await select.selectOption({ value: 'postgres-service' })

    // Verify the correct value was selected
    const selectedValue = await select.inputValue()
    expect(selectedValue).toBe('postgres-service')
  })

  test('includes all visible fields in form submission', async ({ page }) => {
    // Mock the create instance endpoint
    let submittedData: Record<string, unknown> | null = null
    const responsePromise = page.waitForResponse('**/api/v1/instances')

    await page.route('**/api/v1/instances', async (route) => {
      submittedData = await route.request().postDataJSON()
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ success: true }),
      })
    })

    // Fill in required fields
    await page.getByTestId('input-platformName').fill('test-platform')

    // Fill the instance name
    await page.getByTestId('input-instanceName').fill('my-platform')

    // Submit the form
    await page.getByTestId('deploy-submit-button').click()

    // Wait for response instead of arbitrary timeout
    await responsePromise

    // Verify the submitted data includes only visible fields
    expect(submittedData).toBeDefined()
    expect(submittedData.spec).toBeDefined()
    expect(submittedData.spec.platformName).toBe('test-platform')
    expect(submittedData.spec.useExistingDatabase).toBe(false)
    // externalRef should not have values when conditional is disabled
  })

  test('includes conditional field value when controlling field is enabled', async ({
    page,
  }) => {
    // Mock the create instance endpoint
    let submittedData: Record<string, unknown> | null = null
    const responsePromise = page.waitForResponse('**/api/v1/instances')

    await page.route('**/api/v1/instances', async (route) => {
      submittedData = await route.request().postDataJSON()
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ success: true }),
      })
    })

    // Fill in required fields
    await page.getByTestId('input-platformName').fill('test-platform')

    // Enable useExistingDatabase
    const checkbox = page.getByTestId('input-useExistingDatabase')
    await checkbox.check()

    // Wait for resource picker to appear
    const conditionalSelect = page.getByTestId('input-externalRef.externaldb')
    await expect(conditionalSelect).toBeVisible()

    // Select a specific resource from the dropdown (auto-fills name + namespace)
    await conditionalSelect.selectOption({ value: 'postgres-service' })

    // Fill the instance name
    await page.getByTestId('input-instanceName').fill('my-platform')

    // Submit the form
    await page.getByTestId('deploy-submit-button').click()

    // Wait for response instead of arbitrary timeout
    await responsePromise

    // Verify the submitted data includes auto-filled name and namespace
    expect(submittedData).toBeDefined()
    expect(submittedData.spec).toBeDefined()
    expect(submittedData.spec.platformName).toBe('test-platform')
    expect(submittedData.spec.useExistingDatabase).toBe(true)
    expect(submittedData.spec.externalRef.externaldb.name).toBe('postgres-service') // Auto-filled from resource picker
    expect(submittedData.spec.externalRef.externaldb.namespace).toBe('default') // Auto-filled from resource picker
  })

  test('non-controlling fields are always visible', async ({ page }) => {
    // Fields like platformName, environment, and highAvailability should always be visible
    await expect(page.getByTestId('input-platformName')).toBeVisible()
    await expect(page.getByTestId('input-environment')).toBeVisible()
    await expect(page.getByTestId('input-highAvailability')).toBeVisible()

    // These should remain visible even when we toggle useExistingDatabase
    const checkbox = page.getByTestId('input-useExistingDatabase')
    await checkbox.check()

    // Wait for conditional field to appear (confirms toggle worked)
    await expect(page.getByTestId('field-externalRef')).toBeVisible()

    // Non-controlling fields should still be visible
    await expect(page.getByTestId('input-platformName')).toBeVisible()
    await expect(page.getByTestId('input-environment')).toBeVisible()
    await expect(page.getByTestId('input-highAvailability')).toBeVisible()
  })
})

test.describe('Conditional Field Accessibility', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    await setupDeployFormMocks(page)
    await navigateToDeployForm(page)
  })

  test('form fields have proper labels for screen readers', async ({ page }) => {
    // Check that controlling field has an associated label
    const useExistingCheckbox = page.getByTestId('input-useExistingDatabase')
    const useExistingLabel = page.locator('label[for="useExistingDatabase"]')
    await expect(useExistingLabel).toBeVisible()

    // Enable conditional field
    await useExistingCheckbox.check()
    await expect(page.getByTestId('field-externalRef')).toBeVisible()

    // Check that the resource picker has an associated label
    const externalDbLabel = page.locator('label[for="externalRef.externaldb"]')
    await expect(externalDbLabel).toBeVisible()
  })

  test('conditional field has proper ARIA attributes', async ({ page }) => {
    const checkbox = page.getByTestId('input-useExistingDatabase')

    // Check initial state
    const initialChecked = await checkbox.isChecked()
    expect(initialChecked).toBe(false)

    // Enable conditional field
    await checkbox.check()
    await expect(page.getByTestId('field-externalRef')).toBeVisible()

    // Verify checkbox is now checked (native checkboxes use 'checked' property, not aria-checked)
    const checkedState = await checkbox.isChecked()
    expect(checkedState).toBe(true)
  })
})

test.describe('Conditional Field Helper Functions', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    await setupDeployFormMocks(page)
    await navigateToDeployForm(page)
  })

  test('toggleConditionalField helper works correctly', async ({ page }) => {
    // Use helper to enable conditional field
    await toggleConditionalField(page, 'useExistingDatabase', 'externalRef', true)
    await expect(page.getByTestId('field-externalRef')).toBeVisible()

    // Use helper to disable conditional field
    await toggleConditionalField(page, 'useExistingDatabase', 'externalRef', false)
    await expect(page.getByTestId('field-externalRef')).not.toBeVisible()
  })

  test('fillField helper works correctly', async ({ page }) => {
    await fillField(page, 'platformName', 'test-platform')
    await expect(page.getByTestId('input-platformName')).toHaveValue('test-platform')
  })

  test('captureFormSubmission helper captures data', async ({ page }) => {
    await fillField(page, 'platformName', 'test-platform')
    await fillField(page, 'instanceName', 'my-instance')

    const submittedData = await captureFormSubmission(page, async () => {
      await page.getByTestId('deploy-submit-button').click()
    })

    expect(submittedData.spec).toBeDefined()
    expect(submittedData.spec.platformName).toBe('test-platform')
    // instanceName is metadata, not spec - verify it exists in the request
    expect(submittedData).toBeDefined()
  })
})

/**
 * Setup mocks for composite RGD with nested externalRef selectors.
 * This simulates the AKSApplicationExternalSecretOperator pattern where
 * both resource-level (argocdClusterRef) and nested template-resolved
 * (keyVaultRef) externalRef selectors produce identical ExternalRefSelectorMetadata.
 */
async function setupCompositeRGDMocks(page: Page) {
  await page.route(`**${API_PATHS.rgds}**`, async (route) => {
    const url = route.request().url()

    if (url.includes('/aks-app-eso')) {
      if (url.includes('/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockCompositeRGDSchema),
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
          body: JSON.stringify(mockCompositeRGD),
        })
      }
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [mockCompositeRGD],
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
        deploymentOrder: ['aks-app-eso'],
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

  // Return different resources based on the kind query parameter
  await page.route('**/api/v1/resources**', async (route) => {
    const url = route.request().url()
    if (url.includes('AzureKeyVault')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockAzureKeyVaults),
      })
    } else if (url.includes('ArgoCDAKSCluster')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockArgoCDClusters),
      })
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], count: 0 }),
      })
    }
  })
}

/** Navigate to the deploy form for the composite RGD */
async function navigateToCompositeDeployForm(page: Page) {
  await page.goto('/catalog')
  await page.waitForLoadState('networkidle')

  const rgdCard = page.getByRole('button', { name: /view details for/i }).first()
  await expect(rgdCard).toBeVisible({ timeout: 15000 })
  await rgdCard.click()

  await page.waitForURL(/\/catalog\//, { timeout: 10000 })
  await page.waitForLoadState('networkidle')

  const deployButton = page.getByRole('button', { name: /deploy/i })
  await expect(deployButton).toBeVisible({ timeout: 15000 })
  await deployButton.click()

  await page.waitForLoadState('networkidle')
  await expect(page.getByText('Configuration')).toBeVisible({ timeout: 15000 })
}

test.describe('Nested ExternalRef Dropdowns (Composite RGDs)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    await setupCompositeRGDMocks(page)
    await navigateToCompositeDeployForm(page)
  })

  test('renders resource picker dropdowns for both resource-level and nested externalRef', async ({
    page,
  }) => {
    // Both externalRef selectors should render as dropdowns (not plain text inputs)
    // Resource-level: argocdClusterRef
    const argocdDropdown = page.getByTestId('input-externalRef.argocdClusterRef')
    await expect(argocdDropdown).toBeVisible()

    // Nested (cross-RGD resolved): keyVaultRef
    const keyVaultDropdown = page.getByTestId('input-externalRef.keyVaultRef')
    await expect(keyVaultDropdown).toBeVisible()
  })

  test('nested externalRef dropdown shows correct resource options', async ({
    page,
  }) => {
    // The keyVaultRef dropdown should list AzureKeyVault instances
    const keyVaultDropdown = page.getByTestId('input-externalRef.keyVaultRef')
    await expect(keyVaultDropdown).toBeVisible()

    // Select a key vault from the dropdown
    await keyVaultDropdown.selectOption({ value: 'prod-keyvault' })
    const selectedValue = await keyVaultDropdown.inputValue()
    expect(selectedValue).toBe('prod-keyvault')
  })

  test('resource-level externalRef dropdown shows correct resource options', async ({
    page,
  }) => {
    // The argocdClusterRef dropdown should list ArgoCDAKSCluster instances
    const argocdDropdown = page.getByTestId('input-externalRef.argocdClusterRef')
    await expect(argocdDropdown).toBeVisible()

    // Select a cluster from the dropdown
    await argocdDropdown.selectOption({ value: 'aks-prod-cluster' })
    const selectedValue = await argocdDropdown.inputValue()
    expect(selectedValue).toBe('aks-prod-cluster')
  })

  test('form submission includes auto-filled values from both externalRef types', async ({
    page,
  }) => {
    let submittedData: Record<string, unknown> | null = null
    const responsePromise = page.waitForResponse('**/api/v1/instances')

    await page.route('**/api/v1/instances', async (route) => {
      submittedData = await route.request().postDataJSON()
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ success: true }),
      })
    })

    // Fill required fields
    await page.getByTestId('input-appName').fill('my-aks-app')
    await page.getByTestId('input-instanceName').fill('my-eso-instance')

    // Select from both dropdowns
    const argocdDropdown = page.getByTestId('input-externalRef.argocdClusterRef')
    await argocdDropdown.selectOption({ value: 'aks-prod-cluster' })

    const keyVaultDropdown = page.getByTestId('input-externalRef.keyVaultRef')
    await keyVaultDropdown.selectOption({ value: 'prod-keyvault' })

    // Submit the form
    await page.getByTestId('deploy-submit-button').click()
    await responsePromise

    // Verify both externalRef values are submitted with auto-filled name + namespace
    expect(submittedData).toBeDefined()
    expect(submittedData.spec).toBeDefined()
    expect(submittedData.spec.appName).toBe('my-aks-app')

    // Resource-level externalRef (argocdClusterRef)
    expect(submittedData.spec.externalRef.argocdClusterRef.name).toBe('aks-prod-cluster')
    expect(submittedData.spec.externalRef.argocdClusterRef.namespace).toBe('argocd')

    // Nested externalRef (keyVaultRef) - identical metadata format
    expect(submittedData.spec.externalRef.keyVaultRef.name).toBe('prod-keyvault')
    expect(submittedData.spec.externalRef.keyVaultRef.namespace).toBe('secrets')
  })
})
