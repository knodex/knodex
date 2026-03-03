---
title: "OIDC Integration"
linkTitle: "OIDC Integration"
description: "Enterprise SSO authentication setup with provider-specific guides"
weight: 4
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# OIDC Integration

Enterprise SSO authentication setup with provider-specific guides.

## Overview

knodex supports OpenID Connect (OIDC) for enterprise authentication with:

- Standard OIDC 1.0 compliance
- Support for major providers (Okta, Auth0, Azure AD, Google, Keycloak)
- Group-based role assignment
- Automatic user provisioning

## Supported Providers

knodex is designed to work with any OIDC-compliant identity provider:

- Azure AD / Microsoft Entra ID
- Google Workspace
- Okta
- Auth0
- Keycloak
- Any OIDC 1.0 compliant provider

## Configuration

OIDC providers are configured through the **Settings UI** or via **Helm/ArgoCD-managed Kubernetes manifests**. Both methods write to the same Kubernetes resources (ConfigMap + Secret), and the server automatically detects changes within seconds — no pod restart required.

### Settings UI

Admins can manage OIDC providers directly from the knodex UI.

**1. Navigate to Settings > SSO:**

1. Log in as an admin
2. Open **Settings** from the sidebar
3. Click the **SSO** card

**2. Add a Provider:**

1. Click **Add Provider**
2. Fill in the required fields:

| Field             | Description                                             | Example                                                |
| ----------------- | ------------------------------------------------------- | ------------------------------------------------------ |
| **Name**          | DNS-label identifier (lowercase, hyphens, max 63 chars) | `azure-ad`                                             |
| **Issuer URL**    | OIDC discovery endpoint (HTTPS required)                | `https://login.microsoftonline.com/{tenant}/v2.0`      |
| **Client ID**     | OAuth2 client identifier from your IdP                  | `a1b2c3d4-...`                                         |
| **Client Secret** | OAuth2 client secret                                    | `secret-value`                                         |
| **Redirect URL**  | Callback URL registered with your IdP                   | `https://knodex.example.com/api/v1/auth/oidc/callback` |
| **Scopes**        | Comma-separated OIDC scopes                             | `openid,email,profile`                                 |

3. Click **Save**

**3. Scopes:**

The default scopes are `openid,email,profile`. These cover standard identity claims. Additional scopes depend on your IdP:

| Scope            | Purpose                          | Notes                                                                                                                                                     |
| ---------------- | -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `openid`         | Required for OIDC                | Always include this                                                                                                                                       |
| `email`          | Returns email address claim      | Standard                                                                                                                                                  |
| `profile`        | Returns name/display name claims | Standard                                                                                                                                                  |
| `groups`         | Returns group membership claim   | IdP-specific — some IdPs (e.g., Keycloak) expose this as a scope; others (e.g., Azure AD) return groups via token configuration without a dedicated scope |
| `offline_access` | Enables refresh tokens           | Not currently used by knodex                                                                                                                              |

{{< alert title="Note" >}}
`groups` is typically a **claim** in the ID token, not a standard OIDC scope. Whether you need to request a `groups` scope depends on your IdP. See the provider-specific guides below.
{{< /alert >}}

**4. Edit or Delete:**

- Click a provider row to edit its configuration. Client secret is only updated if a new value is provided.
- Click the delete icon and type the provider name to confirm deletion.

### Helm / ArgoCD (GitOps)

For GitOps-managed deployments, configure providers declaratively via Kubernetes manifests. Helm or ArgoCD manages the ConfigMap and Secret; the server watches for changes and reloads automatically.

**ConfigMap** (`knodex-sso-providers`):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: knodex-sso-providers
  namespace: knodex
  labels:
    app.kubernetes.io/managed-by: knodex
    knodex.io/config-type: sso
data:
  providers.json: |
    [
      {
        "name": "azure-ad",
        "issuerURL": "https://login.microsoftonline.com/{tenant-id}/v2.0",
        "redirectURL": "https://knodex.example.com/api/v1/auth/oidc/callback",
        "scopes": ["openid", "email", "profile"]
      }
    ]
```

**Secret** (`knodex-sso-secrets`):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: knodex-sso-secrets
  namespace: knodex
  labels:
    app.kubernetes.io/managed-by: knodex
    knodex.io/config-type: sso
type: Opaque
stringData:
  azure-ad.client-id: "your-client-id"
  azure-ad.client-secret: "your-client-secret"
```

### How SSO Configuration Works

Provider configuration is split across two Kubernetes resources:

| Resource  | Name                   | Contents                                                                |
| --------- | ---------------------- | ----------------------------------------------------------------------- |
| ConfigMap | `knodex-sso-providers` | Non-sensitive config: name, issuer URL, redirect URL, scopes            |
| Secret    | `knodex-sso-secrets`   | Credentials: `{name}.client-id` and `{name}.client-secret` per provider |

**ConfigMap schema** (`providers.json` key):

```json
[
  {
    "name": "provider-name",
    "issuerURL": "https://issuer.example.com",
    "redirectURL": "https://knodex.example.com/api/v1/auth/oidc/callback",
    "scopes": ["openid", "email", "profile"]
  }
]
```

**Secret key format**: each provider has two keys using the pattern `{provider-name}.client-id` and `{provider-name}.client-secret`.

### How SSO Hot-Reload Works

The knodex server uses a Kubernetes informer to watch the `knodex-sso-providers` ConfigMap and `knodex-sso-secrets` Secret. When either resource changes, the server automatically reloads OIDC providers within seconds — no pod restart required.

**Reload behavior:**

| Event                          | Server behavior                            |
| ------------------------------ | ------------------------------------------ |
| ConfigMap updated (valid JSON) | Providers reloaded from new config         |
| Secret updated                 | Credentials refreshed for all providers    |
| ConfigMap deleted              | Last valid providers kept (does not crash) |
| Malformed JSON in ConfigMap    | Error logged, last valid config retained   |
| No ConfigMap at startup        | Server starts with zero OIDC providers     |

The informer performs a full resync every 30 seconds, so even if a watch event is missed, configuration converges within 30 seconds.

{{< alert title="Note" >}}
In-flight authentication flows are not disrupted during a reload. Active OIDC login sessions complete using the provider configuration that was valid when the flow started.
{{< /alert >}}

## Provider-Specific Guides

{{< alert title="Testing Status" >}}
**Only Azure AD (Microsoft Entra ID) has been tested in production.** The other provider guides are based on standard OIDC configuration patterns but have not been validated.

We welcome contributions! If you successfully configure knodex with another provider, please submit a PR with any corrections or improvements to these guides.
{{< /alert >}}

### Azure AD (Microsoft Entra ID)

**1. Register Application:**

1. Navigate to **Azure Active Directory** → **App registrations** → **New registration**
2. Configure:
   - **Name:** knodex
   - **Redirect URI:** `https://knodex.example.com/api/v1/auth/oidc/callback`

**2. Create Client Secret:**

1. Navigate to **Certificates & secrets**
2. Create new client secret
3. Copy the secret value

**3. Configure Groups:**

1. Navigate to **Token configuration**
2. Add optional claim:
   - Token type: **ID**
   - Claim: **groups**

Azure AD returns groups in the ID token via token configuration — no `groups` scope is needed. Use scopes `openid,email,profile`.

**4. API Permissions:**

1. Navigate to **API permissions**
2. Add permissions:
   - Microsoft Graph → Delegated permissions
   - Add: `User.Read`, `GroupMember.Read.All`
3. Grant admin consent

**5. Configure in knodex:**

Via the **Settings UI**, create a provider with:

| Field         | Value                                                  |
| ------------- | ------------------------------------------------------ |
| Name          | `azure-ad`                                             |
| Issuer URL    | `https://login.microsoftonline.com/{tenant-id}/v2.0`   |
| Client ID     | `{application-id}`                                     |
| Client Secret | `{secret-value}`                                       |
| Redirect URL  | `https://knodex.example.com/api/v1/auth/oidc/callback` |
| Scopes        | `openid,email,profile`                                 |

Or via Helm/ArgoCD manifests — add an entry to the `knodex-sso-providers` ConfigMap `providers.json` and set credentials in the `knodex-sso-secrets` Secret (see [Helm / ArgoCD](#helm--argocd-gitops) above).

### Test Login Flow

1. Add a provider via the Settings UI or deploy via Helm/ArgoCD (changes take effect within seconds)
2. Navigate to knodex
3. Click "Login with SSO"
4. Authenticate with your OIDC provider
5. Verify redirect back to knodex
6. Check user profile shows correct email and groups

---

**Next:** [Kubernetes RBAC](../kubernetes-rbac/) →
