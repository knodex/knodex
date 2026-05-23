---
title: Custom Icons
description: Configure custom icons for RGD catalog entries using built-in slugs, ConfigMap overrides, and annotation-based assignment.
sidebar_position: 8
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Custom Icons

Knodex assigns icons to RGD catalog entries based on resource kind annotations. You can use built-in icons, add custom icons via ConfigMaps, or override defaults.

## Icon Resolution Order

When displaying an icon for a catalog entry, Knodex resolves icons in this order:

1. **Built-in icons** - Matched by icon slug from the RGD annotation
2. **ConfigMap icons** - Custom icons loaded from labeled ConfigMaps (override built-ins)
3. **Fallback** - A generic Kubernetes icon if no match is found

## Annotation Usage

Assign an icon to an RGD using the `knodex.io/icon` annotation:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp
  annotations:
    knodex.io/catalog: "true"
    knodex.io/icon: "web-app"
```

The annotation value is an icon **slug** that maps to either a built-in or custom icon.

## Built-in Icon Slugs

| Slug | Description |
|------|-------------|
| `api-gateway` | API gateway / ingress |
| `application` | Generic application |
| `cache` | Caching layer (Redis, Memcached) |
| `certificate` | TLS certificates |
| `ci-cd` | CI/CD pipeline |
| `cloud` | Cloud provider |
| `cluster` | Kubernetes cluster |
| `config` | Configuration / ConfigMap |
| `container` | Container / Docker |
| `cronjob` | Scheduled job |
| `database` | Database (generic) |
| `dns` | DNS management |
| `function` | Serverless function |
| `gpu` | GPU workload |
| `helm` | Helm chart |
| `ingress` | Ingress controller |
| `job` | Batch job |
| `kafka` | Kafka / message queue |
| `load-balancer` | Load balancer |
| `monitoring` | Monitoring / observability |
| `namespace` | Kubernetes namespace |
| `network-policy` | Network policy |
| `node` | Kubernetes node |
| `pod` | Pod |
| `postgres` | PostgreSQL database |
| `queue` | Message queue (generic) |
| `redis` | Redis |
| `secret` | Secret |
| `service` | Kubernetes service |
| `storage` | Persistent storage |
| `web-app` | Web application |

## Adding Custom Icons via ConfigMap

Create a ConfigMap with the `knodex.io/icons` label to register custom icons:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: knodex-custom-icons
  namespace: knodex
  labels:
    knodex.io/icon-registry: "true"
data:
  # Each key is the icon slug, value is an SVG string
  my-service: |
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
    </svg>
  internal-tool: |
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">
      <circle cx="12" cy="12" r="10"/>
    </svg>
```

Then reference the custom slug in your RGD:

```yaml
annotations:
  knodex.io/icon: "my-service"
```

## Merge Order and Collision Behavior

When multiple ConfigMaps provide icons:

- **Custom icons take precedence** over built-in icons with the same slug
- If two ConfigMaps define the same slug, the one that is **alphabetically first** by ConfigMap name wins
- A warning is logged when a collision is detected

Example: If both `knodex-icons-team-a` and `knodex-icons-team-b` define the slug `database`, the icon from `knodex-icons-team-a` is used (alphabetical order), and a warning is logged.

## Verifying the Icon Registry

### Check Registered ConfigMaps

```bash
kubectl get configmaps -n knodex -l knodex.io/icons=true
```

### Check a Specific Icon via API

```bash
kubectl port-forward svc/knodex 8080:8080 -n knodex
curl http://localhost:8080/api/v1/icons/{slug}
```

There is no list endpoint; query individual icons by slug (e.g., `/api/v1/icons/web-app`).

## Slug Format Requirements

Icon slugs must follow these rules:

- Lowercase alphanumeric characters and hyphens only
- Must start with a letter
- Must not end with a hyphen
- Maximum 63 characters (Kubernetes label value constraint)

Valid: `web-app`, `postgres`, `my-custom-icon`
Invalid: `-web-app`, `Web_App`, `my--icon-`
