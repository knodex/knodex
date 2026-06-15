---
title: Agents (kagent)
description: Install and configure the kagent AI agent operator to enable the Knodex Agents Hub.
sidebar_position: 13
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Agents (kagent)

Knodex integrates with [kagent](https://kagent.dev), a Kubernetes-native AI agent operator. When kagent is installed in your cluster, the Agents workspace becomes active and lets users chat with the agents in their accessible namespaces (such as the RGD Builder), deploy agent templates, and manage model configurations.

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| Kubernetes 1.28+ | Tested against standard API servers (Kind, AKS, GKE, EKS) |
| [Kro](https://kro.run) | Required for BYOA agents deployed as RGD instances |
| kagent operator | See installation options below |
| AI model provider | kagent requires a model config (e.g., OpenAI, Azure OpenAI, Gemini). See [kagent docs](https://kagent.dev/docs) for setup |

## Installing kagent

You have two options: install via the Knodex Helm chart flag or install kagent independently.

### Option A — Helm flag (recommended for new deployments)

Set `kagent.enabled=true` when installing or upgrading the Knodex chart. This installs the kagent operator as a subchart into the same namespace as Knodex.

```bash
helm upgrade --install knodex knodex/knodex \
  --namespace knodex \
  --set kagent.enabled=true
```

Or in `values.yaml`:

```yaml
kagent:
  enabled: true
```

:::note[helm dependency update]
If you are building from source or pinning chart versions, run `helm dependency update deploy/charts/knodex` after enabling `kagent.enabled` to fetch the subchart before installing.
:::

The Knodex chart automatically sets `KAGENT_CONTROLLER_BASE_URL` to
`http://kagent-controller.<release-namespace>.svc.cluster.local:8083`
when `kagent.enabled=true`. No further configuration is needed.

### Option B — Standalone installation (recommended for shared clusters)

Install kagent into its own dedicated namespace, then point Knodex at it.

```bash
# 1. Install CRDs
helm install kagent-crds oci://ghcr.io/kagent-dev/kagent/helm/kagent-crds \
  --namespace kagent \
  --create-namespace

# 2. Install the operator
helm install kagent oci://ghcr.io/kagent-dev/kagent/helm/kagent \
  --namespace kagent
```

Then configure the controller URL in your Knodex Helm values:

```yaml
kagent:
  enabled: false
  controllerBaseURL: "http://kagent-controller.kagent.svc.cluster.local:8083"
```

Or set the environment variable directly:

```bash
KAGENT_CONTROLLER_BASE_URL=http://kagent-controller.kagent.svc.cluster.local:8083
```

### Deploying the RGD Builder

Knodex ships one **agent template** out of the box: the RGD Builder, wrapped as the `kagent-rgd-builder-agent` RGD (`deploy/charts/knodex/files/agents/rgd-builder-agent.yaml`, installed with the chart when `kagent.rgds.enabled=true`). Like every published RGD it carries the `knodex.io/catalog: "true"` annotation; its schema kind `KnodexAgentTemplate` routes it to the **Agents → Templates** page (see [Agent Templates](#agent-templates) below) instead of the main Catalog. Deploy one `KnodexAgentTemplate` instance from there — normally into the `kagent` namespace — picking the ModelConfig the agent should run on; the resulting Agent CR is what appears in the agents list for users whose roles grant access to that namespace.

The agent references the `kagent-tool-server` RemoteMCPServer created by a default kagent installation, resolved in the Agent's own namespace. Ensure it exists before deploying.

There is no separate "hub agent" category and no `knodex.io/agent-hub` annotation: the Agents list is a single view of the agents in the caller's accessible namespaces, and invocation is routed by the agent's namespace and name.

## Changing an Agent's Model

Every agent card in the Agents Hub — in both the **Hub Agents** and **Installed** sections — carries a pencil (edit) button. Clicking it opens a dialog that lets you repoint the agent at a different model.

- The dropdown lists the `ModelConfig` resources that exist in the agent's namespace. The agent's current model is pre-selected.
- **Save** patches only `spec.declarative.modelConfig` on the Agent CR. The system message, tools, and agent type are curated agent behaviour and are **never** editable from the UI — only the model reference is exposed.
- A `ModelConfig` must already exist in the namespace before you can change the model. If the dropdown is empty, create one first (see [ModelConfig Wrapper RGDs](#modelconfig-wrapper-rgds) below).

Authorization mirrors the hub/installed split: editing a **hub agent** (in the `kagent` namespace) requires `role:serveradmin`; editing an **installed agent** requires the same project access as deploying instances into that agent's namespace.

The new model is verified to exist before the patch is applied — selecting a config that was deleted underneath you returns a clear error rather than breaking the agent with a dangling reference.

## Agent Templates

The **Agents → Templates** page lists published agent-template RGDs. Publishing follows the same rule as every other RGD: the `knodex.io/catalog: "true"` annotation is the single gateway that makes an RGD visible and deployable at all. The schema kind then routes **where** a published RGD surfaces — an RGD whose `spec.schema.kind` is `KnodexAgentTemplate` appears here instead of the main Catalog. Each row has a **Deploy** button that opens the normal deploy flow (`/deploy/{name}`).

To publish your own agent template, ship an RGD with the `knodex.io/catalog: "true"` annotation and `spec.schema.kind: KnodexAgentTemplate` (group `agents.knodex.io`); it appears on the Templates page when applied to the cluster. Without the annotation the RGD stays invisible everywhere — the annotation is the operator's explicit publishing decision.

## ModelConfig Wrapper RGDs

kagent's `ModelConfig` and `ModelProviderConfig` CRs (and the API-key Secrets they reference) are normally created with raw `kubectl`. Knodex ships ResourceGraphDefinitions under `deploy/charts/knodex/files/agents/` (installed with the chart when `kagent.rgds.enabled=true`, the default) that wrap these as KRO resources. Like the agent template, they carry the `knodex.io/catalog: "true"` annotation and are **routed by their schema kind** to the Agents workspace and the standard instance flow — not the main Catalog.

| RGD | File | Schema kind | Wraps |
|-----|------|-------------|-------|
| `kagent-model-provider-config` | `model-provider-config.yaml` | `KnodexProviderConfig` | `v1/Secret` (API key under key `apiKey`) + `kagent.dev/v1alpha2 ModelProviderConfig` referencing it |
| `kagent-model-config` | `model-config.yaml` | `KnodexAgentModelConfig` | `kagent.dev/v1alpha2 ModelConfig`, referencing an existing API-key Secret via `externalRef` |
| `kagent-rgd-builder-agent` | `rgd-builder-agent.yaml` | `KnodexAgentTemplate` | The RGD Builder `kagent.dev/v1alpha2 Agent` |

The Secret and the ModelConfig have different lifecycles: the key can be rotated without touching the ModelConfig, and several ModelConfigs can share one Secret. The **Models** page in the Agents workspace is the usual way to create a model — it mints the Secret and the `KnodexAgentModelConfig` instance in one step; deploying the wrapper RGD directly additionally allows pointing at a pre-existing Secret.

**Field shape (kagent 0.9.6, `modelconfigs.kagent.dev` v1alpha2):** the wrapper writes the flat `spec.apiKeySecret` (Secret **name**, same namespace as the ModelConfig) + `spec.apiKeySecretKey` fields. `spec.provider` is a strict capitalized enum (`OpenAI`, `AzureOpenAI`, `Anthropic`, `Gemini`, `GeminiVertexAI`, `AnthropicVertexAI`, `Ollama`, `Bedrock`, `SAPAICore`) — verify against the CRD installed in your cluster if you run a different kagent version.

## Configuration Reference

| Helm value | Environment variable | Default | Description |
|------------|---------------------|---------|-------------|
| `kagent.enabled` | — | `false` | Install kagent operator as a Helm subchart |
| `kagent.controllerBaseURL` | `KAGENT_CONTROLLER_BASE_URL` | `http://kagent-controller.kagent.svc.cluster.local:8083` | kagent controller REST base URL |

The server caches the kagent presence check for 15 seconds. A `degraded` result (operator present but controller unhealthy) is never cached — retries always go to the controller.

## Presence Detection

Knodex checks for kagent on every page load of the Agents Hub via `GET /api/v1/agents/status`. The endpoint performs two checks:

1. **CRD check** — confirms `agents.kagent.dev` exists in the cluster
2. **Controller health check** — calls `GET /health` on the kagent controller service

| Status | Meaning |
|--------|---------|
| `ready` | CRD present and controller healthy — hub is fully active |
| `not_installed` | CRD absent — hub shows onboarding instructions |
| `degraded` | CRD present but controller unhealthy — hub shows failure reason and a Retry button |

A `degraded` result does NOT indicate a Knodex problem. Check the kagent controller pod logs:

```bash
kubectl logs -n <kagent-namespace> -l app.kubernetes.io/name=kagent-controller --tail=50
```

## RBAC

No new Casbin authorization resource is introduced for agents. Authorization uses the existing single Casbin enforcement layer:

- **Agents** are filtered by the namespaces the user can access via their project `instances` policies — a single Casbin-scoped list, fail-closed. This includes the RGD Builder: users need access to its deployment namespace (normally `kagent`) to see and invoke it.
- **Agent run history** follows the same namespace visibility — there is no globally-visible run category.

To grant a user access to a BYOA agent's namespace, add the agent's deployment namespace to their project role's `destinations`:

```yaml
roles:
  - name: agent-user
    teams:
      - my-team
    destinations:
      - my-agents-namespace
    policies:
      - "instances/*, get, allow"
```

## Troubleshooting

### Hub shows "not installed" but kagent is running

Verify the CRD name exactly:

```bash
kubectl get crd agents.kagent.dev
```

If missing, reinstall the kagent CRDs chart.

### Hub shows "degraded"

The controller is unreachable. Check:

```bash
# Is the controller pod running?
kubectl get pods -n <kagent-namespace> -l app.kubernetes.io/name=kagent-controller

# Can Knodex reach it?
kubectl exec -n knodex deploy/knodex-server -- \
  wget -qO- http://kagent-controller.<kagent-namespace>.svc.cluster.local:8083/health
```

If `kagent.enabled=true`, confirm the controller is in the same namespace as Knodex or set `kagent.controllerBaseURL` explicitly.

### RGD Builder returns "agent not found"

The invoke endpoint resolves the agent by namespace and name. A 404 means either no `Agent` CR named `rgd-builder` exists in the target namespace, or the caller's roles do not grant access to that namespace (Knodex deliberately returns 404 rather than 403 to avoid leaking existence):

```bash
kubectl get agent rgd-builder -n kagent
```

If the agent is missing, deploy an instance of the `kagent-rgd-builder-agent` RGD (see [Deploying the RGD Builder](#deploying-the-rgd-builder)).
