# knodex

![Version: 0.5.0](https://img.shields.io/badge/Version-0.5.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.5.0](https://img.shields.io/badge/AppVersion-0.5.0-informational?style=flat-square)

A Helm chart for deploying Knodex - a Kubernetes-native UI for browsing and deploying Kro ResourceGraphDefinitions

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Provops | <maintainers@knodex.io> |  |

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | redis | 18.19.4 |
| oci://registry.k8s.io/kro/charts | kro | 0.9.1 |

## Installation

### Prerequisites

- Kubernetes 1.32+
- Helm 3.8+
- [Kro](https://kro.run) installed on your cluster (or enable `kro.enabled: true`)

### Install the chart

```bash
helm repo add knodex https://knodex.github.io/knodex-helm
helm repo update
helm install knodex knodex/knodex -n knodex --create-namespace
```

### Get the initial admin password

```bash
kubectl get secret knodex-initial-admin-password -n knodex -o jsonpath='{.data.password}' | base64 -d
```

## Configuration

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| catalog | object | `{"categoryConfig":[],"customIcons":{}}` | Catalog configuration |
| catalog.categoryConfig | list | `[]` (no sidebar sub-nav) | Sidebar category ordering. Defines which categories appear in the sidebar sub-nav, in what order, and with optional icon overrides. When empty (default), no category sub-nav is shown. Changes require a server restart to take effect. |
| catalog.customIcons | object | `{}` (no custom icons) | Custom icon registry. Add brand SVG icons not included in the built-in set. Each key is the icon slug (lowercase letters, digits, and hyphens only); the value is the inline SVG content. Creates a ConfigMap labeled `knodex.io/icon-registry: "true"` so the server picks it up. Changes require a server restart to take effect. |
| crds.install | bool | `true` | Install the Project CRD (projects.knodex.io) |
| defaultProject.create | bool | `true` | Create the default project on install/upgrade |
| defaultProject.name | string | `"default"` | Name of the default project |
| defaultProject.spec.description | string | `"Default project - allows deployments to any namespace"` |  |
| defaultProject.spec.destinations | list | `[{"namespace":"default"}]` | Allowed deployment destinations |
| defaultProject.spec.roles | list | `[]` | Roles for the default project (optional) |
| dex | object | `{"affinity":{},"config":{"disableTLS":true,"issuerURL":"","knodexClientSecret":"","knodexRedirectURL":"","logLevel":"info","tlsSecretName":""},"enabled":false,"image":{"pullPolicy":"IfNotPresent","repository":"ghcr.io/dexidp/dex","tag":"v2.45.1"},"nodeSelector":{},"podAnnotations":{},"podLabels":{},"replicaCount":1,"resources":{"limits":{},"requests":{"cpu":"50m","memory":"64Mi"}},"service":{"annotations":{},"grpcPort":5557,"httpPort":5556,"metricsPort":5558,"type":"ClusterIP"},"tolerations":[]}` | ---------------------------------------------------------------------------- When enabled, Dex runs on the management cluster as an OIDC proxy. It reads Knodex SSO provider config and translates it into Dex connectors, allowing all managed tools (ArgoCD, Grafana, etc.) to authenticate via a single Dex endpoint backed by the customer's IDP (Entra ID, Okta, Google). |
| dex.affinity | object | `{}` | Affinity for Dex pods (overrides global) |
| dex.config.disableTLS | bool | `true` | Run Dex in HTTP mode (set to false for production with TLS) |
| dex.config.issuerURL | string | `""` | Public URL of the Dex instance (must be reachable by end users' browsers) |
| dex.config.knodexClientSecret | string | `""` | OAuth2 client secret for the Knodex static client. If empty, auto-generated. |
| dex.config.knodexRedirectURL | string | `""` | OAuth2 callback URL for Knodex (e.g., https://knodex.example.com/auth/callback) |
| dex.config.logLevel | string | `"info"` | Dex log level (debug, info, warn, error) |
| dex.config.tlsSecretName | string | `""` | TLS secret name (when disableTLS is false) |
| dex.enabled | bool | `false` | Enable Dex OIDC federation proxy |
| dex.nodeSelector | object | `{}` | Node selector for Dex pods (overrides global) |
| dex.podAnnotations | object | `{}` | Annotations for Dex pods |
| dex.podLabels | object | `{}` | Labels for Dex pods |
| dex.replicaCount | int | `1` | Number of Dex server replicas |
| dex.tolerations | list | `[]` | Tolerations for Dex pods (overrides global) |
| enterprise | object | `{"audit":{"redactFields":["privateKey","password","bearerToken","token","secret","tlsClientCert","tlsClientKey","clientSecret"]},"compliance":{"historyRetentionDays":""},"enabled":false,"gatekeeper":{"enabled":false},"image":{"repository":"ghcr.io/knodex/knodex-ee"},"license":{"existingSecret":"","text":""},"networkPolicy":{"enabled":false,"server":{"additionalEgress":[],"additionalIngress":[],"ingressFrom":[]}},"organization":""}` | Enterprise features |
| enterprise.audit | object | `{"redactFields":["privateKey","password","bearerToken","token","secret","tlsClientCert","tlsClientKey","clientSecret"]}` | Audit configuration (Enterprise feature) |
| enterprise.audit.redactFields | list | `["privateKey","password","bearerToken","token","secret","tlsClientCert","tlsClientKey","clientSecret"]` | Field names to redact from audit event details (case-insensitive). The server strips any matching key from audit Details as a defense-in-depth safety net. Operators have full control: add custom fields or remove defaults that conflict with legitimate field names in your CRDs. |
| enterprise.compliance | object | `{"historyRetentionDays":""}` | Compliance configuration (Enterprise feature) |
| enterprise.compliance.historyRetentionDays | string | `""` (server default) | Violation history retention in days |
| enterprise.enabled | bool | `false` | Enable enterprise edition (uses knodex-ee image) |
| enterprise.gatekeeper | object | `{"enabled":false}` | OPA Gatekeeper integration (Enterprise feature) |
| enterprise.image | object | `{"repository":"ghcr.io/knodex/knodex-ee"}` | Enterprise image configuration (overrides server.image.repository when enterprise.enabled=true) |
| enterprise.license.existingSecret | string | `""` (chart creates its own secret) | Name of an existing Kubernetes Secret containing the license JWT. The secret **must** contain a key named `license.jwt` with the raw JWT token as its value. If the key is missing, the license mount will silently be empty and enterprise features will not activate. The secret must exist in the same namespace as the Knodex release. |
| enterprise.license.text | string | `""` (no inline license) | Inline license JWT text. When set, the chart creates a secret with key `license.jwt` containing this value **and** sets the `KNODEX_LICENSE_TEXT` environment variable. If both `existingSecret` and `text` are set, `existingSecret` takes precedence for the volume mount. |
| enterprise.networkPolicy | object | `{"enabled":false,"server":{"additionalEgress":[],"additionalIngress":[],"ingressFrom":[]}}` | Network policy configuration (Enterprise feature) |
| enterprise.organization | string | `""` (server defaults to `"default"`) | Organization identity for multi-tenant RGD catalog filtering (Enterprise feature). When set, only RGDs labeled with `knodex.io/organization: <value>` (or unlabeled shared RGDs) are visible in the catalog. Must be ≤63 characters (Kubernetes label value limit). |
| externalRedis.host | string | `""` |  |
| externalRedis.password | string | `""` |  |
| externalRedis.port | int | `6379` |  |
| externalRedis.tls.enabled | bool | `false` |  |
| externalRedis.tls.insecureSkipVerify | bool | `false` |  |
| externalRedis.username | string | `""` |  |
| fullnameOverride | string | `""` |  |
| gateway.annotations | object | `{}` | Annotations for the HTTPRoute |
| gateway.enabled | bool | `false` | Enable HTTPRoute resource (requires Gateway API CRDs) |
| gateway.hostnames | list | `["knodex.staging.knodex.io"]` | Hostnames for the HTTPRoute |
| gateway.parentRefs | list | `[{"name":"internal-gateway","namespace":"kube-system"}]` | Parent gateway references |
| global | object | `{"affinity":{},"imagePullSecrets":[{"name":"ghcr-secret"}],"nodeSelector":{},"tolerations":[]}` | Global settings shared across all pods |
| global.affinity | object | `{}` | Affinity rules for all pods |
| global.imagePullSecrets | list | `[{"name":"ghcr-secret"}]` | Image pull secrets for all pods |
| global.nodeSelector | object | `{}` | Node selector for all pods |
| global.tolerations | list | `[]` | Tolerations for all pods |
| ingress.annotations | object | `{}` |  |
| ingress.className | string | `"nginx"` |  |
| ingress.enabled | bool | `false` |  |
| ingress.hosts[0].host | string | `"knodex.local"` |  |
| ingress.hosts[0].paths[0].path | string | `"/"` |  |
| ingress.hosts[0].paths[0].pathType | string | `"Prefix"` |  |
| ingress.tls | list | `[]` |  |
| kro | object | `{"enabled":false}` | ---------------------------------------------------------------------------- |
| nameOverride | string | `""` |  |
| rbac.create | bool | `true` |  |
| redis.architecture | string | `"standalone"` |  |
| redis.auth.enabled | bool | `true` |  |
| redis.auth.existingSecret | string | `'{{ include "knodex.redisSecretName" . }}'` | Secret containing the Redis password. The default tpl expression points to the chart-managed secret (created by a Helm hook Job). For production, replace with an ExternalSecret-managed Secret name. |
| redis.auth.existingSecretPasswordKey | string | `"redis-password"` (Bitnami default) | Key within the existingSecret that holds the password. |
| redis.auth.password | string | `""` | Explicit password (option 3). When set, the chart-managed secret uses this value instead of generating a random password. Ignored when using a custom existingSecret. |
| redis.enabled | bool | `true` |  |
| redis.image.digest | string | `"sha256:5179ef5fcc0aee9b3a16e8030ea7b1a81f94033c06e1676c0c4b18c237de2e82"` |  |
| redis.image.tag | string | `""` |  |
| redis.master.persistence.enabled | bool | `false` |  |
| redis.master.resources.limits.cpu | string | `"200m"` |  |
| redis.master.resources.limits.memory | string | `"128Mi"` |  |
| redis.master.resources.requests.cpu | string | `"50m"` |  |
| redis.master.resources.requests.memory | string | `"64Mi"` |  |
| server.auth | object | `{"adminUsername":"admin","casbin":{"adminUsers":[],"roleTTL":""},"jwt":{"expiry":"1h"},"localAccounts":{"accounts":{},"configMap":{"create":true},"secret":{"create":true}},"localLogin":{"enabled":true},"oidc":{"allowedRedirectOrigins":[],"enabled":false,"existingSecret":"","groupMappings":[],"groupMappingsFile":"","groupsClaim":"groups","providers":[],"rbacDefaultRole":""}}` | Authentication configuration |
| server.auth.casbin.adminUsers | list | `[]` | Bootstrap admin user IDs |
| server.auth.casbin.roleTTL | string | `""` (server defaults to 24h) | Role persistence TTL in Redis (e.g., "24h", "12h") |
| server.auth.localLogin | object | `{"enabled":true}` | Local user login pathway When disabled (false), the server:   - Skips creating the knodex-initial-admin-password Secret   - Returns 403 from POST /api/v1/auth/local/login (blocking ALL local accounts)   - Frontend hides the local login form Use this for SSO-only deployments. To break-glass, flip back to true and re-deploy. |
| server.auth.oidc.allowedRedirectOrigins | list | `[]` | Allowed redirect origins for OIDC callbacks (CWE-601 open redirect protection) |
| server.auth.oidc.enabled | bool | `false` | Enable OIDC authentication |
| server.auth.oidc.existingSecret | string | `""` (chart creates its own secret) | Name of an existing Kubernetes Secret containing SSO credentials. The secret must contain keys: `<provider-name>.client-id` and `<provider-name>.client-secret` for each configured provider. When set, the chart skips creating its own SSO secret. |
| server.auth.oidc.groupMappingsFile | string | `""` | Path to a file-based OIDC group mappings YAML (alternative to inline groupMappings). When set, the server reads group mappings from this file path instead of the OIDC_GROUP_MAPPINGS env var. Mount the file via extraVolumes or an external ConfigMap. |
| server.auth.oidc.groupsClaim | string | `"groups"` | OIDC token claim name that contains user groups |
| server.auth.oidc.providers | list | `[]` | OIDC providers (creates knodex-sso-providers ConfigMap and knodex-sso-secrets Secret) |
| server.auth.oidc.rbacDefaultRole | string | `""` (server default) | Default RBAC role for OIDC users not matching any group mapping |
| server.autoscaling | object | `{"behavior":{"scaleDown":{"percentValue":25,"periodSeconds":60,"stabilizationWindowSeconds":300},"scaleUp":{"percentValue":100,"periodSeconds":15,"podsValue":2,"stabilizationWindowSeconds":0}},"enabled":false,"maxReplicas":5,"minReplicas":1,"targetCPUUtilizationPercentage":80,"targetMemoryUtilizationPercentage":80}` | Server autoscaling configuration (HorizontalPodAutoscaler) |
| server.config.catalogPackageFilter | string | `""` (no filtering) | Comma-separated list of package names to filter RGD catalog ingestion. Only RGDs with a matching `knodex.io/package` label are ingested. When empty (default), all catalog-annotated RGDs are ingested. |
| server.config.cookie.domain | string | `""` (same-origin) | Domain attribute on session cookies. Set for cross-subdomain auth (e.g., ".example.com"). |
| server.config.cookie.secure | bool | `true` | Secure flag on session cookies (requires HTTPS). Set to false for local HTTP development. |
| server.config.corsAllowedOrigins | string | `""` (server defaults to same-origin) | Comma-separated list of allowed CORS origins (required when behind ingress/load balancer) |
| server.config.logFormat | string | `"json"` |  |
| server.config.logLevel | string | `"info"` |  |
| server.config.policyCache.enabled | bool | `true` |  |
| server.config.policyCache.syncIntervalMinutes | int | `10` |  |
| server.config.policyCache.ttlSeconds | int | `300` |  |
| server.config.policyCache.watchEnabled | bool | `true` |  |
| server.config.rateLimit.trustedProxies | list | `[]` | Trusted proxy CIDRs for X-Forwarded-For header parsing. Required behind load balancers to correctly identify client IPs for rate limiting. |
| server.config.rateLimit.userBurstSize | int | `100` |  |
| server.config.rateLimit.userRequestsPerMinute | int | `100` |  |
| server.config.serverAddress | string | `":8080"` |  |
| server.config.swaggerUI | bool | `false` | Enable Swagger UI endpoint for API documentation |
| server.dnsConfig | object | `{}` | DNS configuration for server pods |
| server.dnsPolicy | string | `""` | DNS policy for server pods |
| server.extraEnv | list | `[]` | Extra environment variables for the server container NOTE: Variables here are appended after chart-managed env vars. Duplicate names will override chart defaults (Kubernetes uses last value). |
| server.extraEnvFrom | list | `[]` | Extra environment variable sources (ConfigMaps, Secrets) WARNING: Allows mounting arbitrary Secrets/ConfigMaps into the server process. Ensure Helm release access is restricted in multi-tenant environments. |
| server.extraVolumeMounts | list | `[]` | Extra volume mounts for the server container |
| server.extraVolumes | list | `[]` | Extra volumes for the server pod |
| server.image.pullPolicy | string | `"IfNotPresent"` |  |
| server.image.repository | string | `"ghcr.io/knodex/knodex-ee"` |  |
| server.image.tag | string | `""` | Overrides the image tag. Defaults to the chart appVersion. |
| server.lifecycle | object | `{}` | Lifecycle hooks for the server container |
| server.livenessProbe.failureThreshold | int | `3` |  |
| server.livenessProbe.httpGet.path | string | `"/healthz"` |  |
| server.livenessProbe.httpGet.port | string | `"http"` |  |
| server.livenessProbe.initialDelaySeconds | int | `10` |  |
| server.livenessProbe.periodSeconds | int | `30` |  |
| server.livenessProbe.timeoutSeconds | int | `5` |  |
| server.pdb | object | `{"minAvailable":1}` | PodDisruptionBudget configuration |
| server.pdb.minAvailable | int | `1` | Minimum number of available pods (mutually exclusive with maxUnavailable) |
| server.podAnnotations | object | `{}` | Annotations to add to server pods |
| server.podLabels | object | `{}` | Labels to add to server pods |
| server.podSecurityContext | object | `{"fsGroup":10001,"runAsGroup":10001,"runAsNonRoot":true,"runAsUser":10001,"seccompProfile":{"type":"RuntimeDefault"}}` | Server pod security context (matches upstream Dockerfile UID 10001) |
| server.priorityClassName | string | `""` | Priority class name for server pods |
| server.readinessProbe.failureThreshold | int | `3` |  |
| server.readinessProbe.httpGet.path | string | `"/readyz"` |  |
| server.readinessProbe.httpGet.port | string | `"http"` |  |
| server.readinessProbe.initialDelaySeconds | int | `5` |  |
| server.readinessProbe.periodSeconds | int | `10` |  |
| server.readinessProbe.timeoutSeconds | int | `5` |  |
| server.replicaCount | int | `1` |  |
| server.resources.limits | object | `{}` |  |
| server.resources.requests.cpu | string | `"100m"` |  |
| server.resources.requests.memory | string | `"128Mi"` |  |
| server.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Server container security context |
| server.service.annotations | object | `{}` | Additional annotations for the server service |
| server.service.port | int | `8080` |  |
| server.service.type | string | `"ClusterIP"` |  |
| server.startupProbe | object | `{}` | Startup probe for slow-starting containers |
| server.strategy | object | `{}` | Deployment strategy |
| server.topologySpreadConstraints | list | `[]` | Topology spread constraints for server pods |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |

## Organization (Enterprise)

Knodex Enterprise supports multi-tenant organization isolation. Each deployment can belong to one organization, and RGDs are filtered by the `knodex.io/organization` label:

```yaml
enterprise:
  enabled: true
  organization: "acme-corp"
```

RGDs without a `knodex.io/organization` label are visible to all organizations (shared catalog). See the [Organizations documentation](https://github.com/knodex/knodex/blob/main/docs/enterprise/organizations.md) for details.

## OIDC Configuration

Knodex supports OIDC authentication with providers such as:

- Microsoft Entra ID (Azure AD)
- Keycloak
- Google
- Okta

See `server.auth.oidc` values for configuration options.

## SSO-Only Deployments (Disabling Local Login)

For deployments where SSO is the sole authentication path, local user login
can be disabled with `server.auth.localLogin.enabled: false`. This blocks
login for ALL local accounts (admin and any other). When disabled, the server:

- Skips creating the `knodex-initial-admin-password` Secret on startup.
- **Skips registering** `POST /api/v1/auth/local/login` — requests return 404.
  This prevents attackers from draining the login rate-limit budget or
  flooding the audit log with fabricated login attempts.
- Reports `localLoginEnabled: false` from `GET /api/v1/auth/oidc/providers`,
  causing the frontend to hide the local login form.

```yaml
server:
  auth:
    localLogin:
      enabled: false
    oidc:
      enabled: true
      providers:
        - name: my-idp
          issuerURL: https://idp.example.com
          clientID: knodex
```

### Disabling on an Existing Deployment

> **IMPORTANT:** if the chart was previously installed with local login
> enabled, the `knodex-initial-admin-password` Secret already exists in the
> namespace and will persist after disabling. The login route is removed and
> the handler refuses authentication, so the Secret is no longer reachable
> through the API — but a stale credential sitting in `etcd` is still a
> latent privilege-escalation surface. **Delete it manually:**
>
> ```sh
> kubectl delete secret knodex-initial-admin-password -n <namespace>
> ```

### Break-glass Procedure

To temporarily restore local login (for example, when SSO is unavailable):

1. Set `server.auth.localLogin.enabled: true` in your values and `helm upgrade`.
2. Restart the server pod — startup will recreate the Secret if you previously
   deleted it.
3. Retrieve the admin password:

   ```sh
   kubectl get secret knodex-initial-admin-password \
     -n <namespace> -o jsonpath='{.data.password}' | base64 -d && echo
   ```

4. After break-glass, set `enabled: false` again, re-deploy, and re-delete the
   Secret per the section above.

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
