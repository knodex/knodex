// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient, { ApiError } from "./client";
import type { AgentRun, AgentRunStatus } from "./agent-runs";

/**
 * A past chat conversation, assembled server-side by grouping Knodex-owned run
 * records by conversationId (Story 50.6). Mirrors the server SessionSummary
 * DTO — the heavy run list is omitted; the detail endpoint carries it.
 */
export interface AgentSessionSummary {
  /** Session id — the conversationId, or a run id for legacy singletons. */
  id: string;
  agentType: string;
  /** Agent namespace; empty for built-in agent sessions. */
  agentNamespace: string;
  /** The oldest run's truncated input — identifies the conversation. */
  firstPrompt: string;
  /** Oldest run's invocation time (RFC3339). */
  startedAt: string;
  /** Newest run's invocation time (RFC3339) — the descending sort key. */
  lastActivityAt: string;
  runCount: number;
  /** The newest run's status. */
  status: AgentRunStatus;
}

/** The replay source for one session: its runs ordered oldest→newest. */
export interface AgentSession {
  id: string;
  agentType: string;
  agentNamespace: string;
  runs: AgentRun[];
}

/** Query parameters for GET /api/v1/agents/sessions. */
export interface AgentSessionsListParams {
  agentType?: string;
  page?: number;
  pageSize?: number;
}

/** Response envelope from GET /api/v1/agents/sessions. */
export interface AgentSessionsResponse {
  items: AgentSessionSummary[];
  total: number;
  page: number;
  pageSize: number;
}

/**
 * List chat sessions, most-recent-first. The server groups visible runs into
 * sessions and paginates after the namespace-visibility filter — a user with
 * no project access still sees built-in sessions.
 */
export async function listAgentSessions(
  params?: AgentSessionsListParams
): Promise<AgentSessionsResponse> {
  const queryParams = new URLSearchParams();

  if (params?.agentType) queryParams.append("agentType", params.agentType);
  if (params?.page) queryParams.append("page", params.page.toString());
  if (params?.pageSize) queryParams.append("pageSize", params.pageSize.toString());

  const queryString = queryParams.toString();
  const url = queryString ? `/v1/agents/sessions?${queryString}` : "/v1/agents/sessions";

  const response = await apiClient.get<AgentSessionsResponse>(url);
  return response.data;
}

/**
 * Fetch one session's full run records (oldest→newest) for read-only replay.
 * Returns `null` on 404: a sessionId is minted client-side before the first
 * turn, so a brand-new conversation has no runs yet (and an unknown/not-visible
 * id is the same non-leak 404). Both are an empty chat, not an error — the
 * caller reserves its error state for genuine (non-404) failures.
 */
export async function getAgentSession(id: string): Promise<AgentSession | null> {
  try {
    const response = await apiClient.get<AgentSession>(
      `/v1/agents/sessions/${encodeURIComponent(id)}`
    );
    return response.data;
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      return null;
    }
    throw error;
  }
}
