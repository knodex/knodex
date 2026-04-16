// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import React, { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Clock, Copy, Check } from "@/lib/icons";
import type { Instance } from "@/types/rgd";
import { StatusIndicator } from "@/components/ui/status-indicator";
import { formatDistanceToNow } from "@/lib/date";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import { HEALTH_TO_STATUS, LEFT_BORDER_COLOR, getInstanceUrl } from "./instance-utils";

interface MobileInstanceCardProps {
  instance: Instance;
  onClick?: (instance: Instance) => void;
}

export const MobileInstanceCard = React.memo(function MobileInstanceCard({
  instance,
  onClick,
}: MobileInstanceCardProps) {
  const navigate = useNavigate();
  const status = HEALTH_TO_STATUS[instance.health] ?? "unknown";
  const leftBorderColor = LEFT_BORDER_COLOR[instance.health];
  const serviceUrl = getInstanceUrl(instance);
  const projectLabel = instance.labels?.["knodex.io/project"];
  const age = formatDistanceToNow(instance.createdAt);
  const [copied, setCopied] = useState(false);
  const copiedTimerRef = useRef<ReturnType<typeof setTimeout>>();

  // Clean up timeout on unmount
  useEffect(() => {
    return () => {
      if (copiedTimerRef.current) clearTimeout(copiedTimerRef.current);
    };
  }, []);

  const { namespace, kind, name } = instance;
  const handleClick = useCallback(() => {
    if (onClick) {
      onClick(instance);
      return;
    }
    navigate(`/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`);
  }, [onClick, instance, navigate, namespace, kind, name]);

  const handleCopyUrl = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (!serviceUrl) return;
    if (!navigator.clipboard) {
      toast.error("Clipboard not available");
      return;
    }
    navigator.clipboard.writeText(serviceUrl).then(() => {
      setCopied(true);
      toast.success("Copied!");
      copiedTimerRef.current = setTimeout(() => setCopied(false), 2000);
    }).catch(() => {
      toast.error("Failed to copy");
    });
  }, [serviceUrl]);

  return (
    <div
      data-testid="mobile-instance-card"
      role="button"
      tabIndex={0}
      aria-label={`View details for ${instance.name}`}
      className={cn(
        "cursor-pointer rounded-[var(--radius-token-lg)] p-3",
        "border border-[rgba(255,255,255,0.08)]",
        "bg-[var(--surface-primary)]",
        "transition-[border-color] duration-150",
        "active:bg-[rgba(255,255,255,0.04)]",
        leftBorderColor && "border-l-2"
      )}
      style={leftBorderColor ? { borderLeftColor: leftBorderColor } : undefined}
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          handleClick();
        }
      }}
    >
      {/* Row 1: Large status dot + Name */}
      <div className="flex items-center gap-2.5 mb-1.5">
        <StatusIndicator status={status} className="[&_[data-testid=status-dot]]:!w-3 [&_[data-testid=status-dot]]:!h-3" />
        <h3
          className="min-w-0 truncate text-[15px] font-semibold leading-tight text-[var(--text-primary)]"
          title={instance.name}
        >
          {instance.name}
        </h3>
      </div>

      {/* Row 2: Kind badge + Project + Age */}
      <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--text-secondary)]">
        <span className="rounded bg-[rgba(255,255,255,0.06)] px-1.5 py-0.5 font-mono text-[var(--text-primary)]">
          {instance.kind}
        </span>
        {projectLabel && (
          <span className="truncate max-w-[100px]" title={projectLabel}>
            {projectLabel}
          </span>
        )}
        <span className="inline-flex items-center gap-1 text-[var(--text-muted)]">
          <Clock className="h-3 w-3" />
          {age}
        </span>
      </div>

      {/* Row 3: Copy URL button (if service URL exists) */}
      {serviceUrl && (
        <div className="mt-2 flex items-center">
          <button
            data-testid="copy-url-button"
            onClick={handleCopyUrl}
            className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs text-primary hover:bg-primary/10 transition-colors min-h-[32px]"
            aria-label={`Copy service URL: ${serviceUrl}`}
          >
            {copied ? (
              <Check className="h-3.5 w-3.5 text-green-500" />
            ) : (
              <Copy className="h-3.5 w-3.5" />
            )}
            <span className="truncate max-w-[200px]">{serviceUrl}</span>
          </button>
        </div>
      )}
    </div>
  );
});
