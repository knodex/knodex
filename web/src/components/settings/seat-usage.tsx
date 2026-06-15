// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Shared seat-usage rendering (STORY-465 + Story 16.2 / UM-2).
 *
 * The seat-render logic originally lived inline in `LicenseSettings.tsx`. It is
 * extracted here so the Settings → Users page header can reuse the EXACT same
 * formatter + threshold treatment instead of forking it. `LicenseSettings`
 * consumes `formatActiveUsers` + `SeatThresholdBanner` from this module; the
 * Users page consumes the compact `SeatUsageWidget`.
 *
 * Feature-probe semantics: `seats` is present only on EE builds — OSS omits it
 * entirely, so every entry point here renders nothing when `seats` is undefined.
 */

import { AlertTriangle } from "@/lib/icons";
import type { SeatUsage } from "@/types/license";
import { cn } from "@/lib/utils";

/**
 * formatActiveUsers builds the "Active users: U / N" string (STORY-465 AC #14).
 *
 * Three states:
 *  - cold start (seats undefined or `lastUpdated === ""`) → "calculating…"
 *    so the operator sees the reconciler warming up rather than a misleading "0".
 *  - unlimited license (maxUsers === 0) → "U (Unlimited)".
 *  - bounded license → "U / N".
 */
// eslint-disable-next-line react-refresh/only-export-components -- Seat helpers co-located with the widgets that use them; shared with LicenseSettings.
export function formatActiveUsers(
  maxUsers: number,
  seats: SeatUsage | undefined,
): string {
  // Cold start sentinel — AC #18.
  if (!seats || seats.lastUpdated === "") {
    return "calculating…";
  }
  if (maxUsers === 0) {
    return `${seats.used} (Unlimited)`;
  }
  return `${seats.used} / ${maxUsers}`;
}

interface SeatThresholdBannerProps {
  seats: SeatUsage | undefined;
}

/**
 * SeatThresholdBanner renders the warn / exceeded amber-or-red banner just below
 * the License status banner (STORY-465 AC #15, #16). Returns null when the
 * banner would be silent (`ok`, cold start, or seats omitted by an OSS-style
 * response) so the page stays calm by default.
 *
 * Visual treatment mirrors the grace_period status banner — same AlertTriangle
 * icon, same amber-500 / destructive token palette — so the Settings page
 * doesn't grow a new design language for one alert.
 *
 * Cloud-tenant builds (`seats.advisoryOnly`) substitute a softer subline that
 * points operators at the control plane as the authoritative enforcer (AC #21).
 */
export function SeatThresholdBanner({ seats }: SeatThresholdBannerProps) {
  if (!seats || seats.lastUpdated === "" || seats.threshold === "ok") {
    return null;
  }

  const isExceeded = seats.threshold === "exceeded";
  const bgClass = isExceeded
    ? "bg-destructive/5 border-destructive/30"
    : "bg-amber-500/5 border-amber-500/30";
  const iconClass = isExceeded
    ? "text-destructive"
    : "text-amber-600 dark:text-amber-400";

  const headline = isExceeded ? "Seat limit exceeded" : "Approaching seat limit";
  const percentLabel = `${Math.round(seats.percent * 100)}%`;

  // AC #21 — cloud-tenant copy is shorter and points at the control plane.
  const subline = seats.advisoryOnly
    ? "Cloud-tenant deployments enforce seat limits via the control plane; this view is informational."
    : isExceeded
      ? `${seats.used} of ${seats.allowed} licensed users active in the last ${seats.windowDays} days. Contact sales@knodex.io to extend your license.`
      : `${seats.used} of ${seats.allowed} licensed users active in the last ${seats.windowDays} days (${percentLabel}). Consider upgrading your license before you hit the cap.`;

  return (
    <div
      className={cn("flex items-start gap-3 p-4 rounded-md border", bgClass)}
      data-testid="license-seats-banner"
      data-threshold={seats.threshold}
    >
      <AlertTriangle className={cn("h-5 w-5 mt-0.5 shrink-0", iconClass)} />
      <div className="min-w-0">
        <p className="text-sm font-medium text-foreground">{headline}</p>
        <p className="text-xs text-muted-foreground mt-1">{subline}</p>
      </div>
    </div>
  );
}

interface SeatUsageWidgetProps {
  /** Seat snapshot from `GET /api/v1/license`. Absent on OSS → widget hides. */
  seats?: SeatUsage;
  /** License seat cap (mirrors `seats.allowed`; 0 = unlimited). */
  maxUsers?: number;
}

/**
 * SeatUsageWidget — compact seat-usage indicator for the Settings → Users page
 * header (Story 16.2 / UM-2 AC #1).
 *
 * Renders:
 *  - nothing on OSS (`seats` undefined → no seat-limit framing);
 *  - "calculating…" on the cold-start sentinel (`seats.lastUpdated === ""`),
 *    never a misleading 0;
 *  - "{used} / {allowed}", or "{used} (Unlimited)" when allowed === 0;
 *  - advisory-only copy when `seats.advisoryOnly` (cloud-tenant);
 *  - warn/exceeded token tinting reused from the License Settings treatment
 *    (no new design language).
 */
export function SeatUsageWidget({ seats, maxUsers }: SeatUsageWidgetProps) {
  // OSS / feature-absent: render nothing. AC #1.
  if (!seats) {
    return null;
  }

  const allowed = maxUsers ?? seats.allowed;
  const isCalculating = seats.lastUpdated === "";
  const isUnlimited = allowed === 0;

  const isExceeded = !isCalculating && seats.threshold === "exceeded";
  const isWarn = !isCalculating && seats.threshold === "warn";

  // The seat count carries the only tone — warn amber, exceeded red, else the
  // calm default foreground. No bordered chip: the mockup reads as inline text
  // ("4 / 25 seats used") sitting in the toolbar next to the filters.
  const countToneClass = isExceeded
    ? "text-destructive"
    : isWarn
      ? "text-amber-600 dark:text-amber-400"
      : "text-foreground";
  const iconClass = isExceeded
    ? "text-destructive"
    : "text-amber-600 dark:text-amber-400";

  return (
    <div
      className="inline-flex items-center gap-1.5 text-sm whitespace-nowrap"
      data-testid="users-seat-usage"
      data-threshold={isCalculating ? "calculating" : seats.threshold}
    >
      {(isExceeded || isWarn) && (
        <AlertTriangle className={cn("h-4 w-4 shrink-0", iconClass)} />
      )}
      {isCalculating ? (
        <span className="text-muted-foreground">calculating…</span>
      ) : (
        <span>
          <span className={cn("font-semibold tabular-nums", countToneClass)}>
            {/* "{used} / {allowed}", or "{used} (Unlimited)" when uncapped —
                the value string keeps the formatter's exact wording. */}
            {formatActiveUsers(allowed, seats)}
          </span>
          {/* "Unlimited" is already self-describing; only the bounded form needs
              the trailing "seats used" gloss. */}
          {!isUnlimited && (
            <span className="text-muted-foreground"> seats used</span>
          )}
        </span>
      )}
      {seats.advisoryOnly && !isCalculating && (
        <span className="text-muted-foreground">
          · enforced via the control plane (informational)
        </span>
      )}
    </div>
  );
}
