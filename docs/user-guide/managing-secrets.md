---
title: "Managing Secrets"
linkTitle: "Managing Secrets"
description: "Create, view, and manage Kubernetes Secrets used by RGD instances"
weight: 4
product_tags:
  - enterprise
---

{{< product-tag enterprise >}}

# Managing Secrets

> **Enterprise Feature:** Secrets management requires Knodex Enterprise. OSS builds do not include secrets endpoints or UI.

Create, view, and manage Kubernetes Secrets that your RGD instances reference via `externalRef`.

## Overview

Some RGDs reference existing Kubernetes Secrets (database credentials, API keys, TLS certificates) through the KRO `externalRef` mechanism. Knodex provides:

- **Secrets page** (`/secrets`) — Create, view, and delete Secrets within your projects
- **Catalog Secrets tab** — See which Secrets an RGD requires *before* deploying
- **Deploy form integration** — Select existing Secrets via dropdown when deploying

## Understanding Secret References in RGDs

An RGD author declares secret dependencies using `externalRef` resources with `kind: Secret`. There are three reference types:

| Type | Description | What you see in the catalog |
|------|-------------|-----------------------------|
| **user-provided** | You supply the Secret name and namespace at deploy time | ExternalRef name and description |
| **fixed** | The Secret name and namespace are hardcoded in the RGD | Literal name and namespace |
| **dynamic** | The Secret name is computed from other resources via CEL | CEL expressions |

### Checking Secret Requirements Before Deploying

1. Navigate to **Catalog** and open an RGD detail page
2. If the RGD references any Secrets, a **Secrets** tab appears (with a count badge)
3. Click the tab to see each secret reference with:
   - The **externalRef name** (e.g., `dbSecret`) — the semantic identifier
   - A **description** explaining what the secret is for (when the RGD author provides one)
   - A **type badge** (fixed, dynamic, or user-provided)
   - For fixed refs: the literal Secret name and namespace
   - For dynamic refs: the CEL expressions that resolve at deploy time

{{< alert title="Tip" >}}
Review the Secrets tab before deploying to ensure the required Secrets exist in your target namespace.
{{< /alert >}}

## The Secrets Page

Navigate to **Secrets** in the left sidebar to manage Kubernetes Secrets.

### Viewing Secrets

1. Select a **Project** from the dropdown
2. View all Secrets in namespaces accessible to that project
3. Each Secret shows: name, namespace, keys, and creation time

### Creating a Secret

1. Click **Create Secret**
2. Fill in:
   - **Name** — Kubernetes Secret name (lowercase, alphanumeric, hyphens)
   - **Namespace** — Target namespace (must be within the selected project)
   - **Data** — Key/value pairs (values are base64-encoded automatically)
3. Click **Create**

### Deleting a Secret

1. Click the **Delete** button on a Secret row
2. Confirm the deletion

{{< alert color="warning" title="Warning" >}}
Deleting a Secret that is referenced by a running instance may cause that instance to fail. Check which instances reference a Secret before deleting it.
{{< /alert >}}

### Permissions

Secret operations require Casbin permissions on the `secrets` resource:

| Action | Required Permission |
|--------|-------------------|
| List/View secrets | `secrets:get` on the project |
| Create secrets | `secrets:create` on the project |
| Delete secrets | `secrets:delete` on the project |

## Workflow: Deploying an RGD That Requires Secrets

### Step 1: Check Secret Requirements

1. Open the RGD in the **Catalog**
2. Click the **Secrets** tab
3. Note the required Secrets — their names, namespaces, and descriptions

### Step 2: Create the Secret (if it doesn't exist)

1. Go to **Secrets** in the sidebar
2. Select the same project you plan to deploy into
3. Click **Create Secret**
4. Use a name that matches what the RGD expects:
   - For **user-provided** refs: you choose the name freely (you will select it during deploy)
   - For **fixed** refs: the name must exactly match what the RGD specifies
5. Add the required key/value data

### Step 3: Deploy the Instance

1. Return to the RGD catalog detail page and click **Deploy**
2. For **user-provided** secret references, a resource picker dropdown appears — select your Secret
3. For **fixed** references, the Secret is referenced automatically — no action needed
4. Complete the rest of the form and deploy

## Best Practices

### Naming Secrets

Use descriptive names that indicate their purpose:

| Pattern | Example |
|---------|---------|
| `{service}-{purpose}` | `api-gateway-tls-cert` |
| `{app}-{env}-{type}` | `webapp-prod-db-credentials` |
| `{team}-{service}-{secret}` | `platform-redis-auth` |

### Organizing Secrets by Project

Secrets are scoped to Kubernetes namespaces, and namespaces are scoped to projects. Keep secrets in the same project (and namespace) as the instances that use them.

### Rotating Secrets

When rotating credentials:

1. Update the Secret data in **Secrets** page (or via `kubectl`)
2. Restart affected pods to pick up the new values (KRO does not auto-restart pods on Secret changes)

### RGD Authors: Adding Descriptions

When writing an RGD that references Secrets, add `description` markers to the `name` sub-field in the schema. This description appears in the catalog Secrets tab, helping users understand what each secret is for.

```yaml
spec:
  schema:
    spec:
      externalRef:
        dbSecret:
          name: string | default="" description="Name of the K8s Secret containing database credentials"
          namespace: string | default="" description="Namespace of the Secret"
```

The description from the `name` sub-field is displayed as the secret reference's purpose in the catalog detail view. See [Schema & UI — Secret Descriptions](../../catalog/schema-ui/#secret-reference-descriptions) for details.

---

**Previous:** [Managing Instances](../managing-instances/) | **Next:** [Deployment Modes](../deployment-modes/)
