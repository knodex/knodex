---
title: "Secrets Management"
linkTitle: "Secrets Management"
description: "Configure secrets support and RGD secret references"
weight: 7
product_tags:
  - enterprise
---

{{< product-tag enterprise >}}

# Secrets Management

> **Enterprise Feature:** Secrets management requires Knodex Enterprise. OSS builds return 402 Payment Required for secrets API endpoints.

Configure how knodex manages Kubernetes Secrets and how RGDs reference them.

## Overview

Knodex provides a full secrets management layer on top of Kubernetes Secrets:

- **Secrets API** — CRUD operations scoped to projects via Casbin authorization
- **Secret detection** — Automatic discovery of `externalRef` resources with `kind: Secret` in RGDs
- **Catalog integration** — Secrets tab on RGD detail pages showing required secrets and descriptions

## Kubernetes RBAC Requirements

The knodex server ServiceAccount requires cluster-wide permissions to manage secrets:

```yaml
# Included in the Helm chart ClusterRole
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

These permissions are automatically configured when installing via the Helm chart. For manual installations, add this rule to the knodex ClusterRole. See [Kubernetes RBAC](../kubernetes-rbac/) for the complete permissions reference.

## How Secrets Work

### Labeling and Scoping

Secrets created through knodex are labeled with project ownership:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
  namespace: kro-engineering
  labels:
    knodex.io/project: my-project
    knodex.io/managed-by: knodex
type: Opaque
data:
  username: ...
  password: ...
```

The API lists secrets using these label selectors, ensuring project isolation. Secrets without the `knodex.io/project` label are not visible in the knodex UI.

### Authorization

User access to secrets is controlled by **Casbin policies**, not Kubernetes RBAC. The `secrets` resource supports four actions:

| Action | Description | Policy Example |
|--------|-------------|----------------|
| `get` | List and view secrets | `p, proj:acme:developer, secrets, get, acme, allow` |
| `create` | Create new secrets | `p, proj:acme:developer, secrets, create, acme, allow` |
| `update` | Update secret data | `p, proj:acme:developer, secrets, update, acme, allow` |
| `delete` | Delete secrets | `p, proj:acme:admin, secrets, delete, acme, allow` |

Add these policies to your Project CRD roles or built-in role definitions. See [RBAC Setup](../rbac-setup/) for policy configuration.

### Size Limits

The API enforces size limits to prevent abuse:

| Limit | Value |
|-------|-------|
| Single value size | 256 KB |
| Total secret size | 512 KB |
| List page size | 100 (default), 500 (max) |

### Delete Safety

When a secret is deleted, knodex performs a best-effort scan of up to 500 instances to detect references. If any instance appears to reference the secret, warnings are returned in the API response. This scan has a 5-second timeout to avoid blocking.

## Writing RGDs That Reference Secrets

### ExternalRef Pattern

Use `externalRef` with `kind: Secret` to declare a secret dependency:

```yaml
spec:
  schema:
    apiVersion: v1alpha1
    kind: MyApp
    spec:
      externalRef:
        dbSecret:
          name: string | default="" description="Kubernetes Secret with database credentials"
          namespace: string | default="" description="Namespace of the Secret"

  resources:
    - id: dbSecret
      externalRef:
        apiVersion: v1
        kind: Secret
        metadata:
          name: ${schema.spec.externalRef.dbSecret.name}
          namespace: ${schema.spec.externalRef.dbSecret.namespace}

    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          template:
            spec:
              containers:
                - name: app
                  envFrom:
                    - secretRef:
                        name: ${dbSecret.metadata.name}
```

**Key points:**

1. The resource `id` (e.g., `dbSecret`) must match the schema path `externalRef.dbSecret`
2. Knodex detects the `kind: Secret` and classifies the reference type automatically
3. The `description` marker on the `name` sub-field appears in the catalog Secrets tab

### Secret Reference Types

Knodex classifies each secret reference based on the name/namespace expressions:

| Type | Pattern | Behavior |
|------|---------|----------|
| **user-provided** | Name and namespace use `${schema.spec.externalRef.<id>.name/namespace}` | User selects a secret at deploy time via dropdown |
| **fixed** | Literal strings (no `${...}` expressions) | Secret name is hardcoded in the RGD |
| **dynamic** | Non-passthrough CEL expressions | Secret name is computed at runtime |

### Adding Descriptions

Add `description` markers to the `name` sub-field so users understand what each secret should contain:

```yaml
externalRef:
  dbSecret:
    name: string | default="" description="Kubernetes Secret with PostgreSQL connection string"
    namespace: string | default="" description="Namespace of the database secret"
  tlsCert:
    name: string | default="" description="TLS certificate for HTTPS termination"
    namespace: string | default="" description="Namespace of the TLS secret"
```

These descriptions render in the catalog detail Secrets tab. See [Schema & UI — Secret Descriptions](../../catalog/schema-ui/#secret-reference-descriptions) for rendering details.

## Verification

### Check Secrets API

```bash
# Test the secrets API (requires auth token)
curl -H "Authorization: Bearer $TOKEN" \
  https://knodex.example.com/api/v1/secrets?project=my-project
```

### Check Kubernetes Permissions

```bash
# Verify the ServiceAccount can manage secrets cluster-wide
kubectl auth can-i create secrets \
  --as=system:serviceaccount:knodex:knodex \
  --all-namespaces
# Should return: yes
```

## Troubleshooting

### Secrets Page Shows "Access Denied"

**Cause:** User's Casbin role lacks `secrets:get` permission for the selected project.

**Solution:** Add the policy to the Project CRD or built-in role:

```csv
p, proj:my-project:developer, secrets, get, my-project, allow
```

### Secret Not Visible After Creation via kubectl

**Cause:** Secret is missing the required labels.

**Solution:** Add the knodex labels:

```bash
kubectl label secret my-secret \
  knodex.io/project=my-project \
  knodex.io/managed-by=knodex \
  -n kro-engineering
```

---

**Previous:** [Declarative Repositories](../declarative-repositories/) | **Next:** [Kubernetes RBAC](../kubernetes-rbac/)
