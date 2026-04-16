---
title: Administration
description: Configure and manage the Knodex platform — RBAC, OIDC, projects, repositories, and cluster settings.
sidebar_position: 1
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Administration

This section covers platform configuration and management for Knodex administrators.

## Configuration & Setup

| Guide | Description |
|-------|-------------|
| [Configuration](configuration) | Environment variables, Helm values, Redis, architecture patterns |
| [OIDC Integration](oidc-integration) | SSO setup for Azure AD, Okta, Auth0, Google, Keycloak |
| [Kubernetes RBAC](kubernetes-rbac) | ServiceAccount, ClusterRole, and CRD permissions |

## Access Control

| Guide | Description |
|-------|-------------|
| [RBAC Setup](rbac-setup) | Casbin-based roles, policies, and deployment scenarios |
| [Organizations](organizations) | Multi-tenant organization isolation |
| [Members](members) | Managing role assignments and group bindings |

## Resources

| Guide | Description |
|-------|-------------|
| [Repositories](repositories) | Declarative repository credentials for GitOps |
| [Credentials](credentials) | Repository authentication (PAT, SSH, GitHub App) |
| [Secrets Management](secrets-management) | Kubernetes Secrets for RGD external references |

## Customization

| Guide | Description |
|-------|-------------|
| [Custom Icons](custom-icons) | Brand icons for RGD catalog cards |
| [Catalog Filter](catalog-filter) | Restrict catalog to specific RGD packages |

## Operations

| Guide | Description |
|-------|-------------|
| [Troubleshooting](troubleshooting) | Diagnostic procedures and common issues |
