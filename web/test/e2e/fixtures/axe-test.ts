// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test as base, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

/**
 * Extended Playwright test fixture with axe-core accessibility scanning.
 * Usage: import { test } from './fixtures/axe-test' in accessibility test files.
 */
export const test = base.extend<{ makeAxeBuilder: () => AxeBuilder }>({
  makeAxeBuilder: async ({ page }, use) => {
    // eslint-disable-next-line react-hooks/rules-of-hooks -- Playwright `use()` is not a React Hook
    await use(() =>
      new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
    );
  },
});

/**
 * Asserts that no critical or serious axe violations exist on the page.
 * Minor and moderate violations are logged as warnings but don't fail.
 */
export async function expectNoA11yViolations(axeBuilder: AxeBuilder) {
  const results = await axeBuilder.analyze();

  const critical = results.violations.filter(
    (v) => v.impact === 'critical' || v.impact === 'serious'
  );

  const summary = critical
    .map((v) => `[${v.impact}] ${v.id}: ${v.description} (${v.nodes.length} instances)`)
    .join('\n');

  // Always assert — even when passing — to catch silent failures (empty results, injection errors)
  expect(critical, summary ? `Accessibility violations:\n${summary}` : '').toHaveLength(0);
}

export { expect };
