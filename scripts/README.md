# Scripts

Essential scripts for development, testing, and deployment of Knodex.

## Quick Reference

| Command | Description |
|---------|-------------|
| `make qa-deploy` | Deploy to Kind cluster |
| `make test-all` | Run all tests |
| `./scripts/get-admin-password.sh` | Get admin password |

## Scripts

### qa-deploy.sh

Main deployment script with multi-branch support.

**Usage**:
```bash
# Deploy current branch
./scripts/qa-deploy.sh deploy

# Full deployment and verification
./scripts/qa-deploy.sh all

# Show deployment status
./scripts/qa-deploy.sh status

# Clean up current branch namespace
./scripts/qa-deploy.sh cleanup-namespace

# Delete entire Kind cluster
./scripts/qa-deploy.sh cleanup-cluster
```

### qa-branch-config.sh

Calculates branch-specific namespace and port configuration for multi-branch testing.

**Usage**:
```bash
# Show configuration for current branch
./scripts/qa-branch-config.sh show

# Export variables for use in other scripts
eval "$(./scripts/qa-branch-config.sh export)"
echo $QA_NAMESPACE
```

**Environment Variables**:
- `QA_NAMESPACE` - Kubernetes namespace (e.g., `knodex-feature-sprint-11`)
- `QA_SERVER_PORT` - Server host port (8000-8019)
- `QA_WEB_PORT` - Web host port (9000-9019)

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
```

### test-data-setup.sh

Creates test fixtures (projects, users, RGDs) for E2E tests.

**Usage**:
```bash
# Set up all test data
./scripts/test-data-setup.sh

# Clean up test data
./scripts/test-data-setup.sh cleanup
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

### get-admin-password.sh

Retrieves auto-generated admin password from Kubernetes secret.

**Usage**:
```bash
# Get password from default namespace
./scripts/get-admin-password.sh

# Get password from specific namespace
./scripts/get-admin-password.sh knodex-main
```

### mock-oidc-server.js

Mock OIDC provider for testing authentication flows.

**Usage**:
```bash
node scripts/mock-oidc-server.js
```

### kind-config.yaml

Kind cluster configuration with port mappings for multi-branch support.

Provides port range mappings:
- Server: NodePort 30000-30019 → Host 8000-8019
- Web: NodePort 31000-31019 → Host 9000-9019

## Overlays

Kustomize overlays for different environments:

| Overlay | Purpose |
|---------|---------|
| `overlays/dev/` | Development with NodePort services |
| `overlays/qa-branch-template/` | Template for multi-branch QA |

## Makefile Integration

```bash
make qa-config      # Show branch configuration
make qa-deploy      # Deploy current branch
make qa-status      # Show deployment status
make qa-cleanup     # Clean up current branch
make qa-cleanup-all # Delete entire cluster
make test-all       # Run all tests
```

## Multi-Branch Workflow

1. **Create worktree** (optional):
   ```bash
   git worktree add ../knodex-feature-x -b feature/my-feature
   cd ../knodex-feature-x
   ```

2. **Check configuration**:
   ```bash
   make qa-config
   ```

3. **Deploy**:
   ```bash
   make qa-deploy
   ```

4. **Test**:
   ```bash
   make test-all
   ```

5. **Clean up**:
   ```bash
   make qa-cleanup
   ```

## Troubleshooting

### envsubst not found

Install gettext:
```bash
# macOS
brew install gettext

# Ubuntu/Debian
sudo apt-get install gettext
```

### Port conflicts

Rare hash collision. Solutions:
1. Rename branch to get different hash
2. Clean up conflicting namespace: `kubectl delete namespace knodex-{branch}`
3. Extend port range in `kind-config.yaml`

### Kind cluster unreachable

Recreate cluster:
```bash
make qa-cleanup-all
make qa-deploy
```

## Related Documentation

- [Testing Guide](../docs/developer-guide/testing.md) - Testing overview and E2E test guide
