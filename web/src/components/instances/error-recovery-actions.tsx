// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { AlertCircle, Copy, RotateCcw, Trash2 } from "@/lib/icons";
import { toast } from "sonner";
import type { Instance } from "@/types/rgd";

// --- K8s error mapping ---

const K8S_ERROR_MAP: Record<string, string> = {
  CrashLoopBackOff: "The container keeps crashing and restarting. Check the application logs for errors.",
  ImagePullBackOff: "Unable to pull the container image. Verify the image name and registry access.",
  ErrImagePull: "Failed to download the container image. Check the image reference.",
  OOMKilled: "The container ran out of memory. Consider increasing the memory limit.",
  CreateContainerConfigError: "Invalid container configuration. Review your deployment values.",
  Pending: "Resources are waiting to be scheduled. Check cluster capacity.",
  Evicted: "The pod was evicted due to resource pressure. Check node resources.",
};

// eslint-disable-next-line react-refresh/only-export-components -- Utility co-located with component
export function humanizeError(rawMessage: string): string {
  for (const [key, friendly] of Object.entries(K8S_ERROR_MAP)) {
    if (rawMessage.includes(key)) return friendly;
  }
  return "Something went wrong. Check the resource details for more information.";
}

// --- Needs Attention Banner ---

interface NeedsAttentionBannerProps {
  failedCount: number;
  onScrollToFailed?: () => void;
}

export function NeedsAttentionBanner({ failedCount, onScrollToFailed }: NeedsAttentionBannerProps) {
  if (failedCount === 0) return null;

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={`${failedCount} resource${failedCount !== 1 ? "s" : ""} need attention - click to scroll`}
      className="flex items-center gap-2 rounded-lg p-3 border cursor-pointer"
      style={{
        backgroundColor: "rgba(244,63,94,0.1)",
        borderColor: "rgba(244,63,94,0.5)",
      }}
      onClick={onScrollToFailed}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onScrollToFailed?.(); } }}
    >
      <AlertCircle className="h-4 w-4 shrink-0" style={{ color: "var(--status-error)" }} />
      <span className="text-sm" style={{ color: "#fda4af" }}>
        {failedCount} resource{failedCount !== 1 ? "s" : ""} need{failedCount === 1 ? "s" : ""} attention
      </span>
    </div>
  );
}

// --- Copy Error Button ---

interface CopyErrorButtonProps {
  rawMessage: string;
}

export function CopyErrorButton({ rawMessage }: CopyErrorButtonProps) {
  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(rawMessage);
      toast.success("Error copied to clipboard");
    } catch {
      toast.error("Failed to copy");
    }
  }, [rawMessage]);

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="flex items-center gap-1 text-xs px-2 py-1 rounded"
      style={{ color: "var(--text-muted)" }}
    >
      <Copy className="h-3 w-3" />
      Copy error
    </button>
  );
}

// --- Recovery Action Buttons ---

interface RecoveryActionsProps {
  instance: Instance;
  rgdName: string;
  onDeleteClick?: () => void;
}

export function RecoveryActions({ instance, rgdName, onDeleteClick }: RecoveryActionsProps) {
  const navigate = useNavigate();

  const handleRedeploy = useCallback(() => {
    navigate(`/deploy/${encodeURIComponent(rgdName)}`, {
      state: { prefill: true, instanceId: instance.name, namespace: instance.namespace },
    });
  }, [navigate, rgdName, instance]);

  return (
    <div className="flex gap-2 mt-3">
      <button
        type="button"
        onClick={handleRedeploy}
        className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium border transition-colors"
        style={{ color: "var(--text-secondary)", borderColor: "rgba(255,255,255,0.1)" }}
      >
        <RotateCcw className="h-3.5 w-3.5" />
        Redeploy with changes
      </button>
      {onDeleteClick && (
        <button
          type="button"
          onClick={onDeleteClick}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors"
          style={{ color: "var(--status-error)" }}
        >
          <Trash2 className="h-3.5 w-3.5" />
          Delete instance
        </button>
      )}
    </div>
  );
}
