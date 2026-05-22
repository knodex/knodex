---
title: Wrapper RGDs
description: Bundle bootstrap resources with Project creation using operator-configured wrapper ResourceGraphDefinitions.
sidebar_position: 9
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Wrapper RGDs

A **wrapper RGD** is a kro `ResourceGraphDefinition` that an operator registers in Knodex to bundle bootstrap resources (additional namespaces, secret stores, NetworkPolicies, default repositories, quotas, …) alongside a built-in resource type. When a wrapper is registered, the Knodex HTTP create handler creates the wrapper's RGD instance instead of the resource directly; the kro controller then materializes the whole bundle.

v1 of this feature supports `kind=Project` only. The same mechanism extends to other Kinds in subsequent releases without re-architecture.

## When to use a wrapper

- Standardize Project creation across teams (every Project ships with a namespace, secret store, default quota, …).
- Apply org-wide policies (NetworkPolicies, ResourceQuotas) at Project birth, not retroactively.
- Surface a self-service Project page to developers while keeping operator-defined defaults inviolate.

## How wrapper routing works

1. The Knodex server watches the `knodex-resource-wrappers` ConfigMap for entries of the form `{ "kind": "Project", "rgdName": "<your-wrapper-rgd>" }`.
2. On `POST /api/v1/projects`, the handler looks up the registered RGD name for `Project` in its in-memory cache.
3. If an entry exists, the handler resolves the RGD's instance GVK and creates an instance carrying the request payload as `spec`. The kro controller materializes the bundle (Project + extras).
4. Subsequent `PATCH` / `DELETE` operations route through the same wrapper instance via a marker annotation (`knodex.io/wrapper-rgd-instance`) stamped on the resulting Project.

If no entry is registered, behavior is unchanged: the Project is created directly.

## Authoring contract

Your wrapper RGD MUST satisfy three contracts:

1. **`spec.schema` accepts the Project payload as a superset.** At minimum:
   - `description: string`
   - `destinations: []map[string]string`
   - `sourceRepos: []string`
   - `roles: []map[string]any`
2. **`spec.resources` includes a `Project` resource** pinned to the Knodex install namespace (typically `knodex-system`).
3. **The Project resource template stamps the marker annotation** `knodex.io/wrapper-rgd-instance` with the wrapper-instance name (`${schema.metadata.name}`). Without the marker, lifecycle PATCH/DELETE will silently fall back to direct routing on the Project — bypassing kro reconciliation.

A runtime lint that rejects wrapper RGDs missing the marker annotation is on the roadmap.

## Example

A wrapper RGD that bootstraps a per-project namespace and a defaults ConfigMap:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: wrapped-project-v1
  namespace: knodex-system
spec:
  schema:
    apiVersion: v1alpha1
    kind: WrappedProject
    spec:
      description: string | default=""
      destinations: "[]map[string]string" | default=[]
      sourceRepos: "[]string" | default=[]
      roles: "[]map[string]any" | default=[]
  resources:
    - id: project
      template:
        apiVersion: knodex.io/v1alpha1
        kind: Project
        metadata:
          name: ${schema.metadata.name}
          namespace: knodex-system
          annotations:
            knodex.io/wrapper-rgd-instance: ${schema.metadata.name}  # REQUIRED
        spec:
          description: ${schema.spec.description}
          destinations: ${schema.spec.destinations}
          sourceRepos: ${schema.spec.sourceRepos}
          roles: ${schema.spec.roles}
    - id: bootstrapNamespace
      template:
        apiVersion: v1
        kind: Namespace
        metadata:
          name: proj-${schema.metadata.name}
    - id: bootstrapConfig
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: project-defaults
          namespace: proj-${schema.metadata.name}
        data:
          owner: ${schema.metadata.name}
```

A ready-to-apply copy lives at [`deploy/examples/rgds/wrapped-project-example.yaml`](https://github.com/knodex/knodex/blob/main/deploy/examples/rgds/wrapped-project-example.yaml).

## Registering a wrapper

You can manage entries via **Helm values** (declarative, GitOps-friendly) or the **settings API** (runtime, admin-only). Both write through to the same ConfigMap.

### Via Helm

```yaml
server:
  project:
    wrapperRGD: wrapped-project-v1
```

The chart renders `knodex-resource-wrappers` unconditionally; `server.project.wrapperRGD` defaults to `""` (no wrapper).

### Via the settings API

Admin-only, Casbin-gated on the `settings` resource. Rate-limited to 1/min sustained (burst 5).

```bash
# List
curl -H 'Authorization: Bearer $TOKEN' \
  http://knodex/api/v1/settings/wrappers

# Upsert
curl -X PUT -H 'Authorization: Bearer $TOKEN' -H 'Content-Type: application/json' \
  -d '{"rgdName":"wrapped-project-v1"}' \
  http://knodex/api/v1/settings/wrappers/Project

# Delete
curl -X DELETE -H 'Authorization: Bearer $TOKEN' \
  http://knodex/api/v1/settings/wrappers/Project
```

Only `kind=Project` is accepted in v1; unsupported Kinds return `400 BAD_REQUEST`.

## Failure modes

| Condition | Server response |
|---|---|
| Registered RGD not present in cluster | `422 Unprocessable Entity` with `code=WRAPPER_MISCONFIGURED`, `details.reason=rgd_not_found` |
| Registered RGD missing `spec.schema.apiVersion` or `kind` | `422` with `details.reason=rgd_not_ready` |
| Marker annotation missing on materialized Project | Subsequent PATCH/DELETE silently fall back to direct routing — operator should add the marker template |
| Registry entry removed while wrapped Projects still exist | Existing Projects continue to be managed via their marker-pointed instance; PATCH/DELETE self-heal back to direct routing when the entry is gone |

## Audit

Every wrapper-routed Project create/update/delete emits an audit event with `Resource: "settings"` (for registry mutations) or `Resource: "projects"` (for Project CRUD). The Project events carry these additional keys in `Details`:

- `wrapperUsed: true`
- `wrapperRGD: "<rgd-name>"`

The non-wrapper code path remains byte-identical to today's behavior — no `wrapperUsed` key is emitted.
