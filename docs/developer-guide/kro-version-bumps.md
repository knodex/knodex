# KRO Version Bump Review Process

Knodex depends on [kubernetes-sigs/kro](https://github.com/kubernetes-sigs/kro) for CRD schema parsing and CEL expression handling. Dependabot creates a dedicated PR whenever a new KRO version is released.

## Reviewing a KRO Bump PR

### 1. Check CI results

The CI pipeline runs both OSS and enterprise test suites on every PR:

- **Server Tests** — `go test ./...` catches compile-time breakage
- **Server Tests (Enterprise)** — `go test -tags=enterprise ./...` verifies enterprise-only code
- **Build** — `go build ./...` and `go build -tags=enterprise ./...`

If any of these fail, the KRO update introduces a breaking change that must be addressed before merging.

### 2. Check compat_test.go results

Compatibility tests live alongside the KRO wrapper packages:

| Package | Test File | What It Covers |
|---------|-----------|----------------|
| `internal/kro/cel` | `compat_test.go` | CEL expression compilation and evaluation |
| `internal/kro/parser` | `compat_test.go` | Resource parsing from RGD specs |
| `internal/kro/metadata` | `compat_test.go` | RGD metadata extraction |
| `internal/kro/schema` | `compat_test.go` | CRD/RGD schema enrichment |

These tests exercise real KRO library functions with known inputs. A failure here means KRO changed behavior that Knodex relies on.

### 3. Review the KRO changelog

Check the [KRO releases page](https://github.com/kubernetes-sigs/kro/releases) for:

- **Breaking changes** in exported types or functions Knodex uses
- **Behavioral changes** in CEL evaluation, schema generation, or resource parsing
- **New features** that Knodex should adopt (e.g., new label conventions)

### 4. Test with reference RGDs (if behavioral changes detected)

If the changelog mentions changes to schema generation, CEL handling, or resource parsing:

```bash
make cluster-up
make qa
```

This deploys Knodex with example RGDs and runs E2E tests to verify end-to-end functionality.

## Version Pin Location

The KRO dependency is pinned in `server/go.mod` with a comment explaining the bump process. Dependabot updates this version automatically in its PRs.

### 5. If compat_test failures occur

Typical resolution paths when a KRO bump breaks compatibility tests:

1. **API signature changed** — Update the wrapper function in the corresponding `internal/kro/` package to match the new KRO API
2. **Behavioral change** — Adjust test expectations if the new behavior is correct, or add adapter logic in the wrapper
3. **Breaking change requiring significant work** — Reject the bump PR, pin the current version in `go.mod`, and create a story to plan the migration

## Prerequisites

- **Branch protection:** Ensure branch protection rules on `main` require the `Server Tests`, `Server Tests (Enterprise)`, and `Build Enterprise Image` status checks to pass. Without this, KRO bump PRs could be merged with failing CI.

## Automated Safeguards

- **Dependabot group:** KRO gets its own PR (not grouped with other Go dependencies)
- **Labels:** KRO bump PRs are automatically labeled with `kro` via `.github/workflows/label-kro-prs.yml`
- **CI gate:** PRs cannot merge without passing all test suites including enterprise tests (requires branch protection)
- **Compat tests:** Purpose-built tests that break when KRO changes affect Knodex
