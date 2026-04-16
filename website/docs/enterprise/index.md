---
title: Enterprise Features
description: Overview of Knodex Enterprise features including Gatekeeper compliance, license management, audit trails, and organization isolation.
sidebar_position: 1
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["enterprise"]} />

# Enterprise Features

Knodex Enterprise extends the open-source platform with compliance, auditing, and multi-organization capabilities. Enterprise features require a valid license key.

## What's Included

### Gatekeeper Compliance

Integrate with OPA Gatekeeper to view ConstraintTemplates, Constraints, and Violations directly in the Knodex UI. Change enforcement actions, track violation trends, and ensure policy compliance across your clusters.

### License Management

Activate and manage your Enterprise license. Monitor license status, entitlements, and expiration from the settings page.

### Audit Trails

Record and query a complete audit trail of user actions: project creation, instance deployments, role changes, secret operations, and compliance actions. Configurable retention policies.

### Organization Isolation

Scope RGD visibility to specific organizations using the `knodex.io/organization` label. Multiple Knodex instances serving different organizations can share the same cluster without cross-visibility.

### Secrets Management

Enterprise-scoped secret management with authorization controls, ensuring secrets are only accessible within the correct project and namespace boundaries.

## How to Enable

Enterprise features are activated by providing a license key. See [License Activation](license-activation) for configuration steps.

## Feature Comparison

| Feature | OSS | Enterprise |
|---------|-----|-----------|
| RGD Catalog | Yes | Yes |
| Instance Deployment (Direct, GitOps, Hybrid) | Yes | Yes |
| Project RBAC with Casbin | Yes | Yes |
| OIDC Authentication | Yes | Yes |
| WebSocket Real-Time Updates | Yes | Yes |
| Category-Based Sidebar | Yes | Yes |
| Graph Visualization | Yes | Yes |
| Schema-Driven Deploy Forms | Yes | Yes |
| External References and Secret Pickers | Yes | Yes |
| Repository Management (GitHub) | Yes | Yes |
| Gatekeeper Compliance Dashboard | No | Yes |
| ConstraintTemplate Management | No | Yes |
| Enforcement Action Changes | No | Yes |
| Violation Tracking | No | Yes |
| Audit Trail Recording | No | Yes |
| Audit Query API | No | Yes |
| Organization Scoping | No | Yes |
| License Management | No | Yes |

## Enterprise Build

Enterprise features are included only in Enterprise builds. The server binary is compiled with the `enterprise` build tag:

```bash
go build -tags=enterprise ./...
```

When running an OSS build:
- Enterprise API endpoints return `404 Not Found` (Pattern A features) or `402 Payment Required` (Pattern B features)
- Enterprise UI sections are not rendered
- No enterprise code is included in the binary

## Sections

| Section | Description |
|---------|-------------|
| [License Activation](license-activation) | Configure and verify your license |
| [Organizations](organizations) | Multi-tenant organization isolation |
| [Compliance Management](compliance-management) | Gatekeeper dashboard and violation tracking |
| [ConstraintTemplate Development](constraint-template-development) | Author and deploy compliance policies |
