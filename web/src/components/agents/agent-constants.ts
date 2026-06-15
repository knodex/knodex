// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Fixed name of the agent-wrapping RGD shipped at
 * deploy/charts/knodex/files/agents/kagent-agent.yaml.
 * The Agents tab's Create Agent button opens the Deploy drawer bound to this
 * RGD by name (the RGD is excluded from the catalog list, so the web references
 * it by its known name rather than a discovery query). Component-free module
 * (react-refresh cleanliness).
 */
export const AGENT_WRAPPER_RGD = "kagent-agent";
