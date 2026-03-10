---
title: "Deployment Modes"
linkTitle: "Deployment Modes"
description: "Choose how instances are deployed: directly to Kubernetes, via GitOps, or both"
weight: 5
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Deployment Modes

Choose how instances are deployed: directly to Kubernetes, via GitOps, or both.

## Overview

When deploying an instance from the catalog, you can choose between three deployment modes:

| Mode       | Description                               | Use Case                                    |
| ---------- | ----------------------------------------- | ------------------------------------------- |
| **Direct** | Deploy directly to the Kubernetes cluster | Quick deployments, development environments |
| **GitOps** | Push manifest to Git repository only      | Production environments with ArgoCD/Flux    |
| **Hybrid** | Deploy to cluster AND push to Git         | Immediate deployment with Git audit trail   |

## Selecting a Deployment Mode

### Step 1: Start Deployment

1. Navigate to **Catalog** in the left sidebar
2. Find the RGD you want to deploy
3. Click **Deploy**

### Step 2: Fill in Parameters

Complete the deployment form with the required parameters for your instance.

### Step 3: Choose Deployment Mode

At the bottom of the deployment form, you will see the **Deployment Mode** selector:

```
┌─────────────────────────────────────────┐
│  Deployment Mode                        │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │ Direct  │ │ GitOps  │ │ Hybrid  │   │
│  └─────────┘ └─────────┘ └─────────┘   │
└─────────────────────────────────────────┘
```

Click on your preferred mode before deploying.

## Deployment Modes Explained

### Direct Deployment

**What happens:** The instance manifest is applied directly to the Kubernetes cluster.

**When to use:**

- Development and testing environments
- Quick iterations
- When GitOps is not required

**UI Flow:**

1. Select **Direct** mode
2. Click **Deploy**
3. Instance is created immediately in the cluster
4. View status on the Instances page

### GitOps Deployment

**What happens:** The instance manifest is committed to a configured Git repository. An external GitOps tool (ArgoCD, Flux) syncs the manifest to the cluster.

**When to use:**

- Production environments
- When all changes must go through Git
- Audit and compliance requirements

**Prerequisites:**

- Project must have a repository configured (see [Repository Configuration](../../operator-manual/repository-setup/))
- GitOps tool (ArgoCD/Flux) must be configured to watch the repository

**UI Flow:**

1. Select **GitOps** mode
2. Click **Deploy**
3. Manifest is committed to the configured repository
4. GitOps tool detects the change and syncs to cluster
5. Once synced, instance appears on the Instances page

{{< alert title="Note" >}}
With GitOps mode, there may be a delay before the instance appears in knodex. This depends on your GitOps tool's sync interval.
{{< /alert >}}

{{< alert color="warning" title="Secret Exposure Risk" >}}
Knodex does not currently redact or prevent secrets from being committed to Git. If your RGD spec contains sensitive values (passwords, API keys, connection strings), those values will be stored as plaintext in the Git repository. Use Kubernetes Secrets, ExternalSecrets, or sealed-secrets to manage sensitive data outside of the instance spec.
{{< /alert >}}

### Hybrid Deployment

**What happens:** The instance is deployed directly to the cluster AND the manifest is committed to Git.

**When to use:**

- Immediate deployment needed with Git audit trail
- Environments transitioning to full GitOps
- When you want both speed and traceability

**Prerequisites:**

- Project must have a repository configured

**UI Flow:**

1. Select **Hybrid** mode
2. Click **Deploy**
3. Instance is created immediately in the cluster
4. Manifest is also committed to the repository
5. View status on the Instances page

## Repository Requirements

GitOps and Hybrid modes require a repository to be configured for your project.

**Check repository status:**

1. Navigate to **Settings** → **Projects**
2. Select your project
3. Look for the **Repository** section

If no repository is configured, only **Direct** mode will be available.

**Contact your Platform Administrator** to configure a repository for GitOps deployments.

## Mode Availability

The available deployment modes depend on your project configuration:

| Project Configuration    | Available Modes        |
| ------------------------ | ---------------------- |
| No repository configured | Direct only            |
| Repository configured    | Direct, GitOps, Hybrid |

## Best Practices

- **Development:** Use Direct mode for fast iteration
- **Staging:** Use Hybrid mode for testing with Git trail
- **Production:** Use GitOps mode for full audit compliance

---

**Next:** [Browsing the Catalog](../browsing-catalog/) | **Previous:** [Managing Instances](../managing-instances/)
