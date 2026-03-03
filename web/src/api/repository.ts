import apiClient from "./client";
import type {
  RepositoryConfig,
  RepositoryListResponse,
  CreateRepositoryRequest,
  UpdateRepositoryRequest,
  TestConnectionRequest,
  TestConnectionResponse,
} from "@/types/repository";

// Re-export types for convenience
export type { RepositoryConfig, RepositoryListResponse };

/**
 * List repositories
 * @param projectId Optional project ID to filter repositories
 */
export async function listRepositories(projectId?: string): Promise<RepositoryListResponse> {
  const params = projectId ? { projectId } : undefined;
  const response = await apiClient.get<RepositoryListResponse>(
    "/v1/repositories",
    { params }
  );
  return response.data;
}

/**
 * Get a single repository by ID
 */
export async function getRepository(id: string): Promise<RepositoryConfig> {
  const response = await apiClient.get<RepositoryConfig>(
    `/v1/repositories/${encodeURIComponent(id)}`
  );
  return response.data;
}

/**
 * Create a new repository configuration with inline credentials (ArgoCD-style)
 */
export async function createRepository(
  request: CreateRepositoryRequest
): Promise<RepositoryConfig> {
  const response = await apiClient.post<RepositoryConfig>("/v1/repositories", request);
  return response.data;
}

/**
 * Update an existing repository configuration
 */
export async function updateRepository(
  id: string,
  request: UpdateRepositoryRequest
): Promise<RepositoryConfig> {
  const response = await apiClient.patch<RepositoryConfig>(
    `/v1/repositories/${encodeURIComponent(id)}`,
    request
  );
  return response.data;
}

/**
 * Delete a repository configuration
 */
export async function deleteRepository(id: string): Promise<void> {
  await apiClient.delete(`/v1/repositories/${encodeURIComponent(id)}`);
}

/**
 * Test repository connection with inline credentials (ArgoCD-style)
 */
export async function testConnection(
  request: TestConnectionRequest
): Promise<TestConnectionResponse> {
  const response = await apiClient.post<TestConnectionResponse>(
    "/v1/repositories/test-connection",
    request
  );
  return response.data;
}

/**
 * Test an existing repository's connection
 */
export async function testRepositoryConnection(
  repoId: string
): Promise<TestConnectionResponse> {
  const response = await apiClient.post<TestConnectionResponse>(
    `/v1/repositories/${encodeURIComponent(repoId)}/test`
  );
  return response.data;
}
