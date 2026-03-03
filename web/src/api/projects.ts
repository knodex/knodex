import apiClient from "./client";
import type {
  Project,
  ProjectListResponse,
  CreateProjectRequest,
  UpdateProjectRequest,
} from "@/types/project";

/**
 * List projects the user has access to
 */
export async function listProjects(): Promise<ProjectListResponse> {
  const response = await apiClient.get<ProjectListResponse>("/v1/projects");
  return response.data;
}

/**
 * Get a single project by name
 */
export async function getProject(name: string): Promise<Project> {
  const response = await apiClient.get<Project>(
    `/v1/projects/${encodeURIComponent(name)}`
  );
  return response.data;
}

/**
 * Create a new project
 */
export async function createProject(
  request: CreateProjectRequest
): Promise<Project> {
  const response = await apiClient.post<Project>("/v1/projects", request);
  return response.data;
}

/**
 * Update an existing project
 */
export async function updateProject(
  name: string,
  request: UpdateProjectRequest
): Promise<Project> {
  const response = await apiClient.put<Project>(
    `/v1/projects/${encodeURIComponent(name)}`,
    request
  );
  return response.data;
}

/**
 * Delete a project
 */
export async function deleteProject(name: string): Promise<void> {
  await apiClient.delete(`/v1/projects/${encodeURIComponent(name)}`);
}
