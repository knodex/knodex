// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient, { ApiError } from "./client";

/** Agent run lifecycle status (Story 49.4). */
export type AgentRunStatus = "running" | "completed" | "failed";

/**
 * A single agent invocation record, owned by Knodex (mirrors the server DTO).
 * Written at invocation time and updated when the A2A response returns —
 * there is no kagent CRD behind this.
 */
export interface AgentRun {
  id: string;
  /** Authenticated Knodex user identity at invocation time (email). */
  actor: string;
  /** Agent identifier — the Agent CR name. */
  agentType: string;
  /** Namespace of the Agent CR; empty when unknown. */
  agentNamespace: string;
  /** Optional context reference, e.g. "instance:.." or "rgd:{name}". */
  contextRef: string;
  /** kagent session id (A2A contextId); empty while running. */
  kagentSessionId: string;
  inputSummary: string;
  recommendationSummary: string;
  /** Reserved for future stories; always empty in 49.4. */
  actionTaken: string;
  /** Invocation time (RFC3339). */
  timestamp: string;
  /** Terminal-state time; absent while running. */
  completedAt?: string;
  status: AgentRunStatus;
  /** How the run was triggered; "on_demand" in 49.4. */
  triggerType: string;
}

/** Query parameters for GET /api/v1/agents/runs. */
export interface AgentRunsListParams {
  agentType?: string;
  status?: AgentRunStatus;
  page?: number;
  pageSize?: number;
}

/** Response envelope from GET /api/v1/agents/runs. */
export interface AgentRunsResponse {
  items: AgentRun[];
  total: number;
  page: number;
  pageSize: number;
}

/**
 * List agent runs, newest-first. The server applies the Casbin-derived
 * namespace visibility filter before paginating — page numbers are stable
 * per-caller. A user with no project access sees an empty list.
 */
export async function listAgentRuns(params?: AgentRunsListParams): Promise<AgentRunsResponse> {
  const queryParams = new URLSearchParams();

  if (params?.agentType) queryParams.append("agentType", params.agentType);
  if (params?.status) queryParams.append("status", params.status);
  if (params?.page) queryParams.append("page", params.page.toString());
  if (params?.pageSize) queryParams.append("pageSize", params.pageSize.toString());

  const queryString = queryParams.toString();
  const url = queryString ? `/v1/agents/runs?${queryString}` : "/v1/agents/runs";

  const response = await apiClient.get<AgentRunsResponse>(url);
  return response.data;
}

/**
 * Invoke an agent via the namespaced kagent A2A route (Story 53.2). The caller
 * gets a 202 with the run record (status "running"); the agent call completes
 * asynchronously on the server. An optional conversationId (Story 50.6) groups
 * this turn into a chat session with the other turns of the same mounted chat.
 * An optional kagentContextId resumes an existing kagent session so the agent
 * retains memory of prior turns — obtained from the previous result's
 * kagentSessionId field.
 */
export async function invokeAgent(
  namespace: string,
  name: string,
  message: string,
  conversationId?: string,
  kagentContextId?: string
): Promise<AgentRun> {
  const body: { message: string; conversationId?: string; kagentContextId?: string } = { message };
  if (conversationId) body.conversationId = conversationId;
  if (kagentContextId) body.kagentContextId = kagentContextId;
  const response = await apiClient.post<AgentRun>(
    `/v1/agents/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/invoke`,
    body
  );
  return response.data;
}

/** Policy-validation status values (Story 50.3). */
export type PolicyValidationStatus = "passed" | "failed" | "unavailable";

/**
 * One Gatekeeper constraint violation against a generated spec (Story 50.3).
 * All strings are cluster-sourced and UNTRUSTED — render as escaped text
 * only, never as HTML.
 */
export interface PolicyViolation {
  /** Violated constraint's name (may be empty for unparseable denials). */
  constraint: string;
  /** Constraint kind, enriched from the EE constraint cache. */
  constraintKind?: string;
  /** Enforcement action ("deny", "warn", ...). */
  enforcementAction?: string;
  /** Human-readable violation message from the policy. */
  message: string;
  /** Originating spec.resources[].id; absent for the RGD object itself. */
  resourceId?: string;
}

/**
 * Gatekeeper policy-validation outcome on a run result (Story 50.3,
 * Enterprise). ABSENT on OSS builds and on unlicensed EE builds — the UI
 * keys its Enterprise-license notice on that absence (never on a build
 * flag).
 */
export interface PolicyValidation {
  status: PolicyValidationStatus;
  /** Stable explanation for "unavailable" (e.g. "Gatekeeper unreachable"). */
  reason?: string;
  /** Constraint violations when status is "failed". */
  violations?: PolicyViolation[];
  /** Full agent text of the single automatic revision attempt. */
  revisedResponse?: string;
  /** Re-validation outcome of the revised spec. */
  revisedStatus?: PolicyValidationStatus;
  /** Violations of the revised spec when revisedStatus is "failed". */
  revisedViolations?: PolicyViolation[];
}

/**
 * The full terminal payload of an agent run (Story 50.1) — the complete
 * agent response text, unlike the run record's 1024-char
 * recommendationSummary.
 */
export interface AgentRunResult {
  runId: string;
  /** Mirrors the run record's namespace; empty when unknown. */
  agentNamespace: string;
  /** Terminal status — results only exist once the run is terminal. */
  status: "completed" | "failed";
  /** Full agent response text (completed runs). */
  response: string;
  /** Failure description (failed runs). */
  error: string;
  /** Terminal-state time (RFC3339). */
  completedAt: string;
  /** Gatekeeper policy validation (Story 50.3); absent on OSS/unlicensed. */
  policyValidation?: PolicyValidation;
  /**
   * True when the agent entered the A2A "input-required" state: the response
   * text contains clarifying questions and the agent is waiting for answers
   * before generating a spec. The user should type their answers in the chat
   * input to continue.
   */
  inputRequired?: boolean;
  /**
   * Structured data parts from the A2A response. Present when inputRequired is
   * true. Each element is a raw JSON object — in practice kagent emits an
   * `adk_request_confirmation` object whose questions live at
   * args.originalFunctionCall.args.questions[]. Parse with
   * extractKagentQuestions() in AgentChatPage.
   */
  dataParts?: unknown[];
  /**
   * The A2A contextId returned by kagent for this run. Pass as kagentContextId
   * on the NEXT invoke request so kagent resumes the same session and the
   * agent retains memory of prior turns. Absent when the result has expired.
   */
  kagentSessionId?: string;
  /** LLM token counts from kagent usage metadata. Zero/absent when not reported. */
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
}

/**
 * Fetch the full result of an agent run. Returns `null` while the run is
 * still in-flight (the server answers 404 until the result is persisted) —
 * callers poll until non-null or their own timeout fires.
 */
export async function getAgentRunResult(runId: string): Promise<AgentRunResult | null> {
  try {
    const response = await apiClient.get<AgentRunResult>(
      `/v1/agents/runs/${encodeURIComponent(runId)}/result`
    );
    return response.data;
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      return null;
    }
    throw error;
  }
}
