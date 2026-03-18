---
title: "Annotations & Labels"
linkTitle: "Annotations & Labels"
description: "RGD annotations and labels for catalog discovery, deployment modes, and scoping"
weight: 1
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Annotations & Labels

Annotations and labels control how RGDs appear in the catalog, how users can deploy them, and who can see them.

- **Annotations** store catalog metadata and configuration (discovery, deployment modes)
- **Labels** control visibility scoping (organization, project membership)

## Annotations

### Required Annotation

| Annotation          | Value    | Description                                  |
| ------------------- | -------- | -------------------------------------------- |
| `knodex.io/catalog` | `"true"` | **Required** - Gateway to catalog visibility |

Without this annotation, the RGD is invisible to everyone in the catalog.

```yaml
metadata:
  annotations:
    knodex.io/catalog: "true" # Must be exactly "true" (string)
```

### Metadata Annotations

Enhance how your RGD appears in the catalog UI:

| Annotation              | Description                                 | Example                                 |
| ----------------------- | ------------------------------------------- | --------------------------------------- |
| `knodex.io/title`       | Display title (defaults to `metadata.name`) | `"Prometheus Monitoring Stack"`         |
| `knodex.io/description` | Human-readable description shown in UI      | `"PostgreSQL cluster with replication"` |
| `knodex.io/tags`        | Comma-separated tags for filtering          | `"database,postgres,ha"`                |
| `knodex.io/category`    | Category with icon (see table below)        | `"database"`, `"monitoring"`            |
| `knodex.io/version`     | Version of the RGD                          | `"1.0.0"`                               |

The `knodex.io/title` annotation provides a human-readable display name for the catalog. When set, the title appears in place of the Kubernetes resource name on catalog cards, detail views, and the deploy page. The Kubernetes name is shown as a subtitle or tooltip for reference. Search also matches against the title. If the annotation is absent or empty, the `metadata.name` is used as the title (backward compatible).

### Supported Categories and Icons

The `knodex.io/category` annotation determines both the category label and the icon displayed in the catalog:

| Category Value  | Icon          | Description                           |
| --------------- | ------------- | ------------------------------------- |
| `database`      | Database      | Database systems (PostgreSQL, MySQL)  |
| `storage`       | HardDrive     | Storage solutions (volumes, backups)  |
| `networking`    | Network       | Network resources (ingress, services) |
| `network`       | Network       | Alias for `networking`                |
| `compute`       | Server        | Compute workloads (deployments, jobs) |
| `messaging`     | MessageSquare | Message queues (Kafka, RabbitMQ)      |
| `monitoring`    | Activity      | Observability (Prometheus, Grafana)   |
| `security`      | Shield        | Security tools (cert-manager, vault)  |
| `application`   | Package       | Application stacks                    |
| `app`           | Package       | Alias for `application`               |
| `cloud`         | Cloud         | Cloud provider resources              |
| `auth`          | Lock          | Authentication/authorization          |
| `workflow`      | Workflow      | CI/CD and workflow automation         |
| _(other/empty)_ | Box           | Default fallback icon                 |

{{< alert title="Note" >}}
Category values are case-insensitive. `Database`, `DATABASE`, and `database` are equivalent.
{{< /alert >}}

### Extends-Kind Annotation

Declare that an RGD extends (is an add-on for) a parent RGD Kind.

| Annotation                | Values                          | Description                                   |
| ------------------------- | ------------------------------- | --------------------------------------------- |
| `knodex.io/extends-kind`  | Comma-separated Kind names      | Parent RGD Kinds this RGD extends             |

```yaml
metadata:
  annotations:
    knodex.io/catalog: "true"
    knodex.io/extends-kind: "SimpleAKSCluster"
```

When set:

- The **catalog detail page** for this RGD shows an "Extends" row in the Overview tab, linking to the parent RGD(s)
- The **parent RGD's detail page** gains an "Add-ons (N)" tab listing all child RGDs that extend its Kind
- The **instance detail page** for parent instances shows a "Deploy on this instance" section with one-click add-on deployment

#### Multiple Parents

An RGD can extend multiple parent Kinds:

```yaml
knodex.io/extends-kind: "SimpleAKSCluster,SimpleEKSCluster"
```

#### Parsing Rules

- **Comma-separated**: Multiple Kinds separated by commas
- **Whitespace-tolerant**: `"KindA, KindB"` works the same as `"KindA,KindB"`
- **Case-sensitive**: Kind names must match exactly (e.g., `SimpleAKSCluster`, not `simpleakscluster`)

### Deployment Mode Annotations

Control which deployment modes are available for an RGD.

| Annotation                   | Values                                        | Description                        |
| ---------------------------- | --------------------------------------------- | ---------------------------------- |
| `knodex.io/deployment-modes` | Comma-separated: `direct`, `gitops`, `hybrid` | Restricts allowed deployment modes |

#### Deployment Modes

| Mode     | Description                                            |
| -------- | ------------------------------------------------------ |
| `direct` | Deploy directly to the cluster via Kubernetes API      |
| `gitops` | Generate manifests and commit to a Git repository      |
| `hybrid` | Deploy to cluster AND commit to Git for reconciliation |

**GitOps Only (Production RGDs):**

```yaml
metadata:
  annotations:
    knodex.io/catalog: "true"
    knodex.io/deployment-modes: "gitops"
```

When only one mode is allowed, the UI:

- Auto-selects that mode
- Disables the mode selector
- Shows an info banner: "This RGD restricts deployment to: GitOps only"

#### Default Behavior

| Annotation State   | Behavior                 |
| ------------------ | ------------------------ |
| Missing annotation | All modes allowed        |
| Empty value `""`   | All modes allowed        |
| Invalid values     | Ignored with warning log |

{{< alert title="Backward Compatibility" >}}
If `knodex.io/deployment-modes` is missing or empty, all deployment modes are allowed. This ensures existing RGDs continue to work without modification.
{{< /alert >}}

#### Parsing Rules

- **Case-insensitive**: `GITOPS`, `GitOps`, and `gitops` are equivalent
- **Whitespace-tolerant**: `"direct, gitops"` works the same as `"direct,gitops"`
- **Invalid values ignored**: Unknown modes are logged as warnings but don't cause errors
- **Deduplication**: Duplicate values are removed

## Labels

### Organization Label

{{< product-tag enterprise >}}

Scope an RGD to a specific organization in multi-tenant deployments.

| Label                    | Description                              | Example  |
| ------------------------ | ---------------------------------------- | -------- |
| `knodex.io/organization` | Restricts visibility to one organization | `"orgA"` |

```yaml
metadata:
  labels:
    knodex.io/organization: orgA # Only visible to orgA
  annotations:
    knodex.io/catalog: "true"
```

When set, the RGD is visible only to the matching organization. RGDs without this label are shared across all organizations.

{{< alert title="Important" >}}
`knodex.io/organization` must be a **label**, not an annotation. The server reads it from `metadata.labels` only.
{{< /alert >}}

See [Organizations](../../enterprise/organizations/) for the full multi-tenant configuration guide.

### Project Label

Restrict RGD visibility to members of a specific project.

| Label               | Description                             | Example                |
| ------------------- | --------------------------------------- | ---------------------- |
| `knodex.io/project` | Restricts visibility to project members | `"proj-payments-team"` |

```yaml
metadata:
  labels:
    knodex.io/project: proj-payments-team # Only project members can see this
  annotations:
    knodex.io/catalog: "true"
```

The label value must match the project namespace name exactly (e.g., `proj-alpha-team`), not the display name.

See [Project Scoping](../project-scoping/) for detailed visibility rules.

## Complete Examples

### Standard RGD

An RGD with all metadata annotations, organization scoping, and project visibility:

```yaml
metadata:
  name: postgres-ha-cluster
  labels:
    knodex.io/organization: orgA # Enterprise: restricts to org
    knodex.io/project: proj-platform-team # Restricts to project members
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "PostgreSQL HA Cluster"
    knodex.io/description: "Production-ready PostgreSQL with streaming replication"
    knodex.io/tags: "database,postgres,ha,production"
    knodex.io/category: "database"
    knodex.io/version: "2.1.0"
    knodex.io/deployment-modes: "gitops"
```

### Add-on RGD (Extends a Parent)

An RGD that extends a parent Kind, appearing as an add-on in the parent's catalog detail:

```yaml
metadata:
  name: aks-monitoring-addon
  annotations:
    knodex.io/catalog: "true"
    knodex.io/title: "AKS Monitoring Add-on"
    knodex.io/description: "Deploys Prometheus + Grafana monitoring stack on an AKS cluster"
    knodex.io/tags: "monitoring,prometheus,grafana,aks"
    knodex.io/category: "observability"
    knodex.io/extends-kind: "SimpleAKSCluster"
```

## Reference

### Annotation Summary

| Annotation                   | Required | Purpose                        | Availability |
| ---------------------------- | -------- | ------------------------------ | ------------ |
| `knodex.io/catalog`          | Yes      | Enables catalog visibility     | All          |
| `knodex.io/title`            | No       | Display title (fallback: name) | All          |
| `knodex.io/description`      | No       | Human-readable description     | All          |
| `knodex.io/tags`             | No       | Searchable/filterable tags     | All          |
| `knodex.io/category`         | No       | Category grouping with icon    | All          |
| `knodex.io/version`          | No       | RGD version                    | All          |
| `knodex.io/extends-kind`     | No       | Declares parent RGD Kind(s)    | All          |
| `knodex.io/deployment-modes` | No       | Restricts deployment modes     | All          |

### Label Summary

| Label                    | Required | Purpose                      | Availability |
| ------------------------ | -------- | ---------------------------- | ------------ |
| `knodex.io/organization` | No       | Restricts to organization    | Enterprise   |
| `knodex.io/project`      | No       | Restricts to project members | All          |

### Visibility Filter Chain

Filters apply in this order:

1. **Catalog flag** - `knodex.io/catalog: "true"` required
2. **Organization filter** - Match `knodex.io/organization` label (Enterprise only)
3. **Project filter** - Match `knodex.io/project` label

An RGD must pass all applicable filters to appear in a user's catalog.

---

**Next:** [Schema & UI](../schema-ui/) →
