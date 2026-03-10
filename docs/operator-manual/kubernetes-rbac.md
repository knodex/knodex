---
title: "Kubernetes RBAC"
linkTitle: "Kubernetes RBAC"
description: "ServiceAccount, ClusterRole, and permission requirements for knodex"
weight: 5
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Kubernetes RBAC

ServiceAccount, ClusterRole, and permission requirements for knodex.

## Overview

knodex requires Kubernetes RBAC permissions to:

- **Read ResourceGraphDefinitions (RGDs)** across all namespaces
- **Create/Read/Delete RGD Instances** in project namespaces
- **Manage Secrets** for storing projects and repositories
- **Create Namespaces** for projects

## ServiceAccount

The Helm chart automatically creates a ServiceAccount:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: knodex
  namespace: knodex
```

## Required Permissions

### Cluster-Level Permissions

knodex requires ClusterRole for reading RGDs across all namespaces:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: knodex
rules:
  # Read ResourceGraphDefinitions (RGDs)
  - apiGroups: ["kro.run"]
    resources: ["resourcegraphdefinitions"]
    verbs: ["get", "list", "watch"]

  # Manage RGD instances
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]

  # Manage namespaces for projects
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "create", "delete"]

  # Read pods and services for API discovery and diagnostics
  - apiGroups: [""]
    resources: ["pods", "services", "configmaps", "secrets"]
    verbs: ["get", "list"]

  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets"]
    verbs: ["get", "list"]

  # knodex secrets management
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

### ClusterRoleBinding

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

For each project namespace, knodex needs full control:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: knodex-project
  namespace: kro-engineering # Created per project
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: knodex-project
  namespace: kro-engineering
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: knodex-project
subjects:
  - kind: ServiceAccount
    name: knodex
    namespace: knodex
```

**User authorization** is handled by **Casbin policies** defined in Project CRD `.spec.roles`, not Kubernetes RBAC. See [RBAC Configuration Guide](../../user-guide/rbac-configuration/) for details on configuring user permissions.
{{< /alert >}}

## Verification

### Check ServiceAccount

```bash
kubectl get serviceaccount knodex -n knodex
```

### Check ClusterRole

```bash
kubectl get clusterrole knodex
kubectl describe clusterrole knodex
```

### Check ClusterRoleBinding

```bash
kubectl get clusterrolebinding knodex
```

### Test Permissions

```bash
# Test as ServiceAccount
kubectl auth can-i get resourcegraphdefinitions \
  --as=system:serviceaccount:knodex:knodex \
  --all-namespaces

# Should return: yes
```

### Verify Secrets

```bash
# List project secrets
kubectl get secrets -n knodex -l knodex.io/secret-type=project

# List repository secrets
kubectl get secrets -n knodex -l knodex.io/secret-type=repository

# List user configs
kubectl get configmaps -n knodex -l knodex.io/config-type=user
```

## Security Considerations

### Principle of Least Privilege

While knodex requires broad permissions to manage RGDs, you can restrict:

1. **Namespace Access:** Limit to specific namespaces using Role instead of ClusterRole
2. **Resource Types:** Restrict to specific RGD types
3. **Verbs:** Use read-only for sensitive resources

### Example: Restricted Permissions

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: knodex-restricted
rules:
  # Read-only RGDs
  - apiGroups: ["kro.run"]
    resources: ["resourcegraphdefinitions"]
    verbs: ["get", "list", "watch"]

  # Manage only specific instance types
  - apiGroups: ["example.com"]
    resources: ["webapplications", "databases"]
    verbs: ["get", "list", "watch", "create", "delete"]

  # No access to secrets
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: [] # Explicitly deny
```

## Troubleshooting

### "Forbidden: User Cannot List ResourceGraphDefinitions"

**Cause:** Missing ClusterRole permissions

**Solution:**

```bash
# Check ClusterRoleBinding
kubectl get clusterrolebinding knodex

# Re-apply RBAC
kubectl apply -f clusterrole.yaml
kubectl apply -f clusterrolebinding.yaml

# Restart server
kubectl rollout restart deployment knodex-server -n knodex
```

### "Cannot Create Namespace for Project"

**Cause:** ServiceAccount lacks namespace creation permission

**Solution:**

```yaml
# Add to ClusterRole
rules:
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["create", "delete"]
```

---

**Next:** [Troubleshooting Guide](../troubleshooting/) →
