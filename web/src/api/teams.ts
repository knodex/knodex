// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";
import type {
  Team,
  TeamListResponse,
  CreateTeamRequest,
  UpdateTeamRequest,
  ObservedGroupsResponse,
} from "@/types/team";

/**
 * List all teams (operator-gated).
 */
export async function listTeams(): Promise<TeamListResponse> {
  const response = await apiClient.get<TeamListResponse>("/v1/teams");
  return response.data;
}

/**
 * Get a single team by name.
 */
export async function getTeam(name: string): Promise<Team> {
  const response = await apiClient.get<Team>(
    `/v1/teams/${encodeURIComponent(name)}`
  );
  return response.data;
}

/**
 * Create a new team.
 */
export async function createTeam(request: CreateTeamRequest): Promise<Team> {
  const response = await apiClient.post<Team>("/v1/teams", request);
  return response.data;
}

/**
 * Update an existing team (name is immutable).
 */
export async function updateTeam(
  name: string,
  request: UpdateTeamRequest
): Promise<Team> {
  const response = await apiClient.put<Team>(
    `/v1/teams/${encodeURIComponent(name)}`,
    request
  );
  return response.data;
}

/**
 * Delete a team.
 */
export async function deleteTeam(name: string): Promise<void> {
  await apiClient.delete(`/v1/teams/${encodeURIComponent(name)}`);
}

/**
 * List the OIDC groups Knodex has observed at login (Story 10.3), for the
 * group typeahead. Returns an empty list when discovery is unavailable.
 */
export async function listObservedGroups(): Promise<ObservedGroupsResponse> {
  const response = await apiClient.get<ObservedGroupsResponse>(
    "/v1/groups/observed"
  );
  return response.data;
}
