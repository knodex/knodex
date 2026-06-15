// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Role preset resolution for quick project-role creation.
 *
 * As of Story 18.1 the preset *catalog* is server-backed: templates are fetched
 * via useRoleTemplates() (GET /v1/settings/role-templates) instead of the
 * former static ROLE_PRESETS array. This module keeps only the RESOLUTION
 * logic — placeholder substitution and the enterprise-only compliance-policy
 * injection — which operates on the fetched RoleTemplate objects.
 *
 * A preset is a UI seed: resolvePreset() copies it into a ProjectRole that the
 * project create/edit path persists into Project.spec.roles[]. The single
 * Casbin enforcement layer is unchanged (NFR-T1).
 */
import type { ProjectRole } from '@/types/project';
import type { RoleTemplate } from '@/api/role-templates';
import { isEnterprise } from '@/hooks/useCompliance';

/**
 * RolePreset is the shape consumed by the resolvers. It is structurally the
 * server-backed RoleTemplate; the alias is kept so existing call sites keep
 * reading naturally.
 */
export type RolePreset = RoleTemplate;

/**
 * Enterprise-only admin policies injected at resolution time via
 * resolvePresetPolicies. Kept OUT of the stored catalog default (OSS operators
 * shouldn't see/store an enterprise-only policy) and evaluated per-call so
 * isEnterprise() reflects current runtime state, not module-load state.
 */
const ENTERPRISE_ADMIN_POLICIES = [
  'p, proj:{project}:{role}, compliance, get, {project}/*, allow',
];

/**
 * Enterprise-only developer policies: read-only compliance access.
 * Not stored in the catalog default for the same OSS-isolation reason as admin.
 */
const ENTERPRISE_DEVELOPER_POLICIES = [
  'p, proj:{project}:{role}, compliance, get, {project}/*, allow',
];

/**
 * Resolve placeholders in preset policy strings.
 * Enterprise-only policies (e.g. compliance) are injected here based on current runtime state,
 * not at module load time — ensuring isEnterprise() is evaluated per-call.
 */
export function resolvePresetPolicies(preset: RolePreset, projectName: string): string[] {
  const policies = [
    ...preset.policies,
    ...(preset.name === 'admin' && isEnterprise() ? ENTERPRISE_ADMIN_POLICIES : []),
    ...(preset.name === 'developer' && isEnterprise() ? ENTERPRISE_DEVELOPER_POLICIES : []),
  ];
  return policies.map((p) =>
    p.replaceAll('{project}', projectName).replaceAll('{role}', preset.name)
  );
}

/**
 * Resolve a preset into a complete ProjectRole with policies resolved for a project.
 *
 * Roles bind OIDC groups exclusively through named Teams now (epic-10, teams-only
 * binding) — the legacy roles[].groups[] field was removed, so resolvePreset no
 * longer seeds a groups array. Team binding is added separately via TeamPicker.
 */
export function resolvePreset(preset: RolePreset, projectName: string): ProjectRole {
  return {
    name: preset.name,
    description: preset.description,
    policies: resolvePresetPolicies(preset, projectName),
  };
}
