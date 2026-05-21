---
title: API Documentation
description: REST API reference with curl examples, Swagger UI, authentication, and WebSocket API
sidebar_position: 4
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# API Documentation

This page covers the Knodex REST API, including authentication, endpoint examples, error handling, and the WebSocket API.

## Prerequisites

- A running Knodex environment (local or cluster)
- `curl` and `jq` installed
- A valid JWT token (see [Obtain a JWT Token](#obtain-a-jwt-token) below)

## Start a Development Environment

```bash
# Create cluster and start Tilt with enterprise features
make cluster-up
tilt up -- --enterprise
```

## Swagger UI

Knodex includes an embedded Swagger UI for interactive API exploration.

- **Disabled by default** in production
- **Auto-enabled** when running with Tilt
- Manually enable with `SWAGGER_UI_ENABLED=true`

Once enabled, access it at:

```
http://localhost:8080/swagger/
```

To authenticate in Swagger UI, click the "Authorize" button and enter your Bearer token.

## Obtain a JWT Token

### Local Admin Login

```bash
# Get the admin password
# Tilt deployment:
ADMIN_PASS=$(kubectl get secret knodex-initial-admin-password -n knodex \
  -o jsonpath='{.data.password}' | base64 -d)

# QA deployment:
ADMIN_PASS=$(kubectl get secret knodex-initial-admin-password -n knodex-qa \
  -o jsonpath='{.data.password}' | base64 -d)

# Login and extract the token
TOKEN=$(curl -s http://localhost:8080/api/v1/auth/local/login \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"admin\", \"password\": \"$ADMIN_PASS\"}" \
  | jq -r '.token')

echo $TOKEN
```

### OIDC Login

For OIDC-based authentication:

1. Open `http://localhost:3000` in a browser
2. Complete the OIDC login flow
3. Extract the token from the browser (DevTools > Application > Cookies or Local Storage)

### Using the Token

Pass the token in the `Authorization` header:

```bash
curl -s http://localhost:8080/api/v1/rgds \
  -H "Authorization: Bearer $TOKEN" | jq
```

:::note[Token Expiry]
JWT tokens expire after 1 hour. Re-authenticate to obtain a fresh token.
:::

## API Examples

### List RGDs (Catalog)

```bash
# List all RGDs
curl -s http://localhost:8080/api/v1/rgds \
  -H "Authorization: Bearer $TOKEN" | jq

# Filter by category
curl -s "http://localhost:8080/api/v1/rgds?category=databases" \
  -H "Authorization: Bearer $TOKEN" | jq

# Search by name
curl -s "http://localhost:8080/api/v1/rgds?search=postgres" \
  -H "Authorization: Bearer $TOKEN" | jq

# Filter by extendsKind
curl -s "http://localhost:8080/api/v1/rgds?extendsKind=Deployment" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Get RGD Details

```bash
curl -s http://localhost:8080/api/v1/rgds/my-rgd \
  -H "Authorization: Bearer $TOKEN" | jq
```

### List Projects

```bash
curl -s http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Create a Project

```bash
curl -s -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-project",
    "description": "A test project",
    "destinations": [
      {"namespace": "my-namespace"}
    ]
  }' | jq
```

### Deploy an Instance

```bash
# Namespaced resource
curl -s -X POST http://localhost:8080/api/v1/namespaces/my-namespace/instances/WebApp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-webapp",
    "project": "my-project",
    "values": {
      "image": "nginx:latest",
      "replicas": 2
    }
  }' | jq

# Cluster-scoped resource
curl -s -X POST http://localhost:8080/api/v1/instances/_/ClusterService \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-cluster-svc",
    "project": "my-project",
    "values": {
      "tier": "production"
    }
  }' | jq
```

### List Instances

```bash
curl -s http://localhost:8080/api/v1/instances \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Check Permissions (can-i)

```bash
curl -s http://localhost:8080/api/v1/account/can-i \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Get Account Info

```bash
curl -s http://localhost:8080/api/v1/account \
  -H "Authorization: Bearer $TOKEN" | jq
```

## Enterprise vs OSS Responses

Enterprise endpoints behave differently depending on the build:

| Endpoint | Enterprise Build | OSS Build |
|----------|-----------------|-----------|
| `/api/v1/compliance/*` | Full compliance data | `402 Payment Required` |
| `/api/v1/audit/*` | Audit trail records | `402 Payment Required` |
| `/api/v1/categories/*` | Category management | `402 Payment Required` |
| `/api/v1/license` | License information | `402 Payment Required` |
| `/api/v1/gatekeeper/*` | Gatekeeper violations | `404 Not Found` |

## Rate Limits

| Endpoint | Limit |
|----------|-------|
| `POST /api/v1/auth/local/login` | 5 requests/min |
| `GET /api/v1/auth/oidc/login` | 20 requests/min |
| `GET /api/v1/auth/oidc/callback` | 5 requests/min |
| `GET /api/v1/settings/sso/providers` | 30 requests/min |
| `GET /api/v1/projects/{name}/namespaces` | 20 requests/min |
| All other authenticated endpoints | 100 requests/min/user |

:::note[Rate Limit Headers]
When rate limited, the API returns `429 Too Many Requests` with a `Retry-After` header indicating how long to wait.
:::

## Error Response Format

All API errors follow a consistent JSON format:

```json
{
  "code": "NOT_FOUND",
  "message": "RGD 'my-rgd' not found",
  "details": {}
}
```

### Error Codes

| Code | HTTP Status | Meaning |
|------|-------------|---------|
| `BAD_REQUEST` | 400 | Invalid input or malformed request |
| `UNAUTHORIZED` | 401 | Missing or invalid authentication |
| `LICENSE_REQUIRED` | 402 | Enterprise feature on OSS build |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource does not exist |
| `CONFLICT` | 409 | Resource already exists |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

## OpenAPI Specification

The API is documented with an OpenAPI 3.0 specification.

| Resource | URL / Path |
|----------|-----------|
| Swagger UI | `http://localhost:8080/swagger/` |
| Raw YAML | `http://localhost:8080/swagger/openapi.yaml` |
| Source file | `server/internal/api/swagger/openapi.yaml` |

```bash
# Validate the OpenAPI spec and sync to server embed
make api-docs
```

:::warning[Manual Maintenance]
The OpenAPI specification is maintained manually. When adding or modifying API endpoints, update `server/internal/api/swagger/openapi.yaml` accordingly and run `make api-docs` to validate.
:::

## WebSocket API

Knodex uses WebSocket connections for real-time updates (instance status changes, RGD updates, etc.).

### Endpoint

```
ws://localhost:8080/ws
```

### Authentication

WebSocket connections use ticket-based authentication. First obtain a ticket, then connect:

```bash
# 1. Get a WebSocket ticket
TICKET=$(curl -s -X POST http://localhost:8080/api/v1/ws/ticket \
  -H "Authorization: Bearer $TOKEN" | jq -r '.ticket')

# 2. Connect with the ticket
wscat -c "ws://localhost:8080/ws?ticket=$TICKET"
```

:::note[Ticket Expiry]
WebSocket tickets are single-use and expire after a short time window. Obtain a new ticket for each connection.
:::

The WebSocket sends JSON messages for events such as:

- Instance status changes
- RGD updates
- Deployment progress
- Compliance violations (enterprise)
