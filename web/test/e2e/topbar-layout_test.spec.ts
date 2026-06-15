// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Viewport-sweep test for the rebuilt TopBar layout (Story 48.12).
 *
 * Verifies:
 *  - Hamburger (topbar-menu-trigger) visible below 1024px, hidden at >= 1024px.
 *  - CmdK trigger (cmdk-trigger) visible at >= 768px.
 *  - At 1024 / 1100 / 1440 widths the breadcrumb leaf and the CmdK trigger
 *    do not overlap horizontally (boundingBox comparison with a 4px tolerance).
 */
import { test, expect, TestUserRole, setupAuthAndNavigate } from '../fixture';

const VIEWPORTS = [
  { name: '320x800', width: 320, height: 800 },
  { name: '768x1024', width: 768, height: 1024 },
  { name: '1024x768', width: 1024, height: 768 },
  { name: '1100x800', width: 1100, height: 800 },
  { name: '1440x900', width: 1440, height: 900 },
] as const;

const OVERLAP_TOLERANCE_PX = 4;

test.describe('TopBar layout — viewport sweep', () => {
  for (const vp of VIEWPORTS) {
    test(`renders correctly at ${vp.name}`, async ({ page }) => {
      await page.setViewportSize({ width: vp.width, height: vp.height });
      await setupAuthAndNavigate(page, TestUserRole.GLOBAL_ADMIN, '/catalog');

      // The TopBar is only rendered above the mobile breakpoint (768px) — at
      // sub-768 widths DashboardLayout renders a BottomNav instead and no
      // TopBar mounts (so topbar-menu-trigger / cmdk-trigger / breadcrumb-leaf
      // do not exist at all). Skip the topbar assertions in that range.
      if (vp.width < 768) {
        const hamburger = page.getByTestId('topbar-menu-trigger');
        await expect(hamburger).toHaveCount(0);
        return;
      }

      // Hamburger visibility (tablet-only): visible 768–1023, hidden at 1024+.
      const hamburger = page.getByTestId('topbar-menu-trigger');
      if (vp.width < 1024) {
        await expect(hamburger).toBeVisible();
      } else {
        await expect(hamburger).toBeHidden();
      }

      // CmdK trigger visibility: visible at >= 768px.
      const cmdk = page.getByTestId('cmdk-trigger');
      await expect(cmdk).toBeVisible();

      // Overlap guard at 1024 / 1100 / 1440 — breadcrumb leaf and CmdK must not collide.
      if (vp.width >= 1024) {
        const leaf = page.getByTestId('topbar-breadcrumb-leaf');
        await expect(leaf).toBeVisible();

        const leafBox = await leaf.boundingBox();
        const cmdkBox = await cmdk.boundingBox();
        expect(leafBox, 'breadcrumb leaf box').not.toBeNull();
        expect(cmdkBox, 'cmdk trigger box').not.toBeNull();
        if (!leafBox || !cmdkBox) return;

        const leafRight = leafBox.x + leafBox.width;
        // Allow either ordering (leaf left of cmdk or vice versa), but no overlap.
        const noOverlap =
          leafRight <= cmdkBox.x + OVERLAP_TOLERANCE_PX ||
          cmdkBox.x + cmdkBox.width <= leafBox.x + OVERLAP_TOLERANCE_PX;
        expect(
          noOverlap,
          `breadcrumb leaf (${leafBox.x},${leafBox.x + leafBox.width}) overlaps cmdk (${cmdkBox.x},${cmdkBox.x + cmdkBox.width}) at ${vp.name}`
        ).toBe(true);
      }
    });
  }
});
