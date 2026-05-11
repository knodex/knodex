---
title: Catalog
description: Understand the Knodex RGD catalog, how ResourceGraphDefinitions are discovered, and how annotations control catalog behavior.
sidebar_position: 1
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Catalog

The Knodex catalog automatically discovers ResourceGraphDefinitions (RGDs) in your cluster and presents them as a browsable, searchable collection. RGDs appear in the catalog when they carry the `knodex.io/catalog: "true"` annotation.

## RGD Structure

A catalog-enabled RGD follows the standard KRO ResourceGraphDefinition format with Knodex-specific annotations:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: my-resource
  annotations:
    # Required: enables catalog discovery
    knodex.io/catalog: "true"
    # Optional: metadata for display
    knodex.io/title: "My Resource"
    knodex.io/description: "Provisions a fully configured resource"
    knodex.io/tags: "networking,production"
    knodex.io/category: "networking"
    knodex.io/icon: "globe"
    knodex.io/docs-url: "https://docs.example.com/my-resource"
    # Optional: deployment restrictions
    knodex.io/deployment-modes: "direct,gitops"
  labels:
    # Optional: scope to a project
    knodex.io/project: "alpha"
    # Optional: scope to an organization (Enterprise)
    knodex.io/organization: "acme-corp"
spec:
  schema:
    apiVersion: example.io/v1alpha1
    kind: MyResource
    spec:
      name: string
      replicas: integer | default=1
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.name}
        spec:
          replicas: ${schema.spec.replicas}
```

## Examples

### Simple Public RGD

Available to all users in all projects. No restrictions on deployment mode.

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: static-site
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Static Site"
    knodex.io/description: "Deploy a static website with Nginx"
    knodex.io/tags: "web,static"
    knodex.io/category: "web"
    knodex.io/icon: "globe"
spec:
  schema:
    apiVersion: web.knodex.io/v1alpha1
    kind: StaticSite
    spec:
      siteName: string
      replicas: integer | default=2
      image: string | default="nginx:latest"
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.siteName}
        spec:
          replicas: ${schema.spec.replicas}
          selector:
            matchLabels:
              app: ${schema.spec.siteName}
          template:
            metadata:
              labels:
                app: ${schema.spec.siteName}
            spec:
              containers:
                - name: nginx
                  image: ${schema.spec.image}
                  ports:
                    - containerPort: 80
    - id: service
      template:
        apiVersion: v1
        kind: Service
        metadata:
          name: ${schema.spec.siteName}
        spec:
          selector:
            app: ${schema.spec.siteName}
          ports:
            - port: 80
              targetPort: 80
```

### Project-Scoped GitOps-Only RGD

Visible only to users in the `production` project. Forces GitOps deployment mode.

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: production-api
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Production API"
    knodex.io/description: "Production API deployment with HPA and PDB"
    knodex.io/category: "applications"
    knodex.io/deployment-modes: "gitops"
    knodex.io/docs-url: "https://wiki.internal/production-api"
  labels:
    knodex.io/project: "production"
spec:
  schema:
    apiVersion: apps.knodex.io/v1alpha1
    kind: ProductionAPI
    spec:
      name: string
      image: string
      minReplicas: integer | default=3
      maxReplicas: integer | default=10
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.name}
        spec:
          replicas: ${schema.spec.minReplicas}
          # ... deployment spec
    - id: hpa
      template:
        apiVersion: autoscaling/v2
        kind: HorizontalPodAutoscaler
        metadata:
          name: ${schema.spec.name}
        spec:
          scaleTargetRef:
            apiVersion: apps/v1
            kind: Deployment
            name: ${schema.spec.name}
          minReplicas: ${schema.spec.minReplicas}
          maxReplicas: ${schema.spec.maxReplicas}
    - id: pdb
      template:
        apiVersion: policy/v1
        kind: PodDisruptionBudget
        metadata:
          name: ${schema.spec.name}
        spec:
          minAvailable: 1
          selector:
            matchLabels:
              app: ${schema.spec.name}
```

### Advanced RGD with Hidden Fields

Uses `spec.advanced` to hide optional configuration behind a collapsible section in the deploy form. Includes an `externalRef` dependency on an existing database.

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: microservice
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Microservice"
    knodex.io/description: "Full microservice with ingress, database connection, and monitoring"
    knodex.io/category: "applications"
    knodex.io/tags: "microservice,production,monitoring"
    knodex.io/icon: "box"
    knodex.io/extends-kind: "PostgresCluster"
    knodex.io/property-order: '{"": ["name","image","dbRef.name","dbRef.namespace"]}'
spec:
  schema:
    apiVersion: apps.knodex.io/v1alpha1
    kind: Microservice
    spec:
      name: string
      image: string
      dbRef:
        name: string
        namespace: string
      advanced:
        logLevel: string | default="info"
        metricsEnabled: boolean | default=true
        resourceLimits:
          cpu: string | default="500m"
          memory: string | default="512Mi"
  resources:
    - id: database
      externalRef:
        apiVersion: db.knodex.io/v1alpha1
        kind: PostgresCluster
      readyWhen:
        - key: status.ready
          value: "true"
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.name}
        spec:
          replicas: 2
          template:
            spec:
              containers:
                - name: app
                  image: ${schema.spec.image}
                  env:
                    - name: DB_HOST
                      value: ${database.status.host}
                    - name: LOG_LEVEL
                      value: ${schema.spec.advanced.logLevel}
```

## Documentation Links

| Link | Description |
|------|-------------|
| [Annotations and Labels](annotations-and-labels) | Complete reference for all catalog annotations and labels |
| [Schema and UI Rendering](schema-ui) | How RGD schemas translate to deploy forms |
| [Project Scoping](project-scoping) | Control RGD visibility with project labels |
| [Category Ordering](category-ordering) | Configure sidebar category order with ConfigMap |
