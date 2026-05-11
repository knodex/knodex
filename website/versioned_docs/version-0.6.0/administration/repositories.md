---
title: Declarative Repositories
description: Configure Git repository credentials declaratively using Kubernetes Secrets for GitOps workflows.
sidebar_position: 7
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Declarative Repositories

Git repositories can be configured declaratively using Kubernetes Secrets, enabling GitOps-managed repository credentials. This is the recommended approach for production deployments.

## Secret Format

Repository credentials are stored as Kubernetes Secrets with specific labels:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-repo
  namespace: knodex
  labels:
    knodex.io/secret-type: repository
    knodex.io/project: "alpha"
type: Opaque
stringData:
  url: "https://github.com/my-org/my-repo.git"
  type: "https"
  username: "git"
  password: "ghp_xxxxxxxxxxxxxxxxxxxx"
```

The `knodex.io/secret-type: repository` label is required for Knodex to discover the Secret as a repository credential.

## Authentication Types

### HTTPS with Token

The most common authentication method, using a personal access token or GitHub App token:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-https
  namespace: knodex
  labels:
    knodex.io/secret-type: repository
    knodex.io/project: "alpha"
type: Opaque
stringData:
  url: "https://github.com/my-org/my-repo.git"
  type: "https"
  username: "git"
  password: "ghp_xxxxxxxxxxxxxxxxxxxx"
```

### SSH with Private Key

For SSH-based authentication:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-ssh
  namespace: knodex
  labels:
    knodex.io/secret-type: repository
    knodex.io/project: "alpha"
type: Opaque
stringData:
  url: "git@github.com:my-org/my-repo.git"
  type: "ssh"
  sshPrivateKey: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...
    -----END OPENSSH PRIVATE KEY-----
```

### GitHub App

For GitHub App-based authentication:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-app
  namespace: knodex
  labels:
    knodex.io/secret-type: repository
    knodex.io/project: "alpha"
type: Opaque
stringData:
  url: "https://github.com/my-org/my-repo.git"
  type: "github-app"
  githubAppID: "12345"
  githubAppInstallationID: "67890"
  githubAppPrivateKey: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
```

## Fields Reference

| Field | Required | Description |
|-------|----------|-------------|
| `url` | Yes | Full repository URL |
| `type` | Yes | Authentication type: `https`, `ssh`, or `github-app` |
| `username` | HTTPS only | Username for HTTPS auth (typically `git`) |
| `password` | HTTPS only | Personal access token or password |
| `sshPrivateKey` | SSH only | PEM-encoded SSH private key |
| `githubAppID` | GitHub App only | GitHub App ID |
| `githubAppInstallationID` | GitHub App only | GitHub App installation ID |
| `githubAppPrivateKey` | GitHub App only | PEM-encoded RSA private key for the GitHub App |

## Namespace Requirement

Repository secrets must be created in the **Knodex server namespace** (default: `knodex`), not in project namespaces. The `knodex.io/project` label determines which project the repository belongs to.

```yaml
metadata:
  namespace: knodex          # Must be the Knodex namespace
  labels:
    knodex.io/project: "alpha"  # Assigns to project "alpha"
```

## RBAC

Repository access is controlled through Casbin policies. Users need the `repositories` resource permission:

```yaml
# Full repository management
policies:
  - "repositories/*, *, allow"

# Read-only repository access
policies:
  - "repositories/*, get, allow"
```

See [RBAC Setup](rbac-setup) for complete role configuration.
