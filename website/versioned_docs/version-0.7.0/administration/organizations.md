---
title: Managing Organizations
description: View and configure organization settings that affect catalog visibility and multi-tenant isolation.
sidebar_position: 1
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Managing Organizations

Organizations control which RGDs are visible to users of a particular Knodex server instance. This page covers how to view and configure organization settings from the platform admin perspective.

## Viewing Organization Info

Navigate to **Settings** in the Knodex UI. The **Server Information** section displays the current organization identity.

If `KNODEX_ORGANIZATION` is not configured, the server operates as the `default` organization.

## Configuring KNODEX_ORGANIZATION

The organization identity is set at the server level, not per-project. It is configured via:

**Environment variable:**

```bash
KNODEX_ORGANIZATION=acme-corp
```

**Helm values:**

```yaml
enterprise:
  organization: "acme-corp"
```

Changing this value requires a server restart (pod rollout).

## How Organization Scoping Affects Catalog

When `KNODEX_ORGANIZATION` is set, the catalog shows:

1. **Shared RGDs** -- RGDs without a `knodex.io/organization` label (visible to all organizations)
2. **Organization RGDs** -- RGDs with a `knodex.io/organization` label matching the server's organization
3. **Hidden** -- RGDs labeled for a different organization

This filtering is transparent to end users. They see a unified catalog without knowing which RGDs are shared vs organization-specific.

## Use Cases

| Scenario | Configuration |
|----------|--------------|
| Single-tenant deployment | Leave `KNODEX_ORGANIZATION` unset (defaults to `default`) |
| Multi-tenant shared cluster | Each Knodex instance sets a different `KNODEX_ORGANIZATION` |
| Development environment | Set to match production org for RGD compatibility testing |

## Detailed Configuration

For comprehensive organization scoping documentation including visibility rules, filter chains, and examples, see the [Enterprise Organizations](../enterprise/organizations) page.
