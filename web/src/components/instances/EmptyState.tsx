// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link } from "react-router-dom";
import { ArrowRight, Box, Search } from "lucide-react";
import { Button } from "@/components/ui/button";

interface EmptyStateProps {
  hasFilters: boolean;
}

export function EmptyState({ hasFilters }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-secondary mb-4">
        {hasFilters ? (
          <Search className="h-6 w-6 text-muted-foreground" />
        ) : (
          <Box className="h-6 w-6 text-muted-foreground" />
        )}
      </div>
      <h3 className="text-base font-medium text-foreground mb-1">
        {hasFilters ? "No matching instances" : "No instances deployed"}
      </h3>
      <p className="text-sm text-muted-foreground max-w-sm mb-4">
        {hasFilters
          ? "Try adjusting your filters or search query."
          : "Deploy an RGD from the catalog to create your first instance."}
      </p>
      {!hasFilters && (
        <Button asChild>
          <Link to="/catalog">
            Browse Catalog
            <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
        </Button>
      )}
    </div>
  );
}
