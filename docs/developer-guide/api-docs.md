# API Documentation

Knodex exposes a REST API at `/api/v1/` for managing RGD catalogs, instances,
projects, and RBAC. This guide covers authentication, interactive exploration
with Swagger UI, and working `curl` examples.

## Prerequisites

- Running Knodex environment (Tilt or QA deployment)
- `curl` and `jq` installed

## Start the Dev Environment

```bash
# One-time: create Kind cluster with KRO + CRDs
make cluster-up

# Start Tilt with enterprise features
tilt up -- --enterprise
```

This gives you:
- **Server**: http://localhost:8080
- **Web UI**: http://localhost:3000

## Swagger UI

Swagger UI is **disabled by default** in production but **automatically enabled**
in Tilt dev environments. To enable it manually, set `SWAGGER_UI_ENABLED=true`.

Open http://localhost:8080/swagger/ in your browser to explore the API
interactively.

To authenticate in Swagger UI:
1. Obtain a token (see below)
2. Click the **Authorize** button
3. Enter `Bearer <token>` in the value field
4. Click **Authorize**

All "Try it out" requests will include your token automatically.

## Obtain a JWT Token

### Local Admin Login

```bash
# Login with the local admin account
TOKEN=$(curl -s http://localhost:8080/api/v1/auth/local/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"YOUR_PASSWORD"}' \
  | jq -r '.token')

echo $TOKEN
```

The admin password is auto-generated on first cluster start. Find it with:

```bash
# Tilt environment
kubectl get secret knodex-initial-admin-password -n knodex-tilt -o jsonpath='{.data.password}' | base64 -d

# QA environment
kubectl get secret knodex-initial-admin-password -n knodex -o jsonpath='{.data.password}' | base64 -d
```

### OIDC Login

OIDC authentication uses a browser-based flow:

1. Navigate to `http://localhost:8080/api/v1/auth/oidc/login?provider=default&redirect=http://localhost:3000`
2. Authenticate with your identity provider
3. The callback redirects to the frontend with an auth code
4. Exchange the code for a token:

```bash
TOKEN=$(curl -s http://localhost:8080/api/v1/auth/token-exchange \
  -H "Content-Type: application/json" \
  -d '{"code":"AUTH_CODE_FROM_REDIRECT"}' \
  | jq -r '.token')
```

### Use the Token

Include the token in all API requests:

```bash
curl -s http://localhost:8080/api/v1/rgds \
  -H "Authorization: Bearer $TOKEN" | jq
```

Tokens expire after **1 hour**. Re-authenticate to obtain a new one.

## API Examples

### List RGDs (Catalog)

```bash
# List all catalog RGDs
curl -s http://localhost:8080/api/v1/rgds \
  -H "Authorization: Bearer $TOKEN" | jq

# Filter by category
curl -s "http://localhost:8080/api/v1/rgds?category=database" \
  -H "Authorization: Bearer $TOKEN" | jq

# Search by name
curl -s "http://localhost:8080/api/v1/rgds?search=postgres" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Get RGD Details

```bash
curl -s http://localhost:8080/api/v1/rgds/my-rgd-name \
  -H "Authorization: Bearer $TOKEN" | jq
```

### List Projects

```bash
curl -s http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Create a Project

```bash
curl -s http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-project",
    "description": "My test project",
    "destinations": [{"namespace": "my-namespace"}]
  }' | jq
```

### Deploy an Instance

```bash
# Direct deployment
curl -s http://localhost:8080/api/v1/instances \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-instance",
    "namespace": "my-namespace",
    "rgdName": "my-rgd-name",
    "projectId": "my-project",
    "spec": {},
    "deploymentMode": "direct"
  }' | jq
```

### Check Permissions (can-i)

```bash
# Can I create projects?
curl -s http://localhost:8080/api/v1/account/can-i/projects/create/* \
  -H "Authorization: Bearer $TOKEN" | jq

# Can I deploy to a specific project?
curl -s http://localhost:8080/api/v1/account/can-i/instances/create/my-project \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Get Account Info

```bash
curl -s http://localhost:8080/api/v1/account/info \
  -H "Authorization: Bearer $TOKEN" | jq
```

## Enterprise vs OSS

Enterprise endpoints require the server to be built with `-tags=enterprise`
(Tilt with `--enterprise` flag does this automatically).

| Feature | OSS Response | Enterprise Response |
|---------|-------------|-------------------|
| Compliance (`/api/v1/compliance/*`) | 404 Not Found | Full data |
| Audit trail (`/api/v1/settings/audit/*`) | 404 Not Found | Full data |
| Custom views (`/api/v1/ee/views`) | 404 Not Found | Full data |
| License (`/api/v1/license`) | 404 Not Found | License status |

Toggle enterprise mode in Tilt:

```bash
# Enterprise (default with --enterprise flag)
tilt up -- --enterprise

# OSS only
tilt up
```

## Rate Limits

| Endpoint | Limit | Scope |
|----------|-------|-------|
| `POST /api/v1/auth/local/login` | 5 req/min | Per IP |
| `GET /api/v1/auth/oidc/login` | 20 req/min | Per IP |
| `GET /api/v1/auth/oidc/callback` | 5 req/min | Per IP |
| `GET /api/v1/auth/oidc/providers` | 30 req/min | Per IP |
| `GET /api/v1/projects/{name}/namespaces` | 20 req/min | Per IP |
| All other protected endpoints | 100 req/min | Per user |

Exceeding the limit returns `429 Too Many Requests`.

## Error Response Format

All API errors follow a consistent JSON format:

```json
{
  "code": "NOT_FOUND",
  "message": "RGD not found: my-rgd",
  "details": {
    "resource": "RGD",
    "identifier": "my-rgd"
  }
}
```

Error codes: `BAD_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`,
`VALIDATION_FAILED`, `RATE_LIMIT_EXCEEDED`, `SERVICE_UNAVAILABLE`,
`INTERNAL_ERROR`, `METHOD_NOT_ALLOWED`.

## OpenAPI Specification

The complete OpenAPI 3.0.3 specification is available at:
- **Swagger UI**: http://localhost:8080/swagger/ (requires `SWAGGER_UI_ENABLED=true`)
- **Raw YAML**: http://localhost:8080/swagger/openapi.yaml (requires `SWAGGER_UI_ENABLED=true`)
- **Source file**: `docs/api/openapi.yaml`

### Validate the Spec

```bash
make api-docs
```

### Regenerate

The OpenAPI spec is maintained manually in `docs/api/openapi.yaml`. After
modifying it, validate and sync with:

```bash
make api-docs
```

This validates the spec with Redocly CLI and copies it to the server embed
directory (`server/internal/api/swagger/openapi.yaml`).

## WebSocket API

Real-time updates use WebSocket at `/ws`. Authentication uses a single-use
ticket:

```bash
# 1. Get a ticket
TICKET=$(curl -s http://localhost:8080/api/v1/ws/ticket \
  -H "Authorization: Bearer $TOKEN" \
  -X POST | jq -r '.ticket')

# 2. Connect with the ticket
wscat -c "ws://localhost:8080/ws?ticket=$TICKET"
```

## Next Steps

- [Testing](testing.md)
- [Tilt Development](tilt.md)
