---
title: Category Ordering
description: Configure sidebar category ordering, icons, and visibility using the knodex-category-config ConfigMap.
sidebar_position: 4
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Category Ordering

The Knodex sidebar displays categories from a ConfigMap that controls ordering, icon overrides, and visibility. Categories not listed in the ConfigMap are hidden from the sidebar, even if RGDs with that category exist in the cluster.

![Catalog sidebar showing category navigation with counts](/img/docs/catalog-categories.png)

## How It Works

1. The Knodex server reads the `knodex-category-config` ConfigMap at startup
2. Categories discovered from RGD annotations are matched against ConfigMap entries (case-insensitive)
3. Matched categories are sorted by `weight` (ascending), then alphabetically for equal weights
4. Each visible category is then filtered per-user through Casbin policies

:::warning[Restart Required]
The ConfigMap is read at server startup. Changes to the ConfigMap require a server restart (pod rollout) to take effect.
:::

## ConfigMap Schema

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: knodex-category-config
  namespace: knodex
data:
  categories: |
    - name: "Applications"
      weight: 10
    - name: "Databases"
      weight: 20
      icon: "database"
    - name: "Networking"
      weight: 30
      icon: "globe"
    - name: "Storage"
      weight: 40
    - name: "Security"
      weight: 50
      icon: "shield"
    - name: "Monitoring"
      weight: 60
      icon: "activity"
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Category name. Matched case-insensitively against `knodex.io/category` annotations on RGDs. |
| `weight` | integer | Yes | Sort order. Lower weights appear first in the sidebar. |
| `icon` | string | No | Lucide icon name override. Must match the pattern `[a-z0-9-]+`. |

## Ordering by Weight

Categories are sorted by weight ascending. Equal weights are sorted alphabetically by name.

```yaml
# Sidebar order: Applications (10), Databases (20), Networking (30)
- name: "Networking"
  weight: 30
- name: "Applications"
  weight: 10
- name: "Databases"
  weight: 20
```

Use increments of 10 to leave room for inserting new categories without renumbering.

## Icon Overrides

Icons are resolved in this order:

1. **ConfigMap `icon` field** -- If the ConfigMap entry specifies an icon, it is used (must be a valid Lucide icon name)
2. **Custom icon registry** -- If a `knodex-icon-config` ConfigMap provides SVG icons for the category slug, the SVG is used
3. **RGD annotation** -- The `knodex.io/icon` annotation on individual RGDs (applies to the RGD card, not the category)
4. **Default** -- Falls back to `layout-grid`

The ConfigMap icon only validates against the pattern `[a-z0-9-]+`. Invalid icon names are logged as warnings and the fallback chain continues.

## Casbin Filtering

After ConfigMap filtering, each category is individually checked against the user's Casbin policies. There are two gates:

1. **ConfigMap gate** -- The category must appear in the ConfigMap to be visible in the sidebar at all
2. **Casbin gate** -- The user must have `rgds/{category-slug}/*` permission with `get` action

A user with `rgds/databases/*` permission but no `rgds/networking/*` permission sees only the Databases category, even if both are in the ConfigMap.

The Casbin object path uses the category slug (lowercase, hyphenated form of the name).

## Hidden Categories

To hide a category from the sidebar, simply omit it from the ConfigMap. RGDs with that category still exist and can be accessed via direct URL or API, but they do not appear in the sidebar navigation.

This is useful for:
- Categories under development
- Categories that should only be accessible programmatically
- Gradually rolling out new categories to users

## Deployment Namespace

The ConfigMap must be deployed to the same namespace as the Knodex server. If using the Helm chart, this is the release namespace (typically `knodex`).

```bash
kubectl apply -f category-config.yaml -n knodex
kubectl rollout restart deployment knodex -n knodex
```

## ServiceAccount Permissions

The Knodex server's ServiceAccount needs `get` permission on ConfigMaps in its namespace. The Helm chart's default ClusterRole includes this permission. If you manage RBAC manually, ensure the ServiceAccount can read the ConfigMap.

## Full Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: knodex-category-config
  namespace: knodex
data:
  categories: |
    - name: "Applications"
      weight: 10
      icon: "box"
    - name: "Web"
      weight: 15
      icon: "globe"
    - name: "Databases"
      weight: 20
      icon: "database"
    - name: "Networking"
      weight: 30
      icon: "globe"
    - name: "Storage"
      weight: 40
      icon: "hard-drive"
    - name: "Security"
      weight: 50
      icon: "shield"
    - name: "Monitoring"
      weight: 60
      icon: "activity"
    - name: "CI-CD"
      weight: 70
      icon: "git-branch"
    - name: "Messaging"
      weight: 80
      icon: "mail"
    - name: "Infrastructure"
      weight: 90
      icon: "server"
    - name: "Identity"
      weight: 95
      icon: "users"
    - name: "Compute"
      weight: 100
      icon: "cpu"
```

## Behavior Reference

| Scenario | Result |
|----------|--------|
| Category in ConfigMap, RGDs exist, user has Casbin access | Visible in sidebar |
| Category in ConfigMap, no RGDs exist with that category | Not shown (no discovered categories to match) |
| Category NOT in ConfigMap, RGDs exist | Hidden from sidebar |
| Category in ConfigMap, user lacks Casbin access | Hidden for that user |
| ConfigMap not deployed | Empty sidebar (no categories shown) |
| ConfigMap deployed but `categories` key missing | Empty sidebar |
| Category name case mismatch (e.g., ConfigMap "Databases" vs annotation "databases") | Matched (case-insensitive) |
