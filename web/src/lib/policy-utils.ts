// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Policy utilities for Casbin policy manipulation
 * ArgoCD-aligned policy utilities
 */
import { logger } from '@/lib/logger';

// Available resources in Knodex
export const RESOURCES = [
  "projects",
  "rgds",
  "instances",
  "repositories",
  "settings",
  "compliance",
] as const;

// Available actions (ArgoCD-aligned with Knodex additions)
export const ACTIONS = [
  "get",
  "create",
  "update",
  "delete",
  "list",
  "*", // wildcard for all actions
] as const;

// Permission effects
export const PERMISSIONS = ["allow", "deny"] as const;

export type Resource = (typeof RESOURCES)[number];
export type Action = (typeof ACTIONS)[number];
export type Permission = (typeof PERMISSIONS)[number];

/**
 * Parsed policy rule for UI editing
 */
export interface PolicyRule {
  resource: Resource | string;
  action: Action | string;
  object: string;
  permission: Permission;
}

/**
 * Parse a Casbin policy string into a PolicyRule
 * Format: p, proj:{project}:{role}, {resource}, {action}, {object}, {effect}
 */
export function parsePolicyString(
  policy: string,
  projectId: string,
  roleName: string
): PolicyRule | null {
  // Remove leading "p, " if present
  const cleanPolicy = policy.startsWith("p, ") ? policy.slice(3) : policy;
  const parts = cleanPolicy.split(", ");

  if (parts.length < 5) {
    logger.warn("[PolicyUtils] Invalid policy format:", policy);
    return null;
  }

  const [_subject, resource, action, object, permission] = parts;

  // Verify subject matches expected format
  const expectedSubject = `proj:${projectId}:${roleName}`;
  if (_subject !== expectedSubject) {
    logger.warn(
      `[PolicyUtils] Policy subject mismatch: expected ${expectedSubject}, got ${_subject}`
    );
  }

  return {
    resource: resource || "",
    action: action || "",
    object: object || "",
    permission: (permission as Permission) || "allow",
  };
}

/**
 * Format a PolicyRule back to Casbin policy string
 */
export function formatPolicyString(
  rule: PolicyRule,
  projectId: string,
  roleName: string
): string {
  const subject = `proj:${projectId}:${roleName}`;
  return `p, ${subject}, ${rule.resource}, ${rule.action}, ${rule.object}, ${rule.permission}`;
}

/**
 * RGD category scoping.
 *
 * An `rgds` policy's object segment controls which catalog categories the role
 * can browse. Object `*` grants every category; a category-scoped policy uses
 * object `{slug}/*`, which the server expands to the Casbin object
 * `rgds/{slug}/*` (see FormatRGDObject / addProjectScopedPolicyFromStringWithDests
 * on the Go side). Category visibility in the sidebar and catalog is then a pure
 * keyMatch against that object — no extra enforcement layer. This is the only
 * knob needed to filter role templates by category.
 */
export const RGDS_ALL_CATEGORIES = "*";

/**
 * Derive the category slug (a Select value) from an `rgds` policy object.
 * `*` → RGDS_ALL_CATEGORIES (all categories). `{slug}/*` → `slug`. Any other
 * shape is returned verbatim so unknown/legacy values still round-trip.
 */
export function rgdsObjectToCategory(object: string): string {
  if (!object || object === RGDS_ALL_CATEGORIES) return RGDS_ALL_CATEGORIES;
  const slug = object.endsWith("/*") ? object.slice(0, -2) : object;
  return slug || RGDS_ALL_CATEGORIES;
}

/**
 * Build an `rgds` policy object from a category slug (a Select value).
 * RGDS_ALL_CATEGORIES → `*`. A slug → `{slug}/*`.
 */
export function categoryToRgdsObject(category: string): string {
  if (!category || category === RGDS_ALL_CATEGORIES) return RGDS_ALL_CATEGORIES;
  return `${category}/*`;
}

/**
 * Validate a policy rule
 */
export function validatePolicyRule(rule: PolicyRule): string | null {
  if (!rule.resource) {
    return "Resource is required";
  }
  if (!rule.action) {
    return "Action is required";
  }
  if (!rule.object) {
    return "Object pattern is required";
  }
  if (!["allow", "deny"].includes(rule.permission)) {
    return "Permission must be 'allow' or 'deny'";
  }
  return null;
}
