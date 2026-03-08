// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Page } from "@playwright/test";
import * as fs from "fs";
import * as path from "path";
import { fileURLToPath } from "url";
import {
    generateTestToken,
    TEST_USERS,
    TestUserRole,
    type TestUser as AuthTestUser,
} from "./auth-helper";

// ESM compatibility for __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Constant for global admin Casbin role
export const CASBIN_ROLE_GLOBAL_ADMIN = "role:serveradmin";

// Test user interface (matches the old structure for compatibility)
export interface TestUser {
  user_id: string;
  email: string;
  display_name: string;
  casbin_roles: string[];
  roles: Record<string, string>;
  projects: string[];
  token: string;
}

// Test tokens structure for backward compatibility
export interface TestTokens {
  generated_at: string;
  expires_in_seconds: number;
  jwt_secret: string;
  users: {
    global_admin: TestUser;
    alpha_admin: TestUser;
    alpha_developer: TestUser;
    alpha_viewer: TestUser;
    beta_admin: TestUser;
    beta_developer: TestUser;
    no_orgs: TestUser;
  };
}

// Generate test tokens dynamically using the same approach as working tests
export async function generateTestTokens(): Promise<TestTokens> {
  // Define users matching TEST_USERS but in the old format
  const users = {
    global_admin: {
      user_id: TEST_USERS[TestUserRole.GLOBAL_ADMIN].sub,
      email: TEST_USERS[TestUserRole.GLOBAL_ADMIN].email,
      display_name: TEST_USERS[TestUserRole.GLOBAL_ADMIN].displayName,
      casbin_roles: TEST_USERS[TestUserRole.GLOBAL_ADMIN].casbinRoles,
      roles: TEST_USERS[TestUserRole.GLOBAL_ADMIN].roles || {},
      projects: TEST_USERS[TestUserRole.GLOBAL_ADMIN].projects,
      token: await generateTestToken(TEST_USERS[TestUserRole.GLOBAL_ADMIN]),
    },
    alpha_admin: {
      user_id: TEST_USERS[TestUserRole.ORG_ADMIN].sub,
      email: TEST_USERS[TestUserRole.ORG_ADMIN].email,
      display_name: TEST_USERS[TestUserRole.ORG_ADMIN].displayName,
      casbin_roles: TEST_USERS[TestUserRole.ORG_ADMIN].casbinRoles,
      roles: TEST_USERS[TestUserRole.ORG_ADMIN].roles || {},
      projects: TEST_USERS[TestUserRole.ORG_ADMIN].projects,
      token: await generateTestToken(TEST_USERS[TestUserRole.ORG_ADMIN]),
    },
    alpha_developer: {
      user_id: TEST_USERS[TestUserRole.ORG_DEVELOPER].sub,
      email: TEST_USERS[TestUserRole.ORG_DEVELOPER].email,
      display_name: TEST_USERS[TestUserRole.ORG_DEVELOPER].displayName,
      casbin_roles: TEST_USERS[TestUserRole.ORG_DEVELOPER].casbinRoles,
      roles: TEST_USERS[TestUserRole.ORG_DEVELOPER].roles || {},
      projects: TEST_USERS[TestUserRole.ORG_DEVELOPER].projects,
      token: await generateTestToken(TEST_USERS[TestUserRole.ORG_DEVELOPER]),
    },
    alpha_viewer: {
      user_id: TEST_USERS[TestUserRole.ORG_VIEWER].sub,
      email: TEST_USERS[TestUserRole.ORG_VIEWER].email,
      display_name: TEST_USERS[TestUserRole.ORG_VIEWER].displayName,
      casbin_roles: TEST_USERS[TestUserRole.ORG_VIEWER].casbinRoles,
      roles: TEST_USERS[TestUserRole.ORG_VIEWER].roles || {},
      projects: TEST_USERS[TestUserRole.ORG_VIEWER].projects,
      token: await generateTestToken(TEST_USERS[TestUserRole.ORG_VIEWER]),
    },
    beta_admin: {
      user_id: "user-beta-admin",
      email: "beta-admin@e2e-test.local",
      display_name: "Beta Team Admin",
      casbin_roles: [],
      roles: { "proj-beta-team": "admin" },
      projects: ["proj-beta-team"],
      token: await generateTestToken({
        sub: "user-beta-admin",
        email: "beta-admin@e2e-test.local",
        displayName: "Beta Team Admin",
        casbinRoles: [],
        projects: ["proj-beta-team"],
        roles: { "proj-beta-team": "admin" },
      }),
    },
    beta_developer: {
      user_id: "user-beta-developer",
      email: "beta-dev@e2e-test.local",
      display_name: "Beta Developer",
      casbin_roles: [],
      roles: { "proj-beta-team": "developer" },
      projects: ["proj-beta-team"],
      token: await generateTestToken({
        sub: "user-beta-developer",
        email: "beta-dev@e2e-test.local",
        displayName: "Beta Developer",
        casbinRoles: [],
        projects: ["proj-beta-team"],
        roles: { "proj-beta-team": "developer" },
      }),
    },
    no_orgs: {
      user_id: "user-no-orgs",
      email: "no-orgs@e2e-test.local",
      display_name: "User Without Projects",
      casbin_roles: [],
      roles: {},
      projects: [],
      token: await generateTestToken({
        sub: "user-no-orgs",
        email: "no-orgs@e2e-test.local",
        displayName: "User Without Projects",
        casbinRoles: [],
        projects: [],
      }),
    },
  };

  return {
    generated_at: new Date().toISOString(),
    expires_in_seconds: 3600,
    jwt_secret: "test-secret-key-minimum-32-characters-required",
    users,
  };
}

// Helper to check if user is global admin from casbin_roles
export function isGlobalAdmin(user: TestUser): boolean {
  return user.casbin_roles?.includes(CASBIN_ROLE_GLOBAL_ADMIN) ?? false;
}

// Helper to inject auth token and navigate
export async function loginAs(page: Page, user: TestUser, targetPath: string = "/") {
  // Map target path to actual route
  let actualTarget = targetPath;
  if (targetPath === "/catalog" || targetPath === "/") {
    actualTarget = "/catalog";
  } else if (targetPath === "/projects") {
    actualTarget = "/settings/projects";
  }

  // Convert to AuthTestUser format and use setupAuthAndNavigate
  const authUser: AuthTestUser = {
    sub: user.user_id,
    email: user.email,
    displayName: user.display_name,
    casbinRoles: user.casbin_roles,
    projects: user.projects,
    roles: user.roles,
  };

  // Generate a fresh token and set up auth
  const token = await generateTestToken(authUser);

  // Navigate to login first
  await page.goto("/login", { waitUntil: "domcontentloaded" });

  // Clear and set localStorage
  await page.evaluate(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  // Set auth state with all fields needed by userStore (especially tokenExp for ProtectedRoute)
  await page.evaluate(
    ({ token, user }) => {
      localStorage.setItem("jwt_token", token);
      const tokenExpUnix = Math.floor(Date.now() / 1000) + 3600;
      const userStorage = {
        state: {
          currentProject: user.projects[0] || null,
          token: token,
          isAuthenticated: true,
          roles: user.roles || {},
          projects: user.projects || [],
          tokenExp: tokenExpUnix,
          casbinRoles: user.casbin_roles || [],
          groups: [],
          user: {
            id: user.user_id || "",
            email: user.email || "",
            name: user.display_name || "",
          },
        },
        version: 0,
      };
      localStorage.setItem("user-storage", JSON.stringify(userStorage));
    },
    { token, user }
  );

  await page.goto(actualTarget, { waitUntil: "domcontentloaded" });
  await page.waitForLoadState('networkidle');
}

// Evidence directory - unified at project root test-results/
export const EVIDENCE_DIR = path.join(__dirname, "../../../test-results/e2e/screenshots");

// Ensure evidence directories exist
export function ensureEvidenceDir(subdir: string) {
  const dir = path.join(EVIDENCE_DIR, subdir);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
  return dir;
}
