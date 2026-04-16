// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { Link } from "react-router-dom";
import { Smartphone, Share2, ArrowLeft } from "@/lib/icons";

export default function DeployDisabled() {
  const handleShare = useCallback(() => {
    const url = window.location.href;

    if (navigator.share) {
      navigator.share({ title: "Knodex Deploy", url }).catch(() => {
        // User cancelled or share failed — fall back to clipboard
        navigator.clipboard?.writeText(url);
      });
    } else {
      navigator.clipboard?.writeText(url);
    }
  }, []);

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] gap-6 text-center px-6">
      <Smartphone className="h-16 w-16 text-muted-foreground" />

      <div className="space-y-2">
        <h1 className="text-xl font-semibold text-foreground">
          Deploy is only available on desktop
        </h1>
        <p className="text-sm text-muted-foreground max-w-[280px]">
          The deploy wizard requires a larger screen. Share this link to open it on a desktop browser.
        </p>
      </div>

      <div className="flex flex-col gap-3 w-full max-w-[240px]">
        <button
          onClick={handleShare}
          className="inline-flex items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium text-white bg-[var(--brand-primary)] hover:bg-[var(--brand-hover)] transition-colors min-h-[44px]"
          data-testid="share-link-button"
        >
          <Share2 className="h-4 w-4" />
          Share this link
        </button>

        <Link
          to="/instances"
          className="inline-flex items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium text-foreground border border-border hover:bg-accent transition-colors min-h-[44px]"
          data-testid="back-to-instances"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Instances
        </Link>
      </div>
    </div>
  );
}
