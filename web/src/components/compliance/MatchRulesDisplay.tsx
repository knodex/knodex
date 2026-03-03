import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { ConstraintMatch, MatchKind } from "@/types/compliance";

interface MatchRulesDisplayProps {
  /** Constraint match rules to display */
  match: ConstraintMatch;
  /** Optional className for customization */
  className?: string;
  /** Display mode: compact (inline badges) or full (detailed list) */
  variant?: "compact" | "full";
}

/**
 * Displays constraint match rules in a readable format
 * AC-CON-08: Match scope visible (which kinds, namespaces, etc.)
 */
export function MatchRulesDisplay({
  match,
  className,
  variant = "full",
}: MatchRulesDisplayProps) {
  const hasKinds = match.kinds && match.kinds.length > 0;
  const hasNamespaces = match.namespaces && match.namespaces.length > 0;
  const hasScope = match.scope && match.scope !== "*";

  if (!hasKinds && !hasNamespaces && !hasScope) {
    return (
      <p className={cn("text-sm text-muted-foreground", className)}>
        Matches all resources
      </p>
    );
  }

  if (variant === "compact") {
    return <CompactMatchDisplay match={match} className={className} />;
  }

  return (
    <div className={cn("space-y-4", className)}>
      {/* Kinds section */}
      {hasKinds && (
        <div>
          <h4 className="text-sm font-medium text-muted-foreground mb-2">
            Resource Kinds
          </h4>
          <div className="space-y-2">
            {match.kinds!.map((kind, index) => (
              <KindMatchRow key={index} kind={kind} />
            ))}
          </div>
        </div>
      )}

      {/* Namespaces section */}
      {hasNamespaces && (
        <div>
          <h4 className="text-sm font-medium text-muted-foreground mb-2">
            Namespaces
          </h4>
          <div className="flex flex-wrap gap-1.5">
            {match.namespaces!.map((ns) => (
              <Badge
                key={ns}
                variant="secondary"
                className="font-mono text-xs"
              >
                {ns}
              </Badge>
            ))}
          </div>
        </div>
      )}

      {/* Scope section */}
      {hasScope && (
        <div>
          <h4 className="text-sm font-medium text-muted-foreground mb-2">
            Scope
          </h4>
          <Badge variant="outline" className="font-medium">
            {match.scope}
          </Badge>
        </div>
      )}
    </div>
  );
}

/**
 * Displays a single kind match rule with API groups
 */
function KindMatchRow({ kind }: { kind: MatchKind }) {
  const apiGroups = kind.apiGroups.length > 0
    ? kind.apiGroups.map((g) => g || "core").join(", ")
    : "core";

  return (
    <div className="flex flex-wrap items-center gap-2 p-2 bg-muted/50 rounded-md">
      <span className="text-xs text-muted-foreground min-w-16">
        API Groups:
      </span>
      <Badge variant="outline" className="font-mono text-xs">
        {apiGroups}
      </Badge>
      <span className="text-xs text-muted-foreground min-w-12">
        Kinds:
      </span>
      <div className="flex flex-wrap gap-1">
        {kind.kinds.map((k) => (
          <Badge key={k} variant="secondary" className="font-mono text-xs">
            {k}
          </Badge>
        ))}
      </div>
    </div>
  );
}

/**
 * Compact display mode for match rules (inline badges)
 */
function CompactMatchDisplay({
  match,
  className,
}: {
  match: ConstraintMatch;
  className?: string;
}) {
  const kindsText = getKindsSummary(match.kinds);
  const namespacesText = getNamespacesSummary(match.namespaces);

  return (
    <div className={cn("flex flex-wrap items-center gap-2 text-sm", className)}>
      {kindsText && (
        <span className="text-muted-foreground">
          <span className="font-medium text-foreground">{kindsText}</span>
        </span>
      )}
      {namespacesText && (
        <span className="text-muted-foreground">
          in <span className="font-medium text-foreground">{namespacesText}</span>
        </span>
      )}
      {match.scope && match.scope !== "*" && (
        <Badge variant="outline" className="text-xs">
          {match.scope}
        </Badge>
      )}
    </div>
  );
}

/**
 * Generate a summary string for kinds
 */
function getKindsSummary(kinds?: MatchKind[]): string | null {
  if (!kinds || kinds.length === 0) return null;

  const allKinds = kinds.flatMap((k) => k.kinds);
  if (allKinds.length === 0) return null;

  if (allKinds.length <= 3) {
    return allKinds.join(", ");
  }

  return `${allKinds.slice(0, 2).join(", ")} +${allKinds.length - 2} more`;
}

/**
 * Generate a summary string for namespaces
 */
function getNamespacesSummary(namespaces?: string[]): string | null {
  if (!namespaces || namespaces.length === 0) return null;

  if (namespaces.length <= 2) {
    return namespaces.join(", ");
  }

  return `${namespaces.slice(0, 2).join(", ")} +${namespaces.length - 2} more`;
}

export default MatchRulesDisplay;
