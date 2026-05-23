---
title: Schema and UI Rendering
description: How RGD schemas define deploy form fields, handle external references, secret references, advanced configuration, and conditional resources.
sidebar_position: 2
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Schema and UI Rendering

Knodex auto-generates deployment forms from the RGD schema definition. This page explains how schema types map to UI elements, how external references work, and how to control the user experience.

![Deploy modal showing the schema-driven form with Target step](/img/docs/deploy-target.png)

## Schema Definition

The RGD schema lives under `spec.schema.spec` and defines the fields that users fill in when deploying an instance.

### Basic Types

```yaml
spec:
  schema:
    apiVersion: example.io/v1alpha1
    kind: MyResource
    spec:
      replicas: integer | default=3
      enableHA: boolean | default=false
      memory: string | default="512Mi"
```

The instance name is collected on the General tab of the deploy form and is available as `${schema.metadata.name}` (and the target namespace as `${schema.metadata.namespace}`) — don't declare them as schema fields. See [Use `schema.metadata.name` for the Instance Name](./#use-schemametadataname-for-the-instance-name).

### Type Reference

| Schema Type | UI Element | Notes |
|------------|-----------|-------|
| `string` | Text input | Single-line text field |
| `string` with `default` | Pre-filled text input | Default value shown, editable |
| `integer` | Number input | Validates integer values |
| `boolean` | Toggle switch | Default `false` unless specified |
| `object` (nested) | Field group | Rendered as a nested section |
| `string` with enum | Dropdown select | Renders as select with options |

## Resource Templates

Resource templates reference schema fields using `${schema.spec.fieldName}` for user-supplied schema fields, and `${schema.metadata.name}` / `${schema.metadata.namespace}` for the instance name and namespace.

```yaml
resources:
  - id: deployment
    template:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: ${schema.metadata.name}
      spec:
        replicas: ${schema.spec.replicas}
```

### Conditional Resources with includeWhen

Resources can be conditionally included based on schema field values using CEL expressions:

```yaml
resources:
  - id: hpa
    includeWhen:
      - ${schema.spec.enableHA == true}
    template:
      apiVersion: autoscaling/v2
      kind: HorizontalPodAutoscaler
      metadata:
        name: ${schema.metadata.name}
      spec:
        scaleTargetRef:
          apiVersion: apps/v1
          kind: Deployment
          name: ${schema.metadata.name}
        minReplicas: ${schema.spec.minReplicas}
        maxReplicas: ${schema.spec.maxReplicas}
```

When `enableHA` is `false`, the HPA resource is not created.

## Per-Feature Enabled Toggle

When an object property contains a boolean child named exactly `enabled`, Knodex treats the object as a **feature group**: the deploy form renders the `enabled` checkbox at the top of the group and **hides the peer fields when it is off**. Turning the checkbox on reveals the rest. This keeps optional sub-systems (a database, a bastion host, monitoring) out of the user's way until they opt in.

The pattern works at any depth — nested under another object, or as a top-level object that becomes its own tab in the deploy form (each top-level object key in `spec.schema.spec` gets its own tab).

### Schema Pattern

```yaml
spec:
  schema:
    apiVersion: example.io/v1alpha1
    kind: WebApp
    spec:
      database:
        enabled: boolean | default=false
        name: string | default="mydb"
        sizeGB: integer | default=10
      cache:
        enabled: boolean | default=false
        maxMemory: string | default="256mb"
```

### UI Behavior

| State | What the user sees |
|-------|--------------------|
| `database.enabled = false` (default) | Only the **Enabled** checkbox under the Database section/tab. Peer fields (`name`, `sizeGB`) are hidden. |
| `database.enabled = true` | All peer fields appear immediately below the checkbox. |
| Object has no `enabled` boolean child | All fields render normally (no gating). |

### Pair with includeWhen for Resource Gating

The UI toggle hides input fields, but the **resources still need to be gated** so they aren't created when disabled. Combine the two:

```yaml
spec:
  schema:
    apiVersion: example.io/v1alpha1
    kind: WebApp
    spec:
      database:
        enabled: boolean | default=false
        name: string | default="mydb"
  resources:
    - id: postgres
      includeWhen:
        - ${schema.spec.database.enabled == true}
      template:
        apiVersion: apps/v1
        kind: StatefulSet
        metadata:
          name: ${schema.metadata.name}-postgres
        # ...
```

When `database.enabled` is `false`: peer inputs are hidden in the form **and** the `postgres` resource is omitted from the rendered manifest.

### Combine with Advanced Sub-Sections

The toggle composes with `advanced` sub-objects — when the feature is disabled, the advanced section is hidden along with the rest of the peers:

```yaml
spec:
  schema:
    spec:
      bastion:
        enabled: boolean | default=false
        subnetPrefix: string | default="10.0.0.64/26"
        advanced:
          sshKeyPath: string | default="/home/azureuser/.ssh/authorized_keys"
          asoCredentialSecretName: string | default="aso-credential"
```

### Working Examples

See `deploy/examples/rgds/`:
- `webapp-with-features.yaml` — top-level `database.enabled` and `cache.enabled`
- `webapp-per-feature-advanced.yaml` — toggle + nested `advanced` sub-section
- `webapp-full-featured.yaml` — toggle combined with `includeWhen` resource gating

## Advanced Configuration

Fields under `spec.advanced` receive special UI treatment. They are rendered in a collapsible "Advanced Configuration" section that is collapsed by default.

### Defining Fields Under spec.advanced

```yaml
spec:
  schema:
    apiVersion: example.io/v1alpha1
    kind: MyApp
    spec:
      name: string
      image: string
      advanced:
        logLevel: string | default="info"
        metricsEnabled: boolean | default=true
        resourceLimits:
          cpu: string | default="500m"
          memory: string | default="512Mi"
        nodeSelector:
          key: string | default=""
          value: string | default=""
```

### UX Behavior

- The advanced section is collapsed by default in the deploy form
- Users expand it to see optional configuration
- All advanced fields **must** have defaults so that deploying without expanding the section produces a valid manifest
- Nested properties under `advanced` are rendered as grouped fields within the collapsible section

### Requirements for Defaults

Every field under `spec.advanced` should have a sensible default. If a user deploys without opening the advanced section, all advanced fields use their defaults. An advanced field without a default becomes effectively required, which defeats the purpose.

### Nested Properties

Nested objects under `advanced` are rendered as sub-groups:

```yaml
advanced:
  logging:
    level: string | default="info"
    format: string | default="json"
  resources:
    cpu: string | default="250m"
    memory: string | default="256Mi"
```

This renders as two groups ("logging" and "resources") within the advanced section.

### Referencing in Templates

Advanced fields are referenced using the full path:

```yaml
resources:
  - id: deployment
    template:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        template:
          spec:
            containers:
              - name: app
                resources:
                  limits:
                    cpu: ${schema.spec.advanced.resourceLimits.cpu}
                    memory: ${schema.spec.advanced.resourceLimits.memory}
                env:
                  - name: LOG_LEVEL
                    value: ${schema.spec.advanced.logLevel}
```

## External References

External references (`externalRef`) declare dependencies on existing Kubernetes resources. In the deploy form, they render as resource pickers.

### Wiring: Three Things Must Line Up

The deploy form only renders a picker when **all three** of the following match:

1. **A schema sub-object** declares the `name`/`namespace` input fields.
2. **The `externalRef` resource's `metadata.name` and `metadata.namespace`** are CEL expressions that point at those schema fields and share the same parent path.
3. The resource's `id` matches the schema sub-object's key (recommended for clarity, and required for Secret passthrough classification in the catalog UI).

The canonical convention is to nest the schema sub-object under a key called `externalRef`, but the picker wiring itself only depends on rules 1 and 2.

```yaml
spec:
  schema:
    apiVersion: example.io/v1alpha1
    kind: AppWithDB
    spec:
      # 1) Schema fields that capture the reference coordinates.
      externalRef:
        database:
          name: string | default="" description="Existing PostgresCluster"
          namespace: string | default="" description="Namespace of the PostgresCluster"
  resources:
    # 2) externalRef resource whose metadata.name/namespace point back at the
    #    schema sub-object — this is what wires the picker.
    - id: database                                              # 3) matches schema key
      externalRef:
        apiVersion: db.knodex.io/v1alpha1
        kind: PostgresCluster
        metadata:
          name: ${schema.spec.externalRef.database.name}
          namespace: ${schema.spec.externalRef.database.namespace}
      readyWhen:
        - key: status.ready
          value: "true"

    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.metadata.name}
        spec:
          template:
            spec:
              containers:
                - env:
                    - name: DB_HOST
                      value: ${database.status.host}      # status reference resolved by Kro
```

### How It Works

1. The schema enricher walks each `externalRef` resource and extracts the schema paths used in its `metadata.name` and `metadata.namespace` CEL expressions.
2. It computes the **common parent path** of those two fields (e.g. `externalRef.database`).
3. The form property at that parent path gets `ExternalRefSelector` metadata attached, including the leaf field names (`name`, `namespace`) used for auto-fill.
4. At deploy time the picker queries the API for resources of the given `apiVersion`/`Kind`. When the user selects one, the form writes its `name` and `namespace` into the schema fields automatically.

Without the `metadata.name`/`metadata.namespace` CEL expressions in the resource block, **no picker is attached** — the schema fields render as plain text inputs.

### Namespace Filtering

The resource picker filters instances based on:
- The project's allowed destination namespaces
- The user's Casbin permissions for instances in those namespaces
- The `readyWhen` conditions (optionally, showing only ready resources)

### Multiple External References

An RGD can have multiple `externalRef` resources — give each its own schema sub-object so each gets its own picker:

```yaml
spec:
  schema:
    spec:
      externalRef:
        database:
          name: string | default=""
          namespace: string | default=""
        cache:
          name: string | default=""
          namespace: string | default=""
  resources:
    - id: database
      externalRef:
        apiVersion: db.knodex.io/v1alpha1
        kind: PostgresCluster
        metadata:
          name: ${schema.spec.externalRef.database.name}
          namespace: ${schema.spec.externalRef.database.namespace}
    - id: cache
      externalRef:
        apiVersion: cache.knodex.io/v1alpha1
        kind: RedisCluster
        metadata:
          name: ${schema.spec.externalRef.cache.name}
          namespace: ${schema.spec.externalRef.cache.namespace}
    - id: deployment
      template:
        # uses ${database.status.host} and ${cache.status.host}
```

### Nested References for Composite RGDs

External references can chain across RGDs. If RGD-A produces `KindA` and RGD-B has an `externalRef` to `KindA`, deploying RGD-B shows a picker for existing `KindA` instances.

### Combining with Conditional Resources

Nest the externalRef sub-object inside the same parent that owns the `enabled` toggle, so the picker hides and the resource skips together:

```yaml
spec:
  schema:
    spec:
      cache:
        enabled: boolean | default=false
        externalRef:
          redis:
            name: string | default=""
            namespace: string | default=""
  resources:
    - id: redis
      includeWhen:
        - ${schema.spec.cache.enabled == true}
      externalRef:
        apiVersion: cache.knodex.io/v1alpha1
        kind: RedisCluster
        metadata:
          name: ${schema.spec.cache.externalRef.redis.name}
          namespace: ${schema.spec.cache.externalRef.redis.namespace}
```

The cache tab's `enabled` checkbox hides the picker (and the rest of the `cache` fields) until ticked, and `includeWhen` skips the resource itself when the feature is off. See [Per-Feature Enabled Toggle](#per-feature-enabled-toggle).

## Secret Reference Descriptions

When an `externalRef` references a Kubernetes Secret, you can add a description to help users understand what the secret should contain.

### Adding Descriptions

Secret descriptions are provided through the schema field's structure:

```yaml
spec:
  schema:
    apiVersion: example.io/v1alpha1
    kind: MyApp
    spec:
      externalRef:
        dbSecret:
          name: string | default="" description="Existing Secret holding database credentials"
          namespace: string | default=""
  resources:
    - id: dbSecret
      externalRef:
        apiVersion: v1
        kind: Secret
        metadata:
          name: ${schema.spec.externalRef.dbSecret.name}
          namespace: ${schema.spec.externalRef.dbSecret.namespace}
      readyWhen:
        - key: data.password
          exists: true
```

### How It Renders

| Element | Display |
|---------|---------|
| Secret picker label | Derived from the schema field name (e.g., "DB Secret") |
| Required indicator | Shown if the field has no default |
| Namespace filter | Scoped to the project's destination namespaces |
| Secret list | Shows secrets the user has access to in allowed namespaces |

### Reference Types

| Reference Type | externalRef Kind | UI Element |
|---------------|-----------------|-----------|
| Custom Resource | Any custom Kind (e.g., `PostgresCluster`) | Resource picker with instance list |
| Secret | `Secret` (v1) | Secret picker with secret list |
| ConfigMap | `ConfigMap` (v1) | Resource picker with ConfigMap list |

### Best Practice

Always include `readyWhen` conditions on secret references to ensure the secret exists and has the expected keys before dependent resources are created.

## UI Rendering Behavior

### Field Rendering

| Schema Pattern | UI Behavior |
|---------------|-------------|
| `fieldName: string` | Required text input |
| `fieldName: string \| default="value"` | Optional text input, pre-filled |
| `fieldName: integer` | Required number input |
| `fieldName: integer \| default=3` | Optional number input, pre-filled |
| `fieldName: boolean` | Toggle, default `false` |
| `fieldName: boolean \| default=true` | Toggle, default `true` |
| Nested object | Collapsible field group |
| `spec.advanced.*` | Collapsed "Advanced" section |
| `externalRef` paired field | Resource/Secret picker |

### Default Value Handling

- Fields with defaults are not marked as required in the form
- Fields without defaults are marked as required
- Default values are pre-populated in the form
- Users can clear a default to explicitly set an empty value (for strings)

### Visibility Rules

- Fields under `spec.advanced` are hidden by default (collapsed section)
- Fields paired with `externalRef` resources show picker UI instead of plain text inputs
- Boolean fields always render as toggles, never as text inputs
- Nested objects render as grouped sections with a header derived from the field name
- An object with an `enabled: boolean` child renders its peer fields only when `enabled` is `true` (see [Per-Feature Enabled Toggle](#per-feature-enabled-toggle))

## Complete Example RGD

This example demonstrates most schema and UI features together:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: full-stack-app
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "Full-Stack Application"
    knodex.io/description: "Web app with database, cache, and monitoring"
    knodex.io/category: "applications"
    knodex.io/icon: "layers"
    knodex.io/extends-kind: "PostgresCluster,RedisCluster"
    knodex.io/property-order: '["appName","image","externalRef.database.name","externalRef.cache.name"]'
    knodex.io/deployment-modes: "gitops,hybrid"
spec:
  schema:
    apiVersion: apps.knodex.io/v1alpha1
    kind: FullStackApp
    spec:
      # Top-level required fields (shown first due to property-order)
      appName: string
      image: string

      # External reference fields (render as pickers — see "Wiring" above)
      externalRef:
        database:
          name: string | default=""
          namespace: string | default=""
        cache:
          name: string | default=""
          namespace: string | default=""

      # Optional top-level fields
      replicas: integer | default=2
      port: integer | default=8080

      # Advanced configuration (collapsed by default)
      advanced:
        logLevel: string | default="info"
        metricsEnabled: boolean | default=true
        healthCheck:
          path: string | default="/healthz"
          intervalSeconds: integer | default=30
        resources:
          cpuLimit: string | default="1"
          memoryLimit: string | default="1Gi"
          cpuRequest: string | default="250m"
          memoryRequest: string | default="256Mi"

  resources:
    # External dependencies
    - id: database
      externalRef:
        apiVersion: db.knodex.io/v1alpha1
        kind: PostgresCluster
        metadata:
          name: ${schema.spec.externalRef.database.name}
          namespace: ${schema.spec.externalRef.database.namespace}
      readyWhen:
        - key: status.ready
          value: "true"

    - id: cache
      externalRef:
        apiVersion: cache.knodex.io/v1alpha1
        kind: RedisCluster
        metadata:
          name: ${schema.spec.externalRef.cache.name}
          namespace: ${schema.spec.externalRef.cache.namespace}
      readyWhen:
        - key: status.ready
          value: "true"

    # Application deployment
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.appName}
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
                  env:
                    - name: DB_HOST
                      value: ${database.status.host}
                    - name: REDIS_HOST
                      value: ${cache.status.host}
                    - name: LOG_LEVEL
                      value: ${schema.spec.advanced.logLevel}
                  resources:
                    limits:
                      cpu: ${schema.spec.advanced.resources.cpuLimit}
                      memory: ${schema.spec.advanced.resources.memoryLimit}
                    requests:
                      cpu: ${schema.spec.advanced.resources.cpuRequest}
                      memory: ${schema.spec.advanced.resources.memoryRequest}
                  livenessProbe:
                    httpGet:
                      path: ${schema.spec.advanced.healthCheck.path}
                      port: ${schema.spec.port}
                    periodSeconds: ${schema.spec.advanced.healthCheck.intervalSeconds}

    # Service
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
            - port: 80
              targetPort: ${schema.spec.port}

    # Conditional monitoring
    - id: service-monitor
      includeWhen:
        - ${schema.spec.advanced.metricsEnabled == true}
      template:
        apiVersion: monitoring.coreos.com/v1
        kind: ServiceMonitor
        metadata:
          name: ${schema.spec.appName}
        spec:
          selector:
            matchLabels:
              app: ${schema.spec.appName}
          endpoints:
            - port: metrics
              interval: 30s
```
