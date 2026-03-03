import * as LucideIcons from "lucide-react";
import type { LucideProps } from "lucide-react";

/**
 * Get a Lucide icon component by name.
 * @param name - The icon name (e.g., "flask", "server", "layout-grid")
 * @returns The icon component or a default icon if not found
 */
export function getLucideIcon(name?: string): React.ComponentType<LucideProps> {
  // Return default icon if no name provided
  if (!name) {
    return LucideIcons.LayoutGrid;
  }

  // Convert kebab-case to PascalCase for Lucide icon names
  // e.g., "layout-grid" -> "LayoutGrid"
  const pascalCaseName = name
    .split("-")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join("");

  // Try to find the icon in Lucide exports
  const Icon = (LucideIcons as Record<string, React.ComponentType<LucideProps>>)[pascalCaseName];

  if (Icon) {
    return Icon;
  }

  // Fallback to LayoutGrid if icon not found
  return LucideIcons.LayoutGrid;
}
