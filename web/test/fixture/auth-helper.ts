/**
 * E2E Test Authentication Helper
 * Based on JWT token injection approach
 *
 * Uses properly signed JWT tokens (not mock tokens) for security best practices
 *
 * Two authorization patterns supported:
 * 1. Real backend authorization: Tests authenticate with JWT containing casbin_roles,
 *    and the backend evaluates permissions via Casbin enforcer. Use for integration tests.
 * 2. Mocked API authorization: Tests mock the /api/v1/account/can-i endpoint to simulate
 *    specific permission scenarios. Use for UI permission testing without backend dependencies.
 */

import { Page } from '@playwright/test'
import { SignJWT } from 'jose'

/**
 * User roles for E2E testing
 */
export enum TestUserRole {
  GLOBAL_ADMIN = 'global-admin',
  ORG_ADMIN = 'org-admin',
  ORG_DEVELOPER = 'org-developer',
  ORG_VIEWER = 'org-viewer',
  OIDC_WILDCARD_USER = 'oidc-wildcard-user', // User with OIDC groups for wildcard namespace testing
  UNAUTHENTICATED = 'unauthenticated',
}

// ArgoCD-aligned Casbin role constant (matches backend rbac.CasbinRoleServerAdmin)
const CASBIN_ROLE_ADMIN = 'role:serveradmin'

/**
 * Test user credentials and token claims
 * Note: Also exported via fixtures/index.ts
 */
export interface TestUser {
  sub: string
  email: string
  displayName: string
  casbinRoles: string[] // Casbin roles (e.g., ["role:serveradmin"]) for authorization
  projects: string[]
  roles?: Record<string, string>
  groups?: string[] // OIDC groups from identity provider
  permissions?: Record<string, boolean> // Pre-computed permissions for frontend UI (ArgoCD-aligned)
}

/**
 * Predefined test users matching Project-based RBAC model
 * Uses actual User CRs from test cluster (user-global-admin, user-alpha-viewer, etc.)
 * The 'sub' field must match the User CR name in the cluster
 */
export const TEST_USERS: Record<TestUserRole, TestUser> = {
  [TestUserRole.GLOBAL_ADMIN]: {
    sub: 'user-global-admin',
    email: 'admin@e2e-test.local',
    displayName: 'Global Administrator',
    casbinRoles: [CASBIN_ROLE_ADMIN], // ArgoCD-aligned: Global admin via Casbin role
    projects: ['proj-alpha-team', 'proj-beta-team', 'proj-shared'],
    permissions: { '*:*': true, 'settings:get': true, 'settings:update': true, 'projects:create': true, 'projects:delete': true },
  },
  [TestUserRole.ORG_ADMIN]: {
    sub: 'user-alpha-admin',
    email: 'alpha-admin@e2e-test.local',
    displayName: 'Alpha Team Admin',
    casbinRoles: [], // Not a global admin
    projects: ['proj-alpha-team'],
    roles: { 'proj-alpha-team': 'admin' },
    permissions: { 'settings:get': false, 'settings:update': false, 'projects:create': false, 'projects:delete': false },
  },
  [TestUserRole.ORG_DEVELOPER]: {
    sub: 'user-alpha-developer',
    email: 'alpha-dev@e2e-test.local',
    displayName: 'Alpha Developer',
    casbinRoles: [], // Not a global admin
    projects: ['proj-alpha-team'],
    roles: { 'proj-alpha-team': 'developer' },
    permissions: { 'settings:get': false, 'settings:update': false, 'projects:create': false, 'projects:delete': false },
  },
  [TestUserRole.ORG_VIEWER]: {
    sub: 'user-alpha-viewer',
    email: 'alpha-viewer@e2e-test.local',
    displayName: 'Alpha Viewer',
    casbinRoles: ['proj:proj-alpha-team:readonly'], // Project-scoped readonly role (role:readonly removed as global role)
    projects: ['proj-alpha-team'],
    roles: { 'proj-alpha-team': 'viewer' },
    permissions: { 'settings:get': false, 'settings:update': false, 'projects:create': false, 'projects:delete': false },
  },
  /**
   * OIDC user with groups that map to project with wildcard namespace destinations
   * This user belongs to Azure AD group that has admin role on proj-azuread-staging
   * which has wildcard destinations: staging*, knodex*
   *
   * Key insight: The 'groups' claim in the JWT token is used by GetUserNamespacesWithGroups()
   * to resolve namespaces from projects where this group has a role assignment.
   */
  [TestUserRole.OIDC_WILDCARD_USER]: {
    sub: 'user-oidc-wildcard-test',
    email: 'oidc-wildcard@e2e-test.local',
    displayName: 'OIDC Wildcard User',
    casbinRoles: [], // Not a global admin
    projects: ['proj-azuread-staging'], // via group membership
    roles: { 'proj-azuread-staging': 'admin' },
    groups: ['7e24cb11-e404-4b4d-9e2c-96d6e7b4733c'], // Azure AD group ID that has admin role
    permissions: { 'settings:get': false, 'settings:update': false, 'projects:create': false, 'projects:delete': false },
  },
  [TestUserRole.UNAUTHENTICATED]: {
    sub: '',
    email: '',
    displayName: '',
    casbinRoles: [], // Not authenticated, no Casbin roles
    projects: [],
    permissions: {},
  },
}

/**
 * Generate a properly signed JWT token for testing
 * Uses the same JWT_SECRET as the backend for real validation
 *
 * SECURITY: This uses real JWT signing, not mock tokens
 * The backend validates these tokens using standard JWT verification
 */
export async function generateTestToken(user: TestUser): Promise<string> {
  // Use the same secret the backend uses
  // NOTE: Backend may not validate signatures, just decodes claims
  const jwtSecret = process.env.E2E_JWT_SECRET || 'test-secret-key-minimum-32-characters-required'
  const secret = new TextEncoder().encode(jwtSecret)

  const payload = {
    sub: user.sub,
    email: user.email,
    name: user.displayName,
    casbin_roles: user.casbinRoles, // ArgoCD-aligned: Casbin roles (e.g., ["role:serveradmin"])
    projects: user.projects,
    default_project: user.projects[0] || null, // Set default_project to first project
    roles: user.roles || {},
    groups: user.groups || [], // Include OIDC groups for wildcard namespace resolution
    permissions: user.permissions || {}, // ArgoCD-aligned: Pre-computed permissions for frontend UI
  }

  // Sign with HS256 (matches backend expectation)
  // Must include issuer and audience claims that backend ValidateToken requires
  return await new SignJWT(payload)
    .setProtectedHeader({ alg: 'HS256', typ: 'JWT' })
    .setIssuedAt()
    .setExpirationTime('1h')
    .setIssuer('knodex')
    .setAudience('knodex-api')
    .sign(secret)
}

/**
 * Inject authentication token into browser localStorage
 * This bypasses the login flow and sets up the authenticated session
 *
 * @param page - Playwright page instance
 * @param role - User role to authenticate as
 */
export async function authenticateAs(page: Page, role: TestUserRole): Promise<void> {
  const user = TEST_USERS[role]

  if (role === TestUserRole.UNAUTHENTICATED) {
    // Clear any existing auth (match frontend's keys)
    await page.evaluate(() => {
      localStorage.removeItem('jwt_token')
      localStorage.removeItem('user-storage')
    })
    return
  }

  const token = await generateTestToken(user)

  // Inject token into localStorage (must match frontend's userStore)
  await page.evaluate(
    ({ token, user }) => {
      // Frontend expects 'jwt_token' (see userStore.ts line 90)
      localStorage.setItem('jwt_token', token)

      // Also set Zustand persist storage (key: 'user-storage')
      // This matches how userStore persists authentication state
      // NOTE: isGlobalAdmin was removed - authorization uses Casbin via useCanI() hook
      // The JWT contains casbin_roles and permissions for proper authorization
      const userStorage = {
        state: {
          currentProject: user.projects[0] || null,
          token: token,
          isAuthenticated: true,
          roles: user.roles || {},
          projects: user.projects || [],

        },
        version: 0
      }
      console.log('[AUTH-HELPER] Setting user-storage with roles:', user.roles)
      localStorage.setItem('user-storage', JSON.stringify(userStorage))

      // Verify it was saved correctly
      const saved = JSON.parse(localStorage.getItem('user-storage') || '{}')
      console.log('[AUTH-HELPER] Verified saved state has roles:', saved.state?.roles)
    },
    { token, user }
  )
}

/**
 * Setup authentication before navigating to a page
 * This is the recommended way to authenticate in tests
 *
 * @example
 * test('global admin can see all RGDs', async ({ page }) => {
 *   await setupAuth(page, TestUserRole.GLOBAL_ADMIN)
 *   await page.goto('/catalog')
 *   // Test assertions...
 * })
 */
export async function setupAuth(page: Page, role: TestUserRole): Promise<void> {
  // Go to login page first - this stays on the app domain without redirects
  // (unlike '/' which redirects to '/login' when unauthenticated, causing timing issues)
  await page.goto('/login', { waitUntil: 'domcontentloaded' })

  // Clear any existing auth first
  await page.evaluate(() => {
    localStorage.clear()
    sessionStorage.clear()
  })

  // Inject authentication
  await authenticateAs(page, role)
}

/**
 * Setup authentication and navigate to a target page
 * This is the recommended way to authenticate and navigate in one step
 *
 * @example
 * test('global admin can see all RGDs', async ({ page }) => {
 *   await setupAuthAndNavigate(page, TestUserRole.GLOBAL_ADMIN, '/catalog')
 *   // Test assertions...
 * })
 */
export async function setupAuthAndNavigate(
  page: Page,
  role: TestUserRole,
  targetPath: string = '/catalog'
): Promise<void> {
  // Setup auth first
  await setupAuth(page, role)

  // Navigate to target - Zustand will rehydrate from localStorage
  await page.goto(targetPath, { waitUntil: 'domcontentloaded' })

  // Wait for page to fully load and Zustand to rehydrate auth from localStorage
  await page.waitForLoadState('networkidle')
}

/**
 * Switch to a different user role during a test
 * Useful for testing role transitions
 */
export async function switchRole(page: Page, role: TestUserRole): Promise<void> {
  await authenticateAs(page, role)
  await page.reload()
  await page.waitForLoadState('networkidle')
}

/**
 * Interface for pre-loaded user tokens (e.g., from test-tokens.json)
 */
export interface PreloadedTestUser {
  user_id?: string
  sub?: string
  email: string
  display_name?: string
  displayName?: string
  casbin_roles?: string[]
  casbinRoles?: string[]
  roles: Record<string, string>
  projects: string[]
  token: string
  groups?: string[]
}

/**
 * Inject a pre-loaded token into browser localStorage
 * Use this when you have tokens from a JSON file instead of generating them
 *
 * @param page - Playwright page instance
 * @param user - Pre-loaded user with token
 */
export async function authenticateWithToken(page: Page, user: PreloadedTestUser): Promise<void> {
  // NOTE: isGlobalAdmin was removed - authorization uses Casbin via useCanI() hook
  // The JWT contains casbin_roles and permissions for proper authorization

  await page.evaluate(
    ({ token, user }) => {
      localStorage.setItem('jwt_token', token)

      const userStorage = {
        state: {
          currentProject: user.projects[0] || null,
          token: token,
          isAuthenticated: true,
          roles: user.roles || {},
          projects: user.projects || [],

        },
        version: 0
      }
      localStorage.setItem('user-storage', JSON.stringify(userStorage))
    },
    { token: user.token, user }
  )
}

/**
 * Setup authentication with a pre-loaded token and navigate to target page
 * This is for tests that use tokens from JSON files (e.g., permissions.spec.ts)
 *
 * IMPORTANT: This follows the same pattern as setupAuth/setupAuthAndNavigate
 * which has been proven to work reliably:
 * 1. Navigate to /login first (to be on the app domain)
 * 2. Clear and set localStorage
 * 3. Navigate to target path
 *
 * @param page - Playwright page instance
 * @param user - Pre-loaded user with token
 * @param targetPath - Path to navigate to after auth
 */
export async function setupAuthWithToken(
  page: Page,
  user: PreloadedTestUser,
  targetPath: string = '/catalog'
): Promise<void> {
  // Step 1: Navigate to /login first to be on the app domain
  // This is critical - localStorage is domain-scoped
  await page.goto('/login', { waitUntil: 'domcontentloaded' })

  // Step 2: Clear any existing auth
  await page.evaluate(() => {
    localStorage.clear()
    sessionStorage.clear()
  })

  // Step 3: Set auth tokens in localStorage
  // NOTE: isGlobalAdmin was removed - authorization uses Casbin via useCanI() hook
  // The JWT contains casbin_roles and permissions for proper authorization

  await page.evaluate(
    ({ token, user }) => {
      // Set the JWT token
      localStorage.setItem('jwt_token', token)

      // Set Zustand persist storage (must match userStore format)
      const userStorage = {
        state: {
          currentProject: user.projects[0] || null,
          token: token,
          isAuthenticated: true,
          roles: user.roles || {},
          projects: user.projects || [],

        },
        version: 0
      }
      localStorage.setItem('user-storage', JSON.stringify(userStorage))
      console.log('[setupAuthWithToken] Auth set for user:', user.email || user.sub)
    },
    { token: user.token, user }
  )

  // Step 4: Navigate to target - Zustand will rehydrate from localStorage
  await page.goto(targetPath, { waitUntil: 'domcontentloaded' })

  // Step 5: Wait for page to fully load and Zustand to rehydrate auth from localStorage
  await page.waitForLoadState('networkidle')
}

/**
 * Get the current authenticated user from token
 * This decodes the JWT token to get user information
 */
export async function getCurrentUser(page: Page): Promise<TestUser | null> {
  return page.evaluate(() => {
    const token = localStorage.getItem('jwt_token')
    if (!token) return null

    try {
      // Decode JWT payload (second part of token)
      const parts = token.split('.')
      if (parts.length !== 3) return null

      const payload = JSON.parse(atob(parts[1]))

      return {
        sub: payload.sub,
        email: payload.email,
        displayName: payload.name,
        casbinRoles: payload.casbin_roles || [], // ArgoCD-aligned: Casbin roles (e.g., ["role:serveradmin"])
        projects: payload.projects || [],
        roles: payload.roles || {},
        permissions: payload.permissions || {}, // ArgoCD-aligned: Pre-computed permissions
      }
    } catch (error) {
      console.error('Failed to decode token:', error)
      return null
    }
  })
}

/**
 * Verify that the user is authenticated
 */
export async function isAuthenticated(page: Page): Promise<boolean> {
  return page.evaluate(() => {
    return localStorage.getItem('jwt_token') !== null
  })
}

/**
 * Clear authentication (logout)
 */
export async function clearAuth(page: Page): Promise<void> {
  await authenticateAs(page, TestUserRole.UNAUTHENTICATED)
  await page.reload()
  await page.waitForLoadState('networkidle')
}

/**
 * Check if a permission key matches a request.
 * Supports wildcards: '*:*' (all), 'resource:*' (all actions on resource),
 * 'resource:action' (exact match), 'resource:action:subresource' (with scope)
 */
function checkPermissionFromMap(
  permissions: Record<string, boolean>,
  resource: string,
  action: string,
  subresource: string
): boolean {
  // Check global wildcard first
  if (permissions['*:*'] === true) return true

  // Check resource wildcard (e.g., 'instances:*')
  if (permissions[`${resource}:*`] === true) return true

  // Check exact permission with subresource (e.g., 'instances:create:proj-alpha-team')
  if (subresource && subresource !== '-') {
    const scopedKey = `${resource}:${action}:${subresource}`
    if (permissions[scopedKey] === true) return true
    if (permissions[scopedKey] === false) return false
  }

  // Check general permission (e.g., 'instances:create')
  const generalKey = `${resource}:${action}`
  if (permissions[generalKey] === true) return true
  if (permissions[generalKey] === false) return false

  // Deny by default
  return false
}

/**
 * Set up API route mocking for the /api/v1/account/can-i endpoint.
 * This intercepts permission check requests and returns results based on the user's permissions.
 *
 * Use this for UI permission testing without backend dependencies.
 * The frontend uses useCanI hook which calls this API endpoint.
 *
 * @param page - Playwright page instance
 * @param permissions - Permission map (e.g., {'projects:create': true, 'instances:delete': false})
 *
 * @example
 * await setupPermissionMocking(page, { '*:*': true }) // Admin with all permissions
 * await setupPermissionMocking(page, { 'projects:get': true, 'projects:create': false }) // Read-only
 */
export async function setupPermissionMocking(
  page: Page,
  permissions: Record<string, boolean>
): Promise<void> {
  // Intercept all can-i API calls and return mocked responses
  await page.route('**/api/v1/account/can-i/**', async (route) => {
    const url = route.request().url()
    // Parse the URL to extract resource, action, subresource
    // URL format: /api/v1/account/can-i/{resource}/{action}/{subresource}
    const match = url.match(/\/api\/v1\/account\/can-i\/([^/]+)\/([^/]+)(?:\/([^/?]+))?/)

    if (match) {
      const resource = decodeURIComponent(match[1])
      const action = decodeURIComponent(match[2])
      const subresource = match[3] ? decodeURIComponent(match[3]) : '-'

      const allowed = checkPermissionFromMap(permissions, resource, action, subresource)

      // Return ArgoCD-style response
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: allowed ? 'yes' : 'no' }),
      })
    } else {
      // If URL doesn't match expected format, deny
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'no' }),
      })
    }
  })
}

/**
 * Set up authentication with permission API mocking
 * Use this for UI permission tests that need specific permission scenarios
 *
 * @param page - Playwright page instance
 * @param role - User role to authenticate as
 * @param targetPath - Path to navigate to after auth
 *
 * @example
 * // Test admin UI
 * await setupAuthWithMocking(page, TestUserRole.GLOBAL_ADMIN, '/settings/projects')
 *
 * // Test viewer UI (read-only)
 * await setupAuthWithMocking(page, TestUserRole.ORG_VIEWER, '/catalog')
 */
export async function setupAuthWithMocking(
  page: Page,
  role: TestUserRole,
  targetPath: string = '/catalog'
): Promise<void> {
  const user = TEST_USERS[role]

  // Set up permission mocking based on user's permissions
  if (user.permissions) {
    await setupPermissionMocking(page, user.permissions)
  }

  // Then authenticate and navigate
  await setupAuthAndNavigate(page, role, targetPath)
}
