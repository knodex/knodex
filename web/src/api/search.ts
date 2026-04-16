// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";

export interface RGDSearchResult {
  name: string;
  displayName: string;
  category: string;
  description: string;
}

export interface InstanceSearchResult {
  name: string;
  project: string;
  namespace: string;
  status: string;
  kind: string;
}

export interface ProjectSearchResult {
  name: string;
  description: string;
}

export interface SearchResponse {
  results: {
    rgds: RGDSearchResult[];
    instances: InstanceSearchResult[];
    projects: ProjectSearchResult[];
  };
  query: string;
  totalCount: number;
}

export async function searchAll(query: string): Promise<SearchResponse> {
  const response = await apiClient.get<SearchResponse>("/v1/search", {
    params: { q: query },
  });
  return response.data;
}
