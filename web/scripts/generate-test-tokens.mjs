// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { SignJWT } from 'jose';

const jwtSecret = 'test-secret-key-minimum-32-characters-required';
const secret = new TextEncoder().encode(jwtSecret);

// Constant for global admin Casbin role
const CASBIN_ROLE_GLOBAL_ADMIN = 'role:serveradmin';

async function generateToken(user) {
  const payload = {
    sub: user.user_id,
    email: user.email,
    name: user.display_name,
    casbin_roles: user.casbin_roles || [], // Casbin roles replaces is_global_admin
    projects: user.projects,
    default_project: user.projects[0] || null,
    roles: user.roles || {},
    groups: user.groups || [], // OIDC groups for Project CRD spec.roles.groups
  };

  return await new SignJWT(payload)
    .setProtectedHeader({ alg: 'HS256', typ: 'JWT' })
    .setIssuedAt()
    .setExpirationTime('24h')
    .setIssuer('knodex')
    .setAudience('knodex-api')
    .sign(secret);
}

const users = {
  global_admin: {
    user_id: 'user-global-admin',
    email: 'admin@e2e-test.local',
    display_name: 'Global Administrator',
    casbin_roles: [CASBIN_ROLE_GLOBAL_ADMIN], // Global admin via Casbin role
    projects: ['proj-alpha-team', 'proj-beta-team', 'proj-shared'],
    roles: {},
    groups: ['global-admins'],
  },
  alpha_admin: {
    user_id: 'user-alpha-admin',
    email: 'alpha-admin@e2e-test.local',
    display_name: 'Alpha Team Admin',
    casbin_roles: [], // Non-admin users have empty casbin_roles
    projects: ['proj-alpha-team'],
    roles: { 'proj-alpha-team': 'admin' },
    groups: ['alpha-admins'],
  },
  alpha_developer: {
    user_id: 'user-alpha-developer',
    email: 'alpha-dev@e2e-test.local',
    display_name: 'Alpha Developer',
    casbin_roles: [], // Non-admin users have empty casbin_roles
    projects: ['proj-alpha-team'],
    roles: { 'proj-alpha-team': 'developer' },
    groups: ['alpha-developers'],
  },
  alpha_viewer: {
    user_id: 'user-alpha-viewer',
    email: 'alpha-viewer@e2e-test.local',
    display_name: 'Alpha Viewer',
    casbin_roles: [], // Non-admin users have empty casbin_roles
    projects: ['proj-alpha-team'],
    roles: { 'proj-alpha-team': 'viewer' },
    groups: ['alpha-viewers'],
  },
  beta_admin: {
    user_id: 'user-beta-admin',
    email: 'beta-admin@e2e-test.local',
    display_name: 'Beta Team Admin',
    casbin_roles: [], // Non-admin users have empty casbin_roles
    projects: ['proj-beta-team'],
    roles: { 'proj-beta-team': 'admin' },
    groups: ['beta-admins'],
  },
  beta_developer: {
    user_id: 'user-beta-developer',
    email: 'beta-dev@e2e-test.local',
    display_name: 'Beta Developer',
    casbin_roles: [], // Non-admin users have empty casbin_roles
    projects: ['proj-beta-team'],
    roles: { 'proj-beta-team': 'developer' },
    groups: ['beta-developers'],
  },
  no_orgs: {
    user_id: 'user-no-orgs',
    email: 'no-orgs@e2e-test.local',
    display_name: 'User No Projects',
    casbin_roles: [], // Non-admin users have empty casbin_roles
    projects: [],
    roles: {},
    groups: [],
  },
};

const result = {
  generated_at: new Date().toISOString(),
  expires_in_seconds: 86400,
  jwt_secret: jwtSecret,
  users: {},
};

for (const [key, user] of Object.entries(users)) {
  result.users[key] = {
    ...user,
    token: await generateToken(user),
  };
}

console.log(JSON.stringify(result, null, 2));
