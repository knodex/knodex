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
{{- printf "%s-sso-secrets" (include "knodex.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Redis host
*/}}
{{- define "knodex.redisHost" -}}
{{- if .Values.redis.enabled }}
{{- printf "%s-redis-master" (include "knodex.fullname" .) }}
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
