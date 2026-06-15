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
| `DATABASE_URL` | `""` | PostgreSQL connection string (required for all editions — the server fails fast at startup if unset or unreachable). Format: `postgres://user:pass@host:5432/db?sslmode=disable` |
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
| `KAGENT_CONTROLLER_BASE_URL` | `http://kagent-controller.kagent.svc.cluster.local:8083` | kagent controller REST base URL. Set when kagent is installed in a non-default namespace. See [Agents](agents) |

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

### PostgreSQL Configuration

PostgreSQL is required for **all editions** (OSS, Enterprise, and Cloud). Every edition persists a canonical user roster, and the server fails fast at startup if it cannot reach a database. The chart **bundles an embedded PostgreSQL subchart by default** (`postgresql.enabled: true`) so a fresh install of any edition is healthy out of the box, with a top-level `externalPostgresql` block as the opt-out for a managed database.

:::warning[Bundled PostgreSQL is for demos, dev, and CI only]
The embedded subchart uses **ephemeral storage by default** (data is lost on pod restart) and the chart configures **no backups** (no WAL archiving, snapshots, or PVC backups). **Backup and restore are entirely the operator's responsibility.** Production deployments should set `postgresql.enabled: false` and point at a managed database via `externalPostgresql`.
:::

```yaml
# Option A: Embedded Bitnami PostgreSQL subchart (default — demos / dev / CI)
# Enabled by default; shown here for completeness. Add persistence for a stable
# dev environment (still no backups).
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
# Option B: External managed PostgreSQL via an existing Secret (recommended for production)
# Setting externalPostgresql.existingSecret (or .host) trips the auto-detect guard
# so the bundled subchart's resources are not rendered. Also disable the subchart
# to skip the unused embedded StatefulSet.
postgresql:
  enabled: false

externalPostgresql:
  existingSecret: my-postgres-secret   # Secret holding a full DATABASE_URL DSN
  existingSecretKey: DATABASE_URL      # key within the Secret (default: DATABASE_URL)
```

```yaml
# Option C: External managed PostgreSQL via inline host/credentials (convenience / dev)
# The chart builds the DSN from these fields. Prefer existingSecret for production.
postgresql:
  enabled: false

externalPostgresql:
  host: my-postgres.example.com
  port: 5432
  database: knodex
  username: knodex
  password: "change-me"
  sslMode: require                     # disable | require | verify-ca | verify-full
```

#### Auto-detect guard (upgrade safety)

The bundled subchart is default-on, but when **any external supply is present** (`externalPostgresql.existingSecret`, `externalPostgresql.host`, or the deprecated `enterprise.postgres.connectionString*`), the chart automatically does **not** render the embedded subchart's parent resources (DSN Secret, `DATABASE_URL` source, `wait-for-postgres` credential) — so a `helm upgrade` won't spin up an unwanted bundled PostgreSQL alongside your external database. Set `postgresql.forceEnable: true` to override the guard in the rare case you need the embedded resources rendered anyway.

Because Helm cannot evaluate a computed value for the Bitnami subchart's static `condition: postgresql.enabled`, operators pointing at an external database should **also** set `postgresql.enabled: false` to skip the otherwise-unused embedded StatefulSet.

#### Credential Precedence

The chart resolves the database credential in this order (highest first):

1. **Embedded subchart** (`postgresql.enabled: true`, default) — the chart assembles the full `DATABASE_URL` DSN from `postgresql.auth.*` and stores it in a chart-managed Secret
2. **External Secret** (`externalPostgresql.existingSecret`) — the referenced Secret key (`existingSecretKey`, default `DATABASE_URL`) supplies the DSN; recommended for production
3. **External host** (`externalPostgresql.host` + `username`/`password`/`database`/`sslMode`) — the chart builds a managed DSN Secret from these fields
4. **Deprecated alias** (`enterprise.postgres.connectionString*`) — retained for backward compatibility, resolved last

#### Schema Migrations

Knodex automatically applies schema migrations at startup using `golang-migrate`. No manual migration steps are required. Migrations are idempotent and use advisory locks, so running multiple replicas simultaneously is safe.

The migration state is tracked in the `schema_migrations` table. The current version is visible in logs at startup:

```
INFO  migrations applied  version=3 dirty=false
```

#### Row-Level Security

Data is isolated at the database level using PostgreSQL Row-Level Security (RLS). Every write acquires a connection, begins a transaction, and sets `app.org_id` before executing queries — ensuring data from different organizations cannot bleed across requests even when sharing a single database. RLS protects the shared `identity` schema on every edition, and the Enterprise audit/compliance/license schemas on Enterprise builds. The application connects as a non-`BYPASSRLS` role (`knodex_app`), so the policies actually constrain it.

#### User Identity & Seat Counting

Every edition persists a canonical user roster in the shared `identity` schema:

- `identity.users` — one row per user (`id`, `org_id`, `email`, `state` ∈ `{active, removed}`, `last_seen_at`).
- `identity.federated_identities` — the external identities (`issuer`, `sub`, `internal_user_id`) that map an SSO login to a user row.

The roster is materialised lazily on each OIDC login through a small port-and-hooks flow:

```mermaid
flowchart LR
  A[OIDC login] --> B[EvaluateOIDCUser]
  B --> C["IdentityService.ObserveLogin"]
  C --> D[(base store tx<br/>identity.users +<br/>identity.federated_identities)]
  D --> E["post-commit IdentityHooks"]
  E --> F["EE: emit audit events<br/>(best-effort, never fails login)"]
  E --> G["OSS: zero-value hooks (no-op)"]
```

After a successful login, `EvaluateOIDCUser` calls `IdentityService.ObserveLogin`, which upserts the user and federated-identity rows inside a single transaction. Once that transaction commits, `IdentityHooks` run **post-commit**: on Enterprise they emit `identity.user.*` audit events directly (best-effort — a hook failure is logged and metered but never fails the login); on OSS the hooks are zero-value no-ops. There is **no outbox table** and **no enforced foreign key** from `audit.events` to `identity.users` — the EE resolver method `GetByFederation` and a **nullable** `audit.events.resolved_user_id` column ship today, while an enforced FK and backfill are deferred to a future story.

License-seat usage is computed directly from this roster as `COUNT(*) FROM identity.users WHERE state='active'` — the same entitlement-based, per-seat count on every edition. See [License Activation](../enterprise/license-activation#seat-counting) for seat-counting and reclaim details, the [Upgrade Notes](../enterprise/upgrade-notes#license-seat-counter--storage-rebuild) for the storage rebuild, and the architecture decision record `architecture/adr-user-identity-persistence.md` for the full rationale.

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
