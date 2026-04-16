---
title: ConstraintTemplate Development
description: Author OPA Gatekeeper ConstraintTemplates with Knodex compliance annotations for dashboard integration.
sidebar_position: 4
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["enterprise"]} />

# ConstraintTemplate Development

This guide covers authoring Gatekeeper ConstraintTemplates that integrate with the Knodex compliance dashboard. Templates define policy logic in Rego, and Knodex annotations enrich the dashboard display.

## Structure

A ConstraintTemplate defines a reusable policy. Constraints instantiate the template with specific parameters and match rules.

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8srequiredlabels
  annotations:
    knodex.io/compliance: "true"
    description: "Requires specified labels on resources"
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
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8srequiredlabels

        violation[{"msg": msg}] {
          provided := {label | input.review.object.metadata.labels[label]}
          required := {label | label := input.parameters.labels[_]}
          missing := required - provided
          count(missing) > 0
          msg := sprintf("Missing required labels: %v", [missing])
        }
```

## Compliance Annotations

### Required

| Annotation | Value | Description |
|-----------|-------|-------------|
| `knodex.io/compliance` | `"true"` | Enables enhanced display in the Knodex compliance dashboard |

### Optional

| Annotation | Description |
|-----------|-------------|
| `description` | Human-readable description of what the template enforces |

## Examples

### Required Labels

Ensures resources have specified labels.

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8srequiredlabels
  annotations:
    knodex.io/compliance: "true"
    description: "Requires specified labels on all matched resources"
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
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8srequiredlabels

        violation[{"msg": msg}] {
          provided := {label | input.review.object.metadata.labels[label]}
          required := {label | label := input.parameters.labels[_]}
          missing := required - provided
          count(missing) > 0
          msg := sprintf("Missing required labels: %v", [missing])
        }
```

Constraint using this template:

```yaml
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sRequiredLabels
metadata:
  name: require-team-label
spec:
  enforcementAction: warn
  match:
    kinds:
      - apiGroups: ["apps"]
        kinds: ["Deployment", "StatefulSet"]
  parameters:
    labels: ["team", "environment"]
```

### Image Registry

Restricts container images to approved registries.

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8sallowedregistries
  annotations:
    knodex.io/compliance: "true"
    description: "Restricts container images to approved registries"
spec:
  crd:
    spec:
      names:
        kind: K8sAllowedRegistries
      validation:
        openAPIV3Schema:
          type: object
          properties:
            registries:
              type: array
              description: "List of allowed registry prefixes"
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8sallowedregistries

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          not startswith_any(container.image, input.parameters.registries)
          msg := sprintf("Container image '%v' is not from an approved registry. Approved: %v", [container.image, input.parameters.registries])
        }

        violation[{"msg": msg}] {
          container := input.review.object.spec.initContainers[_]
          not startswith_any(container.image, input.parameters.registries)
          msg := sprintf("Init container image '%v' is not from an approved registry. Approved: %v", [container.image, input.parameters.registries])
        }

        startswith_any(str, prefixes) {
          prefix := prefixes[_]
          startswith(str, prefix)
        }
```

### Container Limits

Requires CPU and memory limits on all containers.

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8scontainerlimits
  annotations:
    knodex.io/compliance: "true"
    description: "Requires CPU and memory limits on containers"
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
          msg := sprintf("Container '%v' must have CPU limits", [container.name])
        }

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          input.parameters.requireMemory
          not container.resources.limits.memory
          msg := sprintf("Container '%v' must have memory limits", [container.name])
        }
```

### Namespace Restrictions

Prevents resource creation in restricted namespaces.

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8srestrictednamespaces
  annotations:
    knodex.io/compliance: "true"
    description: "Prevents resource creation in restricted namespaces"
spec:
  crd:
    spec:
      names:
        kind: K8sRestrictedNamespaces
      validation:
        openAPIV3Schema:
          type: object
          properties:
            restricted:
              type: array
              description: "List of restricted namespace names or patterns"
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8srestrictednamespaces

        violation[{"msg": msg}] {
          namespace := input.review.object.metadata.namespace
          restricted := input.parameters.restricted[_]
          namespace == restricted
          msg := sprintf("Namespace '%v' is restricted. Resources cannot be created here.", [namespace])
        }
```

## Parameter Schema Reference

### Basic Types

```yaml
validation:
  openAPIV3Schema:
    type: object
    properties:
      stringParam:
        type: string
        description: "A text parameter"
      intParam:
        type: integer
        description: "A numeric parameter"
      boolParam:
        type: boolean
        description: "A boolean flag"
      listParam:
        type: array
        items:
          type: string
        description: "A list of strings"
```

### Required Parameters

```yaml
validation:
  openAPIV3Schema:
    type: object
    required: ["labels"]
    properties:
      labels:
        type: array
        items:
          type: string
```

### Enum Values

```yaml
validation:
  openAPIV3Schema:
    type: object
    properties:
      severity:
        type: string
        enum: ["low", "medium", "high", "critical"]
```

## Match Rules

Constraints use `match` to target specific resources.

### By Kind

```yaml
match:
  kinds:
    - apiGroups: ["apps"]
      kinds: ["Deployment", "StatefulSet"]
    - apiGroups: [""]
      kinds: ["Pod"]
```

### By Namespace

```yaml
match:
  namespaces: ["production", "staging"]
  # or exclude:
  excludedNamespaces: ["kube-system", "gatekeeper-system"]
```

### By Label Selector

```yaml
match:
  labelSelector:
    matchExpressions:
      - key: "environment"
        operator: "In"
        values: ["production"]
```

### By Scope

```yaml
match:
  scope: "Namespaced"    # or "Cluster" or "*"
```

## Enforcement Actions

| Action | Admission Behavior | Gatekeeper Audit | Knodex Display |
|--------|--------------------|-----------------|---------------|
| `deny` | Blocks the request | Records violation | Red badge |
| `dryrun` | Allows the request | Records violation | Gray badge |
| `warn` | Allows with warning | Records violation | Yellow badge |

## Deploying

Apply the ConstraintTemplate, then create a Constraint:

```bash
# Apply the template
kubectl apply -f constraint-template.yaml

# Verify the template is ready
kubectl get constrainttemplates k8srequiredlabels

# Apply a constraint
kubectl apply -f constraint.yaml

# Verify the constraint
kubectl get k8srequiredlabels
```

The template and constraint appear in the Knodex compliance dashboard after the Gatekeeper watcher picks them up.

## Rego Patterns

### Input Object

The `input` object in Rego contains:

```
input.review.object          -- The resource being evaluated
input.review.oldObject       -- The previous version (for updates)
input.review.operation       -- CREATE, UPDATE, DELETE
input.parameters             -- Constraint parameters
```

### Parameters

Access constraint parameters via `input.parameters`:

```rego
violation[{"msg": msg}] {
  label := input.parameters.labels[_]
  not input.review.object.metadata.labels[label]
  msg := sprintf("Missing label: %v", [label])
}
```

### Violations

Return violations as objects with a `msg` field:

```rego
violation[{"msg": msg}] {
  # condition
  msg := "Human-readable violation message"
}
```

### Common Patterns

**Iterate containers:**
```rego
container := input.review.object.spec.containers[_]
```

**Check annotations:**
```rego
not input.review.object.metadata.annotations["required-annotation"]
```

**Check nested fields safely:**
```rego
object_get(input.review.object.spec, "field", "default")
```

**String matching:**
```rego
startswith(container.image, "approved-registry.io/")
```

## Best Practices

1. **Start with `dryrun`.** Deploy new constraints in `dryrun` mode to understand the blast radius before enforcing.

2. **Use descriptive violation messages.** Include the specific field or value that caused the violation and what the expected value should be.

3. **Scope constraints narrowly.** Use namespace selectors and kind filters to target only the resources you intend. Broad constraints on all namespaces can block system components.

4. **Add the `knodex.io/compliance` annotation.** This enables enhanced display in the Knodex dashboard with description and metadata.

5. **Test with `kubectl apply --dry-run=server`.** Validate that your constraint catches violations before deploying to production.

## Troubleshooting

| Issue | Resolution |
|-------|-----------|
| Template not appearing in dashboard | Verify `knodex.io/compliance: "true"` annotation. Check Gatekeeper controller logs. |
| Constraint not enforcing | Check `enforcementAction` is `deny` (not `dryrun`). Verify `match` rules target the intended resources. |
| Rego compilation errors | Check Gatekeeper controller logs: `kubectl logs -n gatekeeper-system -l control-plane=controller-manager` |
| Violations not clearing after fix | Gatekeeper audit runs periodically (default 60s). Wait for the next audit cycle or trigger manually. |
