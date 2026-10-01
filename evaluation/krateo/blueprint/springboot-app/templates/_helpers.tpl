{{/* Application name and namespace: the composition name (the dump's demo-app). */}}
{{- define "app.name" -}}{{ .Values.global.compositionName }}{{- end }}
{{/* Suffix used by the dump for composition-scoped RBAC names, e.g. springbootapp-demo-app. */}}
{{- define "app.suffix" -}}{{ .Values.global.compositionKind | lower }}-{{ .Values.global.compositionName }}{{- end }}
{{- define "app.widgetsApiVersion" -}}widgets.templates.krateo.io/v1beta1{{- end }}
