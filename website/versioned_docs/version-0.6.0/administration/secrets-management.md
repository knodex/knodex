---
title: Secrets Management
description: Configure enterprise secrets management with Kubernetes-native scoping, external references in RGDs, and Casbin-based authorization.
sidebar_position: 6
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["enterprise"]} />

# Secrets Management

Embedding sensitive data (passwords, API keys, connection strings) directly in RGD instance specs is insecure — the values are stored in plain text in Kubernetes resources and may be committed to Git in GitOps workflows. To address this, Knodex provides a dedicated Secrets UI where users can create and manage Kubernetes Secrets separately, then reference them from RGD instances via the `externalRef` mechanism. This keeps sensitive data out of instance specs entirely.

Secrets are scoped to projects and namespaces, authorized through Casbin, and referenced at deploy time through resource picker dropdowns.

## Kubernetes RBAC Requirements

The Knodex ServiceAccount must have read/write access to Secrets in project namespaces. See [Kubernetes RBAC](kubernetes-rbac) for the full ClusterRole configuration.

Minimum required permissions:

```yaml
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

## How Secrets Work

### Labeling and Scoping

Secrets managed by Knodex are labeled for discovery and scoped to a project:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: database-credentials
  namespace: alpha-apps
  labels:
    knodex.io/managed-by: knodex
    knodex.io/project: "alpha"
    knodex.io/secret-type: "generic"
  annotations:
    knodex.io/description: "PostgreSQL credentials for the alpha database"
type: Opaque
data:
  username: YWRtaW4=
  password: cGFzc3dvcmQ=
```

The labels serve as the discovery mechanism. Only secrets with `knodex.io/managed-by: knodex` are visible in the Knodex UI.

### Authorization

Secret access is enforced through Casbin policies:

| Action | Casbin Resource | Casbin Action |
|--------|----------------|---------------|
| List secrets | `secrets/<project>/<namespace>` | `get` |
| View secret | `secrets/<project>/<namespace>` | `get` |
| Create secret | `secrets/<project>/<namespace>` | `create` |
| Update secret | `secrets/<project>/<namespace>` | `update` |
| Delete secret | `secrets/<project>/<namespace>` | `delete` |

A role policy granting full secret access:

```
secrets/*, *, allow
```

A read-only policy:

```
secrets/*, get, allow
```

### Size Limits

Kubernetes enforces a 1 MiB size limit per Secret. Knodex does not impose additional limits but validates that secret data does not exceed this threshold.

### Delete Safety

Deleting a secret through Knodex removes the Kubernetes Secret object. If the secret is referenced by running instances, those instances may fail. Knodex displays a warning before deletion if references are detected.

## Writing RGDs with External References

RGDs can reference secrets using the `externalRef` pattern, allowing instance deployers to select secrets at deployment time.

### Example RGD with Secret Reference

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp
  annotations:
    knodex.io/catalog: "true"
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebApp
    spec:
      name:
        type: string
      databaseSecret:
        type: string
        description: "Name of the Kubernetes secret containing database credentials"
        x-knodex-externalRef:
          kind: Secret
          namespace: "from-context"
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.name}
        spec:
          template:
            spec:
              containers:
                - name: app
                  envFrom:
                    - secretRef:
                        name: ${schema.spec.databaseSecret}
```

### Reference Types

| Reference Type | Description | Use Case |
|---------------|-------------|----------|
| `Secret` | Kubernetes Secret | Database credentials, API keys |
| `ConfigMap` | Kubernetes ConfigMap | Application configuration |

### Adding Descriptions

Use the `description` field in the schema to help deployers understand what the secret should contain:

```yaml
databaseSecret:
  type: string
  description: "PostgreSQL secret with keys: username, password, host, port"
  x-knodex-externalRef:
    kind: Secret
```

## Verification

### Test API Access

Port-forward to the Knodex server and test secret listing:

```bash
kubectl port-forward svc/knodex 8080:8080 -n knodex

# List secrets for a project (requires authentication)
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/projects/alpha/secrets
```

### Verify Kubernetes Permissions

```bash
# Check that Knodex can list secrets in a project namespace
kubectl auth can-i list secrets \
  --as=system:serviceaccount:knodex:knodex \
  -n alpha-apps

# Check that Knodex can create secrets
kubectl auth can-i create secrets \
  --as=system:serviceaccount:knodex:knodex \
  -n alpha-apps
```

## Troubleshooting

### Access Denied When Listing Secrets

If users see "access denied" when viewing secrets:

1. Check the user's Casbin role includes `secrets/*, get, allow`
2. Verify the role's `destinations` include the target namespace
3. Check server logs for Casbin enforcement details

### Secrets Not Appearing in the UI

If secrets exist in Kubernetes but are not visible in Knodex:

1. Verify the secret has the `knodex.io/managed-by: knodex` label
2. Verify the secret has the `knodex.io/project` label matching the project name
3. Verify the secret is in a namespace included in the project's destinations
