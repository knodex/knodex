// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";

export interface Settings {
  organization: string;
}

export async function getSettings(): Promise<Settings> {
  const response = await apiClient.get<Settings>("/v1/settings");
  return response.data;
}
