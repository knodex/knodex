// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Box, Search } from "@/lib/icons";
import { Link } from "react-router-dom";
import { EmptyState as BaseEmptyState } from "@/components/ui/empty-state";

interface EmptyStateProps {
  hasFilters?: boolean;
  onClearFilters?: () => void;
}

export function EmptyState({ hasFilters = false, onClearFilters }: EmptyStateProps) {
  if (hasFilters) {
    return (
      <BaseEmptyState
        icon={Search}
        title="No results found"
        description="Try adjusting your search or filter criteria."
        action={
          onClearFilters ? (
            <button
              type="button"
              onClick={onClearFilters}
              className="text-[13px] underline underline-offset-2"
              style={{ color: "var(--text-secondary)" }}
            >
              Clear filters
            </button>
          ) : undefined
        }
      />
    );
  }
  return (
    <BaseEmptyState
      icon={Box}
      title="No RGDs found"
      description="ResourceGraphDefinitions will appear here once they are created."
      action={
        <Link
          to="/projects"
          className="text-[13px] underline underline-offset-2"
          style={{ color: "var(--text-secondary)" }}
        >
          Check your project access
        </Link>
      }
    />
  );
}
