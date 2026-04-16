# Knodex - Kubernetes Resource Orchestrator Dashboard

[![CI](https://github.com/knodex/knodex/actions/workflows/ci.yml/badge.svg)](https://github.com/knodex/knodex/actions/workflows/ci.yml)
[![E2E Tests](https://github.com/knodex/knodex/actions/workflows/e2e-tests.yml/badge.svg)](https://github.com/knodex/knodex/actions/workflows/e2e-tests.yml)
[![KRO 0.9.0](https://img.shields.io/badge/tested_with-KRO_0.9.0-blue)](https://github.com/knodex/knodex/actions/workflows/e2e-tests.yml)

## What is Knodex?

Knodex is a web dashboard for [KRO](https://kro.run/) (Kubernetes Resource Orchestrator). It gives platform teams a visual interface to browse, deploy, and manage ResourceGraphDefinitions across clusters.

## Why Knodex?

KRO lets you define complex multi-resource Kubernetes deployments as a single custom resource. Knodex makes that accessible to your whole team — not just the ones who write YAML.

- Platform engineers publish RGDs as a self-service catalog
- Developers deploy instances from the catalog without touching manifests
- Teams get real-time status, RBAC-scoped visibility, and multi-tenancy out of the box

## Features

| Feature | Description |
|---------|-------------|
| **RGD Catalog** | Browse and search ResourceGraphDefinitions across the cluster |
| **Instance Deployment** | Deploy RGD instances with a form — no kubectl required |
| **Real-Time Updates** | WebSocket-powered live status for RGDs and instances |
| **Project Multi-Tenancy** | ArgoCD-aligned Project CRD for team isolation |
| **RBAC** | Casbin-based authorization with global and project-scoped roles |
| **OIDC Authentication** | SSO via any OpenID Connect provider |
| **Deployment Modes** | Direct, GitOps, or Hybrid deployment strategies |
| **Repository Management** | Connect Git repositories for GitOps workflows |

## Quick Start

See the full [Getting Started guide](docs/getting-started/_index.md) for detailed instructions.

```bash
# Install Knodex from the OCI registry
helm install knodex oci://ghcr.io/knodex/charts/knodex \
  --namespace knodex \
  --create-namespace
```

Then access the UI:

```bash
kubectl port-forward svc/knodex-server 8080:8080 -n knodex
```

Open http://localhost:8080.

## Documentation

Full documentation is available at [knodex.io/docs](https://knodex.io/docs).

- [Getting Started](https://knodex.io/docs/getting-started/) — Installation and first deployment
- [Operator Manual](https://knodex.io/docs/operator-manual/) — Configuration and maintenance
- [User Guide](https://knodex.io/docs/user-guide/) — Browsing catalogs and deploying instances

## Enterprise Edition

Knodex Enterprise adds features for organizations that need advanced governance and compliance capabilities.

| Feature | OSS | Enterprise |
|---------|:---:|:----------:|
| RGD Catalog & Deployment | x | x |
| Project Multi-Tenancy & RBAC | x | x |
| OIDC SSO | x | x |
| Real-Time WebSocket Updates | x | x |
| GitOps Deployment Mode | x | x |
| Compliance Dashboard (OPA) | | x |
| Deployment Compliance Auditing | | Coming soon |
| AI Template Builder | | Coming soon |

Learn more at [knodex.io](https://knodex.io).

## Community

- [GitHub Issues](https://github.com/knodex/knodex/issues) — Bug reports and feature requests
- [GitHub Discussions](https://github.com/knodex/knodex/discussions) — Questions and ideas

## Contributing

We welcome contributions. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

GNU Affero General Public License v3.0 — see [LICENSE](LICENSE) for details.
