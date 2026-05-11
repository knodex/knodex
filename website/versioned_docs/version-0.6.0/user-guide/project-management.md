---
title: Project Management
description: Manage project roles, destinations, and repositories within Knodex projects.
sidebar_position: 6
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Project Management

Projects in Knodex provide isolation and access control for teams. Projects are defined as Kubernetes Custom Resources (Project CRD) and managed through the UI or kubectl.

:::note[Creating Projects]
Project creation is restricted to server administrators (`role:serveradmin`). Contact your platform team to request a new project.
:::

## Project Detail Page

Click a project name in the **Projects** list to open its detail page. The detail page has three tabs:

### Overview Tab

Shows project details including Name, Description, Roles count, Destinations count, Created/Updated timestamps, and an **Edit** button for administrators.

### Roles Tab

Lists all roles defined for the project. Each role includes:

- **Name** -- The role identifier (e.g., `admin`, `developer`, `readonly`)
- **Groups** -- OIDC groups mapped to this role
- **Destinations** -- Namespaces this role is scoped to (empty = project-wide)
- **Policies** -- Casbin policy rules defining permissions

Roles determine what users can do within the project. Users are assigned roles through OIDC group membership -- when a user's OIDC token includes a group listed in a role's `groups` field, they receive that role's permissions.

### Destinations Tab

Lists the namespaces where project members can deploy instances. Each destination shows:

- **Namespace** -- The Kubernetes namespace
- **Name** -- An optional display name for the destination

## Managing Roles

Roles are defined in the Project CRD's `spec.roles` section. Common patterns:

| Role | Typical Permissions |
|------|-------------|
| **admin** | Full access to instances, repositories, secrets, RGDs, and project settings |
| **developer** | Deploy and manage instances, manage secrets, browse catalog |
| **readonly** | View-only access to all project resources |

See [RBAC Setup](../administration/rbac-setup) for detailed policy configuration and examples.

## Managing Repositories

Connecting Git repositories enables GitOps and Hybrid deployment modes for the project.

### Viewing Repositories

Navigate to **Repositories** in the sidebar to see all connected repositories with their URL, branch, status, and last sync time.

### Adding a Repository

1. Click **Add Repository**
2. Fill in the required fields:

| Field | Description |
|-------|-------------|
| **URL** | The Git repository URL (HTTPS or SSH) |
| **Branch** | The default branch to use for deployments |
| **Path** | The directory path within the repository for manifests |
| **Authentication** | GitHub App (recommended), Personal Access Token, or SSH key |

3. Click **Save**

### Testing Connection

After adding a repository, click **Test Connection** to verify that Knodex can reach the repository and has the required permissions.

### Removing a Repository

:::note[Existing Deployments]
Removing a repository does not affect instances already deployed via GitOps. Future deployments will not be able to use this repository until it is re-added.
:::

1. Click the remove button next to the repository
2. Confirm the removal

## Role Permissions Summary

| Action | Readonly | Developer | Admin |
|--------|----------|-----------|-------|
| View project details | Yes | Yes | Yes |
| Browse catalog | Yes | Yes | Yes |
| View instances | Yes | Yes | Yes |
| Deploy instances | No | Yes | Yes |
| Delete instances | No | Yes | Yes |
| Manage secrets | No | Yes | Yes |
| Manage repositories | No | No | Yes |
| Edit project settings | No | No | Yes |

## FAQ

**How do I create a new project?**
Projects are created by server administrators via the Projects page or by applying a Project CRD with kubectl. Contact your platform team to request a new project.

**Why is GitOps not available for my deployments?**
GitOps mode requires at least one repository connected to the project. Ask a project admin to add a repository.

**How are users assigned to projects?**
Users are assigned to projects through OIDC group membership. When a user's identity token includes a group that matches a role's `groups` field in the Project CRD, they receive that role's permissions. There is no per-user assignment -- access is managed through your identity provider's group management.

**What happens to instances when a user loses access?**
Instances continue to run. They belong to the project, not to individual users.
