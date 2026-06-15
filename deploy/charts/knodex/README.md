# knodex

![Version: 0.7.0](https://img.shields.io/badge/Version-0.7.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.7.0](https://img.shields.io/badge/AppVersion-0.7.0-informational?style=flat-square)

A Helm chart for deploying Knodex - a Kubernetes-native UI for browsing and deploying Kro ResourceGraphDefinitions

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Provops | <maintainers@knodex.io> |  |

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | postgresql | 16.7.27 |
| https://charts.bitnami.com/bitnami | redis | 18.19.4 |
| oci://registry.k8s.io/kro/charts | kro | 0.9.2 |

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
| catalog | object | `{"categoryConfig":[],"customIcons":{},"packageCategoryConfigs":{},"packageCustomIcons":{}}` | Catalog configuration |
| catalog.categoryConfig | list | `[]` (no sidebar sub-nav) | Sidebar category ordering. Defines which categories appear in the sidebar sub-nav, in what order, and with optional icon overrides. When empty (default), no category sub-nav is shown. Changes require a server restart to take effect. |
| catalog.customIcons | object | `{}` (no custom icons) | Custom icon registry. Add brand SVG icons not included in the built-in set. Each key is the icon slug (lowercase letters, digits, and hyphens only); the value is the inline SVG content. Creates a ConfigMap labeled `knodex.io/icon-registry: "true"` so the server picks it up. Changes require a server restart to take effect. |
| catalog.packageCategoryConfigs | object | `{}` (no per-package configs) | Per-package category configs. Each key is the package name (must match a knodex.io/package label on RGDs). Generates one ConfigMap per key, labeled knodex.io/category-config=true and knodex.io/package=<package-name>. Activated only when that package is in CATALOG_PACKAGE_FILTER. Weight merging: minimum weight wins across all active configs for the same category name. Icon merging: icon from the alphabetically-first ConfigMap name with a non-empty icon wins. Changes require a server restart to take effect. |
| catalog.packageCustomIcons | object | `{}` (no per-package icon registries) | Per-package custom icon registries. Each key is the package name (must match a knodex.io/package label on the package's RGDs). Generates one ConfigMap per key, labeled knodex.io/icon-registry=true and knodex.io/package=<name>. Active only when that package is in CATALOG_PACKAGE_FILTER (empty filter activates all). Same slug/SVG schema as customIcons. Collision resolution is alphabetical-first by ConfigMap name, so the global customIcons ConfigMap wins over package-suffixed ones for shared slugs. Changes require a server restart to take effect. |
| crds.install | bool | `true` | Install the Project CRD (projects.knodex.io) |
| defaultProject.create | bool | `true` | Create the default project on install/upgrade |
| defaultProject.name | string | `"default"` | Name of the default project |
| defaultProject.spec.description | string | `"Default project - allows deployments to the default namespace"` |  |
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
| enterprise | object | `{"audit":{"redactFields":["privateKey","password","bearerToken","token","secret","tlsClientCert","tlsClientKey","clientSecret"]},"compliance":{"historyRetentionDays":""},"enabled":false,"gatekeeper":{"enabled":false},"image":{"repository":"ghcr.io/knodex/knodex-ee"},"license":{"existingSecret":"","text":""},"networkPolicy":{"enabled":false,"server":{"additionalEgress":[],"additionalIngress":[],"ingressFrom":[]}},"organization":"","postgres":{"connectionString":"","connectionStringSecret":{"key":"DATABASE_URL","name":""},"deploymentMode":"","iamAuth":{"enabled":false},"migrations":{"activeDeadlineSeconds":600,"backoffLimit":3,"resources":{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"50m","memory":"64Mi"}},"runJob":true,"ttlSecondsAfterFinished":300}}}` | Enterprise features |
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
| enterprise.postgres | object | `{"connectionString":"","connectionStringSecret":{"key":"DATABASE_URL","name":""},"deploymentMode":"","iamAuth":{"enabled":false},"migrations":{"activeDeadlineSeconds":600,"backoffLimit":3,"resources":{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"50m","memory":"64Mi"}},"runJob":true,"ttlSecondsAfterFinished":300}}` | PostgreSQL database configuration. DEPRECATED naming surface: Postgres is no longer an enterprise-only feature (R5-5 makes it mandatory on every edition). Prefer the top-level `externalPostgresql` block (canonical, mirrors externalRedis) for external databases and the `postgresql` block for the embedded subchart. The `enterprise.postgres.connectionString*` knobs below are retained as a DEPRECATED ALIAS, resolved LAST in the supply-mode precedence chain (see _helpers.tpl knodex.postgresSecretRef). `deploymentMode`, `iamAuth`, and `migrations` continue to live here.  Migration Job:   The chart renders a pre-install/pre-upgrade Job that runs all pending schema   migrations under an advisory lock whenever Postgres resolves (now every   edition). Disable with `migrations.runJob: false` to manage migrations via   your own pipeline (the server still migrates on startup). |
| enterprise.postgres.connectionString | string | `""` | Inline DATABASE_URL DSN. When set, the chart creates a managed Secret named `<release>-postgres` annotated `helm.sh/resource-policy: keep`. Suitable for dev/test only; production should use connectionStringSecret. Format: `postgres://user:pass@host:5432/dbname?sslmode=require` |
| enterprise.postgres.connectionStringSecret | object | `{"key":"DATABASE_URL","name":""}` | Reference to an externally-managed Secret containing DATABASE_URL. Recommended for production. The chart does not create this Secret — provision it via External Secrets Operator, AKS Secret Provider, CNPG, etc. |
| enterprise.postgres.connectionStringSecret.key | string | `"DATABASE_URL"` | Key within the Secret that contains the DSN. |
| enterprise.postgres.connectionStringSecret.name | string | `""` | Name of the existing Secret holding the DSN. |
| enterprise.postgres.deploymentMode | string | `""` (Postgres disabled) | Postgres deployment topology. Empty disables Postgres entirely (chart is OSS-compatible). `shared` = single DB hosting many orgs. `per-org` = one DB per Knodex tenant. Any other value fails chart rendering with a clear error. |
| enterprise.postgres.iamAuth | object | `{"enabled":false}` | IAM-auth toggle (RDS IAM / Cloud SQL IAM / Azure AD). Plumbs `POSTGRES_IAM_AUTH_ENABLED=true` into the migration Job and server Deployment. Concrete provider implementations are operator-supplied. |
| enterprise.postgres.migrations | object | `{"activeDeadlineSeconds":600,"backoffLimit":3,"resources":{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"50m","memory":"64Mi"}},"runJob":true,"ttlSecondsAfterFinished":300}` | Migration Job control. |
| enterprise.postgres.migrations.activeDeadlineSeconds | int | `600` | Hard deadline for the entire Job (seconds). Protects against indefinite advisory-lock waits. Default 10 minutes. |
| enterprise.postgres.migrations.backoffLimit | int | `3` | Maximum retries before Helm marks the release failed. |
| enterprise.postgres.migrations.resources | object | `{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"50m","memory":"64Mi"}}` | Job pod resources. |
| enterprise.postgres.migrations.runJob | bool | `true` | Render the migration Job. Disable to manage migrations via your own pipeline — the server still migrates on startup. |
| enterprise.postgres.migrations.ttlSecondsAfterFinished | int | `300` | Seconds the Job pod is retained after success. |
| externalPostgresql | object | `{"database":"knodex","existingSecret":"","existingSecretKey":"DATABASE_URL","host":"","password":"","port":5432,"sslMode":"require","username":"knodex"}` | ---------------------------------------------------------------------------- Point a fresh install of any edition at a pre-provisioned external database WITHOUT enabling the embedded subchart (mirrors externalRedis). Setting either `existingSecret` or `host` here trips the auto-detect guard, so the embedded subchart's parent resources step aside automatically (set postgresql.enabled=false to also skip the unused embedded StatefulSet — see the postgresql block above).  TLS & rotation: prefer `existingSecret` holding a full DSN (key DATABASE_URL by default) so credentials rotate out-of-band (External Secrets Operator, CSI, CNPG, cloud-managed rotation) with no chart change. Set sslMode=require (or verify-full) for production. The inline host/username/password path is for convenience/dev only.  ⚠️  Role provisioning for an external DB is OPERATOR-MANAGED: the chart's initdb role hook applies to the embedded subchart only. Provision knodex_app / knodex_migrate (or grant your login role into them) out-of-band; migration 0005 best-effort-creates them NOLOGIN where it has privilege. ⚠️  Backup/restore is the OPERATOR'S RESPONSIBILITY (the chart configures none). |
| externalPostgresql.database | string | `"knodex"` | Database name (used to build the DSN when host is set without existingSecret). |
| externalPostgresql.existingSecret | string | `""` | Reference a pre-provisioned Secret holding a full DATABASE_URL DSN. Wins over host/username/password. Recommended for production. |
| externalPostgresql.existingSecretKey | string | "DATABASE_URL" | Key within existingSecret holding the DSN. |
| externalPostgresql.host | string | `""` | Hostname of the external Postgres. When set (and no existingSecret), the chart builds a managed DSN Secret from host/port/database/username/password. |
| externalPostgresql.password | string | `""` | Password (used to build the DSN when host is set without existingSecret). For production prefer existingSecret over an inline password. |
| externalPostgresql.port | int | `5432` | Port. |
| externalPostgresql.sslMode | string | `"require"` | sslmode for the built DSN (disable | require | verify-ca | verify-full). |
| externalPostgresql.username | string | `"knodex"` | Login user (used to build the DSN when host is set without existingSecret). |
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
| postgresql | object | `{"auth":{"database":"knodex","password":"knodex","username":"knodex"},"enabled":true,"forceEnable":false,"image":{"digest":"sha256:926356130b77d5742d8ce605b258d35db9b62f2f8fd1601f9dbaef0c8a710a8d","registry":"docker.io","repository":"bitnami/postgresql","tag":"17.6.0-debian-12-r4"},"primary":{"initdb":{"scripts":{"00-knodex-roles.sql":"-- Provision the RLS roles the Knodex app + migrations expect.\n-- Runs as the postgres superuser on first init; idempotent.\nDO $$\nBEGIN\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'knodex_app') THEN\n    CREATE ROLE knodex_app NOLOGIN;\n  END IF;\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'knodex_migrate') THEN\n    CREATE ROLE knodex_migrate NOLOGIN;\n  END IF;\n  -- The bundled login user inherits both roles. knodex is a regular\n  -- (non-superuser, non-BYPASSRLS) Bitnami user, so RLS applies.\n  GRANT knodex_app TO knodex;\n  GRANT knodex_migrate TO knodex;\nEND\n$$;\n"}},"persistence":{"enabled":false},"resources":{"limits":{"cpu":"500m","memory":"512Mi"},"requests":{"cpu":"100m","memory":"256Mi"}}}}` | ---------------------------------------------------------------------------- Postgres is MANDATORY on every edition (OSS, Enterprise, Cloud) as of R5-5: every edition persists a canonical user roster (`identity.users`) and the server FAILS FAST at startup if DATABASE_URL is unset / Postgres unreachable. To make a fresh `helm install` of ANY edition come up healthy with no extra values, the embedded Bitnami subchart is ENABLED BY DEFAULT.  Supply-mode precedence (highest first) — see _helpers.tpl knodex.postgresSecretRef:   1. embedded subchart (this block, enabled by default) — the chart assembles      the full DATABASE_URL DSN and stores it in <release>-knodex-postgres-url.   2. externalPostgresql.existingSecret — reference a pre-provisioned external      Secret (canonical production path; see the externalPostgresql block below).   3. externalPostgresql.host (+ username/password) — chart builds a managed DSN      Secret <fullname>-postgres from the block.   4. enterprise.postgres.connectionString* — DEPRECATED alias, resolved last.  AUTO-DETECT GUARD (upgrade-safety): when ANY external supply above is present, the chart automatically does NOT render the embedded subchart's PARENT resources (DSN Secret / DATABASE_URL source / wait-for-postgres credential) — even though it is default-on — UNLESS postgresql.forceEnable=true. Operators pointing at an external DB should ALSO set `postgresql.enabled: false` to skip the (otherwise unused) embedded StatefulSet, because Helm cannot evaluate a computed value for the Bitnami subchart's static `condition: postgresql.enabled`.  ⚠️  PRODUCTION WARNING: the embedded subchart uses EPHEMERAL storage by default (primary.persistence.enabled: false) — ALL DATA IS LOST on pod restart — and the chart configures NO backups (no WAL archiving, snapshots, or PVC backups): backup/restore is entirely the OPERATOR'S RESPONSIBILITY. Production deployments should set postgresql.enabled=false and point at a managed database via the externalPostgresql block below. |
| postgresql.enabled | bool | true | Enable the embedded Bitnami PostgreSQL subchart. Default-on so a fresh install of any edition is healthy out of the box (R5-5). Set to false when using externalPostgresql / an external DSN to skip the unused embedded pod. |
| postgresql.forceEnable | bool | false | Force the embedded subchart's parent resources (DSN Secret, DATABASE_URL source, wait-for-postgres credential) to render even when an external supply is detected. Overrides the auto-detect guard (AC #3). Rarely needed. |
| postgresql.image.digest | string | `"sha256:926356130b77d5742d8ce605b258d35db9b62f2f8fd1601f9dbaef0c8a710a8d"` | Bitnami dependency risk: through 2025–2026 the Bitnami catalog removed floating semver tags from Docker Hub and relocated images (the `17.6.0-debian-12-r4` TAG that ships with subchart 16.7.27 is already gone from docker.io/bitnami/postgresql), so we pin by DIGEST exactly like redis.image.digest above. The digest still resolves under docker.io/bitnami/postgresql even though the tag does not; if Bitnami later DELETES the digest you must repoint `repository` to docker.io/bitnamilegacy/postgresql (which mirrors it) or bump the subchart version in Chart.yaml. To update: docker pull bitnami/postgresql:<tag> && \   docker inspect --format='{{index .RepoDigests 0}}' bitnami/postgresql:<tag> |
| postgresql.primary.initdb | object | `{"scripts":{"00-knodex-roles.sql":"-- Provision the RLS roles the Knodex app + migrations expect.\n-- Runs as the postgres superuser on first init; idempotent.\nDO $$\nBEGIN\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'knodex_app') THEN\n    CREATE ROLE knodex_app NOLOGIN;\n  END IF;\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'knodex_migrate') THEN\n    CREATE ROLE knodex_migrate NOLOGIN;\n  END IF;\n  -- The bundled login user inherits both roles. knodex is a regular\n  -- (non-superuser, non-BYPASSRLS) Bitnami user, so RLS applies.\n  GRANT knodex_app TO knodex;\n  GRANT knodex_migrate TO knodex;\nEND\n$$;\n"}}` | initdb hook (runs ONCE on first init of an empty data dir, as the postgres superuser). Provisions the RLS roles the app expects (NFR-U13): knodex_app (the non-BYPASSRLS role the pgxpool runs as) and knodex_migrate (migration-time DDL). The bundled login user (auth.username, default `knodex`) is GRANTed INTO both, so the app connects as a non-superuser, non-BYPASSRLS role and the identity RLS policies actually constrain it. IDEMPOTENT with migration 0005 (which best-effort-creates these roles NOLOGIN and explicitly defers LOGIN/GRANT provisioning to "Story 15.3"). ⚠️  If you change auth.username, update the GRANT target below to match. |
| postgresql.primary.persistence.enabled | bool | `false` | Persist data across pod restarts. Disabled by default so the embedded instance is ephemeral (data is lost on restart). For stable dev environments, set to true and configure a StorageClass. |
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
| server.project.wrapperRGD | string | `""` | Name of the kro ResourceGraphDefinition to use as a wrapper for Project creation. When set, POST /api/v1/projects creates a kro RGD instance instead of a Project CRD directly; the wrapper RGD materializes the full bundle (Project + any operator-defined bootstrap extras). Leave empty to use direct Project creation (default behavior). Can also be set at runtime via PUT /api/v1/settings/wrappers/Project. See deploy/examples/rgds/wrapped-project-example.yaml for authoring guide. |
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

## PostgreSQL

PostgreSQL is **mandatory on every edition** (OSS, Enterprise, Cloud). Each
edition persists a canonical user roster (`identity.users`) on every OIDC
login, and the server **fails fast at startup** if `DATABASE_URL` is unset or
Postgres is unreachable. Enterprise additionally uses Postgres for the audit /
compliance / license tables.

To make a fresh `helm install` of any edition come up healthy with no extra
values, the chart **bundles the Bitnami PostgreSQL subchart and enables it by
default** (`postgresql.enabled: true`). Production deployments should disable
it and point at a managed database via `externalPostgresql`.

### Supply-mode precedence

`DATABASE_URL` resolves from exactly one source, highest precedence first
(see `_helpers.tpl` `knodex.postgresSecretRef`):

| # | Source | When |
|---|--------|------|
| 1 | Embedded subchart DSN (`<release>-knodex-postgres-url`) | `postgresql.enabled` (default) and no external supply, or `postgresql.forceEnable=true` |
| 2 | `externalPostgresql.existingSecret` | a pre-provisioned external Secret (canonical production path) |
| 3 | `externalPostgresql.host` (+ `username`/`password`) | chart builds a managed DSN Secret `<fullname>-postgres` |
| 4 | `enterprise.postgres.connectionString*` | **deprecated** alias, resolved last |

### Auto-detect guard (upgrade-safety)

Because the embedded subchart is default-on, the chart **auto-detects** an
external supply (any of `externalPostgresql.host`, `externalPostgresql.existingSecret`,
or the deprecated `enterprise.postgres.connectionString*`) and then does **not**
render the embedded subchart's chart-owned resources (the DSN Secret, the
`DATABASE_URL` source, the `wait-for-postgres` credential) — so a single,
unambiguous DSN always wins. Override with `postgresql.forceEnable=true`.

> Helm cannot evaluate a computed value for the Bitnami subchart's static
> `condition: postgresql.enabled`, so operators pointing at an external DB
> should **also set `postgresql.enabled: false`** to skip the (otherwise
> unused) embedded StatefulSet.

### Embedded (bundled) Postgres — demos / dev / CI

Default behavior, no values needed. The bundled image is **digest-pinned**
(`postgresql.image.digest`) per the Bitnami catalog-volatility risk documented
in `values.yaml`. On first init the subchart provisions the RLS roles the app
expects (`knodex_app`, `knodex_migrate`) via `primary.initdb.scripts`,
idempotent with migration `0005`.

> ⚠️ The bundled instance uses **ephemeral storage** by default — data is lost
> on pod restart — and the chart configures **no backups**. Backup/restore is
> entirely the operator's responsibility. Do not use the bundled subchart in
> production.

### External Postgres (recommended for production)

Point at a managed database via an existing Secret holding a full DSN:

```yaml
postgresql:
  enabled: false          # skip the unused embedded subchart

externalPostgresql:
  existingSecret: knodex-db   # key DATABASE_URL by default
  existingSecretKey: DATABASE_URL
```

Provision the Secret out-of-band (External Secrets Operator, CSI driver, CNPG,
cloud-managed rotation) so credentials rotate with no chart change. Set
`sslMode: require` (or `verify-full`) for production. Role provisioning for an
external DB is operator-managed (the chart's initdb hook applies to the
embedded subchart only).

For dev/test convenience the chart can build the DSN from inline fields:

```yaml
postgresql:
  enabled: false

externalPostgresql:
  host: postgres.acme.local
  database: knodex
  username: knodex
  password: secret
  sslMode: require
```

### IAM authentication

`enterprise.postgres.iamAuth.enabled: true` plumbs the
`POSTGRES_IAM_AUTH_ENABLED` env var into the migration Job and the server
Deployment. Concrete provider implementations (RDS IAM, Cloud SQL IAM, Azure
AD) are operator-supplied and demand-driven — they do **not** ship with this
release.

### Migration Job

A Helm pre-install/pre-upgrade Job runs all pending schema migrations under an
advisory lock (the same lock the server uses on startup, so stale Pods cannot
race the Job). The server Deployment rolls out only after the Job exits 0.

If the Job fails, `helm install` / `helm upgrade` exits non-zero with the Job's
failure message. To inspect the failure:

```bash
kubectl describe job/<release>-postgres-migrate -n <namespace>
kubectl logs job/<release>-postgres-migrate -n <namespace>
```

To delegate migrations to your own pipeline, set
`enterprise.postgres.migrations.runJob: false`. The server still runs
migrations on startup, so there is no "neither path migrates" footgun.

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
