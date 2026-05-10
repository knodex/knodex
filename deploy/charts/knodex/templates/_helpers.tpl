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
Dex server URL — used by the Knodex server to discover Dex's OIDC endpoint
*/}}
{{- define "knodex.dex.serverURL" -}}
{{- if .Values.dex.config.issuerURL }}
{{- .Values.dex.config.issuerURL }}
{{- else }}
{{- printf "http://%s-dex-server:%d" (include "knodex.fullname" .) (int (.Values.dex.service.httpPort | default 5556)) }}
{{- end }}
{{- end }}
