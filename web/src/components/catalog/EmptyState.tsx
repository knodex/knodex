import { Box, Search } from "lucide-react";

interface EmptyStateProps {
  hasFilters?: boolean;
}

export function EmptyState({ hasFilters = false }: EmptyStateProps) {
  if (hasFilters) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="h-12 w-12 rounded-lg bg-secondary flex items-center justify-center mb-4">
          <Search className="h-6 w-6 text-muted-foreground" />
        </div>
        <h3 className="text-base font-medium text-foreground mb-1">
          No results found
        </h3>
        <p className="text-sm text-muted-foreground max-w-sm">
          Try adjusting your search or filter criteria.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="h-12 w-12 rounded-lg bg-secondary flex items-center justify-center mb-4">
        <Box className="h-6 w-6 text-muted-foreground" />
      </div>
      <h3 className="text-base font-medium text-foreground mb-1">
        No resources yet
      </h3>
      <p className="text-sm text-muted-foreground max-w-sm">
        ResourceGraphDefinitions will appear here once they are created.
      </p>
    </div>
  );
}
