# Testing

This guide covers the knodex test architecture, how to run tests, and how to write new ones.

---

## Test Tiers

knodex uses a tiered testing approach:

| Tier | Speed | Infrastructure | Scope | When Run |
|------|-------|----------------|-------|----------|
| **Unit** | Fast (seconds) | None | Individual functions | Every `go test`, `npm test` |
| **Integration** | Medium (minutes) | Mocks/stubs | Component interactions | PR builds |
| **E2E** | Slow (5-15 min) | Full cluster | User workflows | Explicit trigger |

### Why E2E Tests Skip by Default

Server E2E tests use build tags:

```go
//go:build e2e
```

**This is intentional.** E2E tests require a running Kind cluster with deployed application, test data, and mock OIDC server. Running `go test ./...` without this infrastructure would fail with confusing errors.

---

## Running Tests

### Unit Tests (Daily Development)

```bash
# All unit tests
make test

# Single server test
cd server && go test -v -run TestHealthz ./internal/api/handlers/

# Single web test (filter by test name)
cd web && npx vitest run -t "should render"
```

### E2E Tests (Before PR/Merge)

```bash
# One-time: Create cluster with prerequisites
make cluster-up

# Run E2E tests (auto-deploys if needed)
make e2e

# Or full QA cycle: deploy + all tests
make qa
```

### Specific E2E Tests

```bash
# Web E2E (Playwright)
cd web && npx playwright test auth_login_test.spec.ts

# Server E2E (Go, requires cluster)
cd server && go test -tags=e2e -v -run TestRBACPermissions ./test/e2e/...
```

---

## Test Locations

| Category | Location | Command |
|----------|----------|---------|
| Server unit tests | `server/internal/*/` | `cd server && go test ./...` |
| Server E2E tests | `server/test/e2e/` | `cd server && go test -tags=e2e ./test/e2e/...` |
| Web unit tests | `web/src/**/*.test.ts` | `cd web && npm test` |
| Web E2E tests | `web/test/e2e/` | `cd web && npx playwright test` |

---

## E2E Test Frameworks

knodex uses two E2E frameworks:

| Framework | Location | Purpose |
|-----------|----------|---------|
| **Playwright** | `web/test/e2e/` | UI tests, browser interactions, user workflows |
| **Go E2E** | `server/test/e2e/` | API tests, Kubernetes resource tests |

Both run against a real Kind cluster with actual CRDs and test data.

### Prerequisites

- Go 1.24+, Node.js 20+, Docker, Kind, kubectl

```bash
# Install Playwright browsers
cd web && npm install && npx playwright install chromium
```

### Configuring Test Secrets (Optional)

Repository connection tests need a GitHub token. Most contributors can skip this.

```bash
cp .env.example .env
# Edit .env and add your GitHub Personal Access Token (repo scope)
# The token is picked up automatically by `make qa`
```

The `.env` file is in `.gitignore` — never commit real tokens.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `E2E_BASE_URL` | `http://localhost:3000` | Web URL for Playwright tests |
| `E2E_API_URL` | `http://localhost:8080` | Server API URL |
| `E2E_JWT_SECRET` | `test-secret-key-minimum-32-characters-required` | JWT signing secret (must match server) |
| `E2E_TIMEOUT` | `60000` | Test timeout in milliseconds |
| `CI` | unset | Enables CI mode (stricter failure handling) |

---

## Authentication in Tests

Frontend E2E tests use JWT token injection to bypass the login flow.

### Using Test Fixtures

```typescript
import { test } from '../fixture';
import { TestUserRole } from '../fixture/auth-helper';

test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

test('admin can view all RGDs', async ({ page }) => {
  await page.goto('/catalog');
  // Test assertions...
});
```

### Manual Authentication

```typescript
import { test } from '@playwright/test';
import { setupAuthAndNavigate, TestUserRole } from '../fixture';

test('viewer cannot deploy', async ({ page }) => {
  await setupAuthAndNavigate(page, TestUserRole.ORG_VIEWER, '/catalog');
  await expect(page.getByRole('button', { name: 'Deploy' })).not.toBeVisible();
});
```

### Available Test Users

| Role | Description | Projects |
|------|-------------|----------|
| `GLOBAL_ADMIN` | Full platform access | All projects |
| `ORG_ADMIN` | Project admin | `proj-alpha-team` |
| `ORG_DEVELOPER` | Project developer | `proj-alpha-team` |
| `ORG_VIEWER` | Read-only access | `proj-alpha-team` |
| `OIDC_WILDCARD_USER` | OIDC user with group-based access | `proj-azuread-staging` |

---

## Test Isolation

**Frontend:** Each test file runs in isolation with a fresh browser context, clean localStorage, and new auth tokens. Tests can run in parallel.

**Backend:** Tests use Kubernetes namespaces for isolation with cleanup after each suite.

---

## Writing New Tests

### Frontend Test Template

```typescript
// test/e2e/feature_name_test.spec.ts
import { test, expect } from '@playwright/test';
import { setupAuthAndNavigate, TestUserRole } from '../fixture';

test.describe('Feature Name', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthAndNavigate(page, TestUserRole.GLOBAL_ADMIN, '/target-page');
  });

  test('should do expected behavior', async ({ page }) => {
    await page.getByRole('button', { name: 'Action' }).click();
    await expect(page.getByText('Success')).toBeVisible();
  });
});
```

### Backend Test Template

```go
// test/e2e/feature_test.go
//go:build e2e

package e2e

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestFeatureName(t *testing.T) {
    t.Run("should do expected behavior", func(t *testing.T) {
        client := getTestClient(t)
        result, err := client.DoSomething()
        require.NoError(t, err)
        require.Equal(t, expected, result)
    })
}
```

### Naming Conventions

- **Frontend:** `{feature}_{subfeature}_test.spec.ts` (e.g., `catalog_list_test.spec.ts`)
- **Backend:** `{feature}_test.go` (e.g., `rbac_test.go`)

---

## Debugging Failed Tests

```bash
# View Playwright HTML report
cd web && npx playwright show-report

# Run in debug mode (step through)
cd web && npx playwright test --debug auth_login_test.spec.ts

# View trace file
npx playwright show-trace test-results/auth_login_test-Authentication--chromium/trace.zip
```

Test artifacts are saved to `web/test-results/` (screenshots, videos, traces).

---

## Common Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| Tests stuck on login page | Token injection failed | Check `E2E_JWT_SECRET` matches server config |
| `browserType.launch: Executable doesn't exist` | Playwright not installed | `cd web && npx playwright install chromium` |
| `connect ECONNREFUSED 127.0.0.1:8080` | Server not running | `make qa` or `make dev` |
| Tests exceed 60s timeout | Slow API / missing test data | Check server logs, verify test data exists |
| OIDC tests skipped | Expected behavior | OIDC tests require mock OIDC server (deployed by `make qa`) |

---

## CI/CD Pipeline

```
PR Opened
    ├── Server CI (go test, go vet)
    ├── Web CI (npm test, eslint)         ← Fast
    └── Docker Build (build, verify)
              ↓
        E2E Tests
        - Deploy to Kind cluster
        - Run Playwright tests              ← Slow
        - Run Go E2E tests
        - Collect artifacts
              ↓
        Merge Gate (all checks must pass)
```

### Pass Rate Requirements

| Test Category | Minimum Pass Rate |
|---------------|-------------------|
| Unit tests | **100%** |
| Critical E2E (auth, RBAC) | **95%** |
| Non-critical E2E | **85%** |

### View CI Results

Check the "E2E Tests" workflow in GitHub Actions for test logs, Playwright HTML report (as artifact), and failure screenshots.

---

## Related Documentation

- [Tilt Development](./tilt.md) - Local development setup
- [API Documentation](./api-docs.md) - REST API reference
- [RBAC Setup](../operator-manual/rbac-setup.md) - Understanding roles and permissions
