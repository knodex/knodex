// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Category represents an auto-discovered category from knodex.io/category
 * annotations on live RGDs. Categories are OSS — no enterprise license required.
 */
export interface Category {
  /** The annotation value as found on RGDs */
  name: string;

  /** URL-safe identifier (lowercase category annotation value) */
  slug: string;

  /** Icon identifier. When iconType is "lucide", this is a Lucide icon name (e.g., "server").
   *  When iconType is "svg", this is the slug for GET /api/v1/icons/{slug} (e.g., "cat:infrastructure"). */
  icon: string;

  /** How to render the icon: "lucide" for a Lucide React component, "svg" for a custom SVG from the icons API */
  iconType: "lucide" | "svg";

  /** Number of RGDs with this category annotation */
  count: number;
}

/**
 * CategoryList represents the response from the categories API endpoint.
 */
export interface CategoryList {
  /** List of discovered categories with counts */
  categories: Category[];
}
