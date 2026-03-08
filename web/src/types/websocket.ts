// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * WebSocket message types
 * These types mirror the backend WebSocket message format
 */

/**
 * Message types matching backend websocket package
 */
export type MessageType =
  | "instance_update"
  | "rgd_update"
  | "violation_update"
  | "template_update"
  | "constraint_update"
  | "counts_update"
  | "error"
  | "pong"
  | "subscribed"
  | "unsubscribed"
  | "subscribe"
  | "unsubscribe"
  | "ping";

/**
 * Action types for resource updates
 */
export type Action = "add" | "update" | "delete";

/**
 * Base WebSocket message structure
 */
export interface WebSocketMessage<T = unknown> {
  type: MessageType;
  timestamp: string;
  data?: T;
}

/**
 * Instance update data
 */
export interface InstanceUpdateData {
  action: Action;
  namespace: string;
  name: string;
  instance?: unknown;
  projectId?: string;
}

/**
 * RGD update data
 */
export interface RGDUpdateData {
  action: Action;
  name: string;
  rgd?: unknown;
  projectId?: string;
}

/**
 * Violation update data - matches backend ViolationUpdateData
 * Used for real-time violation updates from OPA Gatekeeper compliance monitoring
 * (Enterprise-only feature)
 */
export interface ViolationUpdateData {
  /** Action indicates whether the violation was detected (add) or resolved (delete) */
  action: Action;

  /** Kind of constraint that was violated (e.g., K8sRequiredLabels) */
  constraintKind: string;

  /** Name of the constraint that was violated */
  constraintName: string;

  /** Kubernetes resource that has the violation */
  resource: ViolationResourceData;

  /** Human-readable violation message from the Rego policy */
  message: string;

  /** What happens on violation: deny, dryrun, or warn */
  enforcementAction: string;
}

/**
 * Resource data for violation updates
 */
export interface ViolationResourceData {
  /** Resource kind (e.g., Pod, Deployment) */
  kind: string;

  /** Resource namespace (empty for cluster-scoped) */
  namespace: string;

  /** Resource name */
  name: string;

  /** API group of the resource (empty for core API group) */
  apiGroup?: string;
}

/**
 * Template update data - matches backend TemplateUpdateData
 * Used for real-time constraint template updates from OPA Gatekeeper
 * (Enterprise-only feature)
 */
export interface TemplateUpdateData {
  /** Action indicates whether the template was added, updated, or removed */
  action: Action;

  /** Name of the ConstraintTemplate */
  name: string;

  /** Constraint kind produced by this template (e.g., K8sRequiredLabels) */
  kind: string;

  /** Human-readable description of the template */
  description?: string;
}

/**
 * Constraint update data - matches backend ConstraintUpdateData
 * Used for real-time constraint updates from OPA Gatekeeper
 * (Enterprise-only feature)
 */
export interface ConstraintUpdateData {
  /** Action indicates whether the constraint was added, updated, or removed */
  action: Action;

  /** Constraint kind (determined by the ConstraintTemplate) */
  kind: string;

  /** Name of the constraint */
  name: string;

  /** What happens on violation: deny, dryrun, or warn */
  enforcementAction: string;

  /** Current number of violations */
  violationCount: number;
}

/**
 * Counts update data - pushed via WebSocket for sidebar badge updates
 */
export interface CountsUpdateData {
  rgdCount: number;
  instanceCount: number;
}

/**
 * Error data from server
 */
export interface ErrorData {
  code: string;
  message: string;
}

/**
 * Subscription request data
 */
export interface SubscribeData {
  /** Resource type: "instance", "instances", "rgd", "rgds", "violations", "all" */
  resourceType: string;
  /** Optional namespace filter (for instances) */
  namespace?: string;
  /** Optional name filter */
  name?: string;
}

/**
 * Subscription confirmation data
 */
export interface SubscriptionConfirmData {
  resourceType: string;
  namespace?: string;
  name?: string;
  success: boolean;
}

/**
 * Connection status for WebSocket hooks
 */
export type ConnectionStatus = "connecting" | "connected" | "disconnected" | "error";

/**
 * Valid subscription resource types
 */
export type SubscriptionResourceType =
  | "all"
  | "instance"
  | "instances"
  | "rgd"
  | "rgds"
  | "violations";
