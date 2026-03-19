// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * URL handling utilities
 * Sanitizes and validates URL parameters to prevent injection attacks
 */

/**
 * Sanitize URL parameter value
 * Strips all characters outside a safe whitelist to prevent injection attacks.
 */
export function sanitizeUrlParam(value: string): string {
  // Whitelist: alphanumeric, spaces, and common punctuation used in
  // search queries, tag names, project names, and RGD identifiers.
  return value.replace(/[^a-zA-Z0-9\s\-_.,:/()@+]/g, '').trim().slice(0, 200);
}

/**
 * Safely parse catalog filter state from URL
 */
export function getCatalogFiltersFromURL(): { search: string; tags: string[]; project: string } {
  if (typeof window === "undefined") {
    return { search: "", tags: [], project: "" };
  }

  const params = new URLSearchParams(window.location.search);

  // Validate and sanitize each parameter
  const search = sanitizeUrlParam(params.get("q") || "");
  const project = sanitizeUrlParam(params.get("project") || "");
  const tagsParam = params.get("tags") || "";
  const tags = tagsParam
    .split(",")
    .map(tag => sanitizeUrlParam(tag))
    .filter(Boolean)
    .slice(0, 20); // Limit number of tags to prevent DoS

  return { search, tags, project };
}

/**
 * Safely construct URL with catalog filters
 */
export function setCatalogFiltersToURL(filters: { search: string; tags: string[]; project: string }): void {
  if (typeof window === "undefined") return;

  const params = new URLSearchParams();

  if (filters.search) {
    params.set("q", sanitizeUrlParam(filters.search));
  }
  if (filters.tags.length > 0) {
    const safeTags = filters.tags.map(tag => sanitizeUrlParam(tag));
    params.set("tags", safeTags.join(","));
  }
  if (filters.project) {
    params.set("project", sanitizeUrlParam(filters.project));
  }

  const newURL = params.toString()
    ? `${window.location.pathname}?${params.toString()}`
    : window.location.pathname;

  window.history.replaceState({}, "", newURL);
}

/**
 * Safely parse instance filter state from URL
 */
export function getInstanceFiltersFromURL(): {
  search: string;
  rgd: string;
  health: string;
  project: string;
} {
  if (typeof window === "undefined") {
    return { search: "", rgd: "", health: "", project: "" };
  }

  const params = new URLSearchParams(window.location.search);

  // Validate and sanitize each parameter
  const search = sanitizeUrlParam(params.get("q") || "");
  const rgd = sanitizeUrlParam(params.get("rgd") || "");
  const health = sanitizeUrlParam(params.get("health") || "");
  const project = sanitizeUrlParam(params.get("project") || "");

  return { search, rgd, health, project };
}

/**
 * Safely construct URL with instance filters
 */
export function setInstanceFiltersToURL(filters: {
  search: string;
  rgd: string;
  health: string;
  project: string;
}): void {
  if (typeof window === "undefined") return;

  const params = new URLSearchParams();

  if (filters.search) params.set("q", sanitizeUrlParam(filters.search));
  if (filters.rgd) params.set("rgd", sanitizeUrlParam(filters.rgd));
  if (filters.health) params.set("health", sanitizeUrlParam(filters.health));
  if (filters.project) params.set("project", sanitizeUrlParam(filters.project));

  const newURL = params.toString()
    ? `${window.location.pathname}?${params.toString()}`
    : window.location.pathname;

  window.history.replaceState({}, "", newURL);
}
