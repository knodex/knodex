---
title: Managing Credentials
description: Configure repository credentials and manage secrets for Git provider authentication.
sidebar_position: 4
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Managing Credentials

Repository credentials authenticate Knodex with Git providers for GitOps and Hybrid deployment modes. Credentials are stored as Kubernetes Secrets in the Knodex namespace.

## Repository Credentials

Each repository connection requires credentials that grant Knodex read/write access to the target repository. Credentials can be configured:

1. **Through the UI** -- Enter credentials when adding or editing a repository
2. **Declaratively** -- Create Kubernetes Secrets that Knodex discovers automatically

## Secret Management for Repositories

### Creating Credentials via UI

When adding a repository in the project settings, the credential input varies by authentication method:

| Method | Required Input |
|--------|---------------|
| Personal Access Token | Token string |
| GitHub App | App ID, Installation ID, Private Key |
| SSH Key | Private key content |

Credentials entered through the UI are stored as Kubernetes Secrets in the Knodex namespace with appropriate labels for discovery.

### Rotating Credentials

To rotate repository credentials:

1. Navigate to the repository settings
2. Click **Edit Credentials**
3. Enter the new credential values
4. Click **Save** and **Test Connection**

The old credential Secret is updated in place. No deployment interruption occurs.

## Declarative Configuration via Kubernetes Secrets

For GitOps-managed Knodex installations, repository credentials can be declared as Kubernetes Secrets that Knodex discovers automatically.

### Secret Format

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: repo-github-org-repo
  namespace: knodex
  labels:
    knodex.io/secret-type: "repository"
  annotations:
    knodex.io/repo-url: "https://github.com/org/repo.git"
type: Opaque
stringData:
  # For PAT authentication
  token: "ghp_xxxxxxxxxxxxxxxxxxxx"

  # For SSH authentication (use this instead of token)
  # sshPrivateKey: |
  #   -----BEGIN OPENSSH PRIVATE KEY-----
  #   ...
  #   -----END OPENSSH PRIVATE KEY-----
```

### Discovery Labels

| Label | Required | Value | Description |
|-------|----------|-------|-------------|
| `knodex.io/secret-type` | Yes | `"repository"` | Marks the secret for repository credential discovery |

### Discovery Annotations

| Annotation | Required | Description |
|-----------|----------|-------------|
| `knodex.io/repo-url` | Yes | The repository URL this credential authenticates against |

## Security Considerations

- Repository secrets are stored encrypted at rest (via Kubernetes Secret encryption)
- Secrets are only accessible by the Knodex server ServiceAccount
- Token values are never returned in API responses (write-only)
- Audit trails (Enterprise) log credential creation and rotation events

## Full Reference

For the complete declarative repository configuration reference, including multi-repository setups and advanced authentication options, see the [Operator Manual: Declarative Repositories](../administration/repositories).
