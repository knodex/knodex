/**
 * Project utility functions
 * Shared helpers for working with Projects and their destinations
 */

import type { Project } from "@/types/project";

/**
 * Convert a glob pattern (e.g., "staging*") to a RegExp
 * Supports * as wildcard for zero or more characters
 */
function globToRegex(pattern: string): RegExp {
  // Escape special regex characters except *
  const escaped = pattern.replace(/[.+^${}()|[\]\\]/g, "\\$&");
  // Convert * to regex .*
  const regexPattern = escaped.replace(/\*/g, ".*");
  return new RegExp(`^${regexPattern}$`);
}

/**
 * Check if a namespace matches a pattern (supports glob patterns)
 */
function namespaceMatchesPattern(namespace: string, pattern: string): boolean {
  if (pattern === "*") {
    return true;
  }
  if (pattern.includes("*")) {
    return globToRegex(pattern).test(namespace);
  }
  return namespace === pattern;
}

/**
 * Get allowed namespaces from a project's destinations
 * Extracts unique namespace patterns, excluding pure wildcards
 */
export function getAllowedNamespaces(project: Project): string[] {
  if (!project.destinations || project.destinations.length === 0) {
    return [];
  }

  // Extract unique namespaces from destinations
  const namespaces = new Set<string>();
  project.destinations.forEach((dest) => {
    if (dest.namespace && dest.namespace !== "*") {
      namespaces.add(dest.namespace);
    }
  });

  return Array.from(namespaces);
}

/**
 * Check if a project allows all namespaces (has "*" destination)
 */
export function projectAllowsAllNamespaces(project: Project): boolean {
  return project.destinations?.some((dest) => dest.namespace === "*") ?? false;
}

/**
 * Filter items by project's allowed namespaces
 * If project allows all namespaces (*), returns all items
 * If project has specific namespaces or patterns, filters to only those
 * Supports glob patterns like "staging*", "dev*", etc.
 */
export function filterByProjectNamespaces<T extends { namespace: string }>(
  items: T[],
  project: Project | undefined
): T[] {
  if (!project) {
    return items;
  }

  // If project allows all namespaces, don't filter
  if (projectAllowsAllNamespaces(project)) {
    return items;
  }

  const allowedPatterns = getAllowedNamespaces(project);

  // If no destinations defined, show nothing
  if (allowedPatterns.length === 0) {
    return [];
  }

  // Filter items - namespace must match at least one pattern
  return items.filter((item) =>
    allowedPatterns.some((pattern) =>
      namespaceMatchesPattern(item.namespace, pattern)
    )
  );
}
