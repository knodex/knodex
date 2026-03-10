---
title: "Organizations"
linkTitle: "Organizations"
description: "Multi-tenant organization isolation for RGD catalog and instance visibility"
weight: 2
product_tags:
  - enterprise
---

{{< product-tag enterprise >}}

# Organizations

Organizations provide multi-tenant isolation in Knodex Enterprise. Each Knodex deployment belongs to one organization, and RGDs can be scoped so that only the matching organization sees them.

## Overview

The organization model has three layers:

| Layer | Mechanism | Scope |
|-------|-----------|-------|
| **Organization** | `knodex.io/organization` label | Isolates RGDs between tenants |
| **Project** | `knodex.io/project` label | Restricts visibility within an org |
| **Shared catalog** | No scoping labels | Visible to all organizations |

Most RGDs are shared across organizations (common infrastructure templates). Organization scoping is for tenant-specific templates that should not be visible to other tenants.

## Configuration

### Server Identity

Set the organization identity via the `KNODEX_ORGANIZATION` environment variable:

```yaml
# Helm values.yaml
env:
  KNODEX_ORGANIZATION: "orgA"
```

| Configuration | Behavior |
|---------------|----------|
| `KNODEX_ORGANIZATION=orgA` | Server identifies as "orgA" |
| `KNODEX_ORGANIZATION` not set | Defaults to `"default"` |
| `KNODEX_ORGANIZATION=""` | Defaults to `"default"` |

The server logs the configured organization at startup:

```
level=INFO msg="server configuration" organization=orgA
```

## RGD Organization Scoping

### Labeling RGDs

Add the `knodex.io/organization` label to restrict an RGD to a specific organization:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: org-specific-template
  labels:
    knodex.io/organization: orgA
  annotations:
    knodex.io/catalog: "true"
spec:
  # ... RGD spec
```

{{< alert title="Important" >}}
`knodex.io/organization` must be a **label**, not an annotation. The server reads it from `metadata.labels` only.
{{< /alert >}}

### Visibility Rules

| RGD Configuration | Visible To |
|-------------------|------------|
| No `knodex.io/organization` label | All organizations (shared catalog) |
| `knodex.io/organization: "orgA"` | Only organization "orgA" |
| `knodex.io/organization: "orgB"` | Only organization "orgB" |

### Filter Chain

Organization filtering applies as part of the catalog filter chain:

```
1. knodex.io/catalog: "true"    →  Must be set (gateway)
2. knodex.io/organization       →  Must match server org (Enterprise)
3. knodex.io/project            →  Must match user's project membership
```

An RGD must pass all applicable filters to appear in the catalog.

### OSS Behavior

In OSS builds (no enterprise tag), organization filtering is not applied. All RGDs are visible regardless of the `knodex.io/organization` label. This ensures backward compatibility.

## Examples

### Shared RGD (All Organizations)

```yaml
metadata:
  name: postgres-standard
  annotations:
    knodex.io/catalog: "true"
    knodex.io/description: "Standard PostgreSQL database"
    knodex.io/category: "database"
    # No organization label = visible to ALL orgs
```

### Organization-Specific RGD

```yaml
metadata:
  name: acme-payment-service
  labels:
    knodex.io/organization: acme-corp
  annotations:
    knodex.io/catalog: "true"
    knodex.io/description: "ACME payment processing service"
    knodex.io/category: "application"
```

### Organization + Project Scoping

Combine organization and project scoping for fine-grained visibility:

```yaml
metadata:
  name: acme-payments-internal
  labels:
    knodex.io/organization: acme-corp
    knodex.io/project: proj-payments-team
  annotations:
    knodex.io/catalog: "true"
    knodex.io/deployment-modes: "gitops"
    knodex.io/description: "Internal payment service for ACME payments team"
```

This RGD is visible only to members of `proj-payments-team` within the `acme-corp` organization.

## Visibility Matrix

| `knodex.io/catalog` | `knodex.io/organization` | `knodex.io/project` | Visible To |
|---|---|---|---|
| `"true"` | *(not set)* | *(not set)* | All authenticated users in all orgs |
| `"true"` | `"orgA"` | *(not set)* | All authenticated users in orgA |
| `"true"` | *(not set)* | `proj-team` | Members of proj-team in all orgs |
| `"true"` | `"orgA"` | `proj-team` | Members of proj-team in orgA only |
| *(not set)* | any | any | No one (not in catalog) |

## UI Display

The organization name appears in the Knodex header when configured:

- **Named organization** (`KNODEX_ORGANIZATION=acme-corp`): "acme-corp" displays in the header
- **Default** (`KNODEX_ORGANIZATION` not set): Organization name is hidden

Long organization names are truncated with a tooltip showing the full name.

---

**Back to:** [Enterprise Features](..) | **See also:** [Annotations & Labels](../../catalog/annotations-and-labels/)
