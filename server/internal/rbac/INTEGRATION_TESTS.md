# RBAC Integration Tests

## Overview

This document describes the comprehensive integration tests for the Knodex RBAC (Role-Based Access Control) system. These tests verify multi-tenancy, organization isolation, role-based permissions, and authentication flows.

## Test Coverage

The integration test suite (`integration_test.go`) implements 13 test cases covering all acceptance criteria:

### 1. User Organization Visibility
**Test:** `TestIntegration_UserOrganizationVisibility`
**Acceptance Criterion:** User can only see their organizations

Verifies that:
- Users can see organizations they are members of
- Users cannot see organizations they are not members of
- Global admins can see all organizations

### 2. Instance Isolation
**Test:** `TestIntegration_InstanceIsolation`
**Acceptance Criterion:** User cannot see instances from other organizations

Verifies that:
- Users can only see RGD instances in their organization's namespaces
- Cross-organization instance access is blocked
- Instance listings are filtered by organization membership

### 3. Namespace Enforcement
**Test:** `TestIntegration_NamespaceEnforcement`
**Acceptance Criterion:** User cannot deploy to other organization's namespace

Verifies that:
- Users can deploy to their organization's namespaces
- Users cannot deploy to other organizations' namespaces
- Namespace access is strictly enforced

### 4. Platform Admin Permissions
**Test:** `TestIntegration_PlatformAdminPermissions`
**Acceptance Criterion:** Platform Admin can manage their org but not delete it

Verifies Platform Admin can:
- ✅ Read organization details
- ✅ Update organization settings
- ✅ Add members to organization
- ✅ Update member roles
- ✅ Deploy instances
- ❌ Delete the organization (blocked)
- ❌ Create new organizations (requires Global Admin)

### 5. Developer Permissions
**Test:** `TestIntegration_DeveloperPermissions`
**Acceptance Criterion:** Developer can deploy but cannot manage org

Verifies Developer can:
- ✅ Read organization details
- ✅ Deploy instances
- ❌ Update organization settings (blocked)
- ❌ Add members (blocked)
- ❌ Update member roles (blocked)
- ❌ Delete organization (blocked)

### 6. Viewer Permissions
**Test:** `TestIntegration_ViewerPermissions`
**Acceptance Criterion:** Viewer cannot deploy or manage org

Verifies Viewer can:
- ✅ Read organization details
- ❌ Deploy instances (blocked)
- ❌ Update organization settings (blocked)
- ❌ Add members (blocked)
- ❌ Delete organization (blocked)

### 7. Global Admin Organization Management
**Test:** `TestIntegration_GlobalAdminPermissions`
**Acceptance Criterion:** Global Platform Admin can create/delete any org

Verifies Global Admin can:
- ✅ Create organizations
- ✅ Delete any organization
- ✅ Full admin permissions across all organizations

### 8. Global Admin Cross-Organization Visibility
**Test:** `TestIntegration_GlobalAdminSeesAllInstances`
**Acceptance Criterion:** Global Platform Admin can see all instances across orgs

Verifies that:
- Global admins see instances from all organizations
- Regular users only see instances from their organizations
- Organization count matches for global admins

### 9. OIDC Provisioning Flow
**Test:** `TestIntegration_OIDCProvisioningFlow`
**Acceptance Criterion:** OIDC login creates User CRD and default org

Verifies OIDC provisioning:
- User CRD creation with OIDC subject
- Default organization creation
- User is platform-admin of default organization
- User can perform admin actions in default org

### 10. Local Admin Permissions
**Test:** `TestIntegration_LocalAdminPermissions`
**Acceptance Criterion:** Local admin login has Global Platform Admin permissions

Verifies that:
- Local admin user (from K8s Secret) has Casbin `role:serveradmin` assigned
- Local admin has all Global Platform Admin permissions
- Local admin can create/delete organizations

### 11. Permission Cache Invalidation
**Test:** `TestIntegration_PermissionCacheInvalidation`
**Acceptance Criterion:** Permission cache invalidation on user/org updates

Verifies cache invalidation on:
- User role updates
- Organization membership changes
- Cached permissions are properly refreshed

### 12. WebSocket Organization Filtering
**Test:** `TestIntegration_WebSocketOrgFiltering`
**Acceptance Criterion:** WebSocket events filtered by organization

Verifies that:
- Users receive WebSocket events only for their organizations
- Events from other organizations are filtered out
- Organization-scoped event delivery works correctly

### 13. Complete RBAC Flow (End-to-End)
**Test:** `TestIntegration_CompleteRBACFlow`
**Comprehensive test:** Validates entire RBAC system end-to-end

Verifies complete workflow:
1. Global admin creates organization
2. Users with different roles (Platform Admin, Developer, Viewer)
3. Permission enforcement for all roles
4. Organization isolation between multiple organizations

## Running the Tests

### Run All Integration Tests
```bash
cd server
go test -v -run TestIntegration ./internal/rbac/...
```

### Run Specific Test
```bash
go test -v -run TestIntegration_UserOrganizationVisibility ./internal/rbac/...
```

### Skip Integration Tests (Short Mode)
```bash
go test -short ./internal/rbac/...
```

All integration tests include `testing.Short()` checks and will be skipped in short mode.

### With Coverage
```bash
go test -cover -run TestIntegration ./internal/rbac/...
```

## Test Architecture

### Test Setup

Each test uses `setupIntegrationTestServices()` which creates:
- Fake Kubernetes clientset for CRD operations
- Fake dynamic client for Organization/User CRDs
- In-memory Redis client for permission caching
- UserService, OrganizationService, PermissionService instances
- Structured logger

### Test Helpers

**`createTestUser(t, services, email, displayName, isGlobalAdmin)`**
- Creates a test user with specified properties
- Registers user in fake K8s API
- Returns User object

**`setupIntegrationTestServices(t)`**
- Initializes all required services for testing
- Returns `integrationTestServices` struct with all clients

### Test Data Pattern

Tests follow a consistent pattern:
1. Create global admin user
2. Create test organizations with unique namespaces
3. Create test users with specific roles
4. Perform operations and verify permissions
5. Assert expected allow/deny outcomes

## Coverage Report

```bash
go test -coverprofile=coverage.out ./internal/rbac/...
go tool cover -html=coverage.out
```

Overall RBAC package coverage: **52.3%**

Key areas covered:
- PermissionService: 68-100% (core permission checking logic)
- OrganizationService: 55-100% (organization CRUD operations)
- UserService: 57-100% (user management)
- Authorization: 75-100% (authorization checks)

Areas with lower coverage (not critical for integration tests):
- CRD Installer (runtime installation, tested manually)
- Audit logging (observability, not functional)
- Cache implementation details (tested indirectly)

## Security Validation

These integration tests validate critical security requirements:

✅ **Authorization:** All permission checks enforced at service layer
✅ **Multi-Tenancy:** Organization data isolation verified
✅ **RBAC Matrix:** All role capabilities tested (Platform Admin, Developer, Viewer)
✅ **Privilege Escalation:** Tests verify users cannot elevate privileges
✅ **IDOR Prevention:** Cross-organization access is blocked
✅ **Authentication:** OIDC and local admin flows tested

## Test Execution Results

All 13 integration tests pass successfully:

```
=== RUN   TestIntegration_UserOrganizationVisibility
--- PASS: TestIntegration_UserOrganizationVisibility (0.00s)
=== RUN   TestIntegration_InstanceIsolation
--- PASS: TestIntegration_InstanceIsolation (0.00s)
=== RUN   TestIntegration_NamespaceEnforcement
--- PASS: TestIntegration_NamespaceEnforcement (0.00s)
=== RUN   TestIntegration_PlatformAdminPermissions
--- PASS: TestIntegration_PlatformAdminPermissions (0.00s)
=== RUN   TestIntegration_DeveloperPermissions
--- PASS: TestIntegration_DeveloperPermissions (0.00s)
=== RUN   TestIntegration_ViewerPermissions
--- PASS: TestIntegration_ViewerPermissions (0.00s)
=== RUN   TestIntegration_GlobalAdminPermissions
--- PASS: TestIntegration_GlobalAdminPermissions (0.00s)
=== RUN   TestIntegration_GlobalAdminSeesAllInstances
--- PASS: TestIntegration_GlobalAdminSeesAllInstances (0.00s)
=== RUN   TestIntegration_PermissionCacheInvalidation
--- PASS: TestIntegration_PermissionCacheInvalidation (0.10s)
=== RUN   TestIntegration_OIDCProvisioningFlow
--- PASS: TestIntegration_OIDCProvisioningFlow (0.00s)
=== RUN   TestIntegration_LocalAdminPermissions
--- PASS: TestIntegration_LocalAdminPermissions (0.00s)
=== RUN   TestIntegration_WebSocketOrgFiltering
--- PASS: TestIntegration_WebSocketOrgFiltering (0.00s)
=== RUN   TestIntegration_CompleteRBACFlow
--- PASS: TestIntegration_CompleteRBACFlow (0.00s)
PASS
ok  	github.com/provops-org/knodex/server/internal/rbac	2.591s
```

## Maintenance

### Adding New Tests

When adding new RBAC features:
1. Create new test function: `TestIntegration_<FeatureName>`
2. Add `testing.Short()` skip check
3. Use `setupIntegrationTestServices()` for test setup
4. Follow existing test patterns for consistency
5. Document acceptance criterion in test comment
6. Verify test passes in isolation and with full suite

### Updating Existing Tests

When RBAC logic changes:
1. Update relevant integration tests
2. Ensure all tests still pass
3. Update this documentation if behavior changes
4. Re-run coverage report to track impact

## Troubleshooting

### Redis Connection Errors
Integration tests use in-memory `miniredis` - no external Redis required.

### Kubernetes Client Errors
Tests use `k8s.io/client-go/kubernetes/fake` - no real cluster required.

### Cache Issues
Each test gets a fresh Redis instance - no cache pollution between tests.

### Test Timeouts
Default timeout: 120 seconds. Increase with:
```bash
go test -timeout 300s -run TestIntegration ./internal/rbac/...
```

## Related Documentation

- [RBAC Architecture](../../../docs/architecture/rbac-design.md)
- [Sprint Plan](../../../docs/sprint-plan-rbac-knodex-2025-12-31.md)
- [Permission Matrix](../../../docs/rbac-permission-matrix.md)
