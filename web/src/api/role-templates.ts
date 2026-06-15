// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";

/**
 * RoleTemplate is a reusable PROJECT-role preset (Story 18.1), served from the
 * operator-managed catalog at /v1/settings/role-templates.
 *
 * Policy strings keep their {project}/{role} placeholders — they are resolved
 * client-side (resolvePreset / resolvePresetPolicies in @/lib/role-presets)
 * when the template is applied to a project. A template is only a UI seed: it
 * is copied into Project.spec.roles[] at create/edit time and never enforced
 * directly.
 */
export interface RoleTemplate {
  name: string;
  label: string;
  description?: string;
  policies: string[];
}

interface RoleTemplateListResponse {
  templates: RoleTemplate[];
}

/** List the role-template catalog (operator-gated; 403 surfaces as an error). */
export async function listRoleTemplates(): Promise<RoleTemplate[]> {
  const response = await apiClient.get<RoleTemplateListResponse>(
    "/v1/settings/role-templates"
  );
  return response.data.templates ?? [];
}

/** Get a single template by name. */
export async function getRoleTemplate(name: string): Promise<RoleTemplate> {
  const response = await apiClient.get<RoleTemplate>(
    `/v1/settings/role-templates/${encodeURIComponent(name)}`
  );
  return response.data;
}

/** Create a new template (409 if the name already exists). */
export async function createRoleTemplate(
  template: RoleTemplate
): Promise<RoleTemplate> {
  const response = await apiClient.post<RoleTemplate>(
    "/v1/settings/role-templates",
    template
  );
  return response.data;
}

/** Update an existing template (name is taken from the path and immutable). */
export async function updateRoleTemplate(
  name: string,
  template: RoleTemplate
): Promise<RoleTemplate> {
  const response = await apiClient.put<RoleTemplate>(
    `/v1/settings/role-templates/${encodeURIComponent(name)}`,
    template
  );
  return response.data;
}

/** Delete a template. */
export async function deleteRoleTemplate(name: string): Promise<void> {
  await apiClient.delete(`/v1/settings/role-templates/${encodeURIComponent(name)}`);
}
