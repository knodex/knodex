/**
 * Role preset templates for quick role creation
 * Used in ProjectRolesTab (add role) and ProjectForm (create project)
 */
import type { ProjectRole } from '@/types/project';
import { isEnterprise } from '@/hooks/useCompliance';

export interface RolePreset {
  name: string;
  description: string;
  label: string;
  policies: string[];
}

const ADMIN_POLICIES = [
  'p, proj:{project}:{role}, projects, *, {project}, allow',
  'p, proj:{project}:{role}, instances, *, {project}/*, allow',
  'p, proj:{project}:{role}, rgds, get, *, allow',
  'p, proj:{project}:{role}, rgds, list, *, allow',
  'p, proj:{project}:{role}, repositories, *, {project}/*, allow',
  ...(isEnterprise() ? ['p, proj:{project}:{role}, compliance, get, {project}/*, allow'] : []),
];

/**
 * Preset role templates with {project} and {role} placeholders.
 * Each policy is a full Casbin string: p, proj:{project}:{role}, {resource}, {action}, {object}, {effect}
 */
export const ROLE_PRESETS: RolePreset[] = [
  {
    name: 'admin',
    label: 'Admin',
    description: 'Full project management access',
    policies: ADMIN_POLICIES,
  },
  {
    name: 'developer',
    label: 'Developer',
    description: 'Deploy and manage instances',
    policies: [
      'p, proj:{project}:{role}, instances, *, {project}/*, allow',
      'p, proj:{project}:{role}, rgds, get, *, allow',
      'p, proj:{project}:{role}, rgds, list, *, allow',
      'p, proj:{project}:{role}, repositories, get, {project}/*, allow',
      'p, proj:{project}:{role}, repositories, list, {project}/*, allow',
    ],
  },
  {
    name: 'readonly',
    label: 'Readonly',
    description: 'View-only access to project resources',
    policies: [
      'p, proj:{project}:{role}, projects, get, {project}, allow',
      'p, proj:{project}:{role}, instances, get, {project}/*, allow',
      'p, proj:{project}:{role}, instances, list, {project}/*, allow',
      'p, proj:{project}:{role}, rgds, get, *, allow',
      'p, proj:{project}:{role}, rgds, list, *, allow',
      'p, proj:{project}:{role}, repositories, get, {project}/*, allow',
      'p, proj:{project}:{role}, repositories, list, {project}/*, allow',
    ],
  },
];

/**
 * Resolve placeholders in preset policy strings
 */
export function resolvePresetPolicies(preset: RolePreset, projectName: string): string[] {
  return preset.policies.map((p) =>
    p.replaceAll('{project}', projectName).replaceAll('{role}', preset.name)
  );
}

/**
 * Resolve a preset into a complete ProjectRole with policies resolved for a project
 */
export function resolvePreset(preset: RolePreset, projectName: string): ProjectRole {
  return {
    name: preset.name,
    description: preset.description,
    policies: resolvePresetPolicies(preset, projectName),
    groups: [],
  };
}
