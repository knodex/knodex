---
title: RBAC Setup
description: Configure multi-tenant role-based access control with projects, roles, and namespace-scoped permissions using Casbin.
sidebar_position: 3
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# RBAC Setup

Knodex implements ArgoCD-aligned multi-tenant RBAC using [Casbin](https://casbin.org/). Roles are defined in Project CRDs and automatically compiled into namespace-scoped Casbin policies. This provides fine-grained access control without a second authorization layer.

## Built-in Global Role

There is one built-in global role: **`role:serveradmin`**. It grants full access to all resources across all projects.

Users are assigned the `serveradmin` role by mapping their OIDC group to the role in the Knodex settings. For example, users in the `knodex-admins` group receive `role:serveradmin`.

Casbin policy for serveradmin:

```
p, role:serveradmin, *, *, allow
```

## Custom Project Roles

Project roles are defined in the Project CRD and scoped to specific namespaces (destinations). Three common patterns:

### Admin

Full access to all resources within the project's destinations.

| Resource | Actions |
|----------|---------|
| `instances/*` | `get`, `create`, `update`, `delete` |
| `repositories/*` | `get`, `create`, `update`, `delete` |
| `secrets/*` | `get`, `create`, `update`, `delete` |
| `rgds/*` | `get` |
| `projects/*` | `get`, `update` |

### Developer

Can deploy and manage instances, manage repositories and secrets, but cannot modify the project itself.

| Resource | Actions |
|----------|---------|
| `instances/*` | `get`, `create`, `update`, `delete` |
| `repositories/*` | `get`, `create`, `update`, `delete` |
| `secrets/*` | `get`, `create`, `update`, `delete` |
| `rgds/*` | `get` |
| `projects/*` | `get` |

### Readonly

View-only access to all resources.

| Resource | Actions |
|----------|---------|
| `instances/*` | `get` |
| `repositories/*` | `get` |
| `secrets/*` | `get` |
| `rgds/*` | `get` |
| `projects/*` | `get` |

## Defining Roles in the Project CRD

Roles are defined in the `spec.roles` section of a Project custom resource:

```yaml
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: alpha
spec:
  description: "Alpha team project"
  destinations:
    - namespace: alpha-apps
    - namespace: alpha-staging
  roles:
    - name: admin
      groups:
        - "alpha-admins"
      destinations:
        - "alpha-apps"
        - "alpha-staging"
      policies:
        - "instances/*, *, allow"
        - "repositories/*, *, allow"
        - "secrets/*, *, allow"
        - "rgds/*, get, allow"
        - "projects/*, *, allow"

    - name: developer
      groups:
        - "alpha-developers"
      destinations:
        - "alpha-apps"
      policies:
        - "instances/*, *, allow"
        - "repositories/*, *, allow"
        - "secrets/*, *, allow"
        - "rgds/*, get, allow"
        - "projects/*, get, allow"

    - name: readonly
      groups:
        - "alpha-viewers"
      policies:
        - "instances/*, get, allow"
        - "repositories/*, get, allow"
        - "secrets/*, get, allow"
        - "rgds/*, get, allow"
        - "projects/*, get, allow"
```

### How Policy Generation Works

The policy generator combines roles with their destinations to produce namespace-scoped Casbin rules:

```
# Developer role scoped to alpha-apps namespace:
p, proj:alpha:developer, instances/alpha/alpha-apps/*, *, allow
p, proj:alpha:developer, repositories/alpha/alpha-apps/*, *, allow
p, proj:alpha:developer, secrets/alpha/alpha-apps/*, *, allow

# Readonly role (no destinations = project-wide):
p, proj:alpha:readonly, instances/alpha/*, get, allow
p, proj:alpha:readonly, repositories/alpha/*, get, allow
```

## RGD Catalog Access

RGD access policies use the object path format `<project>/<rgd-name>`:

| Pattern | Meaning |
|---------|---------|
| `rgds/*, get, allow` | Read all RGDs in the project |
| `rgds/webapp-rgd, get, allow` | Read only the `webapp-rgd` RGD |

### Common Patterns

| Pattern | Description |
|---------|-------------|
| `instances/*, *, allow` | Full instance access across all destinations |
| `instances/*, get, allow` | Read-only instance access |
| `secrets/*, get, allow` | Read-only secret access |
| `repositories/*, *, allow` | Full repository management |

## Visibility Filter Chain

When a user requests resources, Knodex applies a four-step filter:

1. **Authentication** - Verify the user has a valid OIDC session
2. **Group Resolution** - Map the user's OIDC groups to project roles
3. **Casbin Enforcement** - Check the Casbin policy for the requested resource and action
4. **Namespace Scoping** - If the role has `destinations`, restrict results to those namespaces

## Group Mapping

OIDC groups are mapped to project roles via the `groups` field in each role definition. A user's OIDC token must include the group claim (typically `groups`) matching one of the listed groups.

```yaml
roles:
  - name: developer
    groups:
      - "azure-ad-alpha-devs"    # Azure AD group
      - "okta-alpha-developers"  # Okta group
    policies:
      - "instances/*, *, allow"
```

## Multi-Project Assignment

A user can belong to multiple projects through different OIDC groups:

```yaml
# Project: alpha
roles:
  - name: developer
    groups: ["team-alpha"]
    policies:
      - "instances/*, *, allow"

# Project: beta
roles:
  - name: readonly
    groups: ["team-alpha"]  # Same group, different role
    policies:
      - "instances/*, get, allow"
```

## Role Precedence

When a user matches multiple roles within a project, Casbin evaluates all matching policies. The precedence hierarchy:

1. **`role:serveradmin`** - Global admin, overrides all project roles
2. **Project roles with `destinations`** - Namespace-scoped, most specific
3. **Project roles without `destinations`** - Project-wide scope

Casbin uses a first-match model. Explicit `allow` policies grant access; anything not explicitly allowed is denied by default.

## User Management Lifecycle

| Event | Action |
|-------|--------|
| **New user** | Add their OIDC group to the appropriate role in the Project CRD |
| **Role change** | Move the group assignment between roles, or update the policies |
| **Offboarding** | Remove the user from the OIDC group in your identity provider |
| **Emergency revoke** | Remove the group from all Project CRD roles; takes effect on next API request |

:::note[No Local Users]
Knodex does not maintain a local user database. All user identity comes from the OIDC provider. Revoking access means removing the user from the relevant OIDC groups.
:::

## Deployment Scenarios

### Startup -- No Segregation

A small team where everyone manages everything. One project with broad namespace access.

```yaml
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: default
spec:
  destinations:
    - namespace: "*"
  roles:
    - name: team-member
      groups: ["engineering"]
      policies:
        - "instances/*, *, allow"
        - "secrets/*, *, allow"
        - "rgds/*, get, allow"
```

All categories are visible to everyone. No infrastructure/application split.

### Enterprise -- Segregated Platform and Developer Roles

A larger organization with dedicated platform engineers and application developers. Three namespaces separate concerns.

```yaml
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: alpha
spec:
  destinations:
    - namespace: "alpha-platform"
      name: "Platform Infrastructure"
    - namespace: "alpha-applications"
      name: "Application Workloads"
    - namespace: "alpha-shared"
      name: "Shared Services"
  roles:
    - name: platform-engineer
      groups: ["alpha-platform-team"]
      destinations: ["alpha-platform", "alpha-shared"]
      policies:
        - "instances/*, *, allow"
        - "secrets/*, *, allow"
        - "rgds/*, get, allow"
    - name: developer
      groups: ["alpha-developers"]
      destinations: ["alpha-applications"]
      policies:
        - "instances/*, *, allow"
        - "secrets/*, *, allow"
        - "rgds/*, get, allow"
    - name: shared-reader
      groups: ["alpha-developers"]
      destinations: ["alpha-shared"]
      policies:
        - "secrets/*, get, allow"
```

Platform engineers deploy infrastructure RGDs to `alpha-platform`. Developers deploy application RGDs to `alpha-applications`. Developers can read shared secrets but cannot deploy to `alpha-shared`.

### Fine-Grained Category Control

Categories combined with Casbin policies allow per-team visibility of different RGD types.

```yaml
roles:
  - name: infra-admin
    groups: ["infra-admins"]
    policies:
      - "instances/*, *, allow"
      - "secrets/*, *, allow"
      - "rgds/networking/*, get, allow"
      - "rgds/databases/*, get, allow"
      - "rgds/storage/*, get, allow"
  - name: network-admin
    groups: ["network-team"]
    policies:
      - "instances/*, *, allow"
      - "rgds/networking/*, get, allow"
  - name: developer
    groups: ["app-developers"]
    policies:
      - "instances/*, *, allow"
      - "rgds/applications/*, get, allow"
      - "rgds/web/*, get, allow"
```

The `infra-admin` sees Networking, Databases, and Storage categories in the sidebar. The `network-admin` sees only Networking. The `developer` sees Applications and Web.

## Key Principles

1. **RGDs are equal.** The system does not classify RGDs as "infrastructure" or "application." Categories are purely organizational labels set by the RGD author.

2. **Casbin is the single enforcement layer.** All access decisions flow through Casbin policies. There is no secondary authorization layer for categories or destinations.

3. **Categories control visibility, not capability.** A user who cannot see a category in the sidebar also cannot deploy its RGDs, because the Casbin `rgds/{category}/*` check fails. But category visibility alone does not grant deploy permissions -- the user also needs `instances/*` policies.

4. **Namespace scoping is additive.** Role `destinations` narrow where a role applies. A role without `destinations` has project-wide scope (backward compatible). A role with `destinations: ["dev-*"]` can only act in matching namespaces.
