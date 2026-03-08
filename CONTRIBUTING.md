# Contributing to Knodex

Thanks for your interest in contributing! This document covers the process for contributing to this project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/knodex.git`
3. Create a feature branch: `git checkout -b feature/your-feature`
4. Set up your development environment (see [Developer Guide](docs/developer-guide.md))

## Development Workflow

1. Make changes following project conventions
2. Add tests for new functionality
3. Run linting: `make lint`
4. Run tests: `make test`
5. Commit with a conventional commit message (see below)
6. Push and open a PR against `main`
7. Wait for CI checks to pass
8. Address review feedback
9. Squash merge when approved

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>
```

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `refactor` | Code refactoring |
| `test` | Adding or updating tests |
| `chore` | Maintenance tasks |

Example: `feat(auth): add OIDC provider support`

## Licensing

Knodex uses a **dual-license model**:

- **`server/internal/`, `server/app/`, `web/`, root files** — AGPLv3 (`AGPL-3.0-only`)
- **`server/ee/`** — Proprietary (Knodex Enterprise License)

All new source files (`.go`, `.ts`, `.tsx`) **must** include the correct SPDX header:

```go
// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only
```

For enterprise files in `server/ee/`, use `LicenseRef-Knodex-Enterprise` instead.

Run `scripts/add-spdx-headers.sh` to add headers to new files automatically. CI will reject PRs with missing or incorrect headers.

See the [NOTICE](NOTICE) file for full licensing details. Contributors to `server/ee/` must be authorized by Provops LLC.

## Reporting Issues

- Search existing issues before creating new ones
- Use issue templates when available
- Include reproduction steps for bugs
- Provide environment details (OS, Go/Node versions, K8s version)

## Questions?

Open a [discussion](https://github.com/knodex/knodex/discussions) or issue if you need help getting started.
