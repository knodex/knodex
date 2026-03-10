---
title: "RBAC Setup"
linkTitle: "RBAC Setup"
description: "Role-Based Access Control configuration for knodex"
weight: 3
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# RBAC Setup

Role-Based Access Control configuration for knodex.

## Overview

knodex implements a multi-tenant RBAC system aligned with ArgoCD patterns:

- **1 Built-in Global Role:** `role:serveradmin` (full access)
- **Custom Project Roles:** Define any custom roles in Project CRD with explicit policies
- **Project-scoped Resources:** RGDs, instances, repositories (ArgoCD-aligned)
- **Group-based Assignment:** OIDC groups mapped to roles
- **Permission Caching:** Redis-backed for performance

> **Note:** knodex uses ArgoCD-aligned "Project" terminology. Custom roles like `developer`,
> `platform-admin`, or `readonly` are defined in Project CRD, not as built-in global roles.
> There is no built-in global `role:readonly` — read-only access is project-scoped only
> (`proj:{project}:readonly`).

## Built-in Roles

knodex provides one built-in global role:

### role:serveradmin

Full platform access across all projects and resources.

| Resource         | Permissions                               |
| ---------------- | ----------------------------------------- |
| **Projects**     | Create, read, update, delete all projects |
| **Members**      | Add/remove members from any project       |
| **RGDs**         | View all RGDs (all namespaces)            |
| **Instances**    | Deploy, view, delete all instances        |
| **Repositories** | Configure repositories for any project    |
| **Settings**     | Modify platform-wide settings             |

**Casbin Policy:**

```csv
p, role:serveradmin, *, *, allow
```

## Custom Project Roles

Define custom roles within Project CRD for fine-grained access control. Common patterns:

### admin (Project-scoped)

Full control within assigned project(s).

| Resource         | Permissions                                         |
| ---------------- | --------------------------------------------------- |
| **Project**      | Read, update own project                            |
| **Members**      | Add/remove members within project                   |
| **RGDs**         | View project RGDs + shared RGDs                     |
| **Instances**    | Deploy, view, delete instances in project namespace |
| **Repositories** | Configure repositories for project                  |

### developer (Project-scoped)

Can deploy and manage instances within assigned project(s).

| Resource         | Permissions                        |
| ---------------- | ---------------------------------- |
| **Project**      | Read own project                   |
| **Members**      | Read project members               |
| **RGDs**         | View project RGDs + shared RGDs    |
| **Instances**    | Deploy, view, delete own instances |
| **Repositories** | Read repository configuration      |

### readonly (Project-scoped)

Read-only access to project resources.

| Resource         | Permissions                         |
| ---------------- | ----------------------------------- |
| **Project**      | Read own project                    |
| **Members**      | Read project members                |
| **RGDs**         | View project RGDs + shared RGDs     |
| **Instances**    | View instances in project namespace |
| **Repositories** | Read repository configuration       |

### Defining Custom Roles in Project CRD

```yaml
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: engineering
spec:
  roles:
    - name: admin
      groups:
        - engineering-leads
      policies:
        - "projects/engineering, *, allow"
        - "instances/engineering/*, *, allow"
        - "repositories/engineering/*, *, allow"
    - name: developer
      groups:
        - engineering-devs
      policies:
        - "projects/engineering, get, allow"
        - "instances/engineering/*, create, allow"
        - "instances/engineering/*, get, allow"
        - "instances/engineering/*, delete, allow"
    - name: readonly
      groups:
        - engineering-viewers
      policies:
        - "projects/engineering, get, allow"
        - "instances/engineering/*, get, allow"
```

## Group Mapping Configuration

Map OIDC groups to the built-in global role or project roles:

```yaml
server:
  config:
    oidc:
      groupMappings:
        # Built-in global role
        - group: "platform-admins"
          role: "role:serveradmin" # Full platform access


        # Project-scoped roles are defined in Project CRD
        # See "Defining Custom Roles in Project CRD" section above
        # Read-only access is granted via proj:{project}:readonly,
        # not via a global built-in role.
```

> **Note:** Project-scoped roles (admin, developer, readonly) are defined in Project CRD's
> `spec.roles` section, not in Helm values. This aligns with ArgoCD patterns and allows
> per-project customization. The global `role:readonly` no longer exists — auditors or
> monitoring systems requiring read-only access should be assigned project-scoped readonly
> roles in each relevant project's CRD.

### Multi-Project Assignment

Users can belong to multiple projects with different roles via Project CRD:

```yaml
# engineering Project CRD
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: engineering
spec:
  roles:
    - name: admin
      groups:
        - engineering-leads # John belongs to this group
      # ... policies

---
# infrastructure Project CRD
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: infrastructure
spec:
  roles:
    - name: developer
      groups:
        - infra-devs # John also belongs to this group
      # ... policies
```

### Role Precedence

When a user has multiple roles, the highest privilege applies:

**Role Hierarchy:**

```text
role:serveradmin > project-scoped admin > project-scoped developer > project-scoped readonly
```

The global `role:serveradmin` grants full access to everything and supersedes all
project-scoped roles. Project-scoped roles are evaluated within their project scope.

## User Management

Users are managed through OIDC integration. knodex does not store user credentials locally.

### User Lifecycle

1. **Authentication**: Users authenticate via OIDC provider
2. **Provisioning**: User records created automatically on first login
3. **Role Assignment**: Roles assigned via OIDC group mappings
4. **Multi-Project Membership**: Users can belong to multiple projects

### Project Membership

Users can belong to multiple projects with different roles:

- Users are mapped to projects via OIDC groups defined in Project CRD
- Each membership includes a role assignment (admin, developer, readonly, or custom)
- Project admins manage members within their project
- Users with `role:serveradmin` manage all users across all projects

For OIDC configuration, see [OIDC Integration](../oidc-integration/).

---

**Next:** [OIDC Integration](../oidc-integration/) →
