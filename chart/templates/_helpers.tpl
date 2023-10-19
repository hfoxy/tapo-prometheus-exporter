{{/*
Expand the name of the chart.
*/}}
{{- define "tapo-prometheus-exporter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "tapo-prometheus-exporter.fullname" -}}
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
{{- define "tapo-prometheus-exporter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "tapo-prometheus-exporter.labels" -}}
helm.sh/chart: {{ include "tapo-prometheus-exporter.chart" . }}
{{ include "tapo-prometheus-exporter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "tapo-prometheus-exporter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tapo-prometheus-exporter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Config secret name
*/}}
{{- define "tapo-prometheus-exporter.secretName" -}}
{{- if .Values.config.existingSecret -}}
{{- .Values.config.existingSecret }}
{{- else -}}
{{- include "tapo-prometheus-exporter.fullname" . }}
{{- end }}
{{- end }}

{{/*
Service Monitor labels
*/}}
{{- define "tapo-prometheus-exporter.serviceMonitorLabels" -}}
{{ include "tapo-prometheus-exporter.labels" . }}
{{ toYaml .Values.prometheus.serviceMonitor.labels | nindent 4 }}
{{- end }}
