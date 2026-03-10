---
title: "ConstraintTemplate Development"
linkTitle: "ConstraintTemplate Development"
description: "Create OPA Gatekeeper ConstraintTemplates that integrate with knodex compliance monitoring"
weight: 8
product_tags:
  - enterprise
---

{{< product-tag enterprise >}}

# ConstraintTemplate Development Guide

Create OPA Gatekeeper ConstraintTemplates that are ingested into the knodex compliance dashboard for monitoring and enforcement management.

## Overview

ConstraintTemplates define policy logic using Rego. When properly annotated, they appear in the knodex Compliance dashboard where operators can monitor violations and manage enforcement actions.

**Key Concepts:**

- **Compliance Annotation**: Gateway to make a ConstraintTemplate visible in knodex
- **Description Annotation**: Standard Kubernetes description annotation for human-readable explanation
- **Enforcement Actions**: Control policy behavior (deny, warn, dryrun)
- **Match Rules**: Define which Kubernetes resources the policy applies to

## Prerequisites

- OPA Gatekeeper installed in your Kubernetes cluster ([installation guide](https://open-policy-agent.github.io/gatekeeper/website/docs/install/))
- knodex Enterprise license enabled
- ClusterRole with Gatekeeper permissions (see [Kubernetes RBAC](../operator-manual/kubernetes-rbac/))
- Basic understanding of [Rego](https://www.openpolicyagent.org/docs/latest/policy-language/) (OPA's policy language)

## ConstraintTemplate Structure

A knodex-compatible ConstraintTemplate has the following structure:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8srequiredlabels           # Template name (lowercase)
  annotations:
    # Required for knodex visibility
    knodex.io/compliance: "true"

    # Standard Kubernetes description annotation (optional)
    description: "Requires specified labels on resources"
spec:
  crd:
    spec:
      names:
        kind: K8sRequiredLabels     # Constraint kind (PascalCase)
      validation:
        openAPIV3Schema:
          type: object
          properties:
            # Parameter schema for constraints
            labels:
              type: array
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8srequiredlabels

        # Rego policy logic here
        violation[{"msg": msg}] {
          # ... policy implementation
        }
```

## Compliance Annotations

### Required Annotation

| Annotation | Value | Description |
|------------|-------|-------------|
| `knodex.io/compliance` | `"true"` | **Required** - Gateway to compliance visibility |

Without this annotation, the ConstraintTemplate is invisible in the knodex Compliance dashboard.

### Optional Metadata Annotation

| Annotation | Description | Example |
|------------|-------------|---------|
| `description` | Standard Kubernetes description shown in UI | `"Requires all pods to have specified labels"` |

## Complete Examples

### Example 1: Required Labels Policy

Enforce that specific labels exist on resources:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8srequiredlabels
  annotations:
    knodex.io/compliance: "true"
    description: "Requires specified labels on Kubernetes resources"
spec:
  crd:
    spec:
      names:
        kind: K8sRequiredLabels
      validation:
        openAPIV3Schema:
          type: object
          properties:
            labels:
              type: array
              description: "List of required label keys"
              items:
                type: string
            message:
              type: string
              description: "Custom violation message"
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8srequiredlabels

        violation[{"msg": msg}] {
          provided := {label | input.review.object.metadata.labels[label]}
          required := {label | label := input.parameters.labels[_]}
          missing := required - provided
          count(missing) > 0
          msg := sprintf("Resource is missing required labels: %v", [missing])
        }
```

**Create a constraint using this template:**

```yaml
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sRequiredLabels
metadata:
  name: require-team-label
spec:
  enforcementAction: warn    # Start with warn, then deny
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Pod"]
      - apiGroups: ["apps"]
        kinds: ["Deployment", "StatefulSet"]
  parameters:
    labels:
      - "team"
      - "environment"
```

### Example 2: Container Image Registry Policy

Restrict container images to approved registries:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8sallowedrepos
  annotations:
    knodex.io/compliance: "true"
    description: "Restricts container images to approved registries"
spec:
  crd:
    spec:
      names:
        kind: K8sAllowedRepos
      validation:
        openAPIV3Schema:
          type: object
          properties:
            repos:
              type: array
              description: "List of allowed image registry prefixes"
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8sallowedrepos

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          not startswith_any(container.image, input.parameters.repos)
          msg := sprintf("Container image '%v' is not from an allowed registry. Allowed: %v", [container.image, input.parameters.repos])
        }

        violation[{"msg": msg}] {
          container := input.review.object.spec.initContainers[_]
          not startswith_any(container.image, input.parameters.repos)
          msg := sprintf("Init container image '%v' is not from an allowed registry. Allowed: %v", [container.image, input.parameters.repos])
        }

        startswith_any(str, prefixes) {
          prefix := prefixes[_]
          startswith(str, prefix)
        }
```

**Create a constraint:**

```yaml
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sAllowedRepos
metadata:
  name: approved-registries
spec:
  enforcementAction: deny
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Pod"]
  parameters:
    repos:
      - "gcr.io/my-org/"
      - "docker.io/library/"
      - "ghcr.io/my-org/"
```

### Example 3: Resource Limits Policy

Ensure containers have resource limits defined:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8scontainerlimits
  annotations:
    knodex.io/compliance: "true"
    description: "Requires containers to have CPU and memory limits"
spec:
  crd:
    spec:
      names:
        kind: K8sContainerLimits
      validation:
        openAPIV3Schema:
          type: object
          properties:
            requireCPU:
              type: boolean
              description: "Require CPU limits"
            requireMemory:
              type: boolean
              description: "Require memory limits"
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8scontainerlimits

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          input.parameters.requireCPU
          not container.resources.limits.cpu
          msg := sprintf("Container '%v' must have CPU limits defined", [container.name])
        }

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          input.parameters.requireMemory
          not container.resources.limits.memory
          msg := sprintf("Container '%v' must have memory limits defined", [container.name])
        }
```

### Example 4: Namespace Restrictions Policy

Prevent deployments to protected namespaces:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8sblocknamespaces
  annotations:
    knodex.io/compliance: "true"
    description: "Blocks resource creation in protected namespaces"
spec:
  crd:
    spec:
      names:
        kind: K8sBlockNamespaces
      validation:
        openAPIV3Schema:
          type: object
          properties:
            namespaces:
              type: array
              description: "List of protected namespaces"
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8sblocknamespaces

        violation[{"msg": msg}] {
          namespace := input.review.object.metadata.namespace
          protected := input.parameters.namespaces[_]
          namespace == protected
          msg := sprintf("Cannot create resources in protected namespace '%v'", [namespace])
        }
```

## Parameter Schema Reference

Define constraint parameters using OpenAPI v3 schema:

### Basic Types

```yaml
validation:
  openAPIV3Schema:
    type: object
    properties:
      stringParam:
        type: string
        description: "A text value"
      intParam:
        type: integer
        description: "A whole number"
        minimum: 1
        maximum: 100
      boolParam:
        type: boolean
        description: "True or false"
      arrayParam:
        type: array
        items:
          type: string
        description: "List of strings"
```

### Required Parameters

```yaml
validation:
  openAPIV3Schema:
    type: object
    required:
      - labels        # This parameter must be provided
    properties:
      labels:
        type: array
        items:
          type: string
      message:
        type: string  # Optional parameter
```

### Enum Constraints

```yaml
validation:
  openAPIV3Schema:
    type: object
    properties:
      severity:
        type: string
        enum:
          - low
          - medium
          - high
          - critical
```

## Match Rules

Define which resources a constraint applies to:

### By Resource Kind

```yaml
match:
  kinds:
    - apiGroups: [""]           # Core API group
      kinds: ["Pod", "Service"]
    - apiGroups: ["apps"]
      kinds: ["Deployment", "StatefulSet", "DaemonSet"]
    - apiGroups: ["networking.k8s.io"]
      kinds: ["Ingress"]
```

### By Namespace

```yaml
match:
  namespaces:
    - production
    - staging
  excludedNamespaces:
    - kube-system
    - gatekeeper-system
```

### By Label Selector

```yaml
match:
  labelSelector:
    matchLabels:
      environment: production
    matchExpressions:
      - key: team
        operator: In
        values: ["frontend", "backend"]
```

### By Scope

```yaml
match:
  scope: Namespaced    # Only namespaced resources
  # Or: Cluster        # Only cluster-scoped resources
  # Or: *              # Both (default)
```

## Enforcement Actions

Configure how Gatekeeper responds to violations:

| Action | Behavior | Use Case |
|--------|----------|----------|
| `deny` | Blocks admission request | Production enforcement |
| `warn` | Allows but logs warning | Gradual rollout |
| `dryrun` | Records in audit only | Policy testing |

{{< alert title="Best Practice" >}}
Always start with `dryrun` or `warn` to assess impact before enabling `deny`. Use the knodex Compliance dashboard to monitor violations before changing enforcement.
{{< /alert >}}

## Deploying ConstraintTemplates

### Apply Template

```bash
kubectl apply -f my-constrainttemplate.yaml
```

### Verify Template Status

```bash
# Check template is created
kubectl get constrainttemplates

# Verify the constraint CRD is ready
kubectl get crd | grep constraints.gatekeeper.sh
```

### Apply Constraint

```bash
kubectl apply -f my-constraint.yaml
```

### Verify in knodex

1. Navigate to **Compliance** → **Templates**
2. Verify your template appears with the correct description
3. Navigate to **Compliance** → **Constraints**
4. Verify constraints are listed with violation counts

## Rego Policy Patterns

### Access Input Object

```rego
# The resource being evaluated
input.review.object

# Metadata
input.review.object.metadata.name
input.review.object.metadata.namespace
input.review.object.metadata.labels

# Spec (for most resources)
input.review.object.spec

# Kind information
input.review.kind.kind
input.review.kind.group
```

### Access Parameters

```rego
# Single value
input.parameters.myParam

# Array iteration
label := input.parameters.labels[_]

# With index
input.parameters.items[i]
```

### Generate Violation Messages

```rego
# Simple message
violation[{"msg": "Resource violates policy"}] {
  # condition
}

# Formatted message
violation[{"msg": msg}] {
  # condition
  msg := sprintf("Container %v violates policy", [container.name])
}
```

### Common Patterns

```rego
# Check if label exists
has_label(obj, key) {
  obj.metadata.labels[key]
}

# Check if annotation exists
has_annotation(obj, key) {
  obj.metadata.annotations[key]
}

# String prefix check
startswith(str, prefix)

# String contains
contains(str, substring)

# Set operations
required := {item | item := input.parameters.items[_]}
provided := {item | item := input.review.object.spec.items[_]}
missing := required - provided
```

## Best Practices

### 1. Always Add Compliance Annotation

```yaml
annotations:
  knodex.io/compliance: "true"    # Required for visibility
  description: "Clear description"  # Standard Kubernetes annotation
```

### 2. Write Clear Violation Messages

```rego
# Good - includes context
msg := sprintf("Container '%v' in namespace '%v' must use image from approved registry",
               [container.name, input.review.object.metadata.namespace])

# Avoid - too generic
msg := "Policy violation"
```

### 3. Start with Dryrun

```yaml
spec:
  enforcementAction: dryrun    # Test first
```

### 4. Use Meaningful Names

```yaml
# Good
metadata:
  name: k8srequiredlabels
spec:
  crd:
    spec:
      names:
        kind: K8sRequiredLabels

# Avoid
metadata:
  name: policy1
```

### 5. Document Parameters

```yaml
properties:
  labels:
    type: array
    description: "List of label keys that must be present on resources"
    items:
      type: string
```

## Troubleshooting

### Template Not Appearing in knodex

**Cause:** Missing or incorrect compliance annotation

**Solution:**
```yaml
annotations:
  knodex.io/compliance: "true"  # Must be exactly "true" (case-insensitive)
```

### Constraint Not Enforcing

**Cause:** Match rules not targeting correct resources

**Solution:**
```bash
# Check Gatekeeper audit results
kubectl get k8srequiredlabels.constraints.gatekeeper.sh -o yaml

# Look at status.violations
```

### Violations Not Updating

**Cause:** Gatekeeper audit interval

**Solution:** Wait for audit cycle (default 60 seconds) or check:
```bash
kubectl logs -n gatekeeper-system -l control-plane=audit-controller
```

### Policy Logic Errors

**Cause:** Rego syntax or logic error

**Solution:**
```bash
# Check Gatekeeper controller logs
kubectl logs -n gatekeeper-system -l control-plane=controller-manager

# Test policy with Rego Playground
# https://play.openpolicyagent.org/
```

### Check knodex Server Logs

```bash
kubectl logs -n knodex -l app=knodex-server | grep -i constraint
```

---

**Next:** [OIDC Integration](../operator-manual/oidc-integration/) | **Previous:** [RGD Development](../operator-manual/rgds-development/)
