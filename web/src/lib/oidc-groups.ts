// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Shared client-side validation for OIDC group identifiers, used by the
 * GroupTypeahead (Team editor). This is a convenience check only — the server
 * is the source of truth (Story 10.1 ValidateTeamSpec).
 *
 * Returns an error message, or null when the group is valid.
 */
export function validateGroupId(groupId: string): string | null {
  if (!groupId.trim()) {
    return "Group ID cannot be empty";
  }

  // No leading/trailing whitespace.
  if (groupId.trim() !== groupId) {
    return "Group ID cannot have leading/trailing spaces";
  }

  // Mirror the server's MaxOIDCGroupLength (253) so client and server agree
  // (Story 10.1 ValidateTeamSpec / Project CRD roles[].groups[] maxLength).
  if (groupId.length > 253) {
    return "Group ID is too long (max 253 characters)";
  }

  // Permissive: UUIDs (Azure AD Object IDs), alphanumeric with dashes,
  // underscores, dots, @ and slashes. Different IdPs use different formats.
  if (!/^[a-zA-Z0-9._@\-/]+$/.test(groupId)) {
    return "Group ID contains invalid characters";
  }

  return null;
}
