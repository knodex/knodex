import type { Page, Locator } from '@playwright/test'

/**
 * Helper functions for conditional field E2E tests
 */

/**
 * Toggles a conditional controlling field (checkbox) and waits for the conditional field to appear/disappear
 * @param page - Playwright page object
 * @param controllingFieldName - Name of the controlling field (e.g., 'useExistingDatabase')
 * @param conditionalFieldName - Name of the conditional field (e.g., 'externalRef')
 * @param shouldBeVisible - Whether the conditional field should be visible after toggling
 */
export async function toggleConditionalField(
  page: Page,
  controllingFieldName: string,
  conditionalFieldName: string,
  shouldBeVisible: boolean
): Promise<void> {
  const checkbox = page.getByTestId(`input-${controllingFieldName}`)
  const conditionalField = page.getByTestId(`field-${conditionalFieldName}`)

  if (shouldBeVisible) {
    await checkbox.check()
    await conditionalField.waitFor({ state: 'visible' })
  } else {
    await checkbox.uncheck()
    await conditionalField.waitFor({ state: 'hidden' })
  }
}

/**
 * Fills a form field and waits for it to update
 * @param page - Playwright page object
 * @param fieldName - Name of the field to fill
 * @param value - Value to fill in the field
 */
export async function fillField(
  page: Page,
  fieldName: string,
  value: string
): Promise<void> {
  const input = page.getByTestId(`input-${fieldName}`)
  await input.waitFor({ state: 'visible' })
  await input.fill(value)
}

/**
 * Checks that a field is visible and in the correct position relative to another field
 * @param controllingField - The controlling field locator
 * @param conditionalField - The conditional field locator
 */
export async function assertFieldPositioning(
  controllingField: Locator,
  conditionalField: Locator
): Promise<number> {
  const controllingBox = await controllingField.boundingBox()
  const conditionalBox = await conditionalField.boundingBox()

  if (!controllingBox || !conditionalBox) {
    throw new Error('Could not get bounding boxes for field positioning check')
  }

  // Return the vertical distance between fields
  return conditionalBox.y - (controllingBox.y + controllingBox.height)
}

/**
 * Waits for a form submission to complete and returns the submitted data
 * @param page - Playwright page object
 * @param submitAction - Function that triggers the form submission
 * @returns The submitted data
 */
export async function captureFormSubmission<T = Record<string, unknown>>(
  page: Page,
  submitAction: () => Promise<void>
): Promise<T> {
  let submittedData: T | null = null
  const responsePromise = page.waitForResponse('**/api/v1/instances')

  await page.route('**/api/v1/instances', async (route) => {
    submittedData = await route.request().postDataJSON()
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ success: true }),
    })
  })

  await submitAction()
  await responsePromise

  if (!submittedData) {
    throw new Error('Form submission did not capture data')
  }

  return submittedData
}
