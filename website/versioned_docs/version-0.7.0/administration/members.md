---
title: Managing Members
description: Add, remove, and manage project members and their role assignments in Knodex.
sidebar_position: 2
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Managing Members

Project members are users or OIDC groups that have been granted access to a project through role assignments. This page covers member management operations.

## Viewing Project Members

1. Navigate to **Projects** in the sidebar
2. Select the project
3. The **Members** or **Role Bindings** section shows all current role assignments

Role bindings display:
- User email or OIDC group name
- Assigned role (e.g., platform-admin, developer, viewer)
- Binding type (user or group)

## Adding Members

### Adding a User

```
POST /api/v1/projects/{name}/roles/{role}/users/{user}
```

In the UI:
1. Navigate to the project settings
2. Click **Add Member**
3. Enter the user's email or identifier
4. Select a role from the dropdown
5. Click **Save**

### Adding a Group

OIDC groups can be bound to roles directly in the Project CRD or via the API:

```
POST /api/v1/projects/{name}/roles/{role}/groups/{group}
```

Group bindings automatically grant the role to all users in the OIDC group. This is the recommended approach for managing access at scale.

### Via Project CRD

Groups are typically defined in the Project CRD roles:

```yaml
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: alpha
spec:
  roles:
    - name: developer
      groups: ["alpha-developers", "platform-engineers"]
      policies:
        - "instances/*, *, allow"
        - "secrets/*, *, allow"
        - "rgds/*, get, allow"
    - name: viewer
      groups: ["alpha-viewers"]
      policies:
        - "instances/*, get, allow"
        - "rgds/*, get, allow"
```

## Changing Roles

To change a user's role:
1. Remove the existing role binding
2. Add a new binding with the desired role

Role changes take effect on the user's next API request (Casbin policies are reloaded when project roles change).

## Removing Members

### Removing a User

```
DELETE /api/v1/projects/{name}/roles/{role}/users/{user}
```

### Removing a Group

```
DELETE /api/v1/projects/{name}/roles/{role}/groups/{group}
```

Removing a group binding revokes access for all users in that OIDC group (unless they have individual user bindings or belong to another bound group).

## Listing Role Bindings

```
GET /api/v1/projects/{name}/role-bindings
```

Returns all user and group bindings for the project:

```json
{
  "bindings": [
    {
      "role": "developer",
      "type": "group",
      "subject": "alpha-developers"
    },
    {
      "role": "platform-admin",
      "type": "user",
      "subject": "admin@example.com"
    }
  ]
}
```

## Permission Requirements

| Operation | Required Role |
|-----------|--------------|
| View role bindings | Any project role (viewer+) |
| Add/remove user bindings | Server Admin or Platform Admin |
| Add/remove group bindings | Server Admin or Platform Admin |
| Modify project roles (policies) | Server Admin only |

:::note[Role Policy Updates]
Only Server Admins can modify the Casbin policies within project roles. Platform Admins can assign users and groups to existing roles but cannot change what those roles are allowed to do.
:::
