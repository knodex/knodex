// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import React, { useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { ExternalLink, Copy, Check } from "@/lib/icons";
import type { Instance } from "@/types/rgd";
import { StatusIndicator } from "@/components/ui/status-indicator";
import { formatDistanceToNow } from "@/lib/date";
import { cn } from "@/lib/utils";
import { HEALTH_TO_STATUS, LEFT_BORDER_COLOR, getInstanceUrl, safeHostname } from "./instance-utils";

interface StatusCardProps {
  instance: Instance;
  onClick?: (instance: Instance) => void;
  /** Hide the Kind badge in the card header */
  hideKind?: boolean;
}

export const StatusCard = React.memo(function StatusCard({
  instance,
  onClick,
  hideKind = false,
}: StatusCardProps) {
  const navigate = useNavigate();
  const [copied, setCopied] = React.useState(false);
  const status = HEALTH_TO_STATUS[instance.health] ?? "unknown";
  const leftBorderColor = LEFT_BORDER_COLOR[instance.health];
  const serviceUrl = getInstanceUrl(instance);
  const age = formatDistanceToNow(instance.createdAt);

  const { namespace, kind, name } = instance;
  const handleClick = useCallback(() => {
    if (onClick) {
      onClick(instance);
      return;
    }
    const path = `/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`;
    navigate(path);
  }, [onClick, instance, navigate, namespace, kind, name]);

  const handleUrlClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
  }, []);

  const handleCopyUrl = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (!serviceUrl) return;
    navigator.clipboard.writeText(serviceUrl).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  }, [serviceUrl]);

  return (
    <div
      data-testid="status-card"
      role="button"
      tabIndex={0}
      aria-label={`View details for ${instance.name}`}
      className={cn(
        "group cursor-pointer rounded-[var(--radius-token-lg)]",
        "border bg-[var(--surface-primary)]",
        "transition-all duration-200 ease-out",
        "hover:shadow-[var(--shadow-card-hover)] hover:translate-y-[-1px]",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand-primary)]/40 focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--surface-bg)]",
        leftBorderColor ? "border-l-2 border-[var(--border-default)]" : "border-[var(--border-default)]"
      )}
      style={{
        borderLeftColor: leftBorderColor || undefined,
        borderTopColor: undefined,
      }}
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          handleClick();
        }
      }}
    >
      {/* Header: Status dot + Name + Kind badge */}
      <div className="flex items-center justify-between gap-2 px-3 pt-2.5 pb-1.5">
        <div className="flex items-center gap-1.5 min-w-0">
          <StatusIndicator status={status} />
          <h3
            className="min-w-0 truncate text-sm font-semibold leading-tight text-[var(--text-primary)] tracking-[-0.01em] group-hover:text-white transition-colors duration-200"
            title={instance.name}
          >
            {instance.name}
          </h3>
        </div>
        {!hideKind && (
          <span className="shrink-0 rounded-[var(--radius-token-sm)] bg-[rgba(255,255,255,0.06)] px-1.5 py-0.5 font-mono text-sm font-medium text-[var(--text-secondary)]">
            {instance.kind}
          </span>
        )}
      </div>

      {/* Body: Key-value metadata rows */}
      <div className="px-3 pb-2 space-y-0.5">
        <div className="flex items-center justify-between text-sm">
          <span className="text-[var(--text-muted)]">
            {instance.isClusterScoped ? "Scope" : "Namespace"}
          </span>
          <span className="text-[var(--text-secondary)] truncate max-w-[160px]" title={instance.isClusterScoped ? "cluster" : instance.namespace}>
            {instance.isClusterScoped ? "cluster" : instance.namespace}
          </span>
        </div>
        <div className="flex items-center justify-between text-sm">
          <span className="text-[var(--text-muted)]">Updated</span>
          <span className="text-[var(--text-secondary)]">{age}</span>
        </div>
      </div>

      {/* Footer: Service URL (top border separator) */}
      {serviceUrl && (
        <div
          className="flex items-center justify-between gap-2 px-3 py-1.5"
          style={{ borderTop: "1px solid var(--border-subtle)" }}
        >
          <a
            href={serviceUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-sm font-mono text-[var(--brand-primary)] hover:text-[var(--brand-hover)] truncate transition-colors"
            onClick={handleUrlClick}
            title={serviceUrl}
          >
            <ExternalLink className="h-2.5 w-2.5 shrink-0 opacity-60" />
            <span className="truncate">{safeHostname(serviceUrl)}</span>
          </a>
          <button
            type="button"
            onClick={handleCopyUrl}
            className="shrink-0 p-0.5 rounded text-[var(--text-muted)] hover:text-[var(--text-secondary)] transition-colors"
            aria-label="Copy URL"
          >
            {copied ? <Check className="h-3 w-3 text-[var(--status-healthy)]" /> : <Copy className="h-3 w-3" />}
          </button>
        </div>
      )}
    </div>
  );
});
