// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * UserAvatar — a colored initials avatar shared by every user/member list. It
 * is purely decorative (`aria-hidden`): the person's name is always rendered as
 * adjacent text by the row, so screen readers announce the human, not the glyph.
 *
 * There is no photo source in the roster, so this is a tinted circle whose color
 * is derived deterministically from the seed (`name` falling back to `email`) —
 * the same person gets the same color everywhere.
 */
import { cn } from "@/lib/utils";
import { avatarColorClass, initialsFor } from "@/lib/avatar";

export interface UserAvatarProps {
  name?: string;
  email: string;
  className?: string;
  /** Override the default test id (cloud rows keep `member-avatar`). */
  testId?: string;
}

export function UserAvatar({ name, email, className, testId }: UserAvatarProps) {
  const seed = (name ?? "").trim() || email;
  return (
    <span
      aria-hidden="true"
      data-testid={testId}
      className={cn(
        "inline-flex h-9 w-9 shrink-0 select-none items-center justify-center rounded-full text-xs font-semibold",
        avatarColorClass(seed),
        className,
      )}
    >
      {initialsFor(name, email)}
    </span>
  );
}
