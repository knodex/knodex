import React, { useMemo } from "react";
import {
  Database,
  HardDrive,
  Network,
  Server,
  MessageSquare,
  Activity,
  Shield,
  Box,
  Package,
  Cloud,
  Lock,
  Workflow,
  Clock,
  FolderKanban,
} from "lucide-react";
import type { CatalogRGD } from "@/types/rgd";
import { cn } from "@/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const CATEGORY_ICONS: Record<string, typeof Database> = {
  database: Database,
  storage: HardDrive,
  networking: Network,
  network: Network,
  compute: Server,
  messaging: MessageSquare,
  monitoring: Activity,
  security: Shield,
  application: Package,
  app: Package,
  cloud: Cloud,
  auth: Lock,
  workflow: Workflow,
};

/**
 * Renders the appropriate icon for a category
 */
function CategoryIcon({ category, className }: { category: string; className?: string }) {
  const normalized = category.toLowerCase().trim();
  const Icon = CATEGORY_ICONS[normalized] || Box;
  return <Icon className={className} />;
}

interface RGDCardProps {
  rgd: CatalogRGD;
  onClick?: (rgd: CatalogRGD) => void;
}

export const RGDCard = React.memo(function RGDCard({ rgd, onClick }: RGDCardProps) {
  const normalizedCategory = useMemo(
    () => (rgd.category || "uncategorized").toLowerCase(),
    [rgd.category]
  );

  const normalizedTags = useMemo(() => {
    const uniqueTags = [...new Set(rgd.tags?.map((t) => t.toLowerCase()) || [])];
    return uniqueTags.filter((tag) => tag !== normalizedCategory);
  }, [rgd.tags, normalizedCategory]);

  const formattedTime = useMemo(
    () => formatRelativeTime(rgd.updatedAt),
    [rgd.updatedAt]
  );

  return (
    <div
      role="button"
      aria-label={`View details for ${rgd.title || rgd.name}`}
      className={cn(
        "group relative cursor-pointer rounded-lg border border-border/60 bg-card p-5",
        "transition-all duration-200 ease-out",
        "hover:border-primary/30 hover:bg-accent/5"
      )}
      onClick={() => onClick?.(rgd)}
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-3 mb-4">
        <div className="flex items-center gap-3 min-w-0">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <CategoryIcon category={rgd.category || "uncategorized"} className="h-5 w-5" />
          </div>
          <div className="min-w-0">
            <Tooltip>
              <TooltipTrigger asChild>
                <h3 className="font-semibold text-foreground truncate text-base group-hover:text-primary transition-colors duration-200">
                  {rgd.title || rgd.name}
                </h3>
              </TooltipTrigger>
              <TooltipContent>
                <p>{rgd.title && rgd.title !== rgd.name ? `${rgd.title} (${rgd.name})` : rgd.name}</p>
              </TooltipContent>
            </Tooltip>
            {rgd.labels?.["knodex.io/project"] && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <p className="flex items-center gap-1.5 text-xs text-muted-foreground truncate mt-1">
                    <FolderKanban className="h-3 w-3 shrink-0 text-muted-foreground/70" />
                    <span className="truncate">{rgd.labels["knodex.io/project"]}</span>
                  </p>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{rgd.labels["knodex.io/project"]}</p>
                </TooltipContent>
              </Tooltip>
            )}
          </div>
        </div>
        {rgd.version && (
          <span className="shrink-0 px-2 py-0.5 rounded-md text-xs font-mono font-medium text-muted-foreground bg-muted/80">
            {rgd.version}
          </span>
        )}
      </div>

      {/* Description */}
      <Tooltip>
        <TooltipTrigger asChild>
          <p className="text-sm text-muted-foreground line-clamp-2 mb-4 leading-relaxed">
            {rgd.description || "No description available"}
          </p>
        </TooltipTrigger>
        <TooltipContent>
          <p>{rgd.description || "No description available"}</p>
        </TooltipContent>
      </Tooltip>

      {/* Tags */}
      <div className="flex flex-wrap gap-1.5 mb-4">
        <span className="px-2 py-0.5 rounded-md text-xs font-semibold bg-primary/10 text-primary">
          {normalizedCategory}
        </span>
        {normalizedTags.slice(0, 2).map((tag) => (
          <span
            key={tag}
            className="px-2 py-0.5 rounded-md text-xs font-medium text-muted-foreground bg-muted/60"
          >
            {tag}
          </span>
        ))}
        {normalizedTags.length > 2 && (
          <span className="px-2 py-0.5 rounded-md text-xs font-medium text-muted-foreground bg-muted/60">
            +{normalizedTags.length - 2}
          </span>
        )}
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between pt-3 text-xs text-muted-foreground">
        <span className="flex items-center gap-1.5 font-medium">
          <Package className="h-3.5 w-3.5 text-muted-foreground/70" />
          {rgd.instances} instance{rgd.instances !== 1 ? "s" : ""}
        </span>
        <span className="flex items-center gap-1.5 font-medium">
          <Clock className="h-3.5 w-3.5 text-muted-foreground/70" />
          {formattedTime}
        </span>
      </div>
    </div>
  );
});

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 30) return date.toLocaleDateString();
  if (diffDays > 0) return `${diffDays}d ago`;
  if (diffHours > 0) return `${diffHours}h ago`;
  if (diffMins > 0) return `${diffMins}m ago`;
  return "just now";
}
