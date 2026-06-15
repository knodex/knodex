// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Avatar — deterministic per-user hue from a 7-tone palette.
 *
 * Background uses `hsl(var(--avatar-tone-N-bg-hsl) / 0.18)`; foreground uses
 * `var(--avatar-tone-N-fg)`. Hue is derived from the seed via the prototype's
 * `(h * 31 + char) >>> 0` hash so it is stable across renders.
 */

import * as React from "react";

import { cn } from "@/lib/utils";

const AVATAR_TONE_COUNT = 7;

const SIZE_CLASSES = {
  sm: "h-6 w-6 text-[10px]",
  md: "h-9 w-9 text-[13px]",
  lg: "h-11 w-11 text-[16px]",
} as const;

export type AvatarSize = keyof typeof SIZE_CLASSES;

export interface AvatarProps
  extends Omit<React.HTMLAttributes<HTMLSpanElement>, "aria-label"> {
  name?: string;
  email?: string;
  size?: AvatarSize;
}

/**
 * Deterministic 1-based tone index (1..7) for a given seed string.
 *
 * Matches the prototype `Primitives.jsx:Avatar` hash exactly:
 *   `(h * 31 + ch) >>> 0` accumulator, modulo 7.
 */
// eslint-disable-next-line react-refresh/only-export-components -- hash helper colocated with Avatar
export function avatarToneIndex(seed: string): number {
  let h = 0;
  for (let i = 0; i < seed.length; i++) {
    h = ((h * 31) + seed.charCodeAt(i)) >>> 0;
  }
  return (h % AVATAR_TONE_COUNT) + 1;
}

/** Initials derivation: split on whitespace/dot/at, take up to 2 chars, uppercase. */
// eslint-disable-next-line react-refresh/only-export-components -- initials helper colocated with Avatar
export function avatarInitials(seed: string): string {
  if (!seed) return "?";
  const parts = seed.split(/[\s.@]+/).filter(Boolean);
  if (parts.length === 0) return "?";
  const letters = parts
    .slice(0, 2)
    .map((p) => p.charAt(0))
    .join("")
    .toUpperCase();
  return letters || "?";
}

export const Avatar = React.forwardRef<HTMLSpanElement, AvatarProps>(
  ({ name, email, size = "md", className, style, ...props }, ref) => {
    const seed = name || email || "";
    const tone = seed ? avatarToneIndex(seed) : 1;
    const initials = avatarInitials(seed);
    const ariaLabel = name || email || "?";

    const toneStyle: React.CSSProperties = {
      backgroundColor: `hsl(var(--avatar-tone-${tone}-bg-hsl) / 0.18)`,
      color: `var(--avatar-tone-${tone}-fg)`,
      ...style,
    };

    return (
      <span
        ref={ref}
        role="img"
        aria-label={ariaLabel}
        data-tone={tone}
        className={cn(
          "inline-flex items-center justify-center rounded-full font-medium select-none",
          SIZE_CLASSES[size],
          `kd-avatar-tone-${tone}`,
          className
        )}
        style={toneStyle}
        {...props}
      >
        {initials}
      </span>
    );
  }
);
Avatar.displayName = "Avatar";
