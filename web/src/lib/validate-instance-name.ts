// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

const INSTANCE_NAME_RE = /^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$/;

/**
 * Validate a Kubernetes-style instance name.
 * Returns an empty string when valid, or a user-facing error message otherwise.
 */
export function validateInstanceName(name: string): string {
  if (!name) return "Instance name is required";
  if (name.length > 63) return "Must be 63 characters or fewer";
  if (!INSTANCE_NAME_RE.test(name)) {
    return "Lowercase letters, numbers, and hyphens only; must start and end with alphanumeric";
  }
  return "";
}
