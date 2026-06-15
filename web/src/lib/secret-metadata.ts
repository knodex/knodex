// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { SecretStatus, SecretMetadata, SecretRotation } from "@/types/secret";

/**
 * Local UI shape for the metadata sub-form. Strings rather than the
 * `SecretMetadata` wire shape so the date input can hold `YYYY-MM-DD`
 * directly and rotation can hold `""` for "unset".
 *
 * Lives here (not next to the component) so the component file stays
 * a pure-component module — React Fast Refresh is finicky about
 * mixed exports.
 */
export interface SecretMetadataFormValue {
  rotation: SecretRotation | "";
  docsUrl: string;
  /** YYYY-MM-DD, exactly what an HTML date input emits. */
  expiresAtDate: string;
}

/** Default form value — all three fields blank. */
export const emptyMetadataValue: SecretMetadataFormValue = {
  rotation: "",
  docsUrl: "",
  expiresAtDate: "",
};

/**
 * Convert an HTML date input value (YYYY-MM-DD) into the RFC3339 timestamp
 * the server stores in the `knodex.io/expires-at` annotation.
 *
 * Empty string returns undefined so callers can submit "no expiration".
 * The constructed timestamp is end-of-day UTC (23:59:59Z) — secrets are
 * day-granular so end-of-day matches what a human means when they pick
 * a calendar date for expiration.
 */
export function dateInputToExpiresAt(dateInput: string): string | undefined {
  if (!dateInput) return undefined;
  // Use Z so the timestamp lands at end-of-day UTC regardless of the user's
  // timezone. Avoids the off-by-one-day drift that local-time parsing would
  // introduce for users east of UTC.
  return `${dateInput}T23:59:59Z`;
}

/**
 * Convert a server-side RFC3339 `expiresAt` value back into a YYYY-MM-DD
 * string suitable for an HTML `<input type="date">`.
 *
 * Returns empty string for undefined/malformed input so the input renders
 * empty rather than showing "Invalid Date".
 */
export function expiresAtToDateInput(expiresAt: string | undefined): string {
  if (!expiresAt) return "";
  const d = new Date(expiresAt);
  if (Number.isNaN(d.getTime())) return "";
  // toISOString returns YYYY-MM-DDTHH:mm:ss.sssZ; slice to YYYY-MM-DD.
  return d.toISOString().slice(0, 10);
}

/**
 * Human-friendly label for a status badge. Empty input renders as an em-dash.
 */
export function statusLabel(status: SecretStatus | undefined): string {
  switch (status) {
    case "active":
      return "Active";
    case "expiring-soon":
      return "Expiring soon";
    case "expired":
      return "Expired";
    default:
      return "—";
  }
}

/**
 * Tailwind classes for the status badge background/foreground.
 * Mapped here so the table cell stays declarative.
 */
export function statusBadgeClasses(status: SecretStatus | undefined): string {
  switch (status) {
    case "active":
      return "bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200";
    case "expiring-soon":
      return "bg-amber-50 text-amber-800 ring-1 ring-amber-200";
    case "expired":
      return "bg-rose-50 text-rose-700 ring-1 ring-rose-200";
    default:
      return "bg-muted text-muted-foreground";
  }
}

/**
 * Build a short "expires in X" string for tooltips and inline hints.
 * Returns empty string when no expiration is set.
 */
export function expiryHint(expiresAt: string | undefined, now: Date = new Date()): string {
  if (!expiresAt) return "";
  const d = new Date(expiresAt);
  if (Number.isNaN(d.getTime())) return "";
  const days = Math.round((d.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
  if (days < 0) return `Expired ${Math.abs(days)}d ago`;
  if (days === 0) return "Expires today";
  if (days === 1) return "Expires tomorrow";
  return `Expires in ${days}d`;
}

/**
 * Compare two metadata snapshots for equality. Used by the Edit form to
 * decide whether to include the `metadata` field in the PUT body at all —
 * if the user did not touch the fields, omitting metadata leaves the
 * server-side labels/annotations exactly as they were.
 */
export function metadataEqual(a: SecretMetadata | undefined, b: SecretMetadata | undefined): boolean {
  const na = normalize(a);
  const nb = normalize(b);
  return na.rotation === nb.rotation && na.docsUrl === nb.docsUrl && na.expiresAt === nb.expiresAt;
}

function normalize(m: SecretMetadata | undefined): { rotation: SecretRotation | ""; docsUrl: string; expiresAt: string } {
  return {
    rotation: m?.rotation ?? "",
    docsUrl: m?.docsUrl ?? "",
    expiresAt: m?.expiresAt ?? "",
  };
}
