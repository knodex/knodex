---
title: Deploying Instances
description: Step-by-step guide to deploying instances from ResourceGraphDefinitions in Knodex.
sidebar_position: 2
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Deploying Instances

Deploying an instance creates a running set of Kubernetes resources from a ResourceGraphDefinition (RGD). Knodex generates a tabbed deployment page based on the RGD schema so you can configure and deploy without writing YAML.

## Starting a Deployment

There are four ways to begin a deployment:

1. **From the catalog** -- Click the deploy button on any RGD card in the catalog view.
2. **From RGD details** -- Open an RGD detail page and click **Deploy** in the header.
3. **From an instance add-on** -- On an existing instance's detail page, click **Deploy Add-on**. This pre-fills the `externalRef` field with the parent instance reference.
4. **From an example template** -- On the RGD's Examples tab, click **Use Template** to populate the deployment form with example values.

## The Deploy Page

The deploy page is organized into tabs. Tab order and labels are derived from the RGD schema.

### Basics Tab

The first tab collects instance identity and deployment settings:

- **Name** -- A DNS-compatible name for the instance. Must be lowercase, alphanumeric, and may contain hyphens. Maximum 63 characters. Example: `my-web-app-staging`.
- **Namespace** -- The target Kubernetes namespace (scoped to your project destinations).
- **Project** -- The project this instance belongs to.
- **Deployment Mode** -- How the manifest is applied: Direct, GitOps, or Hybrid. See [Deployment Modes](deployment-modes).

### Schema Tabs

One tab is generated for each top-level object key in the RGD's `spec.schema.spec`. Scalar fields that are not grouped under any top-level object are collected into a single **General** tab. Each tab renders input fields based on the RGD schema:

- Required fields are marked and must be filled before deployment.
- Optional fields show their default values, which you can override.
- Enum fields appear as dropdowns with the allowed values.
- Boolean fields appear as checkboxes.
- Complex nested fields expand into sub-sections.
- An **Advanced** toggle on each tab hides rarely-changed fields by default.
- Some sections (and some entire tabs) start with an **Enabled** checkbox. The rest of the fields in that section are hidden until you tick it -- handy for optional sub-systems like a database, bastion, or monitoring. Untick it to clear the section back out.

### Review and Deploy Tab

The final tab shows a summary of your configuration and runs pre-deployment checks:

- **Compliance check** (Enterprise) -- Validates the manifest against your organization's compliance policies. Deployments blocked by a policy cannot proceed.
- **Admission preflight** -- Runs a dry-run against the cluster's admission webhooks and reports any rejections.
- **Configuration summary** -- Lists the values you entered across all tabs, with an **Edit** link next to each section to jump back to the relevant tab.
- **YAML manifest** -- Shows the full Kubernetes manifest that will be applied.

## Tab Validation Badges

Each tab displays a badge indicating its validation state:

| Badge | Meaning |
|-------|---------|
| Red dot | The tab contains one or more validation errors |
| Green check | All fields on the tab are valid |
| Gray dot | The tab has not been visited yet |

## Action Footer

A fixed footer persists at the bottom of the page throughout the deployment:

| Button | Available when | Action |
|--------|---------------|--------|
| **Previous** | Any tab after the first | Navigate to the prior tab |
| **Next** | Any tab before Review | Validate the current tab and advance |
| **Review and Deploy** | Any tab before Review | Jump directly to the Review and Deploy tab |
| **Deploy** | Review and Deploy tab | Submit the deployment |

## Deploying

### Final Checks

Before clicking **Deploy** on the Review and Deploy tab, confirm:

- No tabs show a red validation badge
- The compliance check passed (Enterprise) or no compliance policy is configured
- The admission preflight shows no blocking errors
- The target namespace and project are correct
- The deployment mode is set appropriately (see [Deployment Modes](deployment-modes))

### After Deploying

On success, you are redirected to the instance detail page.

Monitor the instance's **Status** tab to track conditions reported by KRO and the underlying operators.

## GitOps Deployment

When using the **GitOps** deployment mode, Knodex commits the instance manifest to your configured Git repository but does not apply it to the cluster. The instance is tracked in Knodex but not managed by KRO until your GitOps tool syncs it. After deployment:

- The instance appears in Knodex with an **unhealthy** status — this is expected because the resource does not exist in the cluster yet
- Your platform's GitOps tool (ArgoCD, Flux, etc.) detects the new manifest in the repository and applies it to the cluster
- Once KRO picks up the applied resource and reconciles the underlying resources, the instance status in Knodex updates automatically
