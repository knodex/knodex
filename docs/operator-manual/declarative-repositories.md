---
title: "Declarative Repositories"
linkTitle: "Declarative Repositories"
description: "Configure repository credentials declaratively using Kubernetes Secrets"
weight: 6
product_tags:
  - oss
  - enterprise
---

# Declarative Repository Configuration

Repository credentials can be configured declaratively by creating Kubernetes Secrets with the appropriate labels. This is the recommended approach for GitOps workflows.

## Secret Format

Repository secrets must have the label `knodex.io/secret-type: repository`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-repo
  namespace: knodex # Must match server's namespace
  labels:
    knodex.io/secret-type: repository
type: Opaque
stringData:
  # Required fields
  url: https://github.com/myorg/myrepo.git
  project: my-project
  type: https # https, ssh, or github-app

  # Optional fields
  name: My Repository
  defaultBranch: main
  enabled: "true"

  # Credentials (based on type)
  bearerToken: ghp_xxxxxxxxxxxx
```

## Authentication Types

### HTTPS with Token

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: repo-https-example
  namespace: knodex
  labels:
    knodex.io/secret-type: repository
type: Opaque
stringData:
  url: https://github.com/myorg/myrepo.git
  project: my-project
  type: https
  bearerToken: ghp_xxxxxxxxxxxx
```

### SSH

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: repo-ssh-example
  namespace: knodex
  labels:
    knodex.io/secret-type: repository
type: Opaque
stringData:
  url: git@github.com:myorg/myrepo.git
  project: my-project
  type: ssh
  sshPrivateKey: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...
    -----END OPENSSH PRIVATE KEY-----
```

### GitHub App

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: repo-ghapp-example
  namespace: knodex
  labels:
    knodex.io/secret-type: repository
type: Opaque
stringData:
  url: https://github.com/myorg/myrepo.git
  project: my-project
  type: github-app
  githubAppId: "123456"
  githubAppInstallationId: "789012"
  githubAppPrivateKey: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
```

## Secret Fields Reference

| Field                     | Required       | Description                                          |
| ------------------------- | -------------- | ---------------------------------------------------- |
| `url`                     | Yes            | Repository URL                                       |
| `project`                 | Yes            | Project ID this repository belongs to                |
| `type`                    | Yes            | Authentication type: `https`, `ssh`, or `github-app` |
| `name`                    | No             | Display name (defaults to repo name from URL)        |
| `defaultBranch`           | No             | Default branch (defaults to `main`)                  |
| `enabled`                 | No             | Whether repository is enabled (defaults to `true`)   |
| `bearerToken`             | For HTTPS      | GitHub/GitLab personal access token                  |
| `sshPrivateKey`           | For SSH        | SSH private key in PEM format                        |
| `githubAppId`             | For GitHub App | GitHub App ID                                        |
| `githubAppInstallationId` | For GitHub App | GitHub App Installation ID                           |
| `githubAppPrivateKey`     | For GitHub App | GitHub App private key in PEM format                 |

## Namespace

Secrets must be created in the same namespace as the knodex server. The default namespace is `knodex`.

## RBAC

The knodex service account requires permissions to manage secrets:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: knodex-secret-manager
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["create", "get", "list", "update", "delete"]
```
