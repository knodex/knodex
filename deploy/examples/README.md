# Knodex Examples

Example resources used for demos, local development, and QA deployments.
`scripts/qa-deploy.sh` applies these directories when deploying the app to a
test cluster.

| Directory | Contents | Applied by qa-deploy.sh |
|-----------|----------|-------------------------|
| `rgds/` | Example ResourceGraphDefinitions for the Catalog | Always |
| `instances/` | Example instances of the RGDs above | Always (best-effort) |
| `projects/` | Example Project CRs (RBAC) | Always |
| `gatekeeper/` | Gatekeeper ConstraintTemplates/Constraints | Enterprise QA only |

## agents/ (moved to `deploy/charts/knodex/files/agents/`)

The agent assets no longer live here. The canonical agent-wrapping RGD ships
at **`deploy/charts/knodex/files/agents/kagent-agent.yaml`** (alongside
`model-config.yaml`, `model-provider-config.yaml` and
`rgd-builder-agent.yaml`), packaged by the Helm chart when
`kagent.rgds.enabled=true` (the default). Like every published RGD it
carries `knodex.io/catalog: "true"` — the single publishing gateway (the
watcher only ingests annotated RGDs, and the Deploy drawer fetches by name) —
while its agent schema kind (`KagentAgent`) keeps it out of the Catalog list
and category counts: agent-kinded RGDs surface through the Agents workspace
instead. Users create agents from the **Agents tab** (Create Agent →
Deploy drawer), which binds the existing instance deploy flow (and Casbin
`instances/create` authorization) to that RGD by its fixed name. The Agent
template carries no `metadata.namespace`, so it inherits the instance's
namespace and lands in the project namespace selected at deploy time, where the
cluster-wide kagent controller serves it automatically.

**Prerequisite:** the [kagent](https://kagent.dev) operator must be installed
in the cluster (CRD `agents.kagent.dev`).

**Why the agent RGDs are not auto-applied unconditionally by QA:** QA clusters
do not install kagent (`scripts/ensure-prereqs.sh` installs KRO + Knodex CRDs
only). KRO cannot build the resource graph for an RGD whose produced kind's
CRD is absent — the RGD would sit permanently non-Active.
`scripts/qa-deploy.sh` therefore applies the files/agents/ directory only when
`kubectl get crd agents.kagent.dev` succeeds, and logs a skip message
otherwise. On kagent-enabled clusters (demos,
future kagent E2E) the same script lights the flow up end-to-end.
