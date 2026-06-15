// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { FileCode } from "@/lib/icons";
import { Badge } from "@/components/ui/badge";

interface TemplateCardProps {
  name: string;
  title?: string;
  description: string;
  instances: number;
  tags?: string[];
  onDeploy: () => void;
}

/**
 * Grid card for an agent template — the grid-mode counterpart to the templates
 * list row, mirroring AgentCard so the Templates page matches the Agents page.
 * Deploys via the standard /deploy/{name} flow.
 */
export function TemplateCard({
  name,
  title,
  description,
  instances,
  tags,
  onDeploy,
}: TemplateCardProps) {
  const shownTags = (tags ?? []).filter((t) => t.trim());
  return (
    <div
      data-testid="agents-templates-row"
      className="flex flex-col rounded-lg border border-border/60 bg-card p-5"
    >
      <div className="flex items-start gap-3 mb-4">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <FileCode className="h-5 w-5" aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="font-semibold text-foreground line-clamp-2 text-base">
            {title || name}
          </h3>
          <Badge variant="soft" className="mt-1.5">
            {instances} {instances === 1 ? "instance" : "instances"}
          </Badge>
        </div>
      </div>

      <p className="text-sm text-muted-foreground line-clamp-2 mb-4 leading-relaxed flex-1">
        {description || "No description available"}
      </p>

      {shownTags.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mb-4">
          {shownTags.slice(0, 3).map((tag) => (
            <span
              key={tag}
              className="px-2 py-0.5 rounded-md text-xs font-medium text-muted-foreground bg-muted/60"
            >
              {tag}
            </span>
          ))}
          {shownTags.length > 3 && (
            <span className="px-2 py-0.5 rounded-md text-xs font-medium text-muted-foreground bg-muted/60">
              +{shownTags.length - 3}
            </span>
          )}
        </div>
      )}

      <button
        type="button"
        onClick={onDeploy}
        data-testid="deploy-template-button"
        className="inline-flex w-full items-center justify-center rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"
      >
        Deploy
      </button>
    </div>
  );
}
