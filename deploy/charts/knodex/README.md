# knodex

![Version: 0.0.9](https://img.shields.io/badge/Version-0.0.9-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

A Helm chart for deploying Knodex - a Kubernetes-native UI for browsing and deploying Kro ResourceGraphDefinitions

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Provops | <tparis@provops.com> |  |

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | redis | 18.19.4 |
| oci://registry.k8s.io/kro/charts | kro | 0.8.5 |

## Installation

### Prerequisites

- Kubernetes 1.32+
- Helm 3.8+
- [Kro](https://kro.run) installed on your cluster (or enable `kro.enabled: true`)

### Install the chart

```bash
helm repo add knodex https://provops-org.github.io/knodex-helm
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
| crds.install | bool | `true` | Install the Project CRD (projects.knodex.io) |
| enterprise | object | `{"compliance":{"historyRetentionDays":""},"enabled":false,"gatekeeper":{"enabled":false},"image":{"repository":"ghcr.io/provops-org/knodex-ee"},"license":{"secretName":"","text":""},"networkPolicy":{"enabled":false,"server":{"additionalEgress":[],"additionalIngress":[],"ingressFrom":[]}},"organization":"","views":{"enabled":false,"items":[]}}` | Enterprise features |
| enterprise.compliance | object | `{"historyRetentionDays":""}` | Compliance configuration (Enterprise feature) |
| enterprise.compliance.historyRetentionDays | string | `""` (server default) | Violation history retention in days |
| enterprise.enabled | bool | `false` | Enable enterprise edition (uses knodex-ee image) |
| enterprise.gatekeeper | object | `{"enabled":false}` | OPA Gatekeeper integration (Enterprise feature) |
| enterprise.image | object | `{"repository":"ghcr.io/provops-org/knodex-ee"}` | Enterprise image configuration (overrides server.image.repository when enterprise.enabled=true) |
| enterprise.license.secretName | string | `""` (chart creates its own secret) | Name of an existing Kubernetes Secret containing the license JWT. The secret **must** contain a key named `license.jwt` with the raw JWT token as its value. If the key is missing, the license mount will silently be empty and enterprise features will not activate. The secret must exist in the same namespace as the Knodex release. If not set, the chart creates its own license secret (empty unless `enterprise.license.text` is also set). |
| enterprise.license.text | string | `""` (no inline license) | Inline license JWT text. When set, the chart creates a secret with key `license.jwt` containing this value **and** sets the `KNODEX_LICENSE_TEXT` environment variable. If both `secretName` and `text` are set, `secretName` takes precedence for the volume mount. |
| enterprise.networkPolicy | object | `{"enabled":false,"server":{"additionalEgress":[],"additionalIngress":[],"ingressFrom":[]}}` | Network policy configuration (Enterprise feature) |
| enterprise.organization | string | `""` (server defaults to `"default"`) | Organization identity for multi-tenant RGD catalog filtering (Enterprise feature). When set, only RGDs labeled with `knodex.io/organization: <value>` (or unlabeled shared RGDs) are visible in the catalog. Must be ≤63 characters (Kubernetes label value limit). |
| enterprise.views | object | `{"enabled":false,"items":[]}` | Custom views configuration (Enterprise feature) |
| externalRedis.host | string | `""` |  |
| externalRedis.password | string | `""` |  |
| externalRedis.port | int | `6379` |  |
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
| redis.auth.enabled | bool | `false` |  |
| redis.enabled | bool | `true` |  |
| redis.image.tag | string | `"7.2.4"` |  |
| redis.master.persistence.enabled | bool | `false` |  |
| redis.master.resources.limits.cpu | string | `"200m"` |  |
| redis.master.resources.limits.memory | string | `"128Mi"` |  |
| redis.master.resources.requests.cpu | string | `"50m"` |  |
| redis.master.resources.requests.memory | string | `"64Mi"` |  |
| server.auth | object | `{"adminUsername":"admin","casbin":{"adminUsers":[],"roleTTL":""},"jwt":{"expiry":"1h"},"localAccounts":{"accounts":{},"configMap":{"create":true},"secret":{"create":true}},"oidc":{"allowedRedirectOrigins":[],"enabled":false,"groupMappings":[],"groupMappingsFile":"","groupsClaim":"groups","providers":[],"rbacDefaultRole":""}}` | Authentication configuration |
| server.auth.casbin.adminUsers | list | `[]` | Bootstrap admin user IDs |
| server.auth.casbin.roleTTL | string | `""` (server defaults to 24h) | Role persistence TTL in Redis (e.g., "24h", "12h") |
| server.auth.oidc.allowedRedirectOrigins | list | `[]` | Allowed redirect origins for OIDC callbacks (CWE-601 open redirect protection) |
| server.auth.oidc.enabled | bool | `false` | Enable OIDC authentication |
| server.auth.oidc.groupMappingsFile | string | `""` | Path to a file-based OIDC group mappings YAML (alternative to inline groupMappings). When set, the server reads group mappings from this file path instead of the OIDC_GROUP_MAPPINGS env var. Mount the file via extraVolumes or an external ConfigMap. |
| server.auth.oidc.groupsClaim | string | `"groups"` | OIDC token claim name that contains user groups |
| server.auth.oidc.providers | list | `[]` | OIDC providers (creates knodex-sso-providers ConfigMap and knodex-sso-secrets Secret) |
| server.auth.oidc.rbacDefaultRole | string | `""` (server default) | Default RBAC role for OIDC users not matching any group mapping |
| server.autoscaling | object | `{"behavior":{"scaleDown":{"percentValue":25,"periodSeconds":60,"stabilizationWindowSeconds":300},"scaleUp":{"percentValue":100,"periodSeconds":15,"podsValue":2,"stabilizationWindowSeconds":0}},"enabled":false,"maxReplicas":5,"minReplicas":1,"targetCPUUtilizationPercentage":80,"targetMemoryUtilizationPercentage":80}` | Server autoscaling configuration (HorizontalPodAutoscaler) |
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
| server.image.pullPolicy | string | `"IfNotPresent"` |  |
| server.image.repository | string | `"ghcr.io/provops-org/knodex"` |  |
| server.image.tag | string | `"0.0.1"` |  |
| server.livenessProbe.failureThreshold | int | `3` |  |
| server.livenessProbe.httpGet.path | string | `"/healthz"` |  |
| server.livenessProbe.httpGet.port | string | `"http"` |  |
| server.livenessProbe.initialDelaySeconds | int | `10` |  |
| server.livenessProbe.periodSeconds | int | `30` |  |
| server.livenessProbe.timeoutSeconds | int | `5` |  |
| server.podSecurityContext | object | `{"fsGroup":10001,"runAsGroup":10001,"runAsNonRoot":true,"runAsUser":10001,"seccompProfile":{"type":"RuntimeDefault"}}` | Server pod security context (matches upstream Dockerfile UID 10001) |
| server.readinessProbe.failureThreshold | int | `3` |  |
| server.readinessProbe.httpGet.path | string | `"/readyz"` |  |
| server.readinessProbe.httpGet.port | string | `"http"` |  |
| server.readinessProbe.initialDelaySeconds | int | `5` |  |
| server.readinessProbe.periodSeconds | int | `10` |  |
| server.readinessProbe.timeoutSeconds | int | `5` |  |
| server.replicaCount | int | `1` |  |
| server.resources.requests.cpu | string | `"100m"` |  |
| server.resources.requests.memory | string | `"128Mi"` |  |
| server.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Server container security context |
| server.service.annotations | object | `{}` | Additional annotations for the server service |
| server.service.port | int | `8080` |  |
| server.service.type | string | `"ClusterIP"` |  |
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

RGDs without a `knodex.io/organization` label are visible to all organizations (shared catalog). See the [Organizations documentation](https://github.com/provops-org/knodex-ee/blob/main/docs/enterprise/organizations.md) for details.

## OIDC Configuration

Knodex supports OIDC authentication with providers such as:

- Microsoft Entra ID (Azure AD)
- Keycloak
- Google
- Okta

See `server.auth.oidc` values for configuration options.

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
