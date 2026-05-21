---
title: Project Scoping
description: Control which projects can see an RGD using the knodex.io/project label for project-scoped visibility.
sidebar_position: 3
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Project Scoping

By default, RGDs in the catalog are visible to all users across all projects. The `knodex.io/project` label restricts an RGD to users who are members of a specific project.

## Public RGD (No Label)

An RGD without the `knodex.io/project` label is public. All authenticated users who pass the category Casbin check can see it.

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: nginx-site
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Nginx Site"
    knodex.io/category: "web"
  # No knodex.io/project label = public
spec:
  schema:
    apiVersion: web.knodex.io/v1alpha1
    kind: NginxSite
    spec:
      name: string
```

## Project-Scoped RGD (With Label)

An RGD with the `knodex.io/project` label is visible only to members of the named project.

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: internal-api
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Internal API"
    knodex.io/category: "applications"
  labels:
    knodex.io/project: "alpha"
spec:
  schema:
    apiVersion: apps.knodex.io/v1alpha1
    kind: InternalAPI
    spec:
      name: string
      image: string
```

Only users who are members of the `alpha` project see this RGD in their catalog.

## Visibility Matrix

| RGD Label | User Project Membership | Visible? |
|-----------|------------------------|----------|
| No `knodex.io/project` label | Any project | Yes (public) |
| `knodex.io/project: "alpha"` | Member of `alpha` | Yes |
| `knodex.io/project: "alpha"` | Member of `beta` only | No |
| `knodex.io/project: "alpha"` | Server admin | Yes |
| No label | No project membership | Yes (but may have no deploy targets) |

## Project Label Format

The label value must exactly match the project name as defined in the Project CRD:

```yaml
labels:
  knodex.io/project: "my-project-name"
```

- The value is case-sensitive
- It must be a valid Kubernetes label value (63 characters max, alphanumeric with `-` and `.`)
- It must match a Project resource name in the cluster

## Finding the Project Namespace

The project name used in the label corresponds to the `metadata.name` of the Project CRD:

```yaml
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: alpha    # <-- This is the value used in knodex.io/project label
spec:
  destinations:
    - namespace: "alpha-apps"
```

## Multiple Project Access

A single RGD can only be scoped to one project. If the same RGD template should be available to multiple projects, you have two options:

1. **Make it public** by removing the `knodex.io/project` label entirely. This is the simplest approach when the RGD is safe for general use.

2. **Create separate RGD instances** for each project, each with its own `knodex.io/project` label. This allows per-project customization of defaults or deployment mode restrictions.

:::note[Current Limitation]
There is no support for multi-value project labels (e.g., `knodex.io/project: "alpha,beta"`). An RGD is scoped to exactly zero projects (public) or one project.
:::

## Combining with Deployment Modes

Project scoping and deployment mode restrictions are independent and can be combined:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: production-database
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Production Database"
    knodex.io/category: "databases"
    knodex.io/deployment-modes: "gitops"
  labels:
    knodex.io/project: "production"
spec:
  schema:
    apiVersion: db.knodex.io/v1alpha1
    kind: ProductionDB
    spec:
      name: string
      storage: string | default="100Gi"
```

This RGD is visible only to `production` project members and can only be deployed via GitOps mode.
