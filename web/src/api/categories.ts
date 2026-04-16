// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";
import type { Category, CategoryList } from "@/types/category";

/**
 * List all categories auto-discovered from knodex.io/category annotations.
 * OSS feature — returns categories filtered by the user's Casbin rgds/{category}/* policies.
 * @returns List of visible categories with counts
 */
export async function listCategories(): Promise<CategoryList> {
  const response = await apiClient.get<CategoryList>("/v1/categories");
  return response.data;
}

/**
 * Get a specific category by slug.
 * @param slug - Category slug
 * @returns Category details
 */
export async function getCategory(slug: string): Promise<Category> {
  const response = await apiClient.get<Category>(
    `/v1/categories/${encodeURIComponent(slug)}`
  );
  return response.data;
}
