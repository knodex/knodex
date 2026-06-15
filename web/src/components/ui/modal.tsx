// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Modal — higher-level dialog convenience wrapping the shadcn `Dialog`
 * (Radix-backed). Provides sticky header + scrollable body + sticky footer
 * with focus trap + Esc/overlay close inherited from Radix.
 */

import * as React from "react";

import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export type ModalWidth = "sm" | "md" | "lg" | number;

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  width?: ModalWidth;
  /** Optional className applied to the content wrapper. */
  className?: string;
}

const WIDTH_CLASSES: Record<Exclude<ModalWidth, number>, string> = {
  sm: "max-w-sm",
  md: "max-w-lg",
  lg: "max-w-2xl",
};

export function Modal({
  open,
  onClose,
  title,
  children,
  footer,
  width = "md",
  className,
}: ModalProps) {
  const numericWidth = typeof width === "number" ? width : undefined;
  const widthClass = typeof width === "string" ? WIDTH_CLASSES[width] : "";

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <DialogContent
        aria-modal="true"
        className={cn(
          "flex max-h-[84vh] flex-col gap-0 p-0",
          widthClass,
          className
        )}
        style={numericWidth ? { maxWidth: numericWidth } : undefined}
      >
        <DialogHeader className="sticky top-0 z-10 border-b bg-background px-6 py-4 text-left">
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-y-auto px-6 py-4">{children}</div>
        {footer ? (
          <div className="sticky bottom-0 z-10 flex items-center justify-end gap-2 border-t bg-background px-6 py-3">
            {footer}
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
