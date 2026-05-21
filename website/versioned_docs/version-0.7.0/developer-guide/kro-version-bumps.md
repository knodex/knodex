---
title: KRO Version Bumps
description: Process for reviewing and merging KRO dependency updates from Dependabot
sidebar_position: 5
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# KRO Version Bumps

Knodex depends on [kubernetes-sigs/kro](https://github.com/kubernetes-sigs/kro) for resource graph definitions and reconciliation. Dependabot automatically creates pull requests when new KRO versions are released. This guide covers the review process.

## Overview

When a new KRO version is published:

1. Dependabot opens a PR updating `server/go.mod`
2. CI runs all checks automatically
3. A developer reviews the PR following the steps below
4. After verification, the PR is merged

## Review Steps

### 1. Check CI Status

Verify all CI checks pass on the Dependabot PR:

- Server unit tests (`go test ./...`)
- Enterprise unit tests (`go test -tags=enterprise ./...`)
- Build (both OSS and enterprise)
- Lint

### 2. Check Compatibility Test Results

The CI pipeline runs `compat_test.go` files that validate Knodex's integration with KRO internal packages. Review the test output for each package:

| Package | Test File | What It Validates |
|---------|-----------|-------------------|
| `kro/cel` | `compat_test.go` | CEL expression evaluation compatibility |
| `kro/parser` | `compat_test.go` | RGD resource parsing behavior |
| `kro/metadata` | `compat_test.go` | KRO metadata extraction |
| `kro/schema` | `compat_test.go` | Schema extraction and type handling |

### 3. Review KRO Changelog

Read the KRO release notes for the new version:

```bash
# Open the KRO releases page
open https://github.com/kubernetes-sigs/kro/releases
```

Look for:
- Breaking API changes
- New fields or status conditions
- Deprecated features
- Changes to the reconciliation loop

### 4. Test with Reference RGDs

Deploy and verify reference RGDs against the new KRO version:

```bash
# Create a fresh cluster with the updated KRO
make cluster-up

# Run full QA suite
make qa
```

Verify that all reference RGDs reconcile successfully and instances deploy correctly.

### 5. If Compatibility Failures Occur

If `compat_test.go` tests fail, choose one of three resolution paths:

1. **Update Knodex code** to match the new KRO behavior (preferred if the change is intentional)
2. **Update test baselines** if the new behavior is correct but test expectations are stale
3. **Pin to the previous KRO version** if the change is a regression (open an issue on KRO)

## Version Pin Location

The KRO dependency version is pinned in:

```
server/go.mod
```

Look for the `github.com/kubernetes-sigs/kro` line in the `require` block.

## Prerequisites

Branch protection rules require the following status checks to pass before the PR can be merged:

- All CI jobs green
- No failing compatibility tests
- Successful build for both editions

## Automated Safeguards

| Safeguard | Description |
|-----------|-------------|
| Dependabot group | KRO updates are grouped into a single PR |
| Labels | Dependabot PRs are labeled `dependencies` and `go` |
| CI gate | All tests must pass before merge is allowed |
| Compatibility tests | `compat_test.go` files catch integration regressions |

## KRO Canary Testing

### Purpose

Canary tests continuously validate that Knodex works correctly with the current KRO version. They deploy reference RGDs, create instances, and verify reconciliation -- catching subtle compatibility issues that unit tests might miss.

### Weekly Schedule

Canary tests run automatically:

- **When**: Every Monday at 06:00 UTC
- **Where**: GitHub Actions
- **What**: Deploys a Kind cluster, installs KRO, applies all reference RGDs, verifies reconciliation

### Running Manually

```bash
# Trigger the GitHub Actions workflow
gh workflow run kro-canary

# Or run locally
make cluster-up
make qa
```

### Updating Baselines

When KRO introduces intentional behavior changes:

1. Run canary tests and review all failures
2. Confirm the new behavior is expected (check KRO release notes)
3. Update test fixtures and expected values
4. Commit baseline changes with a descriptive message

### Reference RGDs

| RGD | Description | KRO Min Version |
|-----|-------------|----------------|
| `basic-types` | Simple resource with scalar fields | 0.9.0 |
| `conditional-resources` | Resources with CEL-based conditions | 0.9.0 |
| `external-refs` | Cross-resource references | 0.9.0 |
| `nested-external-refs` | Deeply nested cross-resource references | 0.10.0 |
| `advanced-section` | Complex schema sections | 0.10.0 |
| `cel-expressions` | Advanced CEL expression usage | 0.11.0 |

### Interpreting Canary Issues

When canary tests fail, follow these steps:

1. **Check KRO controller logs**: Look for reconciliation errors or panics
2. **Compare with previous run**: Identify which RGDs started failing
3. **Review recent KRO changes**: Check if a new release was cut since the last passing run
4. **Open an issue**: If the failure is a genuine KRO regression, report it upstream

### Test Locations

| Test Type | Location |
|-----------|----------|
| Reference RGD fixtures | `deploy/examples/` |
| Canary test scripts | `scripts/` |
| Server compatibility tests | `server/internal/kro/**/compat_test.go` |
| E2E canary tests | `server/test/e2e/` |
