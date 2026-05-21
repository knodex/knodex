// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect } from "react";

interface DiscardDialogProps {
  hasUnsavedChanges: boolean;
}

// useBlocker requires a data router (createBrowserRouter). The app currently
// uses BrowserRouter, so we fall back to beforeunload-only protection.
export function DiscardDialog({ hasUnsavedChanges }: DiscardDialogProps) {
  useEffect(() => {
    if (!hasUnsavedChanges) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [hasUnsavedChanges]);

  return null;
}
