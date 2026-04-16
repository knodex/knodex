// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Formats a raw Kubernetes condition message into a user-friendly string.
 *
 * Handles:
 * - Gatekeeper admission webhook denials (validation.gatekeeper.sh)
 * - Generic admission webhook denials
 * - KRO reconciliation error prefixes
 */
export function formatConditionMessage(message: string): string {
  if (!message) return message;

  // Strip KRO reconciliation prefix before parsing the inner error
  const reconciliationMatch = message.match(
    /resource reconciliation failed: apply results contain errors: (.+)/s
  );
  if (reconciliationMatch) {
    return formatConditionMessage(reconciliationMatch[1]);
  }

  // Gatekeeper pattern:
  // admission webhook "validation.gatekeeper.sh" denied the request: [constraint-name] reason
  const gatekeeperMatch = message.match(
    /admission webhook "validation\.gatekeeper\.sh" denied the request: \[([^\]]+)\] (.+)/s
  );
  if (gatekeeperMatch) {
    const constraint = gatekeeperMatch[1];
    const reason = gatekeeperMatch[2].trim();
    return `Blocked by Gatekeeper policy "${constraint}": ${reason}`;
  }

  // Generic admission webhook denial
  const webhookMatch = message.match(
    /admission webhook "([^"]+)" denied the request: (.+)/s
  );
  if (webhookMatch) {
    const webhook = webhookMatch[1];
    const reason = webhookMatch[2].trim();
    return `Blocked by admission webhook "${webhook}": ${reason}`;
  }

  return message;
}
