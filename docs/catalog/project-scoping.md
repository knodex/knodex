---
title: "Project Scoping"
linkTitle: "Project Scoping"
description: "Control RGD visibility by project membership"
weight: 3
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Project Scoping

Control who can see your RGD in the catalog based on project membership.

## Overview

By default, RGDs with the `knodex.io/catalog: "true"` annotation are visible to all authenticated users. You can restrict visibility to specific project members by adding a project label.

## Public RGD (Visible to All)

Add only the catalog annotation with **no project label**:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: shared-postgres
  namespace: default
  annotations:
    knodex.io/catalog: "true" # Gateway to catalog
    # NO project label = visible to ALL authenticated users
spec:
  # ... RGD spec
```

**Use cases:**

- Shared infrastructure components (databases, caches)
- Organization-wide application templates
- Public examples and starter templates

## Project-Scoped RGD (Restricted)

Add the catalog annotation **AND** the project label:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: team-specific-app
  namespace: default
  labels:
    knodex.io/project: proj-alpha-team # Restricts to project members
  annotations:
    knodex.io/catalog: "true" # Gateway to catalog
spec:
  # ... RGD spec
```

**Use cases:**

- Team-specific application configurations
- Compliance-restricted resources (PCI, HIPAA)
- Pre-production or experimental RGDs

## Visibility Matrix

| Configuration                               | Catalog Visible To      |
| ------------------------------------------- | ----------------------- |
| No `knodex.io/catalog` annotation           | No one (not in catalog) |
| `knodex.io/catalog: "true"` only            | All authenticated users |
| `knodex.io/catalog: "true"` + project label | Project members only    |

## Project Label Format

The project label must match the project namespace name exactly:

```yaml
labels:
  knodex.io/project: proj-<team-name>
```

{{< alert title="Important" >}}
Use the project **namespace name** (e.g., `proj-alpha-team`), not the project display name (e.g., "Alpha Team").
{{< /alert >}}

## Finding Your Project Namespace

```bash
# List all projects
kubectl get projects -A

# Get project details
kubectl get project <project-name> -o yaml
```

The namespace is typically prefixed with `proj-` followed by the team or project identifier.

## Multiple Project Access

To make an RGD visible to multiple projects, you currently need to:

1. **Create copies** - Deploy the same RGD with different project labels
2. **Use public visibility** - Remove the project label for shared access

{{< alert title="Note" >}}
Multi-project labels on a single RGD are not currently supported. Each RGD can only be scoped to one project.
{{< /alert >}}

## Combining with Deployment Modes

Project scoping works independently of deployment mode restrictions:

```yaml
metadata:
  name: production-database
  labels:
    knodex.io/project: proj-platform-team # Only platform team can see
  annotations:
    knodex.io/catalog: "true"
    knodex.io/deployment-modes: "gitops" # Only GitOps allowed
    knodex.io/category: "database"
```

This RGD:

- Is only visible to `proj-platform-team` members
- Can only be deployed via GitOps (not direct or hybrid)

## Example: Team-Specific Production RGD

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: payments-api
  namespace: default
  labels:
    knodex.io/project: proj-payments-team
  annotations:
    knodex.io/catalog: "true"
    knodex.io/deployment-modes: "gitops"
    knodex.io/description: "Payments API with PCI compliance configuration"
    knodex.io/tags: "api,payments,pci,production"
    knodex.io/category: "security"
spec:
  schema:
    apiVersion: v1alpha1
    kind: PaymentsAPI
    spec:
      environment: string | default="staging"
      replicas: integer | default=3

  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: payments-api-${schema.spec.environment}
          labels:
            app: payments-api
            environment: ${schema.spec.environment}
            compliance: pci-dss
        spec:
          replicas: ${schema.spec.replicas}
          selector:
            matchLabels:
              app: payments-api
          template:
            metadata:
              labels:
                app: payments-api
            spec:
              securityContext:
                runAsNonRoot: true
                fsGroup: 1000
              containers:
                - name: api
                  image: payments/api:latest
                  securityContext:
                    readOnlyRootFilesystem: true
                    allowPrivilegeEscalation: false
```

---

**Back to:** [RGD Development](..) | **Next:** [Annotations & Labels](../annotations-and-labels/) →
