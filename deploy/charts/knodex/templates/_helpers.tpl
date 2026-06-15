{{/*
Expand the name of the chart.
*/}}
{{- define "knodex.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "knodex.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "knodex.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "knodex.labels" -}}
helm.sh/chart: {{ include "knodex.chart" . }}
{{ include "knodex.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "knodex.selectorLabels" -}}
app.kubernetes.io/name: {{ include "knodex.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Server labels
*/}}
{{- define "knodex.server.labels" -}}
{{ include "knodex.labels" . }}
app.kubernetes.io/component: server
{{- end }}

{{/*
Server selector labels
*/}}
{{- define "knodex.server.selectorLabels" -}}
{{ include "knodex.selectorLabels" . }}
app.kubernetes.io/component: server
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "knodex.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "knodex.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Server image repository
Returns enterprise image when enterprise.enabled is true, otherwise the standard image
*/}}
{{- define "knodex.server.image" -}}
{{- if .Values.enterprise.enabled }}
{{- .Values.enterprise.image.repository }}
{{- else }}
{{- .Values.server.image.repository }}
{{- end }}
{{- end }}

{{/*
License secret name
Returns the user-provided existingSecret or the chart-generated secret name
*/}}
{{- define "knodex.licenseSecretName" -}}
{{- if .Values.enterprise.license.existingSecret }}
{{- .Values.enterprise.license.existingSecret }}
{{- else }}
{{- printf "%s-license" (include "knodex.fullname" .) }}
{{- end }}
{{- end }}

{{/*
SSO secret name
Returns the user-provided existingSecret or the chart-generated secret name
*/}}
{{- define "knodex.ssoSecretName" -}}
{{- if .Values.server.auth.oidc.existingSecret }}
{{- .Values.server.auth.oidc.existingSecret }}
{{- else }}
{{- print "knodex-sso-secrets" }}
{{- end }}
{{- end }}

{{/*
Redis subchart fullname — mirrors Bitnami common.names.fullname for the redis subchart
so that host references resolve correctly. Logic: fullnameOverride > nameOverride > default ("redis").
*/}}
{{- define "knodex.redis.fullname" -}}
{{- if .Values.redis.fullnameOverride }}
{{- .Values.redis.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default "redis" .Values.redis.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Redis host
*/}}
{{- define "knodex.redisHost" -}}
{{- if .Values.redis.enabled }}
{{- printf "%s-master" (include "knodex.redis.fullname" .) }}
{{- else }}
{{- .Values.externalRedis.host }}
{{- end }}
{{- end }}

{{/*
Redis port
*/}}
{{- define "knodex.redisPort" -}}
{{- if .Values.redis.enabled }}
{{- 6379 }}
{{- else }}
{{- .Values.externalRedis.port }}
{{- end }}
{{- end }}

{{/*
Redis auth secret name — chart-managed secret: <fullname>-redis-password.
This helper is called from both the parent chart and the Bitnami redis subchart
(via tpl evaluation of redis.auth.existingSecret). It must produce the same name
in both contexts. It reconstructs the parent chart's fullname using .Release.Name
and the hardcoded parent chart name "knodex" (rather than .Chart.Name, which
differs in the subchart). nameOverride / fullnameOverride are read from
.Values.global so they are accessible in both parent and subchart contexts.
*/}}
{{- define "knodex.redisSecretName" -}}
{{- $globalOverrides := default dict .Values.global }}
{{- $knodexOverrides := default dict (index $globalOverrides "knodex") }}
{{- $fullnameOverride := default "" (index $knodexOverrides "fullnameOverride") }}
{{- $nameOverride := default "" (index $knodexOverrides "nameOverride") }}
{{- if $fullnameOverride }}
{{- printf "%s-redis-password" ($fullnameOverride | trunc 63 | trimSuffix "-") }}
{{- else }}
{{- $name := default "knodex" $nameOverride }}
{{- if contains $name .Release.Name }}
{{- printf "%s-redis-password" (.Release.Name | trunc 63 | trimSuffix "-") }}
{{- else }}
{{- printf "%s-%s-redis-password" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Redis secret name for use in the parent chart (server deployment, init containers).
If the user supplied a custom existingSecret (not the default tpl expression),
use that; otherwise fall back to the chart-managed name from knodex.redisSecretName.
*/}}
{{- define "knodex.redisSecretRef" -}}
{{- if and .Values.redis.auth.existingSecret (not (contains "knodex.redisSecretName" .Values.redis.auth.existingSecret)) }}
{{- .Values.redis.auth.existingSecret }}
{{- else }}
{{- include "knodex.redisSecretName" . }}
{{- end }}
{{- end }}

{{/*
Redis auth secret key
Returns existingSecretPasswordKey if set, otherwise the Bitnami default key.
*/}}
{{- define "knodex.redisSecretKey" -}}
{{- if .Values.redis.auth.existingSecretPasswordKey }}
{{- .Values.redis.auth.existingSecretPasswordKey }}
{{- else }}
{{- "redis-password" }}
{{- end }}
{{- end }}

{{/*
Postgres chart-managed Secret name — used when an inline
`postgres.connectionString` is supplied. Mirrors the redisSecretName pattern.
*/}}
{{- define "knodex.postgresSecretName" -}}
{{- printf "%s-postgres" (include "knodex.fullname" .) }}
{{- end }}

{{/*
Postgres Secret reference — resolves the correct Secret name for DATABASE_URL
across every Postgres supply mode. Precedence (highest first), matching
values.yaml comments and knodex.postgresEmbeddedEnabled / the guard:
  1. embedded subchart active (knodex.postgresEmbeddedEnabled) — chart-built DSN Secret
  2. externalPostgresql.existingSecret — pre-existing external Secret (canonical)
  3. externalPostgresql.host (no existingSecret) — chart-managed DSN Secret from the block
  4. enterprise.postgres.connectionStringSecret.name — DEPRECATED alias, pre-existing Secret
  5. enterprise.postgres.connectionString — DEPRECATED alias, inline DSN; chart-managed Secret
Both the migration Job and the server Deployment resolve to the same
secret/key pair via this helper (mirrors knodex.redisSecretRef).
*/}}
{{- define "knodex.postgresSecretRef" -}}
{{- if include "knodex.postgresEmbeddedEnabled" . }}
{{- printf "%s-knodex-postgres-url" .Release.Name }}
{{- else if .Values.externalPostgresql.existingSecret }}
{{- .Values.externalPostgresql.existingSecret }}
{{- else if .Values.externalPostgresql.host }}
{{- include "knodex.postgresSecretName" . }}
{{- else if .Values.enterprise.postgres.connectionStringSecret.name }}
{{- .Values.enterprise.postgres.connectionStringSecret.name }}
{{- else }}
{{- include "knodex.postgresSecretName" . }}
{{- end }}
{{- end }}

{{/*
PostgreSQL subchart fullname — mirrors knodex.redis.fullname for the postgresql subchart
so that DSN hostnames resolve correctly when fullnameOverride / nameOverride are set.
Logic: fullnameOverride > nameOverride > default ("postgresql").
*/}}
{{- define "knodex.postgresql.fullname" -}}
{{- if .Values.postgresql.fullnameOverride }}
{{- .Values.postgresql.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default "postgresql" .Values.postgresql.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Postgres Secret key holding the DSN. Defaults to `DATABASE_URL`.
When the embedded subchart is active, the DSN Secret always uses key `DATABASE_URL`.
For externalPostgresql.existingSecret, honor existingSecretKey; for the legacy
enterprise.postgres alias, honor connectionStringSecret.key. Precedence mirrors
knodex.postgresSecretRef.
*/}}
{{- define "knodex.postgresSecretKey" -}}
{{- if include "knodex.postgresEmbeddedEnabled" . }}
{{- "DATABASE_URL" }}
{{- else if .Values.externalPostgresql.existingSecret }}
{{- default "DATABASE_URL" .Values.externalPostgresql.existingSecretKey }}
{{- else if .Values.externalPostgresql.host }}
{{- "DATABASE_URL" }}
{{- else }}
{{- default "DATABASE_URL" .Values.enterprise.postgres.connectionStringSecret.key }}
{{- end }}
{{- end }}

{{/*
Whether an EXTERNAL Postgres supply is present (operator points the chart at a
database it does not provision). Any of: the canonical externalPostgresql block
(host or existingSecret) OR the DEPRECATED enterprise.postgres.connectionString*
alias. Returns "true" or empty string (Helm-truthy convention).
*/}}
{{- define "knodex.postgresExternalSupplyPresent" -}}
{{- if or .Values.externalPostgresql.host .Values.externalPostgresql.existingSecret .Values.enterprise.postgres.connectionString .Values.enterprise.postgres.connectionStringSecret.name -}}
true
{{- end -}}
{{- end }}

{{/*
Whether the embedded Bitnami PostgreSQL subchart's PARENT-rendered resources
(the DSN Secret, the DATABASE_URL source, the wait-for-postgres credential)
should render. Auto-detect guard (AC #3): the embedded path is active iff the
operator knob `postgresql.enabled` is true AND no external supply is present —
UNLESS `postgresql.forceEnable` overrides it. Default-on for a fresh install
(postgresql.enabled defaults true) so OSS comes up healthy (R5-5), and it steps
aside automatically when an external DSN is supplied.

NOTE: the Bitnami subchart's own StatefulSet/Service are gated by the static
`condition: postgresql.enabled` in Chart.yaml — Helm cannot evaluate a computed
helper for a subchart condition. Operators supplying an external DB should set
`postgresql.enabled: false` to avoid an unused embedded pod; this guard still
guarantees the chart never emits a conflicting embedded DSN.
*/}}
{{- define "knodex.postgresEmbeddedEnabled" -}}
{{- if and .Values.postgresql.enabled (or (not (include "knodex.postgresExternalSupplyPresent" .)) .Values.postgresql.forceEnable) -}}
true
{{- end -}}
{{- end }}

{{/*
Whether Postgres-related resources should render. Postgres is mandatory on every
edition (R5-5 / FR-U12): true whenever ANY supply mode resolves — the embedded
subchart (per the guard) OR an external supply. Returns "true" or empty string.
*/}}
{{- define "knodex.postgresEnabled" -}}
{{- if or (include "knodex.postgresEmbeddedEnabled" .) (include "knodex.postgresExternalSupplyPresent" .) -}}
true
{{- end -}}
{{- end }}

{{/*
Postgres host — resolves the reachable hostname per mode (mirrors knodex.redisHost).
Embedded subchart ⇒ the Bitnami primary Service (<subchart-fullname>).
externalPostgresql.host ⇒ that host. For an external Secret / inline DSN the host
is embedded in DATABASE_URL and not known at template time (wait-for-postgres
probes the DSN directly via `pg_isready -d "$DATABASE_URL"`).
*/}}
{{- define "knodex.postgresHost" -}}
{{- if include "knodex.postgresEmbeddedEnabled" . }}
{{- include "knodex.postgresql.fullname" . }}
{{- else if .Values.externalPostgresql.host }}
{{- .Values.externalPostgresql.host }}
{{- end }}
{{- end }}

{{/*
Postgres port (mirrors knodex.redisPort).
*/}}
{{- define "knodex.postgresPort" -}}
{{- if include "knodex.postgresEmbeddedEnabled" . }}
{{- 5432 }}
{{- else }}
{{- .Values.externalPostgresql.port | default 5432 }}
{{- end }}
{{- end }}

{{/*
Dex labels
*/}}
{{- define "knodex.dex.labels" -}}
{{ include "knodex.labels" . }}
app.kubernetes.io/component: dex-server
{{- end }}

{{/*
Dex selector labels
*/}}
{{- define "knodex.dex.selectorLabels" -}}
{{ include "knodex.selectorLabels" . }}
app.kubernetes.io/component: dex-server
{{- end }}

{{/*
kagent controller base URL — used for Agents Hub presence detection and A2A invocation.
Explicit controllerBaseURL takes precedence; when kagent is installed as a subchart
the URL is derived from the release namespace.
*/}}
{{- define "knodex.kagentControllerBaseURL" -}}
{{- if .Values.kagent.controllerBaseURL }}
{{- .Values.kagent.controllerBaseURL }}
{{- else if .Values.kagent.enabled }}
{{- printf "http://kagent-controller.%s.svc.cluster.local:8083" .Release.Namespace }}
{{- else }}
{{- "http://kagent-controller.kagent.svc.cluster.local:8083" }}
{{- end }}
{{- end }}

{{/*
Dex server URL — used by the Knodex server to discover Dex's OIDC endpoint
*/}}
{{- define "knodex.dex.serverURL" -}}
{{- if .Values.dex.config.issuerURL }}
{{- .Values.dex.config.issuerURL }}
{{- else }}
{{- printf "http://%s-dex-server:%d" (include "knodex.fullname" .) (int (.Values.dex.service.httpPort | default 5556)) }}
{{- end }}
{{- end }}
