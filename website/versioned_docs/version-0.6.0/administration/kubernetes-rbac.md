---
title: Kubernetes RBAC
description: Configure Kubernetes ServiceAccount permissions, ClusterRoles, and RoleBindings required by the Knodex server.
sidebar_position: 5
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Kubernetes RBAC

Knodex requires specific Kubernetes RBAC permissions to watch and manage resources. The Helm chart creates these automatically, but this guide documents the full requirements for custom deployments and troubleshooting.

## ServiceAccount

The Knodex server runs under a dedicated ServiceAccount:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: knodex
  namespace: knodex
```

## ClusterRole

The server requires cluster-wide read access to KRO resources, namespaces, and workloads, plus write access for instance management:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: knodex
rules:
  # ResourceGraphDefinitions (read)
  - apiGroups: ["kro.run"]
    resources: ["resourcegraphdefinitions"]
    verbs: ["get", "list", "watch"]

  # Instances - dynamic resources created by KRO (full access)
  - apiGroups: ["kro.run"]
    resources: ["*"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Namespaces (list for project scoping)
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch"]

  # Workload status (read for instance monitoring)
  - apiGroups: [""]
    resources: ["pods", "services", "configmaps"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "replicasets"]
    verbs: ["get", "list", "watch"]

  # Secrets (read/write for secrets management)
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Knodex CRDs (projects, settings)
  - apiGroups: ["knodex.io"]
    resources: ["*"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

## ClusterRoleBinding

Bind the ClusterRole to the ServiceAccount:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: knodex
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: knodex
subjects:
  - kind: ServiceAccount
    name: knodex
    namespace: knodex
```

## Project Namespace Permissions

For managing resources within project namespaces, you may optionally create namespace-scoped Roles for tighter control:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: knodex
  namespace: alpha-apps
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["kro.run"]
    resources: ["*"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---

apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: knodex
  namespace: alpha-apps
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: knodex
subjects:
  - kind: ServiceAccount
    name: knodex
    namespace: knodex
```

## Casbin vs Kubernetes RBAC

Knodex uses two layers of authorization:

| Layer | Scope | Purpose |
|-------|-------|---------|
| **Kubernetes RBAC** | Server-to-API-server | Controls what the Knodex server pod can do in the cluster |
| **Casbin RBAC** | User-to-Knodex | Controls what individual users can do through the Knodex API |

The Kubernetes RBAC permissions define the maximum capability of the Knodex server. Casbin further restricts what each user can access within those boundaries. A user cannot exceed the server's Kubernetes permissions, even if their Casbin role allows it.

## Verification Commands

### Check ServiceAccount

```bash
kubectl get serviceaccount knodex -n knodex
```

### Check ClusterRole

```bash
kubectl get clusterrole knodex -o yaml
```

### Check ClusterRoleBinding

```bash
kubectl get clusterrolebinding knodex -o yaml
```

### Test Permissions

Use `kubectl auth can-i` to verify the ServiceAccount has the expected permissions:

```bash
# Check RGD access
kubectl auth can-i list resourcegraphdefinitions.kro.run \
  --as=system:serviceaccount:knodex:knodex

# Check secret access in a project namespace
kubectl auth can-i get secrets \
  --as=system:serviceaccount:knodex:knodex \
  -n alpha-apps

# Check instance creation
kubectl auth can-i create '*' \
  --as=system:serviceaccount:knodex:knodex \
  --subresource="" \
  -n alpha-apps
```

### Verify Secrets Access

```bash
kubectl auth can-i list secrets \
  --as=system:serviceaccount:knodex:knodex \
  -n knodex

kubectl auth can-i create secrets \
  --as=system:serviceaccount:knodex:knodex \
  -n alpha-apps
```

## Security Considerations

### Least Privilege

The default ClusterRole grants broad permissions for simplicity. In hardened environments, consider:

- Using namespace-scoped Roles instead of ClusterRoles where possible
- Restricting `secrets` access to specific namespaces
- Limiting KRO resource verbs to only what is needed

### Restricted Permissions Example

A more restrictive configuration that limits secret access to specific namespaces:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: knodex-restricted
rules:
  # Read-only cluster-wide access
  - apiGroups: ["kro.run"]
    resources: ["resourcegraphdefinitions"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["knodex.io"]
    resources: ["*"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

Then add namespace-scoped Roles for each project namespace that needs instance and secret management.

## Troubleshooting

### Forbidden Errors in Server Logs

If you see `403 Forbidden` errors in the Knodex server logs when accessing Kubernetes resources:

1. Verify the ServiceAccount exists: `kubectl get sa knodex -n knodex`
2. Verify the ClusterRoleBinding: `kubectl get clusterrolebinding knodex`
3. Test permissions: `kubectl auth can-i list pods --as=system:serviceaccount:knodex:knodex`

### Namespace Creation Issues

Knodex does not create namespaces. If a project references a namespace that does not exist, the namespace must be created separately:

```bash
kubectl create namespace alpha-apps
```
