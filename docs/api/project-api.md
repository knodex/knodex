# Project API

REST API for managing Projects in the Knodex RBAC system.

## Overview

The Project API provides CRUD operations for managing Projects, which define deployment boundaries (allowed namespaces). Projects integrate with the Casbin-based RBAC system to enforce fine-grained access control.

## Authentication

All endpoints require a valid JWT token in the `Authorization` header:

```bash
Authorization: Bearer <token>
```

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/projects` | List all accessible projects |
| GET | `/api/v1/projects/{name}` | Get a project by name |
| POST | `/api/v1/projects` | Create a new project |
| PUT | `/api/v1/projects/{name}` | Update an existing project |
| DELETE | `/api/v1/projects/{name}` | Delete a project |

---

## List Projects

Returns all projects the authenticated user has access to view. Global admins see all projects.

```
GET /api/v1/projects
```

### Response

```json
{
  "items": [
    {
      "name": "my-project",
      "description": "Production deployments",
      "destinations": [
        {
          "namespace": "production"
        }
      ],
      "roles": [],
      "resourceVersion": "12345",
      "createdAt": "2026-01-10T10:00:00Z",
      "createdBy": "admin@example.com"
    }
  ],
  "totalCount": 1
}
```

### Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 401 | Authentication required |
| 500 | Internal server error |

---

## Get Project

Returns a single project by name.

```
GET /api/v1/projects/{name}
```

### Parameters

| Name | Location | Required | Description |
|------|----------|----------|-------------|
| name | path | Yes | Project name (DNS-1123 subdomain) |

### Response

```json
{
  "name": "my-project",
  "description": "Production deployments",
  "destinations": [
    {
      "namespace": "production"
    }
  ],
  "roles": [
    {
      "name": "deployer",
      "description": "Can deploy applications",
      "policies": ["p, role:deployer, applications, create, my-project/*"],
      "groups": ["deployers"]
    }
  ],
  "resourceVersion": "12345",
  "createdAt": "2026-01-10T10:00:00Z",
  "createdBy": "admin@example.com",
  "updatedAt": "2026-01-11T15:30:00Z",
  "updatedBy": "admin@example.com"
}
```

### Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 400 | Invalid project name |
| 401 | Authentication required |
| 404 | Project not found (or access denied) |
| 500 | Internal server error |

---

## Create Project

Creates a new project. Requires global admin privileges.

```
POST /api/v1/projects
```

### Request Body

```json
{
  "name": "my-project",
  "description": "Production deployments",
  "destinations": [
    {
      "namespace": "production"
    }
  ]
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Unique project identifier (DNS-1123 subdomain format: lowercase alphanumeric with hyphens, 1-63 chars) |
| description | string | No | Human-readable description |
| destinations | object[] | No | Allowed deployment destinations |
| destinations[].namespace | string | Yes | Target namespace (supports wildcards: "*", "dev-*") |
| destinations[].name | string | No | Friendly name for the destination |

### Response

Returns the created project with `resourceVersion` and `createdAt` populated.

### Status Codes

| Code | Description |
|------|-------------|
| 201 | Project created |
| 400 | Invalid request (validation errors) |
| 401 | Authentication required |
| 403 | Forbidden (requires global admin) |
| 409 | Project already exists |
| 500 | Internal server error |

### Example

```bash
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production",
    "description": "Production environment",
    "destinations": [
      {
        "namespace": "prod-*"
      }
    ]
  }'
```

---

## Update Project

Updates an existing project. Requires global admin privileges or project-level update permission.

```
PUT /api/v1/projects/{name}
```

### Parameters

| Name | Location | Required | Description |
|------|----------|----------|-------------|
| name | path | Yes | Project name |

### Request Body

```json
{
  "description": "Updated description",
  "destinations": [
    {
      "namespace": "production"
    },
    {
      "namespace": "staging"
    }
  ],
  "resourceVersion": "12345"
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| resourceVersion | string | Yes | Current resource version for optimistic locking |
| description | string | No | Human-readable description |
| destinations | object[] | No | Allowed deployment destinations |

### Optimistic Locking

The `resourceVersion` field is required and must match the current version. If the project was modified since you retrieved it, the update fails with 409 Conflict.

### Response

Returns the updated project with new `resourceVersion`.

### Status Codes

| Code | Description |
|------|-------------|
| 200 | Project updated |
| 400 | Invalid request (validation errors or missing resourceVersion) |
| 401 | Authentication required |
| 403 | Forbidden (insufficient permissions) |
| 404 | Project not found |
| 409 | Conflict (stale resourceVersion) |
| 500 | Internal server error |

### Example

```bash
curl -X PUT http://localhost:8080/api/v1/projects/production \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Production environment - updated",
    "destinations": [
      {
        "namespace": "prod-*"
      }
    ],
    "resourceVersion": "12345"
  }'
```

---

## Delete Project

Deletes a project. Requires global admin privileges.

```
DELETE /api/v1/projects/{name}
```

### Parameters

| Name | Location | Required | Description |
|------|----------|----------|-------------|
| name | path | Yes | Project name |

### Status Codes

| Code | Description |
|------|-------------|
| 204 | Project deleted (no content) |
| 400 | Invalid project name |
| 401 | Authentication required |
| 403 | Forbidden (requires global admin) |
| 404 | Project not found |
| 500 | Internal server error |

### Example

```bash
curl -X DELETE http://localhost:8080/api/v1/projects/my-project \
  -H "Authorization: Bearer $TOKEN"
```

---

## Error Responses

All error responses follow a consistent format:

```json
{
  "code": "BAD_REQUEST",
  "message": "Validation failed",
  "details": {
    "name": "name must be a valid DNS-1123 subdomain"
  }
}
```

### Common Errors

| Status | Error | Description |
|--------|-------|-------------|
| 400 | Bad Request | Invalid input or validation failure |
| 401 | Unauthorized | Missing or invalid authentication |
| 403 | Forbidden | Insufficient permissions |
| 404 | Not Found | Resource not found (or hidden due to access control) |
| 409 | Conflict | Resource already exists or version conflict |
| 500 | Internal Server Error | Unexpected server error |

---

## Authorization Model

### Global Admins

Users assigned to `role:serveradmin` via Casbin policy can:
- List all projects
- Create new projects
- Update any project
- Delete any project

### Project-Level Access

Non-admin users can:
- List projects they have explicit access to
- View projects where they have `get` permission
- Update projects where they have `update` permission

Access is determined by the Casbin PolicyEnforcer using policies defined in each project's roles.

---

## Data Models

### Project

```typescript
interface Project {
  name: string;              // DNS-1123 subdomain (1-63 chars)
  description?: string;      // Human-readable description
  destinations?: Destination[];
  roles?: Role[];
  resourceVersion: string;   // For optimistic locking
  createdAt: string;         // ISO 8601 timestamp
  createdBy?: string;        // User who created the project
  updatedAt?: string;        // ISO 8601 timestamp
  updatedBy?: string;        // User who last updated
}

interface Destination {
  namespace?: string;        // Target namespace (supports wildcards: "*", "dev-*")
  name?: string;             // Optional friendly name
}

interface Role {
  name: string;              // Role name within project
  description?: string;      // Human-readable description
  policies?: string[];       // Casbin policy strings
  groups?: string[];         // OIDC groups assigned to role
}
```

---

## Related Documentation

- [RBAC Overview](../rbac/overview.md)
- [Policy Enforcement](../rbac/policy-enforcement.md)
- [Role Binding API](./role-binding-api.md)
