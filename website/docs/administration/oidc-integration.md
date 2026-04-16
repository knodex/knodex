---
title: OIDC Integration
description: Configure single sign-on with OIDC providers including Azure AD, Google Workspace, Okta, Auth0, and Keycloak.
sidebar_position: 4
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# OIDC Integration

Knodex uses OpenID Connect (OIDC) for user authentication. This guide covers configuring OIDC providers, session management, and declarative SSO configuration.

## Overview

Knodex supports any OIDC-compliant identity provider. The server validates ID tokens, extracts user claims (email, name, groups), and maps OIDC groups to project roles via Casbin.

### Supported Providers

| Provider | Status | Notes |
|----------|--------|-------|
| Azure AD (Entra ID) | Tested in production | Recommended for enterprise |
| Google Workspace | Community tested | Requires domain-wide delegation for groups |
| Okta | Community tested | Standard OIDC configuration |
| Auth0 | Community tested | Requires Rules/Actions for group claims |
| Keycloak | Community tested | Self-hosted identity provider |

## Settings UI Configuration

The simplest way to configure OIDC is through the Knodex Settings UI.

### Add a Provider

Navigate to **Settings > SSO** and add a new OIDC provider with the following fields:

| Field | Required | Description |
|-------|----------|-------------|
| Provider Name | Yes | Display name (e.g., "Azure AD") |
| Issuer URL | Yes | OIDC discovery endpoint (e.g., `https://login.microsoftonline.com/<tenant>/v2.0`) |
| Client ID | Yes | Application/client ID from your identity provider |
| Client Secret | Yes | Client secret for confidential client flow |
| Redirect URL | Yes | Callback URL: `https://knodex.example.com/api/v1/auth/oidc/callback` |
| Scopes | No | Additional scopes beyond `openid` (see below) |

### Scopes

| Scope | Purpose | Required |
|-------|---------|----------|
| `openid` | Core OIDC | Always included automatically |
| `profile` | User name and profile info | Recommended |
| `email` | User email address | Recommended |
| `groups` | Group membership claims | Required for RBAC group mapping |

:::warning[Groups Scope]
The `groups` scope is essential for RBAC. Without it, users will authenticate but will not be assigned to any project roles. The exact scope name varies by provider (Azure AD uses `GroupMember.Read.All` as an API permission, not a scope).
:::

## Declarative Configuration (Helm / ArgoCD)

For GitOps workflows, configure SSO declaratively using a ConfigMap and Secret.

### ConfigMap for Non-Sensitive Settings

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
        "name": "Azure AD",
        "issuerURL": "https://login.microsoftonline.com/<tenant-id>/v2.0",
        "redirectURL": "https://knodex.example.com/api/v1/auth/oidc/callback",
        "scopes": ["openid", "email", "profile"]
      }
    ]
```

### Secret for Sensitive Settings

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

SSO configuration is split across two Kubernetes resources:

| Resource | Contains | Labels |
|----------|----------|--------|
| ConfigMap (`knodex-sso-providers`) | Provider name, issuer URL, redirect URL, scopes (as `providers.json`) | `app.kubernetes.io/managed-by: knodex`, `knodex.io/config-type: sso` |
| Secret (`knodex-sso-secrets`) | Client ID and client secret per provider (`<name>.client-id`, `<name>.client-secret`) | `app.kubernetes.io/managed-by: knodex`, `knodex.io/config-type: sso` |

Knodex watches for resources with the `knodex.io/config-type: sso` label and automatically loads them at startup.

### Hot-Reload Behavior

| Change | Behavior |
|--------|----------|
| ConfigMap updated | Picked up on next server restart |
| Secret updated | Picked up on next server restart |
| New ConfigMap/Secret created | Picked up on next server restart |
| SSO configured via Settings UI | Applied immediately |

:::note[Restart Required]
Declarative SSO changes (ConfigMap/Secret) require a server restart to take effect. Changes made through the Settings UI are applied immediately.
:::

## Provider Guides

### Azure AD (Entra ID)

Azure AD is the most thoroughly tested provider. Follow these steps:

**Step 1: Register an Application**

1. Go to **Azure Portal > Microsoft Entra ID > App registrations > New registration**
2. Set the redirect URI to `https://knodex.example.com/api/v1/auth/oidc/callback` (Web platform)
3. Note the **Application (client) ID** and **Directory (tenant) ID**

**Step 2: Create a Client Secret**

1. Go to **Certificates & secrets > New client secret**
2. Set an expiration period and save the secret value

**Step 3: Configure API Permissions**

1. Go to **API permissions > Add a permission > Microsoft Graph**
2. Add **Delegated permissions**: `openid`, `profile`, `email`, `User.Read`
3. For group claims: Add **Delegated permission** `GroupMember.Read.All`
4. Grant admin consent for the permissions

**Step 4: Configure Token Claims**

1. Go to **Token configuration > Add groups claim**
2. Select **Security groups** (or All groups)
3. For the ID token, select **Group ID**

**Step 5: Configure Knodex**

```yaml
server:
  config:
    OIDC_ISSUER_URL: "https://login.microsoftonline.com/<tenant-id>/v2.0"
    OIDC_CLIENT_ID: "<application-client-id>"
  secrets:
    OIDC_CLIENT_SECRET: "<client-secret-value>"
```

### Google Workspace

1. Create an OAuth 2.0 client in the [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Set the authorized redirect URI to `https://knodex.example.com/api/v1/auth/oidc/callback`
3. Enable the **Google Workspace Admin SDK** for group membership
4. Configure:

```yaml
server:
  config:
    OIDC_ISSUER_URL: "https://accounts.google.com"
    OIDC_CLIENT_ID: "<client-id>.apps.googleusercontent.com"
  secrets:
    OIDC_CLIENT_SECRET: "<client-secret>"
```

### Okta

1. Create an OIDC Web Application in the Okta Admin Console
2. Set the sign-in redirect URI to `https://knodex.example.com/api/v1/auth/oidc/callback`
3. Assign groups to the application
4. Add a `groups` claim to the ID token under **Security > API > Authorization Servers**
5. Configure:

```yaml
server:
  config:
    OIDC_ISSUER_URL: "https://<your-domain>.okta.com"
    OIDC_CLIENT_ID: "<client-id>"
  secrets:
    OIDC_CLIENT_SECRET: "<client-secret>"
```

### Auth0

1. Create a Regular Web Application in the Auth0 Dashboard
2. Set the callback URL to `https://knodex.example.com/api/v1/auth/oidc/callback`
3. Add a Rule or Action to include `groups` in the ID token
4. Configure:

```yaml
server:
  config:
    OIDC_ISSUER_URL: "https://<your-tenant>.auth0.com/"
    OIDC_CLIENT_ID: "<client-id>"
  secrets:
    OIDC_CLIENT_SECRET: "<client-secret>"
```

### Keycloak

1. Create a client in your Keycloak realm with **Client authentication** enabled
2. Set the valid redirect URI to `https://knodex.example.com/api/v1/auth/oidc/callback`
3. Add a **Group Membership** mapper to include groups in the ID token
4. Configure:

```yaml
server:
  config:
    OIDC_ISSUER_URL: "https://keycloak.example.com/realms/<realm>"
    OIDC_CLIENT_ID: "<client-id>"
  secrets:
    OIDC_CLIENT_SECRET: "<client-secret>"
```

:::note[Provider Testing]
Only Azure AD has been tested in production environments. Other providers follow standard OIDC flows and should work, but may require provider-specific adjustments.
:::

## Session Cookie Configuration

Session cookies control how the browser maintains the authenticated session.

### Helm Values

```yaml
server:
  config:
    COOKIE_SECURE: "true"       # Require HTTPS (disable for local dev only)
    COOKIE_DOMAIN: ".example.com"  # Share cookies across subdomains
```

### Cookie Reference

| Setting | Default | Description |
|---------|---------|-------------|
| `COOKIE_SECURE` | `true` | Set the `Secure` flag. Must be `true` for HTTPS deployments |
| `COOKIE_DOMAIN` | `""` (request host) | Cookie domain. Set to share across subdomains |
| Cookie Name | `knodex_session` | Not configurable |
| SameSite | `Lax` | Not configurable |
| HttpOnly | `true` | Not configurable |

## Test the Login Flow

After configuring OIDC:

1. Access Knodex at your configured URL
2. You should be redirected to the OIDC provider's login page
3. After authentication, you are redirected back to Knodex
4. Verify your identity by checking **Account** in the UI
5. Confirm group membership by checking that you see the expected projects
