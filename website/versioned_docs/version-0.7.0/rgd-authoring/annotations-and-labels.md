---
title: Annotations and Labels Reference
description: Complete reference for all Knodex catalog annotations and labels that control RGD discovery, display, deployment behavior, and visibility.
sidebar_position: 1
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Annotations and Labels Reference

This page is the comprehensive reference for all annotations and labels that Knodex recognizes on ResourceGraphDefinitions.

## Required Annotation

Every RGD that should appear in the Knodex catalog must have this annotation:

```yaml
metadata:
  annotations:
    knodex.io/catalog: "true"
```

Without this annotation, the RGD is ignored by the catalog watcher. The value must be the string `"true"` (case-sensitive).

## Metadata Annotations

These annotations control how the RGD appears in the catalog UI.

| Annotation | Type | Default | Description |
|-----------|------|---------|-------------|
| `knodex.io/title` | string | RGD `.metadata.name` | Human-readable display name shown in the catalog card and detail page |
| `knodex.io/description` | string | _(empty)_ | Short description shown below the title in catalog listings |
| `knodex.io/tags` | string | _(empty)_ | Comma-separated tags for filtering (e.g., `"networking,production,tier-1"`) |
| `knodex.io/category` | string | _(uncategorized)_ | Category slug for sidebar grouping (e.g., `"databases"`, `"networking"`) |
| `knodex.io/icon` | string | Category default or `"layout-grid"` | Lucide icon name for the catalog card (e.g., `"database"`, `"globe"`, `"box"`) |
| `knodex.io/docs-url` | string | _(empty)_ | External documentation URL. Displayed as a link on the RGD detail page |
| `knodex.io/catalog-tier` | string | `"both"` | Controls visibility by project type. Values: `"app"`, `"platform"`, `"both"` |

## Supported Categories

Categories group RGDs in the sidebar. Any string is valid as a category slug, but the following are commonly used and have default icon mappings:

| Category | Default Icon | Typical Use |
|----------|-------------|-------------|
| `applications` | `box` | Application workloads, microservices |
| `databases` | `database` | Database clusters, caches, message queues |
| `networking` | `globe` | Ingress, load balancers, DNS, service mesh |
| `storage` | `hard-drive` | PVCs, backup policies, object storage |
| `security` | `shield` | Certificates, network policies, secrets |
| `monitoring` | `activity` | Prometheus, Grafana, alerting rules |
| `ci-cd` | `git-branch` | Pipelines, build configs, artifacts |
| `web` | `globe` | Static sites, CDN configs, web apps |
| `compute` | `cpu` | VMs, GPU workloads, batch jobs |
| `messaging` | `mail` | Kafka, RabbitMQ, event buses |
| `identity` | `users` | Service accounts, IAM roles |
| `infrastructure` | `server` | Cluster addons, platform services |

### Category Sidebar Visibility

A category appears in the sidebar only if:

1. At least one active RGD has that category annotation
2. The category is listed in the `knodex-category-config` ConfigMap (see [Category Ordering](category-ordering))
3. The user has Casbin `rgds/{category}/*` permission

Categories not in the ConfigMap are hidden from the sidebar even if RGDs exist with that category.

## Extends-Kind Annotation

The `knodex.io/extends-kind` annotation declares that this RGD is an add-on to another RGD. The UI uses this to show related RGDs on the parent's detail page.

```yaml
annotations:
  knodex.io/extends-kind: "PostgresCluster"
```

**Multiple parents** are supported with comma separation:

```yaml
annotations:
  knodex.io/extends-kind: "PostgresCluster,MySQLCluster"
```

**Parsing rules:**
- Values are trimmed of whitespace
- Empty values after trimming are ignored
- Kind matching is exact (case-sensitive)
- The referenced Kind must match the `spec.schema.kind` of another RGD in the catalog

## Deployment Mode Annotations

The `knodex.io/deployment-modes` annotation restricts which deployment modes are available when deploying instances of this RGD.

```yaml
annotations:
  knodex.io/deployment-modes: "direct,gitops"
```

### Valid Modes

| Mode | Description |
|------|-------------|
| `direct` | Apply manifest directly to the Kubernetes API |
| `gitops` | Commit manifest to a Git repository for external reconciliation |
| `hybrid` | Apply directly and commit to Git simultaneously |

### Default Behavior

When the annotation is absent or empty, all three modes are available. This ensures backward compatibility with existing RGDs.

### Parsing Rules

- Values are comma-separated and case-insensitive
- Whitespace around values is trimmed
- Duplicate values are deduplicated
- Unrecognized values are logged as warnings and ignored
- If all values are invalid, the result is the same as no annotation (all modes allowed)

## Property Order Annotation

The `knodex.io/property-order` annotation controls the display order of fields in the deployment form.

```yaml
annotations:
  knodex.io/property-order: '{"": ["name","image","replicas","dbRef.name","dbRef.namespace"]}'
```

The value is a JSON object where keys are spec-relative dot-paths (empty string `""` for top-level spec) and values are ordered arrays of field names.

### Ordering Rules

| Rule | Behavior |
|------|----------|
| Listed fields | Displayed first, in the order specified |
| Unlisted fields | Displayed after all listed fields, in schema order |
| Invalid paths | Silently ignored (no error, no effect) |
| Duplicate paths | Only the first occurrence is used |
| Nested paths | Use dot notation: `"dbRef.name"` refers to `spec.dbRef.name` |

### Where Ordering Applies

Property ordering affects the main deploy form and the advanced section independently. Fields under `spec.advanced` are ordered within the advanced collapsible section, not interleaved with top-level fields.

### Full Example

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: web-app
  annotations:
    knodex.io/catalog: "true"
    knodex.io/property-order: '{"": ["appName","image","replicas","config.logLevel"]}'
spec:
  schema:
    apiVersion: web.knodex.io/v1alpha1
    kind: WebApp
    spec:
      appName: string
      image: string
      replicas: integer | default=3
      config:
        logLevel: string | default="info"
        metricsPort: integer | default=9090
```

The deploy form renders fields in this order: `appName`, `image`, `replicas`, `config.logLevel`, then `config.metricsPort` (unlisted, appears last).

## Labels

Labels control visibility and scoping. Unlike annotations, labels participate in Kubernetes label selectors and server-side filtering.

| Label | Required | Scope | Description |
|-------|----------|-------|-------------|
| `knodex.io/project` | No | RGD | Scopes the RGD to a specific project. Only users in that project see it. |
| `knodex.io/organization` | No | RGD | Scopes the RGD to an organization (Enterprise only). Users outside the org cannot see it. |
| `knodex.io/package` | No | RGD | Package identifier for server-side filtering via `CATALOG_PACKAGE_FILTER` config. |

### Organization Label (Enterprise Only)

The `knodex.io/organization` label restricts RGD visibility to users whose server is configured with a matching `KNODEX_ORGANIZATION` value. RGDs without this label are visible to all organizations (shared/public).

### Project Label

The `knodex.io/project` label restricts RGD visibility to users who are members of the specified project. RGDs without this label are public (visible to all projects). See [Project Scoping](project-scoping) for details.

### Package Label

The `knodex.io/package` label works with the server-side `CATALOG_PACKAGE_FILTER` environment variable. When `CATALOG_PACKAGE_FILTER` is set, only RGDs with a matching `knodex.io/package` label are ingested by the catalog watcher. RGDs without the label or with a non-matching value are excluded entirely.

## Complete Examples

### Standard RGD

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: redis-cluster
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Redis Cluster"
    knodex.io/description: "High-availability Redis cluster with Sentinel"
    knodex.io/tags: "cache,database,ha"
    knodex.io/category: "databases"
    knodex.io/icon: "database"
    knodex.io/deployment-modes: "direct,gitops,hybrid"
    knodex.io/docs-url: "https://redis.io/docs"
    knodex.io/property-order: '{"": ["name","replicas","storage"]}'
spec:
  schema:
    apiVersion: cache.knodex.io/v1alpha1
    kind: RedisCluster
    spec:
      name: string
      replicas: integer | default=3
      storage: string | default="10Gi"
```

### Add-On RGD

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: redis-monitoring
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Redis Monitoring"
    knodex.io/description: "Prometheus ServiceMonitor and Grafana dashboard for Redis"
    knodex.io/category: "monitoring"
    knodex.io/icon: "activity"
    knodex.io/extends-kind: "RedisCluster"
  labels:
    knodex.io/project: "platform"
spec:
  schema:
    apiVersion: monitoring.knodex.io/v1alpha1
    kind: RedisMonitoring
    spec:
      clusterRef:
        name: string
        namespace: string
      alerting: boolean | default=true
  resources:
    - id: redis
      externalRef:
        apiVersion: cache.knodex.io/v1alpha1
        kind: RedisCluster
```

## Annotation Summary

| Annotation | Purpose | Required |
|-----------|---------|----------|
| `knodex.io/catalog` | Enable catalog discovery | Yes |
| `knodex.io/title` | Display name | No |
| `knodex.io/description` | Short description | No |
| `knodex.io/tags` | Searchable tags | No |
| `knodex.io/category` | Sidebar grouping | No |
| `knodex.io/icon` | Card icon (Lucide name) | No |
| `knodex.io/docs-url` | External documentation link | No |
| `knodex.io/catalog-tier` | Project type visibility filter | No |
| `knodex.io/deployment-modes` | Restrict deployment modes | No |
| `knodex.io/extends-kind` | Declare parent RGD dependency | No |
| `knodex.io/property-order` | Form field ordering | No |

## Label Summary

| Label | Purpose | Required |
|-------|---------|----------|
| `knodex.io/project` | Project-scope visibility | No |
| `knodex.io/organization` | Organization-scope visibility (Enterprise) | No |
| `knodex.io/package` | Server-side package filtering | No |

## Visibility Filter Chain

An RGD is visible to a user only if ALL of the following pass:

1. **Catalog gate** -- `knodex.io/catalog: "true"` annotation is present
2. **Package filter** -- If `CATALOG_PACKAGE_FILTER` is set, the RGD's `knodex.io/package` label must match
3. **Organization filter** -- If the server has `KNODEX_ORGANIZATION` set, the RGD must either have no org label or a matching org label
4. **Project filter** -- If the RGD has a `knodex.io/project` label, the user must be a member of that project. Public RGDs (no project label) are visible to all users.
5. **Category Casbin filter** -- The user must have `rgds/{category}/*` permission for the RGD's category
