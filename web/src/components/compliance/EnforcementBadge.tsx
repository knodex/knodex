import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { getEnforcementColors } from "@/types/compliance";

interface EnforcementBadgeProps {
  /** Enforcement action: deny, warn, or dryrun */
  action: string;
  /** Optional className for customization */
  className?: string;
}

/**
 * Colored badge for enforcement action display
 * AC-VIO-05: Enforcement action badge (deny=red, warn=yellow, dryrun=blue)
 */
export function EnforcementBadge({ action, className }: EnforcementBadgeProps) {
  const colors = getEnforcementColors(action);

  return (
    <Badge
      variant="outline"
      className={cn(colors.bg, colors.text, colors.border, "font-medium", className)}
    >
      {action}
    </Badge>
  );
}

export default EnforcementBadge;
