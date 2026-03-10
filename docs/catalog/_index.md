---
title: "Catalog"
linkTitle: "Catalog"
description: "Configure RGDs for the knodex catalog"
weight: 20
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Catalog

The knodex catalog displays ResourceGraphDefinitions (RGDs) that users can deploy as instances. This section covers how to configure RGDs for catalog visibility and deployment.

## Overview

RGDs are Kubernetes Custom Resources that define templated resource graphs. When properly annotated, they appear in the catalog where users can browse and deploy them.

**Key Concepts:**

- **Annotations & Labels**: Control catalog visibility, metadata, deployment modes, and scoping
- **Project Scoping**: Restrict which projects can see and deploy an RGD
- **Schema Definition**: Define the input parameters users provide when deploying
- **UI Rendering**: How the deployment form is generated from the schema

## RGD Structure

A complete RGD has the following structure:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: my-rgd-name # Unique name (DNS-compatible)
  namespace: default # Namespace where RGD is deployed
  labels:
    knodex.io/project: proj-team-name # Optional: restricts to project
  annotations:
    # Required for catalog visibility
    knodex.io/catalog: "true"

    # Deployment mode control
    knodex.io/deployment-modes: "direct,gitops,hybrid"

    # Optional metadata
    knodex.io/description: "Human-readable description"
    knodex.io/tags: "tag1,tag2,tag3"
    knodex.io/category: "database"
    knodex.io/version: "1.0.0"
spec:
  schema:
    apiVersion: v1alpha1
    kind: MyResourceKind # Instance kind users will create
    spec:
      # Input parameters with types and defaults
      paramName: string | default="value"

  resources:
    # Kubernetes resources to create
    - id: resource-id
      template:
        # Standard K8s resource with ${schema.spec.*} substitutions
```

## Complete Examples

### Example 1: Simple Public RGD

A basic application RGD visible to all users:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: simple-app
  namespace: default
  annotations:
    knodex.io/catalog: "true"
    knodex.io/description: "Simple application with deployment and service"
    knodex.io/tags: "simple,basic,starter"
    knodex.io/category: "examples"
spec:
  schema:
    apiVersion: v1alpha1
    kind: SimpleApp
    spec:
      appName: string
      image: string | default="nginx:latest"
      port: integer | default=80
      replicas: integer | default=1

  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.appName}
          labels:
            app: ${schema.spec.appName}
        spec:
          replicas: ${schema.spec.replicas}
          selector:
            matchLabels:
              app: ${schema.spec.appName}
          template:
            metadata:
              labels:
                app: ${schema.spec.appName}
            spec:
              containers:
                - name: app
                  image: ${schema.spec.image}
                  ports:
                    - containerPort: ${schema.spec.port}

    - id: service
      template:
        apiVersion: v1
        kind: Service
        metadata:
          name: ${schema.spec.appName}
        spec:
          selector:
            app: ${schema.spec.appName}
          ports:
            - port: ${schema.spec.port}
              targetPort: ${schema.spec.port}
```

### Example 2: Project-Scoped RGD with GitOps-Only Deployment

An RGD visible only to the payments team, restricted to GitOps deployments:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: payment-service
  namespace: default
  labels:
    knodex.io/project: proj-payments-team # Restricts visibility
  annotations:
    knodex.io/catalog: "true"
    knodex.io/deployment-modes: "gitops" # GitOps only
    knodex.io/description: "Payment processing service with PCI compliance settings"
    knodex.io/tags: "payment,pci,secure"
    knodex.io/category: "finance"
spec:
  schema:
    apiVersion: v1alpha1
    kind: PaymentService
    spec:
      serviceName: string
      environment: string | default="staging"
      enableEncryption: boolean | default=true

  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.serviceName}
          labels:
            app: ${schema.spec.serviceName}
            compliance: pci-dss
        spec:
          replicas: 3
          selector:
            matchLabels:
              app: ${schema.spec.serviceName}
          template:
            metadata:
              labels:
                app: ${schema.spec.serviceName}
            spec:
              securityContext:
                runAsNonRoot: true
              containers:
                - name: payment
                  image: payments/service:latest
                  securityContext:
                    readOnlyRootFilesystem: true
                  env:
                    - name: ENVIRONMENT
                      value: ${schema.spec.environment}
                    - name: ENCRYPTION_ENABLED
                      value: "${schema.spec.enableEncryption}"
```

### Example 3: RGD with Advanced Configuration

An RGD with basic fields visible and advanced options hidden by default:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp-advanced
  namespace: default
  annotations:
    knodex.io/catalog: "true"
    knodex.io/description: "Web application with advanced configuration options"
    knodex.io/tags: "webapp,configurable,production"
    knodex.io/category: "application"
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebAppAdvanced
    spec:
      # Basic fields - always visible
      name: string
      image: string
      port: integer | default=80

      # Advanced fields - hidden by default with secure defaults
      advanced:
        replicas: integer | default=1
        resources:
          limits:
            cpu: string | default="500m"
            memory: string | default="256Mi"
          requests:
            cpu: string | default="100m"
            memory: string | default="128Mi"
        securityContext:
          runAsNonRoot: boolean | default=true
          readOnlyRootFilesystem: boolean | default=true

  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.name}
          labels:
            app: ${schema.spec.name}
        spec:
          replicas: ${schema.spec.advanced.replicas}
          selector:
            matchLabels:
              app: ${schema.spec.name}
          template:
            metadata:
              labels:
                app: ${schema.spec.name}
            spec:
              securityContext:
                runAsNonRoot: ${schema.spec.advanced.securityContext.runAsNonRoot}
              containers:
                - name: app
                  image: ${schema.spec.image}
                  ports:
                    - containerPort: ${schema.spec.port}
                  securityContext:
                    readOnlyRootFilesystem: ${schema.spec.advanced.securityContext.readOnlyRootFilesystem}
                  resources:
                    limits:
                      cpu: ${schema.spec.advanced.resources.limits.cpu}
                      memory: ${schema.spec.advanced.resources.limits.memory}
                    requests:
                      cpu: ${schema.spec.advanced.resources.requests.cpu}
                      memory: ${schema.spec.advanced.resources.requests.memory}

    - id: service
      template:
        apiVersion: v1
        kind: Service
        metadata:
          name: ${schema.spec.name}
        spec:
          selector:
            app: ${schema.spec.name}
          ports:
            - port: ${schema.spec.port}
              targetPort: ${schema.spec.port}
```

---

## Documentation

| Guide                                           | Description                                      |
| ----------------------------------------------- | ------------------------------------------------ |
| [Annotations & Labels](annotations-and-labels/) | Catalog discovery, deployment modes, and scoping |
| [Schema & UI](schema-ui/)                       | Schema definition and UI rendering behavior      |
| [Project Scoping](project-scoping/)             | Control RGD visibility by project membership     |

---

**Next:** [Annotations & Labels](annotations-and-labels/) →
