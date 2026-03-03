/**
 * View represents a custom view configuration.
 * Views are defined in a ConfigMap and provide filtered views of the RGD catalog.
 */
export interface View {
  /** Display name shown in the sidebar and page header */
  name: string;

  /** URL-safe identifier used in routing (e.g., "testing" -> /views/testing) */
  slug: string;

  /** Lucide icon name to display in the sidebar */
  icon: string;

  /** Value to match against knodex.io/category annotation */
  category: string;

  /** Sidebar display order (lower values appear first) */
  order: number;

  /** Optional description shown on the view page */
  description?: string;

  /** Number of RGDs matching this view's category */
  count: number;
}

/**
 * ViewList represents the response from the views API endpoint.
 */
export interface ViewList {
  /** List of configured views with counts */
  views: View[];
}
