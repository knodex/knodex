// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { Locator, Page } from "@playwright/test";

/**
 * Helper for interacting with shadcn/Radix Select components in E2E tests.
 *
 * Native `<select>` works with Playwright's `selectOption(value)`. The shadcn
 * Select wraps a Radix popover button + portal'd listbox, so we click the
 * trigger to open it, then click the option by accessible name (label).
 */
export async function selectShadcnOption(
  trigger: Locator,
  optionName: string,
): Promise<void> {
  await trigger.click();
  // Radix renders options into a portal; query at page level.
  const page = trigger.page();
  await page.getByRole("option", { name: optionName, exact: true }).click();
}

/**
 * Pick the Nth option from a shadcn Select (0-indexed).
 * Useful when the option's label is variable / cluster-dependent.
 */
export async function selectShadcnOptionByIndex(
  trigger: Locator,
  index: number,
): Promise<void> {
  await trigger.click();
  const page: Page = trigger.page();
  await page.getByRole("option").nth(index).click();
}
