/**
 * Project types for the dashboard
 * Projects are ArgoCD-compatible constructs that define deployment boundaries
 */

/**
 * Destination defines an allowed deployment target
 * Simplified for single-cluster deployments
 */
export interface Destination {
  /** Target namespace (supports wildcards: "*", "dev-*", "team-*") */
  namespace?: string;
  /** Optional friendly name for the destination */
  name?: string;
}

/**
 * Role defines a set of policies for a project
 */
export interface ProjectRole {
  /** Unique name of the role within the project */
  name: string;
  /** Human-readable description of the role */
  description?: string;
  /** Policy strings defining permissions */
  policies?: string[];
  /** OIDC groups assigned to this role */
  groups?: string[];
}

/**
 * Project represents an ArgoCD-compatible project
 */
export interface Project {
  /** Unique identifier for the project (DNS-1123 subdomain format) */
  name: string;
  /** Human-readable description of the project */
  description?: string;
  /** Allowed deployment destinations */
  destinations?: Destination[];
  /** Roles defined in this project */
  roles?: ProjectRole[];
  /** Version used for optimistic locking */
  resourceVersion: string;
  /** When the project was created */
  createdAt: string;
  /** Who created the project */
  createdBy?: string;
  /** When the project was last updated */
  updatedAt?: string;
  /** Who last updated the project */
  updatedBy?: string;
}

/**
 * Request body for creating a project
 */
export interface CreateProjectRequest {
  /** Unique identifier for the project (DNS-1123 subdomain format) */
  name: string;
  /** Human-readable description of the project */
  description?: string;
  /** Allowed deployment destinations */
  destinations?: Destination[];
  /** Roles to create with the project */
  roles?: ProjectRole[];
}

/**
 * Request body for updating a project
 * Added roles for project admin to update policies and groups
 */
export interface UpdateProjectRequest {
  /** Human-readable description of the project */
  description?: string;
  /** Allowed deployment destinations */
  destinations?: Destination[];
  /** Roles to update (project admins can update role policies and groups) */
  roles?: ProjectRole[];
  /** Resource version for optimistic locking */
  resourceVersion: string;
}

/**
 * Response from listing projects
 */
export interface ProjectListResponse {
  /** List of projects */
  items: Project[];
  /** Total number of projects */
  totalCount: number;
}
