/* eslint-disable react-refresh/only-export-components */
import { useState, useCallback } from "react";
import { ChevronDown, ChevronRight, Settings } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { AdvancedSection } from "@/types/rgd";

export interface AdvancedConfigToggleProps {
  /** Advanced section metadata from the schema */
  advancedSection: AdvancedSection | null;
  /** Whether the advanced section is expanded */
  isExpanded: boolean;
  /** Callback when toggle is clicked */
  onToggle: () => void;
  /** Children to render when expanded (the advanced form fields) */
  children: React.ReactNode;
  /** Additional CSS classes */
  className?: string;
}

/**
 * Toggle component for showing/hiding advanced configuration options.
 *
 * Features:
 * - Collapsed by default for cleaner initial form experience
 * - Shows count of advanced options available
 * - Visual distinction with muted background when expanded
 * - Persists toggle state during form session
 */
export function AdvancedConfigToggle({
  advancedSection,
  isExpanded,
  onToggle,
  children,
  className,
}: AdvancedConfigToggleProps) {
  // Don't render if no advanced section or no affected properties
  if (!advancedSection || advancedSection.affectedProperties.length === 0) {
    return null;
  }

  // Count unique top-level advanced properties (excluding parent paths)
  const optionCount = countAdvancedOptions(advancedSection.affectedProperties);

  return (
    <div className={cn("mt-6 border-t border-border pt-4", className)}>
      <button
        type="button"
        onClick={onToggle}
        className={cn(
          "flex w-full items-center gap-2 rounded-md px-2 py-2 text-sm font-medium transition-colors",
          "text-muted-foreground hover:text-foreground hover:bg-muted/50",
          "focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
        )}
        aria-expanded={isExpanded}
        aria-controls="advanced-config-section"
      >
        {isExpanded ? (
          <ChevronDown className="h-4 w-4 shrink-0" />
        ) : (
          <ChevronRight className="h-4 w-4 shrink-0" />
        )}
        <Settings className="h-4 w-4 shrink-0" />
        <span>
          {isExpanded ? "Hide" : "Show"} Advanced Configuration
        </span>
        <Badge variant="outline" className="ml-2">
          {optionCount} {optionCount === 1 ? "option" : "options"}
        </Badge>
      </button>

      {isExpanded && (
        <div
          id="advanced-config-section"
          className="mt-4 rounded-lg bg-muted/30 p-4 border border-border/50"
          role="region"
          aria-label="Advanced configuration options"
        >
          <p className="mb-4 text-sm text-muted-foreground">
            These settings have secure defaults. Only modify if you need custom values.
          </p>
          <div className="space-y-4">
            {children}
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * Count the number of unique top-level advanced options.
 * This filters out parent paths to only count leaf fields.
 *
 * For example:
 * - ["advanced.replicas", "advanced.resources", "advanced.resources.limits.memory"]
 * - Would count: replicas, resources.limits.memory (not resources since it's a parent)
 */
function countAdvancedOptions(affectedProperties: string[]): number {
  // Filter to only include leaf properties (not parents of other properties)
  const leafProperties = affectedProperties.filter((path) => {
    // Check if any other path starts with this path + "."
    const isParent = affectedProperties.some(
      (otherPath) => otherPath !== path && otherPath.startsWith(path + ".")
    );
    return !isParent;
  });

  return leafProperties.length;
}

/**
 * Custom hook for managing advanced config toggle state.
 * Provides consistent state management across the form session.
 */
export function useAdvancedConfigToggle(initialExpanded = false) {
  const [isExpanded, setIsExpanded] = useState(initialExpanded);

  const toggle = useCallback(() => {
    setIsExpanded((prev) => !prev);
  }, []);

  const expand = useCallback(() => {
    setIsExpanded(true);
  }, []);

  const collapse = useCallback(() => {
    setIsExpanded(false);
  }, []);

  return {
    isExpanded,
    toggle,
    expand,
    collapse,
  };
}

export default AdvancedConfigToggle;
