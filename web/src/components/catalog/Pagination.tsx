// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { ChevronLeft, ChevronRight } from "@/lib/icons";
import { cn } from "@/lib/utils";

interface PaginationProps {
  page: number;
  pageSize: number;
  totalCount: number;
  onPageChange: (page: number) => void;
}

export function Pagination({
  page,
  pageSize,
  totalCount,
  onPageChange,
}: PaginationProps) {
  const totalPages = Math.ceil(totalCount / pageSize);
  const startItem = (page - 1) * pageSize + 1;
  const endItem = Math.min(page * pageSize, totalCount);

  if (totalCount === 0) return null;

  return (
    <nav className="flex items-center justify-between pt-4 border-t border-border">
      <p className="text-sm text-muted-foreground">
        {startItem}-{endItem} of {totalCount}
      </p>

      <div className="flex items-center gap-1">
        <button
          type="button"
          onClick={() => onPageChange(page - 1)}
          disabled={page <= 1}
          className={cn(
            "inline-flex items-center justify-center h-8 w-8 rounded-md text-sm",
            "transition-colors",
            "hover:bg-secondary",
            "disabled:opacity-40 disabled:pointer-events-none"
          )}
          aria-label="Previous page"
        >
          <ChevronLeft className="h-4 w-4" />
        </button>

        <span className="px-3 text-sm text-muted-foreground">
          {page} / {totalPages}
        </span>

        <button
          type="button"
          onClick={() => onPageChange(page + 1)}
          disabled={page >= totalPages}
          className={cn(
            "inline-flex items-center justify-center h-8 w-8 rounded-md text-sm",
            "transition-colors",
            "hover:bg-secondary",
            "disabled:opacity-40 disabled:pointer-events-none"
          )}
          aria-label="Next page"
        >
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>
    </nav>
  );
}
