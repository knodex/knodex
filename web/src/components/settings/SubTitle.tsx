// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ReactNode } from "react";

export interface SubTitleProps {
  /** 18px right-pane heading that anchors each settings sub-page. */
  title: string;
  /** Optional supporting line under the title. */
  description?: string;
  /** Optional right-aligned action slot (e.g. a promoted primary button). */
  action?: ReactNode;
}

/**
 * SubTitle — the single right-pane heading for a Settings sub-page (Story 48.10).
 *
 * Replaces the bespoke icon-puck `<h2>+<p>` headers each pane carried. Renders an
 * 18px (`text-lg`) semibold title, an optional muted description, and an optional
 * right-aligned `action` slot, separated from the pane body by a bottom divider.
 * `items-end justify-between` keeps the action bottom-aligned with the title.
 *
 * Lives in `components/settings/` (settings-scoped per the design diff). Cloud
 * panes import it directly — cloud → shared is allowed; the reverse is not.
 * Semantic tokens only; no literal hex/HSL.
 */
export function SubTitle({ title, description, action }: SubTitleProps) {
  return (
    <div className="mb-6 flex items-end justify-between gap-4 border-b border-[var(--border-default)] pb-4">
      <div className="min-w-0">
        <h2 className="text-lg font-semibold tracking-tight text-foreground">{title}</h2>
        {description && (
          <p className="mt-1 text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}

export default SubTitle;
