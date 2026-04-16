---
title: Categories
description: How RGD categories work in Knodex and how to browse by category.
sidebar_position: 7
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Categories

Categories provide a way to organize ResourceGraphDefinitions (RGDs) into logical groups, making it easier to browse and discover resources in the catalog.

## How Categories Work

Categories are defined through three mechanisms:

1. **Annotation** -- Each RGD is assigned to a category via the `knodex.io/category` annotation on the RGD resource.
2. **ConfigMap** -- Category metadata (display name, description, icon, ordering) is defined in a `knodex-categories` ConfigMap in the Knodex namespace.
3. **Casbin** -- Visibility of categories is governed by Casbin RBAC policies. You only see categories that contain RGDs you have permission to view.

## Browsing by Category

When categories are configured, they appear in the sidebar under the **Categories** section. Click a category to filter the catalog to only RGDs in that group.

Within a category view, you can still use search and filters to further narrow results.

## Uncategorized RGDs

RGDs without a `knodex.io/category` annotation appear in an **Uncategorized** group. This ensures that all RGDs remain discoverable even if they have not been assigned a category.

## Permissions

Category visibility follows the same RBAC rules as catalog visibility. If you do not have access to any RGDs within a category, that category is hidden from your sidebar.

## Configuring Categories

Category configuration is an operator task. For details on creating and managing category definitions, see [Category Ordering](../rgd-authoring/category-ordering).
