---
title: "Custom Views"
linkTitle: "Custom Views"
description: "Organize the RGD catalog into filtered sidebar sections using category annotations"
weight: 3
product_tags:
  - enterprise
---

{{< product-tag enterprise >}}

# Custom Views

Custom Views let administrators organize the RGD catalog into curated sidebar sections. Each view filters RGDs by the `knodex.io/category` annotation and appears in the sidebar with a badge count, icon, and dedicated page.

## Overview

| Concept | Description |
|---------|-------------|
| **View** | A named sidebar entry that shows RGDs matching a specific category |
| **Category** | The value of the `knodex.io/category` annotation on an RGD |
| **Slug** | URL-safe identifier for the view (used in `/views/{slug}`) |

Views appear between the core navigation items (Catalog, Instances) and enterprise items (Audit, Compliance) in a collapsible "Views" section.

## Prerequisites

- Knodex Enterprise license with the `views` feature
- At least one RGD annotated with `knodex.io/category`

## Quick Start

### 1. Annotate RGDs

Add a `knodex.io/category` annotation to your ResourceGraphDefinitions:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: postgres-cluster
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: "database"
```

### 2. Enable Views in Helm

```yaml
# values.yaml
enterprise:
  enabled: true
  views:
    enabled: true
    items:
      - name: Database Templates
        slug: databases
        icon: database
        category: database
        order: 1
        description: Database provisioning templates
```

### 3. Deploy

```bash
helm upgrade --install knodex deploy/charts/knodex -f values.yaml
```

The "Database Templates" view now appears in the sidebar showing all RGDs with `knodex.io/category: database`.

## Configuration

### Helm Values

```yaml
enterprise:
  views:
    enabled: false    # Set to true to enable
    items: []         # List of view definitions
```

### View Definition

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | — | Display name in sidebar and page header |
| `slug` | string | Yes | — | URL-safe identifier (`/views/{slug}`) |
| `icon` | string | No | `layout-grid` | [Lucide](https://lucide.dev/icons) icon name in kebab-case |
| `category` | string | Yes | — | Exact match for `knodex.io/category` annotation |
| `order` | int | No | Array index | Sort order in sidebar (ascending) |
| `description` | string | No | — | Subtitle shown on the view page |

### Slug Rules

Slugs must be:

- Lowercase alphanumeric characters and hyphens only
- Start and end with an alphanumeric character
- No consecutive hyphens (`--`)
- 1–63 characters

Valid: `testing`, `dev-tools`, `network-v2`
Invalid: `Testing`, `test_ing`, `test--ing`, `-testing`

If two views share the same slug, the last definition wins.

### Full Example

```yaml
enterprise:
  enabled: true
  views:
    enabled: true
    items:
      - name: Testing Resources
        slug: testing
        icon: flask
        category: testing
        order: 1
        description: RGDs for testing and QA environments

      - name: Database Templates
        slug: databases
        icon: database
        category: database
        order: 2
        description: Database provisioning templates

      - name: Network Infrastructure
        slug: networking
        icon: network
        category: networking
        order: 3
        description: Network and connectivity resources
```

## How It Works

### Category Matching

Views use **exact, case-sensitive** string matching against the `knodex.io/category` annotation. A view with `category: testing` matches only RGDs with:

```yaml
annotations:
  knodex.io/category: "testing"
```

There is no wildcard, prefix, or case-insensitive matching.

### Badge Counts

Each view displays a live count of matching RGDs. Counts are computed at request time from the in-memory RGD cache and respect RBAC visibility filters — users only see counts for RGDs they have access to.

### Icons

Views use [Lucide](https://lucide.dev/icons) icons. Specify the icon name in kebab-case (e.g., `database`, `flask`, `network`, `layout-grid`). If the icon name is unrecognized, it falls back to `layout-grid`.

Browse available icons at [lucide.dev/icons](https://lucide.dev/icons).

### Sidebar Behavior

- Views section is collapsible; collapsed state persists in the browser
- Navigating to a view auto-expands the section
- In icon-only (collapsed sidebar) mode, hovering shows a flyout menu with all views

## API Reference

Both endpoints require authentication and a valid enterprise license with the `views` feature.

### List Views

```
GET /api/v1/ee/views
```

```json
{
  "views": [
    {
      "name": "Database Templates",
      "slug": "databases",
      "icon": "database",
      "category": "database",
      "order": 1,
      "description": "Database provisioning templates",
      "count": 5
    }
  ]
}
```

### Get View

```
GET /api/v1/ee/views/{slug}
```

Returns a single view object with its current count.

### Error Responses

| Status | Code | Condition |
|--------|------|-----------|
| 402 | `LICENSE_REQUIRED` | No valid license or `views` feature not licensed |
| 404 | `NOT_FOUND` | Unknown slug |
| 503 | `SERVICE_UNAVAILABLE` | Views not configured (no config file or OSS build) |

## Troubleshooting

**Views section not showing in sidebar**

- Verify `enterprise.enabled: true` and `enterprise.views.enabled: true` in Helm values
- Confirm the license includes the `views` feature
- Check that at least one view is defined in `items`

**View shows count of 0**

- Verify RGDs have the `knodex.io/category` annotation with the exact value (case-sensitive)
- Check that `knodex.io/catalog: "true"` is also set on the RGD
- Confirm the user has RBAC access to the matching RGDs

**Configuration changes not taking effect**

A server restart is required to pick up views configuration changes. Restart the pod after updating the ConfigMap:

```bash
kubectl rollout restart deployment knodex-server -n knodex
```
