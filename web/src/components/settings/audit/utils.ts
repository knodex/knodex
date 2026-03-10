// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/** Convert ISO UTC string back to a local datetime-local input value (YYYY-MM-DDTHH:mm) */
export function isoToLocalDatetime(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
