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

## Reporting Issues

- Search existing issues before creating new ones
- Use issue templates when available
- Include reproduction steps for bugs
- Provide environment details (OS, Go/Node versions, K8s version)

## Questions?

Open a [discussion](https://github.com/knodex/knodex/discussions) or issue if you need help getting started.
