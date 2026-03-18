---
title: "Browsing the Catalog"
linkTitle: "Browsing Catalog"
description: "Discover, search, and explore available ResourceGraphDefinitions (RGDs) in knodex"
weight: 1
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Browsing the Catalog

Discover, search, and explore available ResourceGraphDefinitions (RGDs) in knodex.

## Overview

The RGD Catalog is a searchable collection of deployment templates:

- **Web Applications**: Nginx, Apache, Node.js, Python, Go services
- **Databases**: PostgreSQL, MySQL, MongoDB, Redis
- **Message Queues**: RabbitMQ, Kafka, NATS
- **Monitoring**: Prometheus, Grafana, Jaeger
- **Infrastructure**: Ingress, Service Mesh, Storage

## Accessing the Catalog

### Via Sidebar

1. Click **Catalog** in the left sidebar
2. View grid of available RGDs
3. Use filters and search to narrow results

### Via Views

1. Navigate to a **View** in the sidebar
2. Click **View All** to open catalog

## Catalog Layout

### Card View (Default)

Each RGD displayed as a card with:

| Element           | Description                               |
| ----------------- | ----------------------------------------- |
| **Icon**          | Visual indicator of RGD type              |
| **Name**          | RGD display name                          |
| **Description**   | Brief summary (1-2 sentences)             |
| **Version**       | RGD schema version                        |
| **Labels**        | Tags (webapp, database, monitoring, etc.) |
| **Deploy Button** | Quick deploy action                       |

**Example Card:**

```
┌─────────────────────────────────────┐
│  🌐  Nginx Web Application          │
│                                     │
│  High-performance web server and    │
│  reverse proxy for serving static   │
│  content and applications.          │
│                                     │
│  Version: v1.2.0                    │
│  Labels: [webapp] [proxy]           │
│                                     │
│  [View Details]  [Deploy]          │
└─────────────────────────────────────┘
```

### List View

Switch to list view for compact display:

| Name          | Type     | Version | Updated    | Actions            |
| ------------- | -------- | ------- | ---------- | ------------------ |
| Nginx Web App | WebApp   | v1.2.0  | 2 days ago | [Details] [Deploy] |
| PostgreSQL DB | Database | v2.1.5  | 1 week ago | [Details] [Deploy] |

**Toggle Views:**

- Card view: Click grid icon (☷) in toolbar
- List view: Click list icon (☰) in toolbar

## Search and Filtering

### Search Bar

**Location:** Top of catalog page

**Search by:**

- RGD name
- Description keywords
- Labels/tags
- Version

**Examples:**

| Query                 | Results                                      |
| --------------------- | -------------------------------------------- |
| `nginx`               | All RGDs with "nginx" in name or description |
| `webapp`              | All web application RGDs                     |
| `database postgresql` | PostgreSQL database RGDs                     |
| `v1.2`                | RGDs with version matching `v1.2.*`          |

**Keyboard Shortcut:** Press `/` to focus search

### Filter Panel

**Location:** Left sidebar of catalog page

**Filter Categories:**

#### 1. Type Filter

Filter by RGD category:

- [ ] Web Applications (12)
- [ ] Databases (8)
- [ ] Message Queues (5)
- [ ] Monitoring (7)
- [ ] Infrastructure (4)

#### 2. Labels Filter

Filter by tags:

- [ ] webapp (12)
- [ ] database (8)
- [ ] proxy (4)
- [ ] monitoring (7)
- [ ] storage (6)

#### 3. Source Filter

Filter by RGD source:

- [ ] Organization RGDs (catalog specific to your org)
- [ ] Shared RGDs (available to all organizations)
- [ ] Repository: provops/kro-rgd-catalog (from GitHub)

#### 4. Version Filter

- [ ] Latest versions only
- [ ] All versions

### Combining Filters

Filters are cumulative (AND logic):

**Example:**

- Type: "Databases" ✓
- Labels: "postgres" ✓
- Source: "Shared RGDs" ✓

**Result:** Only PostgreSQL database RGDs from shared catalog

## Viewing RGD Details

### Click RGD Card

1. Click on any RGD card or name
2. Opens detailed view with tabs:
   - **Overview**
   - **Schema**
   - **Add-ons** (parent RGDs only)
   - **Examples**
   - **Deployments**

### Overview Tab

**RGD Information:**

| Field           | Example                                       |
| --------------- | --------------------------------------------- |
| **Name**        | Nginx Web Application                         |
| **API Version** | `kro.run/v1alpha1`                            |
| **Kind**        | `WebApplication`                              |
| **Extends**     | `SimpleAKSCluster` (links to parent RGD)      |
| **Description** | High-performance web server and reverse proxy |
| **Version**     | `v1.2.0`                                      |
| **Created**     | 2024-01-15                                    |
| **Updated**     | 2024-01-20                                    |
| **Namespace**   | `default` (or organization namespace)         |

The **Extends** row appears only for RGDs that have the `knodex.io/extends-kind` annotation. Each parent Kind links to its catalog detail page.

**Quick Actions:**

- **Deploy** - Open deployment form
- **Copy YAML** - Copy RGD spec to clipboard
- **Export** - Download RGD YAML file

### Schema Tab

**Interactive Schema Viewer:**

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: nginx-webapp
spec:
  parameters:
    - name: name
      type: string
      description: "Application name"
      required: true

    - name: replicas
      type: integer
      description: "Number of replicas"
      default: 2
      minimum: 1
      maximum: 10

    - name: image
      type: string
      description: "Container image"
      default: "nginx:latest"

    - name: port
      type: integer
      description: "Container port"
      default: 80
```

**Schema Features:**

- Syntax highlighting
- Collapsible sections
- Copy button for each section
- Parameter descriptions and constraints

### Examples Tab

**Pre-configured Templates:**

#### Example 1: Development Deployment

```yaml
name: my-webapp-dev
replicas: 1
image: nginx:1.25
port: 80
resources:
  requests:
    cpu: 100m
    memory: 128Mi
```

[Copy] [Deploy with this config]

#### Example 2: Production Deployment

```yaml
name: my-webapp-prod
replicas: 5
image: nginx:1.25
port: 80
resources:
  requests:
    cpu: 500m
    memory: 1Gi
  limits:
    cpu: 2000m
    memory: 4Gi
ingress:
  enabled: true
  host: webapp.example.com
```

[Copy] [Deploy with this config]

### Add-ons Tab

The **Add-ons** tab appears on parent RGDs when other RGDs declare `knodex.io/extends-kind` pointing to this RGD's Kind. It shows a count badge (e.g., "Add-ons (3)") and lists all extending RGDs as cards.

Each add-on card displays:

| Element         | Description                                       |
| --------------- | ------------------------------------------------- |
| **Name**        | Add-on RGD display name                           |
| **Description** | Brief summary                                     |
| **Category**    | Category badge                                    |
| **View Button** | Navigate to the add-on's catalog detail page      |

**Use Case:** Discover monitoring, logging, security, or networking add-ons that can be deployed on top of an existing parent instance.

See [Annotations & Labels](../../catalog/annotations-and-labels/) for how to configure the `knodex.io/extends-kind` annotation.

### Deployments Tab

**Active Instances:**

List of instances deployed using this RGD:

| Instance Name   | Namespace       | Status  | Created    | Owner             |
| --------------- | --------------- | ------- | ---------- | ----------------- |
| my-webapp-prod  | kro-engineering | Running | 2 days ago | alice@example.com |
| api-service-dev | kro-engineering | Running | 1 week ago | bob@example.com   |

**Actions:**

- Click instance name to view details
- Deploy similar instance (uses same RGD)

## Understanding RGD Parameters

### Required vs Optional

Parameters marked as **required** must be provided:

```yaml
- name: name
  type: string
  required: true # ← Must provide value
```

Optional parameters have defaults:

```yaml
- name: replicas
  type: integer
  default: 2 # ← Used if not provided
```

### Parameter Types

| Type      | Example              | Description          |
| --------- | -------------------- | -------------------- |
| `string`  | `"my-app"`           | Text value           |
| `integer` | `3`                  | Whole number         |
| `boolean` | `true` / `false`     | True/false flag      |
| `array`   | `["item1", "item2"]` | List of values       |
| `object`  | `{key: "value"}`     | Nested configuration |

### Parameter Constraints

**Minimum/Maximum:**

```yaml
- name: replicas
  type: integer
  minimum: 1 # ← Cannot be less than 1
  maximum: 10 # ← Cannot be more than 10
```

**Pattern Matching:**

```yaml
- name: name
  type: string
  pattern: "^[a-z0-9-]+$" # ← Only lowercase, numbers, hyphens
```

**Enum (Limited Choices):**

```yaml
- name: tier
  type: string
  enum: # ← Must be one of these values
    - "free"
    - "standard"
    - "premium"
```

## Comparing RGDs

### Side-by-Side Comparison

Compare multiple RGDs:

1. Select RGDs using checkboxes
2. Click **Compare** button (top right)
3. View comparison table:

| Feature               | Nginx Web App       | Apache Web App      |
| --------------------- | ------------------- | ------------------- |
| **Version**           | v1.2.0              | v2.0.1              |
| **Parameters**        | 8                   | 12                  |
| **Default Replicas**  | 2                   | 3                   |
| **Resource Requests** | 100m CPU, 128Mi RAM | 250m CPU, 256Mi RAM |
| **Ingress Support**   | ✅ Yes              | ✅ Yes              |
| **Auto-scaling**      | ❌ No               | ✅ Yes              |

**Use Case:** Deciding which RGD best fits requirements

## Bookmarking Favorite RGDs

### Add to Favorites

1. Click **⭐ Star** icon on RGD card
2. RGD added to "Favorites" filter

### View Favorites

1. Open catalog
2. Click **Starred** filter in sidebar
3. See only bookmarked RGDs

**Use Case:** Quick access to frequently used RGDs

## RGD Sources

### Organization RGDs

RGDs specific to your organization (not visible to other orgs).

**Indicator:** Badge showing organization name

**Example:** `[Engineering Org]`

### Shared RGDs

Available to all organizations in cluster.

**Indicator:** `[Shared]` badge

**Use Case:** Common templates (Nginx, PostgreSQL, Redis)

### Repository RGDs

Synced from connected GitHub repositories.

**Indicator:** Shows repository source

**Example:** `[provops/kro-rgd-catalog]`

**Auto-Updated:** Synced every 5 minutes from GitHub

## Advanced Search

### Search Syntax

**AND operator** (implicit):

```
webapp nginx  →  RGDs containing both "webapp" AND "nginx"
```

**OR operator:**

```
nginx | apache  →  RGDs containing "nginx" OR "apache"
```

**Exclude:**

```
database -mongo  →  Database RGDs excluding MongoDB
```

**Exact phrase:**

```
"web application"  →  Exact phrase match
```

## Keyboard Shortcuts

| Shortcut | Action                       |
| -------- | ---------------------------- |
| `/`      | Focus search bar             |
| `Esc`    | Clear search / close filters |
| `↑` `↓`  | Navigate RGD cards           |
| `Enter`  | Open selected RGD details    |
| `Ctrl+K` | Open command palette         |

## Tips and Tricks

### Quick Deploy

Deploy without opening details:

1. Hover over RGD card
2. Click **Deploy** button
3. Opens deployment form directly

### Recent RGDs

See recently viewed RGDs:

1. Click **History** in catalog toolbar
2. View last 10 RGDs you accessed

### Export Catalog

Download catalog as JSON or CSV:

1. Click **Export** button (top right)
2. Choose format (JSON, CSV, YAML)
3. Save file

**Use Case:** Offline reference, documentation, inventory

---

**Next:** [Deploying Instances](../deploying-instances/) →
