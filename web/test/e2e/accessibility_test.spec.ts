// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expectNoA11yViolations } from './fixtures/axe-test';

test.describe('Accessibility - axe-core scans', () => {
  test('instances page has no critical/serious a11y violations', async ({
    page,
    makeAxeBuilder,
  }) => {
    await page.goto('/instances');
    await page.waitForLoadState('networkidle');

    await expectNoA11yViolations(makeAxeBuilder());
  });

  test('catalog page has no critical/serious a11y violations', async ({
    page,
    makeAxeBuilder,
  }) => {
    await page.goto('/catalog');
    await page.waitForLoadState('networkidle');

    await expectNoA11yViolations(makeAxeBuilder());
  });
});
