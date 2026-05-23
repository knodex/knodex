---
title: Deployment Modes
description: Understand the three deployment modes in Knodex -- Direct, GitOps, and Hybrid -- and when to use each.
sidebar_position: 5
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Deployment Modes

Knodex supports three deployment modes that control how instance manifests are applied to the cluster. Each mode offers different trade-offs between convenience and auditability.

## Overview

| Mode | How It Works | Best For |
|------|-------------|----------|
| **Direct** | Manifest is applied directly to the Kubernetes cluster | Development, testing, quick iterations |
| **GitOps** | Manifest is committed to a Git repository; a GitOps controller (e.g., ArgoCD) applies it | Production, auditable environments |
| **Hybrid** | Manifest is applied directly AND committed to Git for record-keeping | Staging, environments transitioning to GitOps |

## Selecting a Deployment Mode

When deploying an instance, select the deployment mode in the deployment form:

```
+--------------------------------------------------+
| Deployment Mode                                   |
+--------------------------------------------------+
|  (o) Direct    ( ) GitOps    ( ) Hybrid           |
|                                                   |
|  Repository:   [ select repository...       v ]   |
|  Branch:       [ main                        ]    |
|  Path:         [ manifests/                  ]    |
+--------------------------------------------------+
```

The repository, branch, and path fields appear only when GitOps or Hybrid mode is selected.

## Direct Mode

### What Happens

The instance manifest is applied directly to the Kubernetes cluster using the Knodex server's service account. No Git repository is involved.

### When to Use

- Development and testing environments
- Quick prototyping and iteration
- Environments where GitOps infrastructure is not available

### UI Flow

1. Fill in the deployment form.
2. Ensure **Direct** is selected as the deployment mode.
3. Click **Deploy**.
4. The manifest is applied immediately to the cluster.

## GitOps Mode

### What Happens

The instance manifest is committed to a connected Git repository. A GitOps controller (such as ArgoCD or Flux) detects the change and applies the manifest to the cluster.

### When to Use

- Production environments
- Regulated environments requiring audit trails
- Teams following GitOps practices

### Prerequisites

- A Git repository must be connected to the project (see [Project Management](project-management))
- The repository must be accessible from the Knodex server
- A GitOps controller must be configured to watch the target repository and path

:::warning[Secret Exposure Risk]
When using GitOps mode, instance manifests are committed to a Git repository. If your manifest references secrets by value (rather than by Kubernetes Secret reference), those values will be stored in Git. Always use Kubernetes Secret references or sealed secrets to avoid exposing sensitive data in your repository.
:::

### UI Flow

1. Fill in the deployment form.
2. Select **GitOps** as the deployment mode.
3. Choose the target repository, branch, and path.
4. Click **Deploy**.
5. The manifest is committed to the repository. The GitOps controller applies it to the cluster.

## Hybrid Mode

### What Happens

The manifest is applied directly to the cluster AND committed to the Git repository. This provides immediate deployment with Git-based record-keeping.

### When to Use

- Staging environments
- Teams transitioning from direct deployments to full GitOps
- Scenarios requiring immediate deployment with an audit trail

### Prerequisites

Same as GitOps mode -- a connected repository and appropriate access.

### UI Flow

1. Fill in the deployment form.
2. Select **Hybrid** as the deployment mode.
3. Choose the target repository, branch, and path.
4. Click **Deploy**.
5. The manifest is applied to the cluster immediately and committed to the repository.

## Repository Requirements

For GitOps and Hybrid modes, the connected repository must meet these requirements:

- The Knodex server must have write access (via a configured access token)
- The target branch must exist
- The target path must be within the repository root
- The GitOps controller must be configured to reconcile from the same repository and path

See [Project Management](project-management) for how to connect repositories to your project.

## Mode Availability

Not all modes are available in every configuration:

| Condition | Direct | GitOps | Hybrid |
|-----------|--------|--------|--------|
| No repository connected | Available | Unavailable | Unavailable |
| Repository connected | Available | Available | Available |
| Viewer role | Unavailable | Unavailable | Unavailable |
| Developer role or higher | Available | Available | Available |

## Best Practices

| Environment | Recommended Mode | Reason |
|-------------|-----------------|--------|
| Development | Direct | Fast iteration, no Git overhead |
| Staging | Hybrid | Immediate deployment with audit trail |
| Production | GitOps | Full auditability, rollback via Git, GitOps controller reconciliation |
