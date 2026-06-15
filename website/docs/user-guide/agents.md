---
title: AI Agents
description: Use the Agents Hub to invoke AI agents, deploy Bring Your Own Agents from the catalog, and track run history.
sidebar_position: 8
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# AI Agents

The Agents workspace connects Knodex to [kagent](https://kagent.dev), an open-source Kubernetes-native AI agent operator. Once kagent is installed in your cluster, the workspace provides four tabs: **Overview**, **Agents** (the agents you can access), **Templates** (deployable agent templates), and **Models** (model configurations).

## Navigating to the Workspace

Navigate to **Agents** in the sidebar (or the bottom navigation on mobile). If kagent is not yet installed, the workspace shows an onboarding screen with installation instructions. If the operator is installed but its controller is not responding, it shows a degraded state with the specific failure reason and a **Retry** button.

The **Agents** tab lists every agent in the namespaces your project roles can access — there is no separate "hub" category. An agent is visible to you exactly when your roles grant access to its namespace.

## Shipped Agent Template: RGD Builder

Knodex ships the RGD Builder as an agent template (the `kagent-rgd-builder-agent` RGD, listed on **Agents → Templates**); your administrator deploys an instance of it during setup, which creates the `rgd-builder` Agent CR.

### RGD Builder

The RGD Builder generates cluster-grounded [ResourceGraphDefinitions (RGDs)](/docs/rgd-authoring/) from natural language. It introspects your cluster's available CRDs and API resources before generating, so every produced spec only references resources that actually exist in your environment.

#### Using the RGD Builder

1. Open **Agents → Agents** and click the **RGD Builder** agent to start a chat session.
2. Describe what you want to deploy in the prompt box. Be as specific as possible about the Kubernetes resources involved.
   - Example: _"Create an RGD that deploys a Deployment and Service for a stateless web app with configurable replica count and image"_
3. Click **Generate**. A spinner appears while the agent runs (typically 15–60 seconds).
4. When complete, the result pane shows:
   - A structured preview of the generated spec
   - A **YAML** toggle to view the raw manifest
   - **Copy spec** to copy the YAML to your clipboard
   - **Download YAML** to save the file locally
   - **Use this spec** to open the deploy form pre-filled with the generated manifest (editable before deploying)

:::note[Cluster-grounded generation]
The agent calls your cluster's API server to discover available resources before writing a single line. If a CRD you requested doesn't exist, the agent explains why and lists the closest available alternatives instead of inventing a spec.
:::

#### Policy Validation (Enterprise)

When Knodex Enterprise is configured with OPA Gatekeeper, generated specs are automatically validated against your active policies before the result is shown.

| Badge | Meaning |
|-------|---------|
| **Policy validated ✓** | Spec passed all active Gatekeeper constraints |
| **Policy violations** | One or more constraints flagged the spec; violations are listed inline |
| **Validation unavailable** | Gatekeeper is unreachable; spec is shown without policy status |

If violations are found, Knodex attempts one automatic revision: the agent is given the original requirement, the failing spec, and the violation details and asked to produce a compliant version. The revised spec is then validated once more. Both the original and revised specs are shown so you can compare them.

## Bring Your Own Agent (BYOA)

BYOA lets you deploy any [kagent `Agent` resource](https://kagent.dev/docs/concepts/agents) from the Knodex catalog — the same deploy flow you use for any other RGD instance.

### Deploying a BYOA Agent

1. Open the **Catalog** and find an RGD whose `producesKinds` includes `kagent.dev/Agent`. These cards display an **AI** badge.
2. Deploy the RGD instance into a project namespace following the normal [deploy flow](deploying-instances).
3. Return to the **Agents** tab. The deployed agent appears in the list, filtered to namespaces your project roles can access.

When you delete the instance, KRO removes the underlying `Agent` CR and the agent disappears from the list.

### Invoking a BYOA Agent

1. Click **Invoke** on an installed agent card.
2. Enter your message (1–8 192 characters).
3. Optionally paste a **context reference** — a Knodex instance or RGD name to associate with this run (used to link the run in history).
4. Click **Send**. You receive a `202 Accepted` response immediately; the run completes asynchronously.

## Agent Run History

The run history on the Agents workspace lists all past and in-flight agent invocations visible to you.

### Reading the Table

| Column | Description |
|--------|-------------|
| **Agent** | Agent name and its namespace |
| **Triggered by** | Email of the user who invoked the agent |
| **Context** | Optional context reference, linked to the relevant instance or RGD |
| **Status** | `Running`, `Completed`, or `Failed` — updates in real time via WebSocket |
| **Timestamp** | When the invocation was initiated |

Running invocations you can see update live via WebSocket push. For invocations triggered by other users, status converges via a 5-second polling fallback.

### Filtering

Use the **Agent type** and **Status** dropdowns above the table to narrow results. The table paginates at 20 rows by default (up to 100 per page).

### Visibility Rules

Run visibility is uniform: a run is visible only to users whose project roles include access to the agent's namespace. There is no globally-visible run category.

### Viewing the Full Response

Click any completed or failed row to open the run detail panel. For completed runs, the full agent response is shown, including the generated YAML for RGD Builder runs.

Results follow the same namespace visibility rules as the history table.
