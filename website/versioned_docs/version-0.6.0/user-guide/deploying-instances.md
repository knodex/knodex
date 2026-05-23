---
title: Deploying Instances
description: Step-by-step guide to deploying instances from ResourceGraphDefinitions in Knodex.
sidebar_position: 2
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Deploying Instances

Deploying an instance creates a running set of Kubernetes resources from a ResourceGraphDefinition (RGD). Knodex generates a deployment form based on the RGD schema so you can configure and deploy without writing YAML.

## Starting a Deployment

There are four ways to begin a deployment:

1. **From the catalog** -- Click the deploy button on any RGD card in the catalog view.
2. **From RGD details** -- Open an RGD detail page and click **Deploy** in the header.
3. **From an instance add-on** -- On an existing instance's detail page, click **Deploy Add-on**. This pre-fills the `externalRef` field with the parent instance reference.
4. **From an example template** -- On the RGD's Examples tab, click **Use Template** to populate the deployment form with example values.

## The Deployment Form

### Basic Information

The first section collects the instance identity:

- **Name** -- A DNS-compatible name for the instance. Must be lowercase, alphanumeric, and may contain hyphens. Maximum 63 characters. Example: `my-web-app-staging`.
- **Namespace** -- The target Kubernetes namespace (scoped to your project destinations).
- **Project** -- The project this instance belongs to.

### RGD-Specific Parameters

The form dynamically generates input fields based on the RGD schema. Each field shows its type, description, and any constraints.

```
+--------------------------------------------------+
| Instance Parameters                               |
+--------------------------------------------------+
| Image *            [ nginx:latest            ]    |
| Replicas           [ 3                       ]    |
| Port *             [ 8080                    ]    |
| Enable Ingress     [x]                            |
| Environment        [ production         v ]       |
+--------------------------------------------------+
  * = required field
```

- Required fields are marked with an asterisk and must be filled before deployment.
- Optional fields show their default values, which you can override.
- Enum fields appear as dropdowns with the allowed values.
- Boolean fields appear as checkboxes.
- Complex object fields expand into nested form sections.

### Advanced Options

Expand the **Advanced Options** section to configure:

- **Resource limits** -- CPU and memory requests/limits for the deployed pods
- **Ingress settings** -- Hostname, TLS, and path configuration
- **Environment variables** -- Additional environment variables injected into containers

### Form Validation

The form validates input in real time. Validation covers:

| Check | Example |
|-------|---------|
| Required fields | "Name is required" |
| Type constraints | "Replicas must be an integer" |
| Range constraints | "Port must be between 1 and 65535" |
| Pattern constraints | "Name must match DNS label format" |
| Length constraints | "Description must be under 256 characters" |

Fix all validation errors before proceeding. Invalid fields are highlighted in red with descriptive error messages.

## YAML Preview

Before deploying, click **YAML Preview** to see the full Kubernetes manifest that will be applied. The preview shows the complete instance resource.

The YAML preview is read-only and shows the manifest that will be applied based on your current form values. To make changes, update the form fields directly.

## Deploying

### Final Checks

Before clicking **Deploy**, verify:

- All required fields are filled and valid
- The target namespace is correct
- Secret requirements are satisfied (check the Secrets tab on the RGD)
- The deployment mode is set appropriately (see [Deployment Modes](deployment-modes))

### After Deploying

On success, you are redirected to the instance detail page. The time it takes for an instance to become healthy depends entirely on the resources the RGD creates and the controllers or operators responsible for reconciling them. Knodex does not control the lifecycle of the underlying resources — it only creates the instance custom resource.

Monitor the instance's **Status** tab to track conditions reported by KRO and the underlying operators.

## GitOps Deployment

When using the **GitOps** deployment mode, Knodex commits the instance manifest to your configured Git repository but does not apply it to the cluster. The instance is tracked in Knodex but not managed by KRO until your GitOps tool syncs it. After deployment:

- The instance appears in Knodex with an **unhealthy** status — this is expected because the resource does not exist in the cluster yet
- Your platform's GitOps tool (ArgoCD, Flux, etc.) detects the new manifest in the repository and applies it to the cluster
- Once KRO picks up the applied resource and reconciles the underlying resources, the instance status in Knodex updates automatically

See [Deployment Modes](deployment-modes) for details on configuring Direct, GitOps, and Hybrid modes.

