// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Deployment History types for tracking instance lifecycle events
 */

/**
 * DeploymentEventType represents the type of deployment event
 */
export type DeploymentEventType =
  | "Created"
  | "ManifestGenerated"
  | "PushedToGit"
  | "WaitingForSync"
  | "Synced"
  | "Creating"
  | "Ready"
  | "Degraded"
  | "Failed"
  | "Deleted"
  | "Updated"
  | "StatusChanged";

/**
 * DeploymentMode represents how the instance was deployed
 */
export type DeploymentMode = "direct" | "gitops" | "hybrid";

/**
 * DeploymentEvent represents a single event in the deployment history
 */
export interface DeploymentEvent {
  id: string;
  timestamp: string;
  eventType: DeploymentEventType;
  status: string;
  user?: string;
  deploymentMode?: DeploymentMode;
  gitCommitSha?: string;
  gitRepository?: string;
  gitBranch?: string;
  details?: Record<string, unknown>;
  message?: string;
}

/**
 * DeploymentHistory contains the full deployment history for an instance
 */
export interface DeploymentHistory {
  instanceId: string;
  instanceName: string;
  namespace: string;
  rgdName: string;
  events: DeploymentEvent[];
  createdAt: string;
  currentStatus: string;
  deploymentMode: DeploymentMode;
  lastGitCommit?: string;
}

/**
 * TimelineEntry represents a simplified timeline entry for UI display
 */
export interface TimelineEntry {
  timestamp: string;
  eventType: DeploymentEventType;
  status: string;
  user?: string;
  message?: string;
  gitCommitUrl?: string;
  isCompleted: boolean;
  isCurrent: boolean;
}

/**
 * TimelineResponse from the timeline API
 */
export interface TimelineResponse {
  namespace: string;
  name: string;
  timeline: TimelineEntry[];
}

/**
 * HistoryExportFormat for export requests
 */
export type HistoryExportFormat = "csv" | "json";

/**
 * Event type display information
 */
export const EVENT_TYPE_INFO: Record<
  DeploymentEventType,
  { label: string; color: string; icon: string }
> = {
  Created: { label: "Created", color: "blue", icon: "plus-circle" },
  ManifestGenerated: {
    label: "Manifest Generated",
    color: "purple",
    icon: "file-code",
  },
  PushedToGit: { label: "Pushed to Git", color: "orange", icon: "git-commit" },
  WaitingForSync: {
    label: "Waiting for Sync",
    color: "yellow",
    icon: "clock",
  },
  Synced: { label: "Synced", color: "cyan", icon: "refresh-cw" },
  Creating: { label: "Creating", color: "blue", icon: "loader" },
  Ready: { label: "Ready", color: "green", icon: "check-circle" },
  Degraded: { label: "Degraded", color: "yellow", icon: "alert-triangle" },
  Failed: { label: "Failed", color: "red", icon: "x-circle" },
  Deleted: { label: "Deleted", color: "gray", icon: "trash-2" },
  Updated: { label: "Updated", color: "blue", icon: "edit" },
  StatusChanged: { label: "Status Changed", color: "purple", icon: "activity" },
};

/**
 * Get the display info for an event type
 */
export function getEventTypeInfo(eventType: DeploymentEventType) {
  return EVENT_TYPE_INFO[eventType] || EVENT_TYPE_INFO.StatusChanged;
}

/**
 * Format a timestamp for display
 */
export function formatEventTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  return date.toLocaleString();
}

/**
 * Get relative time from timestamp
 */
export function getRelativeTime(timestamp: string): string {
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();

  const seconds = Math.floor(diffMs / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) return `${days}d ago`;
  if (hours > 0) return `${hours}h ago`;
  if (minutes > 0) return `${minutes}m ago`;
  return "just now";
}
