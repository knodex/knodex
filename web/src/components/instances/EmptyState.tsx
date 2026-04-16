// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Box, Search, ArrowRight } from "@/lib/icons";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { EmptyState as BaseEmptyState } from "@/components/ui/empty-state";

interface EmptyStateProps {
  hasFilters: boolean;
  onClearFilters?: () => void;
}

export function EmptyState({ hasFilters, onClearFilters }: EmptyStateProps) {
  if (hasFilters) {
    return (
      <BaseEmptyState
        icon={Search}
        title="No matching instances"
        description="Try adjusting your filters or search query."
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
      title="No instances running yet"
      description="Deploy an RGD from the catalog to create your first instance."
      action={
        <Button asChild>
          <Link to="/catalog">
            Browse Catalog
            <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
        </Button>
      }
    />
  );
}
