---
title: Configuration
description: Complete configuration reference for Knodex, including environment variables, Helm values, Redis settings, and security headers.
sidebar_position: 2
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Configuration

Knodex is configured primarily through environment variables, which can be set directly or via Helm values. This page is the complete reference for all configuration options.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_ADDRESS` | `:8080` | Server bind address (host:port) |
| `REDIS_ADDRESS` | `localhost:6379` | Redis connection address |
| `REDIS_PASSWORD` | `""` | Redis authentication password |
| `DATABASE_URL` | `""` | PostgreSQL connection string (required for Enterprise). Format: `postgres://user:pass@host:5432/db?sslmode=disable` |
| `KUBERNETES_IN_CLUSTER` | `false` | Use in-cluster Kubernetes config. Set to `true` when running inside a pod |
| `OIDC_ISSUER_URL` | | OIDC provider issuer URL (e.g., `https://login.microsoftonline.com/<tenant>/v2.0`) |
| `OIDC_CLIENT_ID` | | OIDC client/application ID |
| `OIDC_CLIENT_SECRET` | | OIDC client secret |
| `KNODEX_ORGANIZATION` | `default` | Organization identity for multi-tenant scoping |
| `SWAGGER_UI_ENABLED` | `false` | Enable Swagger UI at `/swagger/` |
| `COOKIE_SECURE` | `true` | Set `Secure` flag on session cookies. Disable only for local development |
| `COOKIE_DOMAIN` | `""` | Domain for session cookies. Leave empty to use the request host |
| `KNODEX_LICENSE_PATH` | | Path to enterprise license file |
| `KNODEX_LICENSE_TEXT` | | Enterprise license content (alternative to file path) |
| `CATALOG_PACKAGE_FILTER` | `""` | Filter RGDs by `knodex.io/package` label. See [Catalog Filter](catalog-filter) |

## Helm Values Reference

Environment variables are set through the Helm chart's `server.config` and `server.secrets` sections.

### Server Configuration

```yaml
server:
  config:
    # Non-sensitive configuration (stored in ConfigMap)
    SERVER_ADDRESS: ":8080"
    KUBERNETES_IN_CLUSTER: "true"
    OIDC_ISSUER_URL: "https://login.microsoftonline.com/<tenant>/v2.0"
    OIDC_CLIENT_ID: "your-client-id"
    KNODEX_ORGANIZATION: "my-org"
    SWAGGER_UI_ENABLED: "false"
    COOKIE_SECURE: "true"
    COOKIE_DOMAIN: "knodex.example.com"
    CATALOG_PACKAGE_FILTER: "platform-team"

  secrets:
    # Sensitive configuration (stored in Secret)
    OIDC_CLIENT_SECRET: "your-client-secret"
```

### Redis Configuration

```yaml
redis:
  # Use the embedded Bitnami Redis subchart
  enabled: true
  architecture: standalone  # or "replication" for HA
  auth:
    enabled: true
    password: "your-redis-password"
  master:
    persistence:
      enabled: true
      size: 1Gi
  image:
    # Image is pinned by digest in default values
    # Override only if you need a specific version
    digest: ""
    tag: "7.4"
```

To use an external Redis instance instead of the embedded subchart:

```yaml
redis:
  enabled: false

server:
  config:
    REDIS_ADDRESS: "my-redis.example.com:6379"
  secrets:
    REDIS_PASSWORD: "external-redis-password"
```

### PostgreSQL Configuration (Enterprise)

PostgreSQL is required for Enterprise builds. The Helm chart supports three deployment modes controlled by `postgres.deploymentMode`.

```yaml
# Option A: Embedded Bitnami PostgreSQL subchart (development / small deployments)
enterprise:
  enabled: true
  postgres:
    deploymentMode: shared

postgresql:
  enabled: true
  auth:
    username: knodex
    password: "change-me-in-production"
    database: knodex
  primary:
    persistence:
      enabled: true
      size: 10Gi
```

```yaml
# Option B: External managed PostgreSQL (RDS, CloudSQL, Azure Database, etc.)
enterprise:
  enabled: true
  postgres:
    deploymentMode: shared
    connectionStringSecret:
      name: my-postgres-secret   # Secret must have key DATABASE_URL
      key: DATABASE_URL

postgresql:
  enabled: false
```

```yaml
# Option C: CloudNativePG operator (Kubernetes-native HA)
enterprise:
  enabled: true
  postgres:
    deploymentMode: shared
    connectionStringSecret:
      name: knodex-cluster-app   # CNPG-managed secret (type: kubernetes.io/basic-auth)
      key: uri                   # CNPG stores the full DSN under the "uri" key

postgresql:
  enabled: false
```

#### Credential Precedence

The chart resolves the database credential in this order:

1. **Embedded subchart** (`postgresql.enabled: true`) — DSN is assembled from `postgresql.auth.*` values and injected automatically
2. **External secret** (`enterprise.postgres.connectionStringSecret.name`) — the referenced Secret key is mounted as `DATABASE_URL`
3. **Inline DSN** (`enterprise.postgres.connectionString`) — provided directly in Helm values (not recommended for production)

#### Schema Migrations

Knodex automatically applies schema migrations at startup using `golang-migrate`. No manual migration steps are required. Migrations are idempotent and use advisory locks, so running multiple replicas simultaneously is safe.

The migration state is tracked in the `schema_migrations` table. The current version is visible in logs at startup:

```
INFO  migrations applied  version=3 dirty=false
```

#### Row-Level Security

Enterprise data is isolated at the database level using PostgreSQL Row-Level Security (RLS). Every write acquires a connection, begins a transaction, and sets `app.org_id` before executing queries — ensuring data from different organizations cannot bleed across requests even when sharing a single database.

### KRO

```yaml
kro:
  enabled: false  # Set to true to install KRO as a dependency
```

## Architecture Patterns

### Single-Node (Development)

Suitable for development and small environments:

```yaml
server:
  replicaCount: 1

redis:
  architecture: standalone
```

### High Availability with Redis Sentinel

For production environments requiring resilience:

```yaml
server:
  replicaCount: 3

redis:
  architecture: replication
  sentinel:
    enabled: true
    masterSet: knodex
  replica:
    replicaCount: 3
```

When using Redis Sentinel, set the Redis address to the Sentinel endpoint:

```yaml
server:
  config:
    REDIS_ADDRESS: "knodex-redis:26379"
```

## Redis Configuration

### Embedded vs External

| Mode | When to Use | Configuration |
|------|------------|---------------|
| Embedded (default) | Development, single-cluster | `redis.enabled: true` |
| External | Production, shared Redis, managed services | `redis.enabled: false` + `REDIS_ADDRESS` |

### Authentication

When `redis.auth.enabled` is `true` (default), the server deployment includes a `wait-for-redis` init container that uses the `REDIS_PASSWORD` environment variable to verify connectivity before starting the server.

:::note[Redis Image Digest]
The Redis init container image is pinned by digest in `values.yaml` because Bitnami no longer publishes short semver tags. Use `redis.image.digest` (preferred) over `redis.image.tag` when overriding.
:::

## Logging Configuration

Knodex uses structured JSON logging. Log verbosity is controlled by the environment:

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Logging level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` |

In development, `text` format produces human-readable output. In production, use `json` for structured log aggregation.

## Security Headers

Knodex applies security headers to all HTTP responses by default:

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Content-Type-Options` | `nosniff` | Prevent MIME-type sniffing |
| `X-Frame-Options` | `DENY` | Prevent clickjacking |
| `X-XSS-Protection` | `1; mode=block` | Enable XSS filter |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Control referrer information |
| `Content-Security-Policy` | Restrictive default | Prevent XSS and injection |

These headers are applied by the security headers middleware and are not configurable. If you need to adjust CSP for a custom deployment, consider using your ingress controller's header configuration.

## CORS Configuration

CORS is handled by the CORS middleware. In development mode, permissive origins are allowed. In production, CORS is restricted to the configured domain.

| Variable | Default | Description |
|----------|---------|-------------|
| `CORS_ALLOWED_ORIGINS` | `""` | Comma-separated list of allowed origins |

If not set, CORS defaults to same-origin only.
