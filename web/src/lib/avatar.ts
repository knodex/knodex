// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Avatar helpers shared by every user/member surface (Settings → Users on
 * self-hosted/EE and the cloud org roster). Kept framework-free in `lib/` so
 * both the on-prem `UserAvatar` primitive and the cloud `MemberAvatar` derive
 * identical initials + colors from the same seed.
 */

/**
 * Derives up to two uppercase initials from a name, falling back to the email
 * local part. Single-word names use their first two letters; multi-word names
 * use first+last initials; dotted/underscored email local parts split too.
 */
export function initialsFor(name: string | undefined, email: string): string {
  const source = (name ?? "").trim() || email.trim();
  if (!source) return "?";
  const atIndex = source.indexOf("@");
  const base = atIndex > 0 ? source.slice(0, atIndex) : source;
  const parts = base.split(/[\s._-]+/).filter(Boolean);
  if (parts.length === 0) return source[0]!.toUpperCase();
  if (parts.length === 1) return parts[0]!.slice(0, 2).toUpperCase();
  return (parts[0]![0]! + parts[parts.length - 1]![0]!).toUpperCase();
}

/**
 * A small palette of translucent-surface + saturated-foreground class pairs.
 * Each entry is a complete static string (NOT interpolated) so Tailwind keeps
 * them in the build. Tones are picked deterministically from a seed so a given
 * person always gets the same color across renders and pages.
 */
const AVATAR_PALETTE = [
  "bg-teal-500/15 text-teal-700 dark:text-teal-300",
  "bg-rose-500/15 text-rose-700 dark:text-rose-300",
  "bg-sky-500/15 text-sky-700 dark:text-sky-300",
  "bg-violet-500/15 text-violet-700 dark:text-violet-300",
  "bg-amber-500/15 text-amber-700 dark:text-amber-300",
  "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300",
  "bg-fuchsia-500/15 text-fuchsia-700 dark:text-fuchsia-300",
  "bg-indigo-500/15 text-indigo-700 dark:text-indigo-300",
  "bg-cyan-500/15 text-cyan-700 dark:text-cyan-300",
] as const;

/** Deterministic surface+foreground class pair for an avatar seed. */
export function avatarColorClass(seed: string): string {
  const key = seed.trim().toLowerCase() || "?";
  let hash = 0;
  for (let i = 0; i < key.length; i++) {
    hash = (hash * 31 + key.charCodeAt(i)) >>> 0;
  }
  return AVATAR_PALETTE[hash % AVATAR_PALETTE.length]!;
}
