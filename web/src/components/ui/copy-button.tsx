// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import * as React from "react";
import { Copy, Check } from "@/lib/icons";
import { Button, type ButtonProps } from "@/components/ui/button";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import { cn } from "@/lib/utils";

export interface CopyButtonProps
  extends Omit<ButtonProps, "onClick" | "children"> {
  /** Text written to the clipboard when clicked. */
  text: string;
  /** Label shown next to the icon (omit for an icon-only button). */
  label?: string;
  /** Label shown while in the copied state (defaults to "Copied"). */
  copiedLabel?: string;
  /** How long the copied state persists (ms). */
  resetDelay?: number;
  /** Called when the copy succeeds. */
  onCopied?: () => void;
  /** Called when the copy fails. */
  onCopyError?: (error: unknown) => void;
  /** Extra className for the icon element. */
  iconClassName?: string;
}

/**
 * Shared copy-to-clipboard button with transient Copy → Check feedback.
 * Wraps `useCopyToClipboard`; renders a `Copy` icon by default and a `Check`
 * icon for ~`resetDelay` ms after a successful click.
 */
export function CopyButton({
  text,
  label,
  copiedLabel = "Copied",
  resetDelay,
  onCopied,
  onCopyError,
  iconClassName,
  variant = "outline",
  size = "sm",
  className,
  ...buttonProps
}: CopyButtonProps) {
  const { copied, copy } = useCopyToClipboard({
    resetDelay,
    onSuccess: onCopied,
    onError: onCopyError,
  });

  const handleClick = React.useCallback(
    (e: React.MouseEvent<HTMLButtonElement>) => {
      e.stopPropagation();
      void copy(text);
    },
    [copy, text]
  );

  const iconCls = cn("h-3.5 w-3.5", iconClassName);

  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      onClick={handleClick}
      className={cn(label && "gap-1.5", className)}
      {...buttonProps}
    >
      {copied ? <Check className={iconCls} /> : <Copy className={iconCls} />}
      {label ? (copied ? copiedLabel : label) : null}
    </Button>
  );
}
