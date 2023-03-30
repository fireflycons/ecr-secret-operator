{{- define "ecr-secret-operator.toml" }}
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

{{- define "ecr-secret-operator.secretData" -}}
  config.toml: {{ (include "ecr-secret-operator.toml" .) | b64enc }}
{{- end }}

{{- define "ecr-secret-operator.secretName" -}}
{{ (include "ecr-secret-operator.fullname" .) }}-config
{{- end }}

{{- define "ecr-secret-operator.contollerManagerLabel" -}}
{{ (include "ecr-secret-operator.fullname" .) }}-controller-manager
{{- end }}
