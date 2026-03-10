---
title: "Core Concepts"
linkTitle: "Core Concepts"
description: "Understand the fundamental concepts behind knodex"
weight: 15
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Core Concepts

Understand the fundamental concepts behind knodex.

## knodex and KRO

knodex is a web interface for [Kubernetes Resource Orchestrator (KRO)](https://kro.run). Understanding the relationship between them is essential:

| Component  | Responsibility                                                          |
| ---------- | ----------------------------------------------------------------------- |
| **KRO**    | Defines and manages ResourceGraphDefinitions (RGDs) and their instances |
| **knodex** | Provides a UI to browse RGDs and deploy instances                       |

{{< alert title="Learn KRO First" >}}
To understand RGDs and instances, refer to the **official KRO documentation**:

- [What is KRO?](https://kro.run/docs/overview/) - Introduction to Kubernetes Resource Orchestrator
- [ResourceGraphDefinitions](https://kro.run/docs/concepts/resourcegraphdefinition/) - How RGDs define resource templates
- [Instances](https://kro.run/docs/concepts/instance/) - How instances are created from RGDs

knodex does not manage the lifecycle of instances or underlying resources. It only provides a convenient interface for deployment.
{{< /alert >}}

## knodex-Specific Concepts

The following concepts are specific to knodex:

### Projects

Projects provide multi-tenant isolation for your team:

- Each project has its own namespace for deploying instances
- Users belong to one or more projects with specific roles
- RGDs can be scoped to specific projects or shared globally

See [Project Management](../user-guide/project-management/) for details.

### RBAC Roles

knodex uses role-based access control within projects:

| Role               | Capabilities                                                             |
| ------------------ | ------------------------------------------------------------------------ |
| **Global Admin**   | Full access to all resources and settings across all projects            |
| **Platform Admin** | Manage project members, repositories, and all instances within a project |
| **Developer**      | Deploy and manage instances within a project                             |
| **Viewer**         | Read-only access to catalog and instances                                |

See [RBAC Setup](../operator-manual/rbac-setup/) for configuration details.

### Deployment Modes

When deploying an instance through knodex, you can choose how it's applied:

| Mode       | Description                                       | Best For                        |
| ---------- | ------------------------------------------------- | ------------------------------- |
| **Direct** | Apply directly to Kubernetes cluster              | Development, quick iterations   |
| **GitOps** | Commit manifest to Git repository for ArgoCD/Flux | Production with audit trail     |
| **Hybrid** | Deploy to cluster AND commit to Git               | Staging with immediate feedback |

See [Deployment Modes](../user-guide/deployment-modes/) for details.

---

**Next:** [Getting Started](../getting-started/) to begin using knodex.
