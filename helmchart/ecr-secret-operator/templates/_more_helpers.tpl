{{/* comment */}}

{{- define "ecr-secret-operator.configMapData" -}}
  config.toml: | 
    # AWS account ID forms each table header
    # 
    # IAM User account used should only require ecr:GetAuthorizationToken permission.
    # 
    {{- range $k, $v := .Values.AWS }}
    [{{ $k }}]
    access_key = "{{ $v.accessKey }}"
    secret_key = "{{ $v.secretKey }}"
    {{- end }}
{{- end }}

{{- define "ecr-secret-operator.configMapName" -}}
{{ (include "ecr-secret-operator.fullname" .) }}-cm
{{- end }}

{{- define "ecr-secret-operator.contollerManagerLabel" -}}
{{ (include "ecr-secret-operator.fullname" .) }}-controller-manager
{{- end }}
