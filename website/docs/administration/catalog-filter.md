---
title: Catalog Package Filter
description: Filter RGDs in the catalog by package label to scope visibility for multi-team environments.
sidebar_position: 9
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Catalog Package Filter

The `CATALOG_PACKAGE_FILTER` environment variable controls which RGDs appear in the Knodex catalog by filtering on the `knodex.io/package` label. This is useful in multi-team environments where each Knodex instance should only show relevant RGDs.

## Configuration

### Helm Values

```yaml
server:
  config:
    CATALOG_PACKAGE_FILTER: "platform-team"
```

### Kubernetes Manifest

Set the environment variable directly on the Knodex deployment:

```yaml
env:
  - name: CATALOG_PACKAGE_FILTER
    value: "platform-team"
```

## Labeling RGDs

Add the `knodex.io/package` label to your RGDs:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp
  labels:
    knodex.io/package: "platform-team"
  annotations:
    knodex.io/catalog: "true"
spec:
  # ...
```

## Behavior Matrix

| `CATALOG_PACKAGE_FILTER` | RGD `knodex.io/package` label | Visible in catalog? |
|--------------------------|-------------------------------|---------------------|
| Not set (empty) | Any value or missing | Yes |
| Not set (empty) | `platform-team` | Yes |
| `platform-team` | `platform-team` | Yes |
| `platform-team` | `other-team` | No |
| `platform-team` | Missing | No |

## Backward Compatibility

When `CATALOG_PACKAGE_FILTER` is not set or empty, the filter is disabled and all RGDs with the `knodex.io/catalog: "true"` annotation are visible. This preserves backward compatibility with existing deployments.

## Startup Logging

When the filter is active, the server logs the configured package filter at startup:

```
INFO  catalog package filter enabled  filter=platform-team
```

When the filter is not set:

```
INFO  catalog package filter disabled (showing all packages)
```

:::note[Multiple Packages]
The filter currently supports a single package value. To expose RGDs from multiple packages, either leave the filter empty (show all) or run separate Knodex instances per package.
:::
