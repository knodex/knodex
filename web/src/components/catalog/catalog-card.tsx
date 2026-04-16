// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import React from "react";
import type { CatalogRGD } from "@/types/rgd";
import { cn } from "@/lib/utils";
import { RGDIcon } from "@/components/ui/rgd-icon";

interface CatalogCardProps {
  rgd: CatalogRGD;
  onCardClick?: (rgd: CatalogRGD) => void;
}

export const CatalogCard = React.memo(function CatalogCard({
  rgd,
  onCardClick,
}: CatalogCardProps) {
  const handleCardClick = () => {
    onCardClick?.(rgd);
  };

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={`View details for ${rgd.title || rgd.name}`}
      className={cn(
        "group relative flex w-full flex-col cursor-pointer overflow-hidden",
        "border border-[var(--border-default)] bg-[var(--surface-primary)] rounded-[var(--radius-token-lg)]",
        "transition-all duration-200 ease-out",
        "hover:border-[var(--border-hover)] hover:shadow-[var(--shadow-card-hover)] hover:translate-y-[-1px]",
        "focus-visible:border-[var(--border-hover)] focus-visible:shadow-[var(--shadow-card-hover)]",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand-primary)]/40 focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--surface-bg)]"
      )}
      onClick={handleCardClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          handleCardClick();
        }
      }}
    >
      {/* Accent edge — visible on hover */}
      <div
        className="absolute left-0 top-0 bottom-0 w-[3px] opacity-0 group-hover:opacity-100 transition-opacity duration-200"
        style={{ backgroundColor: "var(--brand-primary)" }}
      />

      {/* Header: icon + display name */}
      <div className="flex items-start justify-between gap-3 px-5 pt-5 pb-2">
        <div className="flex items-center gap-3 min-w-0">
          <div
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--radius-token-md)]"
            style={{
              backgroundColor: "rgba(45, 212, 191, 0.08)",
              color: "var(--brand-primary)",
            }}
          >
            <RGDIcon icon={rgd.icon} category={rgd.category || "uncategorized"} className="h-5 w-5" />
          </div>
          <h3
            className="font-semibold leading-snug line-clamp-1 group-hover:text-[var(--brand-primary)] transition-colors duration-200"
            style={{ fontSize: "17px", fontWeight: 600, color: "var(--text-primary)" }}
          >
            {rgd.title || rgd.name}
          </h3>
        </div>
      </div>

      {/* Body: description with fade-out for overflow */}
      <div className="relative flex-1 px-5 pt-1 pb-5 overflow-hidden" style={{ maxHeight: "5.5rem" }}>
        <p
          style={{
            fontSize: "var(--text-size-base)",
            lineHeight: "1.65",
            color: "#b4b4bc",
          }}
        >
          {rgd.description || "No description available"}
        </p>
        {/* Fade overlay — softens truncation instead of hard clip */}
        <div
          className="absolute bottom-0 left-0 right-0 h-10 pointer-events-none"
          style={{
            background: "linear-gradient(to bottom, transparent, var(--surface-primary))",
          }}
        />
      </div>
    </div>
  );
});
