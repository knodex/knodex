// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import {
  Sheet,
  SheetContent,
  SheetTitle,
} from "@/components/ui/sheet";
import { SidebarNav } from "./Sidebar";

interface SidebarDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * SidebarDrawer — overlay navigation drawer for tablet/mobile viewports.
 * Uses Radix Sheet (Dialog) for built-in focus trap, Escape-to-close, and backdrop.
 * 280px wide, full height, slides in from the left.
 */
export function SidebarDrawer({ open, onOpenChange }: SidebarDrawerProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="left"
        className="w-[280px] p-0 bg-[var(--surface-primary)] border-none [&>button:last-child]:hidden"
      >
        <SheetTitle className="sr-only">Navigation menu</SheetTitle>
        <SidebarNav onNavItemClick={() => onOpenChange(false)} />
      </SheetContent>
    </Sheet>
  );
}
