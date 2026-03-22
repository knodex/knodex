---
title: "Operator Manual"
linkTitle: "Operator Manual"
description: "Deploy, configure, and maintain knodex in production"
weight: 25
product_tags:
  - oss
  - enterprise
---

# Operator Manual

This manual covers production deployment, configuration, and operations for knodex.

## Requirements

- Kubernetes 1.32+
- [KRO](https://kro.run) installed (version 0.7.1)
- Helm 4.x
- Redis (included or external)

## Installation & Configuration

| Guide                           | Description                                                                        |
| ------------------------------- | ---------------------------------------------------------------------------------- |
| [Installation](installation/)   | Production deployment with architecture patterns, HA setup, and security hardening |
| [Configuration](configuration/) | Complete reference for server, web, Redis, and OIDC settings                       |

## Security & Access Control

| Guide                                 | Description                                                       |
| ------------------------------------- | ----------------------------------------------------------------- |
| [RBAC Setup](rbac-setup/)             | Role-based access control with 5 built-in roles and group mapping |
| [OIDC Integration](oidc-integration/) | SSO setup for Okta, Auth0, Azure AD, Google, and Keycloak         |
| [Kubernetes RBAC](kubernetes-rbac/)   | ServiceAccount, ClusterRole, and CRD permissions                  |

## Operations

| Guide                                                              | Description                                                   |
| ------------------------------------------------------------------ | ------------------------------------------------------------- |
| [Secrets Management](secrets-management/)                          | Configure secrets support and RGD secret references            |
| [Declarative Repositories](declarative-repositories/)              | Configure repository credentials via Kubernetes Secrets       |
| [ConstraintTemplate Development](../enterprise/constraint-template-development/) | Create Gatekeeper policies for knodex compliance (Enterprise) |
| [Troubleshooting](troubleshooting/)                                | Diagnostic procedures, common issues, and monitoring          |

## Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Browser   │───▶│     Web     │───▶│   Server    │
└─────────────┘    │  (React)    │    │    (Go)     │
                   └─────────────┘    └──────┬──────┘
                                             │
                   ┌─────────────────────────┼─────────────────────────┐
                   │                         │                         │
                   ▼                         ▼                         ▼
            ┌─────────────┐          ┌─────────────┐          ┌─────────────┐
            │    Redis    │          │  K8s API    │          │    OIDC     │
            │   (Cache)   │          │   (RGDs)    │          │  Provider   │
            └─────────────┘          └─────────────┘          └─────────────┘
```

## Quick Start

```bash
# Install
helm install knodex oci://ghcr.io/knodex/charts/knodex \
  --namespace knodex \
  --create-namespace \
  --values production-values.yaml

# Verify
kubectl get pods -n knodex
kubectl logs -n knodex -l app=knodex-server
```

## Support

- [GitHub Issues](https://github.com/knodex/knodex/issues)
- [KRO Documentation](https://kro.run)

---

**Start with:** [Installation](installation/) →
