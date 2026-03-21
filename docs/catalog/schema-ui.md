---
title: "Schema & UI"
linkTitle: "Schema & UI"
description: "Schema definition and how knodex renders fields in the deployment form"
weight: 2
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Schema & UI Rendering

The RGD schema defines input parameters that users provide when deploying. Knodex parses the schema and renders an appropriate form in the UI.

## Schema Definition

The schema defines the input parameters users provide when deploying an instance.

{{< alert title="Official Documentation" >}}
For complete schema syntax, advanced types, and CEL expressions, refer to the [kro official documentation](https://kro.run/docs/concepts/resourcegraphdefinition/#schema).
{{< /alert >}}

### Basic Types

```yaml
spec:
  schema:
    apiVersion: v1alpha1
    kind: MyApp
    spec:
      # String with default
      name: string | default="my-app"

      # Integer with default
      replicas: integer | default=3

      # Boolean with default
      enableMetrics: boolean | default=false

      # Required string (no default)
      image: string
```

### Type Reference

| Type           | Description   | Example                         |
| -------------- | ------------- | ------------------------------- |
| `string`       | Text value    | `name: string`                  |
| `integer`      | Whole number  | `replicas: integer`             |
| `boolean`      | True/false    | `enabled: boolean`              |
| `\| default=X` | Default value | `port: integer \| default=8080` |

## Resource Templates

Resources define the Kubernetes objects created when an instance is deployed.

### Variable Substitution

Use `${schema.spec.fieldName}` to reference user inputs:

```yaml
template:
  metadata:
    name: ${schema.spec.appName}
    labels:
      environment: ${schema.spec.environment}
  spec:
    replicas: ${schema.spec.replicas}
    containers:
      - image: ${schema.spec.image}
        ports:
          - containerPort: ${schema.spec.port}
```

### Conditional Resources

Include resources only when certain conditions are met:

```yaml
resources:
  # Always included
  - id: app
    template:
      # ... deployment spec

  # Only included when enableDatabase is true
  - id: database
    includeWhen:
      - ${schema.spec.enableDatabase == true}
    template:
      apiVersion: apps/v1
      kind: StatefulSet
      # ... database spec
```

## Advanced Configuration Section

Fields placed under a `spec.advanced` path in the schema are hidden by default in the deployment form. This improves user experience by showing only essential fields initially.

### Defining Advanced Fields

Place configuration options that most users won't need to modify under `spec.advanced`:

```yaml
spec:
  schema:
    apiVersion: v1alpha1
    kind: MyApp
    spec:
      # Basic fields (always visible)
      name: string
      image: string
      port: integer | default=80

      # Advanced fields (hidden by default, shown via toggle)
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
```

### User Experience

When users deploy an RGD with an `advanced` section:

1. Basic fields appear at the top of the configuration section
2. A "Show Advanced Configuration" toggle appears (collapsed by default)
3. Clicking the toggle reveals all advanced fields with their defaults pre-populated
4. Users can modify advanced values or leave the secure defaults

### Requirements for Advanced Fields

{{< alert title="Important" >}}
**All advanced fields MUST have default values defined.** Since advanced fields are hidden by default, users may deploy without seeing them. Without defaults, required fields would cause validation errors.
{{< /alert >}}

```yaml
# Good - all advanced fields have defaults
advanced:
  replicas: integer | default=1
  enableMetrics: boolean | default=true
  logLevel: string | default="info"

# Bad - missing defaults will cause issues
advanced:
  replicas: integer        # No default!
  customConfig: string     # No default!
```

### Nested Advanced Properties

Advanced sections support deep nesting. All nested properties inherit the "advanced" designation:

```yaml
advanced:
  security: # Advanced
    runAsNonRoot: boolean | default=true # Advanced
    allowPrivilegeEscalation: boolean | default=false # Advanced
  resources: # Advanced
    limits: # Advanced
      cpu: string | default="500m" # Advanced
      memory: string | default="256Mi" # Advanced
```

### Referencing Advanced Fields in Templates

Use the full path to reference advanced fields:

```yaml
resources:
  - id: deployment
    template:
      spec:
        replicas: ${schema.spec.advanced.replicas}
        template:
          spec:
            securityContext:
              runAsNonRoot: ${schema.spec.advanced.securityContext.runAsNonRoot}
            containers:
              - resources:
                  limits:
                    cpu: ${schema.spec.advanced.resources.limits.cpu}
                    memory: ${schema.spec.advanced.resources.limits.memory}
```

## External References (externalRef)

When an RGD resource uses `externalRef` instead of `template`, it references an existing Kubernetes resource. Knodex detects paired `externalRef.<id>.name` and `externalRef.<id>.namespace` schema fields and renders a **resource picker dropdown** that auto-fills both values when a resource is selected.

### Paired Pattern (Resource Picker)

Define an `externalRef` object in the schema with `name` and `namespace` sub-fields. The resource ID in the schema path must match the resource `id` in the `resources` list:

```yaml
spec:
  schema:
    apiVersion: v1alpha1
    kind: MyApp
    spec:
      useExternalConfig: boolean | default=false
      externalRef:
        configmap:
          name: string | default=""
          namespace: string | default=""

  resources:
    - id: configmap
      includeWhen:
        - ${schema.spec.useExternalConfig == true}
      externalRef:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: ${schema.spec.externalRef.configmap.name}
          namespace: ${schema.spec.externalRef.configmap.namespace}
```

In the deploy form, the `externalRef.configmap` object renders as a single dropdown listing ConfigMaps from the deployment namespace. Selecting a resource auto-fills both `name` and `namespace`.

### How It Works

1. The parser detects paired `${schema.spec.externalRef.<id>.name}` and `${schema.spec.externalRef.<id>.namespace}` expressions in the externalRef metadata
2. The enricher attaches resource picker metadata to the parent object (`externalRef.<id>`) with `autoFillFields` mapping
3. The UI renders an `ExternalRefSelector` component on the parent object instead of individual text inputs
4. When a user selects a resource, both `name` and `namespace` are set via the form context

### Namespace Filtering

The resource picker always queries resources from the **deployment namespace** selected at the top of the deploy form. Until a namespace is selected, the dropdown shows "Select a deployment namespace to view available resources."

### Example: Multiple External References

An RGD can reference multiple external resources. Each gets its own resource picker:

```yaml
spec:
  schema:
    apiVersion: v1alpha1
    kind: PlatformApp
    spec:
      useExistingDatabase: boolean | default=false
      externalRef:
        database:
          name: string | default=""
          namespace: string | default=""
        configmap:
          name: string | default=""
          namespace: string | default=""

  resources:
    - id: database
      includeWhen:
        - ${schema.spec.useExistingDatabase == true}
      externalRef:
        apiVersion: v1
        kind: Service
        metadata:
          name: ${schema.spec.externalRef.database.name}
          namespace: ${schema.spec.externalRef.database.namespace}

    - id: configmap
      externalRef:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: ${schema.spec.externalRef.configmap.name}
          namespace: ${schema.spec.externalRef.configmap.namespace}
```

### Nested External References (Composite RGDs)

When an RGD uses **template** resources whose specs reference `${schema.spec.externalRef.*}` fields, Knodex automatically resolves the target Kind by performing a cross-RGD lookup. This enables resource picker dropdowns for composite RGDs that reference other KRO-managed resources through template resources.

```yaml
spec:
  schema:
    apiVersion: v1alpha1
    kind: AppWithESO
    spec:
      externalRef:
        argocdClusterRef:
          name: string | default=""
          namespace: string | default=""
        keyVaultRef:
          name: string | default=""
          namespace: string | default=""

  resources:
    # Resource-level externalRef (direct — always gets a dropdown)
    - id: argocdClusterRef
      externalRef:
        apiVersion: kro.run/v1alpha1
        kind: ArgoCDAKSCluster
        metadata:
          name: ${schema.spec.externalRef.argocdClusterRef.name}
          namespace: ${schema.spec.externalRef.argocdClusterRef.namespace}

    # Template resource that also uses externalRef schema fields
    # The target Kind is resolved by looking up the child RGD (AKVESOBinding)
    - id: esoBinding
      template:
        apiVersion: kro.run/v1alpha1
        kind: AKVESOBinding
        spec:
          externalRef:
            keyVault:
              name: ${schema.spec.externalRef.keyVaultRef.name}
              namespace: ${schema.spec.externalRef.keyVaultRef.namespace}
```

**How it works:**

1. The enricher detects paired `spec.externalRef.*.name/namespace` patterns in template resource schema fields
2. It looks up the child RGD by the template resource's Kind (e.g., `AKVESOBinding`) in the catalog
3. It parses the child RGD's resources to find the matching `externalRef` entry and extracts the target `apiVersion`/`kind`
4. The resource picker dropdown is attached to the parent object using the resolved Kind

**Graceful degradation:** If the child RGD is not in the catalog (not yet deployed or in a different cluster), the field renders as a plain text object instead of a dropdown.

### Combining with Conditional Resources

External references pair well with `includeWhen` conditions. Use a boolean controlling field to show/hide the resource picker:

```yaml
spec:
  schema:
    apiVersion: v1alpha1
    kind: MicroservicesPlatform
    spec:
      platformName: string
      useExistingDatabase: boolean | default=false
      externalRef:
        externaldb:
          name: string | default=""
          namespace: string | default=""

  resources:
    - id: externaldb
      includeWhen:
        - ${schema.spec.useExistingDatabase == true}
      externalRef:
        apiVersion: v1
        kind: Service
        metadata:
          name: ${schema.spec.externalRef.externaldb.name}
          namespace: ${schema.spec.externalRef.externaldb.namespace}

    - id: internaldb
      includeWhen:
        - ${schema.spec.useExistingDatabase == false}
      template:
        apiVersion: apps/v1
        kind: StatefulSet
        # ... internal database spec
```

When `useExistingDatabase` is false, the `externalRef` section is hidden. When checked, a Service resource picker appears.

## Secret Reference Descriptions

When an `externalRef` resource has `kind: Secret`, Knodex detects it as a secret dependency and displays it in the catalog detail **Secrets tab**. RGD authors can add a `description` marker to help users understand what each secret is for.

### Adding Descriptions

Add a `description` marker to the `name` sub-field of the externalRef schema entry:

```yaml
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebAppWithSecret
    spec:
      externalRef:
        dbSecret:
          name: string | default="" description="Name of the Kubernetes Secret containing database credentials"
          namespace: string | default="" description="Namespace of the Secret"

  resources:
    - id: dbSecret
      externalRef:
        apiVersion: v1
        kind: Secret
        metadata:
          name: ${schema.spec.externalRef.dbSecret.name}
          namespace: ${schema.spec.externalRef.dbSecret.namespace}
```

### How It Renders

The description from the `name` sub-field is extracted and shown in the catalog detail Secrets tab as the purpose of that secret reference. Users see:

| Element | Source | Example |
|---------|--------|---------|
| **Title** | The externalRef field name (`dbSecret`) | `dbSecret` |
| **Description** | The `name` sub-field's `description` marker | "Name of the Kubernetes Secret containing database credentials" |
| **Type badge** | Computed from the name/namespace expressions | `user-provided`, `fixed`, or `dynamic` |
| **Name/Namespace** | Literal values or CEL expressions | `my-db-secret` / `production` |

{{< alert title="Why the name sub-field?" >}}
In KRO SimpleSchema, parent objects (like `dbSecret: {...}`) have no description slot of their own — descriptions live on leaf fields. The `name` sub-field best describes *what* the secret is, while `namespace` describes *where* it lives. Knodex uses the `name` description by convention.
{{< /alert >}}

### Secret Reference Types

Knodex classifies each secret reference based on how the name and namespace are resolved:

| Type | Condition | Catalog Display |
|------|-----------|-----------------|
| **user-provided** | Both name and namespace use the passthrough pattern `${schema.spec.externalRef.<id>.name}` | Title, description, and a "user-provided" badge. No name/namespace shown (user supplies these at deploy time). |
| **fixed** | Name and namespace are literal strings (no `${...}` expressions) | Literal name and namespace displayed |
| **dynamic** | Name or namespace uses a non-passthrough CEL expression | CEL expressions displayed in monospace |

### Best Practice: Always Add Descriptions

Without a description, the Secrets tab shows only the externalRef field name and type — which may not be meaningful to users deploying the RGD. Add descriptions to help users understand what credentials or certificates each secret should contain.

```yaml
# Without description — users see only "dbSecret" with no context
externalRef:
  dbSecret:
    name: string | default=""
    namespace: string | default=""

# With description — users understand what to provide
externalRef:
  dbSecret:
    name: string | default="" description="Name of the Kubernetes Secret containing database credentials"
    namespace: string | default="" description="Namespace of the Secret"
```

For the user-facing guide on working with secrets, see [Managing Secrets](../../user-guide/managing-secrets/).

## UI Rendering Behavior

### Field Rendering by Type

| Schema Type        | UI Component             | Notes                                                  |
| ------------------ | ------------------------ | ------------------------------------------------------ |
| `string`           | Text input               | Single line                                            |
| `integer`          | Number input             | Integer validation                                     |
| `boolean`          | Toggle/Checkbox          | On/Off state                                           |
| Nested object      | Grouped fields           | Collapsible section                                    |
| externalRef object | Resource picker dropdown | Auto-fills name + namespace from selected K8s resource |

### Default Value Handling

| Scenario                    | UI Behavior                       |
| --------------------------- | --------------------------------- |
| Field has default           | Pre-filled with default value     |
| Field has no default        | Empty, marked as required         |
| Advanced field with default | Hidden until toggle expanded      |
| Advanced field no default   | **Validation error** (avoid this) |

### Visibility Rules

| Path Pattern      | Visibility                      |
| ----------------- | ------------------------------- |
| `spec.*`          | Always visible                  |
| `spec.advanced.*` | Hidden behind "Advanced" toggle |

## Complete Example

RGD with basic fields, advanced configuration, and conditional resources:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp-full-featured
  annotations:
    knodex.io/catalog: "true"
    knodex.io/deployment-modes: "direct,gitops"
    knodex.io/description: "Full-featured web application"
    knodex.io/tags: "webapp,fullstack"
    knodex.io/category: "applications"
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebApp
    spec:
      # Basic fields - always visible
      name: string
      image: string
      port: integer | default=8080
      enableCache: boolean | default=false
      visibleAnnotation: string
      hiddenAnnotation: string
      # enableDatabase is at the includeWhen scope of only one resource, so this is a conditional resource.
      # If the resource is conditional, the spec used in this resource should be hidden ONLY if the spec is only in this conditional resource or in another conditional false by default.
      # As database.name is used only in this resource, it should be hidden when enableDatabase is false.
      # As visibleAnnotation is used in the annotation of this conditionnal resource AND in other part of the RGD, the name
      # as hiddenAnnotation is only used in conditionnal resources, it should be hidden when both conditional resources are false, but appear only once if both are true.
      enableDatabase: boolean | default=false
      database:
        name: string | default=""
      # Advanced fields - hidden by default
      advanced:
        replicas: integer | default=2
        resources:
          limits:
            cpu: string | default="500m"
            memory: string | default="512Mi"
        healthCheck:
          enabled: boolean | default=true
          path: string | default="/health"
          interval: integer | default=30

  resources:
    # Main application - always included
    - id: app
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.name}
          annotations:
            spec: "${schema.spec.visibleAnnotation}"
        spec:
          replicas: ${schema.spec.advanced.replicas}
          selector:
            matchLabels:
              app: ${schema.spec.name}
          template:
            spec:
              containers:
                - name: app
                  image: ${schema.spec.image}
                  ports:
                    - containerPort: ${schema.spec.port}
                  resources:
                    limits:
                      cpu: ${schema.spec.advanced.resources.limits.cpu}
                      memory: ${schema.spec.advanced.resources.limits.memory}

    # Database - conditionally included
    - id: database
      includeWhen:
        - ${schema.spec.enableDatabase == true}
      template:
        apiVersion: apps/v1
        kind: StatefulSet
        metadata:
          name: ${schema.spec.database.name}
          annotations:
            spec: "${schema.spec.visibleAnnotation}"
            spec: "${schema.spec.hiddenAnnotation}"
        # ... database spec

    # Cache - conditionally included
    - id: cache
      includeWhen:
        - ${schema.spec.enableCache == true}
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.name}-cache
          annotations:
            spec: "${schema.spec.visibleAnnotation}"
            spec: "${schema.spec.hiddenAnnotation}"
        # ... cache spec
```

---

**Back to:** [RGD Development](..) | [Annotations & Labels](../annotations-and-labels/)
