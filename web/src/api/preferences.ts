// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";

export interface UserPreferences {
  favoriteRgds: string[];
  recentRgds: string[];
}

export async function getPreferences(): Promise<UserPreferences> {
  const response = await apiClient.get<UserPreferences>("/v1/users/preferences");
  return response.data;
}

export async function putPreferences(prefs: UserPreferences): Promise<UserPreferences> {
  const response = await apiClient.put<UserPreferences>("/v1/users/preferences", prefs);
  return response.data;
}
