# Scripts

Essential scripts for development, testing, and deployment of Knodex.

## Quick Reference

| Script | Description |
|--------|-------------|
| `qa-deploy.sh` | Deploy to Kind cluster |
| `e2e-test-all.sh` | Run all E2E tests (server + web) |
| `ensure-prereqs.sh` | Install KRO, CRDs, and prerequisites |
| `test-cleanup.sh` | Clean up test fixtures and environment |
| `mock-oidc-server.js` | Mock OIDC provider for local E2E tests |

## Scripts

### qa-deploy.sh

Main deployment script for QA and E2E testing.

**Usage**:
```bash
# Deploy current branch
./scripts/qa-deploy.sh deploy

# Deploy only (skip prerequisites — used in CI)
./scripts/qa-deploy.sh deploy-only

# Show deployment status
./scripts/qa-deploy.sh status

# Clean up current branch namespace
./scripts/qa-deploy.sh cleanup-namespace

# Delete entire Kind cluster
./scripts/qa-deploy.sh cleanup-cluster
```

### e2e-test-all.sh

Unified E2E test runner for both server (Go) and web (Playwright) tests.

**Usage**:
```bash
# Run all E2E tests (server + web)
./scripts/e2e-test-all.sh

# Run only server E2E tests
./scripts/e2e-test-all.sh server

# Run only web E2E tests
./scripts/e2e-test-all.sh web

# Run OIDC-related tests
./scripts/e2e-test-all.sh oidc

# Skip setup (assumes already deployed)
./scripts/e2e-test-all.sh --no-setup

# Keep environment after tests (for debugging)
./scripts/e2e-test-all.sh --no-cleanup
```

### ensure-prereqs.sh

Auto-installs KRO, CRDs, and other prerequisites into the cluster. Called automatically by `make e2e` and `make qa`.

**Usage**:
```bash
./scripts/ensure-prereqs.sh
```

### test-cleanup.sh

Cleans up test fixtures and test environment.

**Usage**:
```bash
# Clean up test fixtures only (default)
./scripts/test-cleanup.sh

# Clean up fixtures and deployment namespace
./scripts/test-cleanup.sh full

# Delete entire Kind cluster
./scripts/test-cleanup.sh cluster

# Clean up integration test cluster
./scripts/test-cleanup.sh integration
```

### mock-oidc-server.js

Mock OIDC provider for local E2E testing. Used automatically by `web/test/global-setup.ts` during Playwright tests.

**Usage**:
```bash
node scripts/mock-oidc-server.js
```

### kind-config.yaml / kind-config-ci.yaml

Kind cluster configuration files. `kind-config.yaml` is for local development, `kind-config-ci.yaml` is optimized for CI environments.

## Overlays

Kustomize overlays for different environments:

| Overlay | Purpose |
|---------|---------|
| `overlays/dev/` | Development with NodePort services |
| `overlays/tilt/` | Tilt live development environment |

## Makefile Integration

```bash
make cluster-up     # Create Kind cluster with KRO + CRDs
make cluster-down   # Delete Kind cluster
make qa             # Full QA: deploy + run all tests
make e2e            # E2E tests (auto-deploys if needed)
make qa-stop        # Remove app deployment (keeps cluster)
make test           # Unit tests (no cluster needed)
make lint           # Run linters
```

## Related Documentation

- [Testing Guide](../docs/developer-guide/testing.md) - Testing overview and E2E test guide
