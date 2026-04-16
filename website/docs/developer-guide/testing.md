---
title: Testing
description: Test tiers, running unit and E2E tests, writing new tests, and CI/CD pass rate requirements
sidebar_position: 3
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Testing

Knodex has three tiers of tests, from fast unit tests to full end-to-end tests running against a real Kubernetes cluster.

## Test Tiers

| Tier | Speed | Dependencies | Typical Duration |
|------|-------|-------------|-----------------|
| Unit | Fast | None | Seconds |
| Integration | Medium | Mocks / test doubles | Seconds to minutes |
| E2E | Slow | Kubernetes cluster | 5-15 minutes |

:::note[E2E Tests Skip by Default]
Server E2E tests use the `//go:build e2e` build tag and are excluded from `go test ./...`. They require a running cluster with Knodex deployed. Use `make e2e` or pass `-tags=e2e` explicitly.
:::

## Running Tests

### Unit Tests

```bash
# All unit tests (server + web)
make test

# Server only
cd server && go test ./...

# Single server test
cd server && go test -v -run TestHealthz ./internal/api/handlers/

# Web only
cd web && npm test

# Single web test file
cd web && npm test -- ProjectCard.test.tsx
```

### E2E Tests

```bash
# Full QA: deploy to Kind cluster + run all tests
make cluster-up    # One-time setup
make qa

# E2E only (auto-deploys if not already deployed)
make e2e

# Enterprise E2E (includes compliance/Gatekeeper tests)
ENTERPRISE_BUILD=true make qa
```

### Running Specific E2E Tests

```bash
# Playwright (web E2E) - run a specific test file
cd web && npx playwright test test/e2e/catalog.spec.ts

# Playwright UI mode (interactive)
cd web && npm run test:e2e:ui

# Go E2E - run a specific test
cd server && go test -tags=e2e -v -run TestRGDList ./test/e2e/
```

## Test Locations

| Type | Location | Framework |
|------|----------|-----------|
| Server unit tests | `server/internal/**/` (next to source) | Go `testing` |
| Server E2E tests | `server/test/e2e/` | Go `testing` + `e2e` build tag |
| Web unit tests | `web/src/**/*.test.{ts,tsx}` (next to source) | Vitest + React Testing Library |
| Web E2E tests | `web/test/e2e/` | Playwright |

## E2E Frameworks

| Framework | Scope | What It Tests |
|-----------|-------|---------------|
| Playwright | Web UI | Browser-based user workflows, page rendering, interactions |
| Go E2E | Server API | API endpoints directly against a real Kubernetes cluster |

## Prerequisites

Ensure you have these tools installed for E2E testing:

```bash
# Playwright browsers (one-time setup)
cd web && npx playwright install

# Kind cluster
make cluster-up
```

## Configuring Test Secrets

Some E2E tests (repository connection tests) require a GitHub token. This is optional -- tests that need it will skip gracefully.

```bash
# Copy the example env file
cp .env.example .env

# Add your GitHub token
# GITHUB_TOKEN=ghp_...
```

:::tip[Secrets Are Created Automatically]
When running `make qa`, secrets from your `.env` are automatically created in the cluster.
:::

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `E2E_BASE_URL` | `http://localhost:3000` | Web E2E base URL |
| `E2E_API_URL` | `http://localhost:8080` | API E2E base URL |
| `E2E_JWT_SECRET` | (from cluster) | JWT secret for test token generation |
| `E2E_TIMEOUT` | `30000` | Test timeout in milliseconds |
| `CI` | (unset) | Set in CI to enable CI-specific behaviors |

## Authentication in Tests

### Playwright Fixtures

Web E2E tests use fixtures that handle authentication automatically:

```typescript
import { test, expect } from "../fixtures/auth";

test("admin can view catalog", async ({ adminPage }) => {
  await adminPage.goto("/catalog");
  await expect(adminPage.getByRole("heading", { name: "Catalog" })).toBeVisible();
});
```

### Manual Authentication

For tests that need explicit token handling:

```typescript
import { getAuthToken } from "../helpers/auth";

test("custom auth test", async ({ page }) => {
  const token = await getAuthToken("GLOBAL_ADMIN");
  await page.goto("/", {
    extraHTTPHeaders: { Authorization: `Bearer ${token}` },
  });
});
```

### Available Test Users

| User Constant | Email | Groups | Role |
|--------------|-------|--------|------|
| `GLOBAL_ADMIN` | `admin@test.local` | `knodex-admins` | Server admin |
| `ORG_ADMIN` | `platform-admin@test.local` | `platform-admins` | Platform admin |
| `ORG_DEVELOPER` | `developer@test.local` | `alpha-developers` | Project developer |
| `ORG_VIEWER` | `viewer@test.local` | `alpha-viewers` | Project viewer |
| `OIDC_WILDCARD_USER` | `multi-group@test.local` | Multiple groups | Multi-group testing |

## Test Isolation

### Frontend (Playwright)

Each test gets a fresh browser context. No state leaks between tests. Authentication fixtures create isolated sessions.

### Backend (Go E2E)

Tests create and clean up Kubernetes namespaces. Each test suite uses a unique namespace prefix to avoid collisions when running in parallel.

## Writing New Tests

### Frontend E2E Template

```typescript
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect } from "../fixtures/auth";

test.describe("Feature Name", () => {
  test("should perform expected behavior", async ({ adminPage }) => {
    // Navigate
    await adminPage.goto("/feature");

    // Interact
    await adminPage.getByRole("button", { name: "Create" }).click();

    // Assert
    await expect(
      adminPage.getByText("Created successfully")
    ).toBeVisible();
  });

  test("should handle error case", async ({ viewerPage }) => {
    await viewerPage.goto("/feature");

    // Viewer should not see the create button
    await expect(
      viewerPage.getByRole("button", { name: "Create" })
    ).not.toBeVisible();
  });
});
```

### Backend E2E Template

```go
//go:build e2e

// SPDX-License-Identifier: AGPL-3.0-only

package e2e

import (
    "net/http"
    "testing"
)

func TestFeatureName(t *testing.T) {
    client := newTestClient(t)

    t.Run("list returns success", func(t *testing.T) {
        resp, err := client.Get("/api/v1/features")
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            t.Errorf("expected 200, got %d", resp.StatusCode)
        }
    })

    t.Run("unauthorized without token", func(t *testing.T) {
        resp, err := newUnauthClient(t).Get("/api/v1/features")
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusUnauthorized {
            t.Errorf("expected 401, got %d", resp.StatusCode)
        }
    })
}
```

## Naming Conventions

- **Server unit tests**: `*_test.go` in the same package as the source
- **Server E2E tests**: `*_test.go` in `server/test/e2e/` with `//go:build e2e`
- **Web unit tests**: `*.test.ts` or `*.test.tsx` next to the source file
- **Web E2E tests**: `*.spec.ts` in `web/test/e2e/`

## Debugging Failed Tests

### Playwright Report

After a test run, open the HTML report:

```bash
cd web && npx playwright show-report
```

### Debug Mode

Run a single test with the Playwright inspector:

```bash
cd web && npx playwright test --debug test/e2e/catalog.spec.ts
```

### Trace Viewer

If traces are enabled (they are in CI), view them:

```bash
cd web && npx playwright show-trace test-results/trace.zip
```

## Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| `Timeout waiting for selector` | Element not rendered in time | Increase timeout or add `waitForSelector` |
| `net::ERR_CONNECTION_REFUSED` | Server not running | Ensure `make qa` or `make e2e` deployed the app |
| `401 Unauthorized` | Token expired or missing | Check auth fixture setup, regenerate token |
| `CRD not found` | Missing CRDs in cluster | Run `make cluster-up` to install prerequisites |
| `Context deadline exceeded` | Kubernetes API slow | Increase test timeout via `E2E_TIMEOUT` |

## CI/CD Pipeline

Tests run automatically on every pull request:

```
Push to PR
  ├── Lint (golangci-lint + eslint)
  ├── Server unit tests (go test ./...)
  ├── Web unit tests (npm test)
  ├── Build (make build)
  └── E2E tests (make e2e)
       ├── Deploy to Kind cluster
       ├── Run Playwright tests
       └── Run Go E2E tests
```

### Pass Rate Requirements

| Test Tier | Required Pass Rate |
|-----------|--------------------|
| Unit tests | 100% |
| Critical E2E paths | >= 95% |
| Non-critical E2E | >= 85% |

:::warning[Failing E2E Tests Block Merge]
Pull requests with failing E2E tests cannot be merged. If failures are caused by known infrastructure issues, document them in the PR description.
:::

## KRO Canary Testing

### Purpose

Canary tests validate that Knodex remains compatible with KRO by deploying reference ResourceGraphDefinitions and verifying they reconcile correctly. These tests catch compatibility regressions early, especially after KRO version bumps.

### Schedule

Canary tests run on a weekly schedule:

- **When**: Every Monday at 06:00 UTC
- **Trigger**: GitHub Actions scheduled workflow
- **Scope**: Deploys all reference RGDs and verifies reconciliation

### Running Manually

```bash
# Trigger the canary workflow manually
gh workflow run kro-canary

# Or run locally against your cluster
make cluster-up
make qa
```

### Updating Baselines

When KRO changes its behavior intentionally (new fields, different status conditions), update the baselines:

1. Run the canary tests and review failures
2. Verify the new behavior is correct
3. Update the expected values in the test fixtures
4. Commit the baseline changes

### Reference RGDs

| RGD | Description | KRO Min Version |
|-----|-------------|----------------|
| `basic-types` | Simple resource with scalar fields | 0.9.0 |
| `conditional-resources` | Resources with CEL conditions | 0.9.0 |
| `external-refs` | Cross-resource references | 0.9.0 |
| `nested-external-refs` | Deeply nested cross-resource references | 0.10.0 |
| `advanced-section` | Complex schema sections | 0.10.0 |
| `cel-expressions` | Advanced CEL expression usage | 0.11.0 |

### Interpreting Canary Issues

When canary tests fail:

1. **Check the KRO controller logs** for reconciliation errors
2. **Compare with the previous passing run** to identify what changed
3. **Review recent KRO releases** for breaking changes
4. **Open an issue** if the failure is a genuine KRO regression

### Test Locations

| Test Type | Location |
|-----------|----------|
| Reference RGD fixtures | `deploy/examples/` |
| Canary test scripts | `scripts/` |
| Server compatibility tests | `server/internal/kro/**/compat_test.go` |
| E2E canary tests | `server/test/e2e/` |
