---
title: Organizations
description: Multi-tenant organization isolation for RGD visibility using the KNODEX_ORGANIZATION environment variable and knodex.io/organization label.
sidebar_position: 2
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["enterprise"]} />

# Organizations

Organizations provide multi-tenant isolation at the RGD catalog level. Each Knodex server instance is configured with an organization identity, and RGDs can be scoped to specific organizations using labels.

## Server Identity

The server's organization identity is set via the `KNODEX_ORGANIZATION` environment variable:

```bash
export KNODEX_ORGANIZATION=acme-corp
```

| Configuration | Behavior |
|---------------|----------|
| `KNODEX_ORGANIZATION` not set | Defaults to `"default"`. All RGDs without an org label are visible. Org-labeled RGDs are filtered. |
| `KNODEX_ORGANIZATION=acme-corp` | Shows RGDs with `knodex.io/organization: acme-corp` and RGDs with no org label (shared). Hides RGDs labeled for other organizations. |
| `KNODEX_ORGANIZATION=default` | Same as not setting the variable. |

In Helm:

```yaml
# values.yaml
enterprise:
  organization: "acme-corp"
```

## RGD Organization Scoping

Scope an RGD to a specific organization using the `knodex.io/organization` **label** (not annotation):

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: acme-internal-service
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "ACME Internal Service"
  labels:
    knodex.io/organization: "acme-corp"
```

:::warning[Label, Not Annotation]
Organization scoping uses a **label** (`labels.knodex.io/organization`), not an annotation. Labels participate in Kubernetes server-side filtering, which is important for performance at scale.
:::

## Visibility Rules

| RGD Org Label | Server Org | Visible? |
|---------------|-----------|----------|
| _(none)_ | Any | Yes (shared/public RGD) |
| `acme-corp` | `acme-corp` | Yes |
| `acme-corp` | `beta-inc` | No |
| `beta-inc` | `acme-corp` | No |
| _(none)_ | _(not set / default)_ | Yes |
| `acme-corp` | _(not set / default)_ | No |

## Filter Chain

Organization filtering is one step in the full visibility chain:

1. **Catalog gate** -- `knodex.io/catalog: "true"` annotation present
2. **Organization filter** -- RGD has no org label (shared) OR org label matches server's `KNODEX_ORGANIZATION`
3. **Project filter** -- RGD has no project label (public) OR user is a member of the labeled project

All three filters must pass for an RGD to be visible.

## OSS Behavior

In OSS builds, the organization label is still processed. Setting `KNODEX_ORGANIZATION` and labeling RGDs works in OSS, but the license management UI and enterprise-specific organization features are not available.

## Examples

### Shared RGD (No Organization Label)

Visible to all Knodex server instances regardless of their `KNODEX_ORGANIZATION` setting:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: nginx-ingress
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Nginx Ingress"
    knodex.io/category: "networking"
  # No knodex.io/organization label = shared
```

### Organization-Specific RGD

Visible only to servers configured with `KNODEX_ORGANIZATION=acme-corp`:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: acme-payment-gateway
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "ACME Payment Gateway"
    knodex.io/category: "applications"
  labels:
    knodex.io/organization: "acme-corp"
```

### Organization + Project Scoped

Visible only to members of the `payments` project on servers configured for `acme-corp`:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: acme-payment-processor
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "ACME Payment Processor"
    knodex.io/category: "applications"
  labels:
    knodex.io/organization: "acme-corp"
    knodex.io/project: "payments"
```

## Visibility Matrix

| RGD Labels | Server: `acme-corp` | Server: `beta-inc` | Server: `default` |
|-----------|--------------------|--------------------|-------------------|
| No labels | Visible | Visible | Visible |
| `org: acme-corp` | Visible | Hidden | Hidden |
| `org: beta-inc` | Hidden | Visible | Hidden |
| `org: acme-corp`, `project: payments` | Visible (if in `payments` project) | Hidden | Hidden |
| `project: payments` (no org) | Visible (if in `payments` project) | Visible (if in `payments` project) | Visible (if in `payments` project) |

## UI Display

When the server has a `KNODEX_ORGANIZATION` configured, the organization name is displayed in the **Settings** page under server information. This helps administrators confirm which organization the server instance is configured for.
