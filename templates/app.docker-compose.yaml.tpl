# Automatically generated by Outblocks Docker plugin.
# DO NOT EDIT. Will be overwritten on next run.

version: "3.9"

services:
  {{.Name}}:
    build:
      context: .
      dockerfile: {{.Dockerfile}}
    command: sh -c "{{.DockerCommand }}"
    working_dir: {{.WorkDir}}
    ports:
      - ${{`{`}}{{.EnvPrefix}}_PORT}:${{`{`}}{{.EnvPrefix}}_PORT}

    environment:
{{.Env | toYaml | indent 6}}
    volumes:
      - .:{{.DockerPath | default "/app"}}
{{- range $key, $value := .Volumes }}
      - {{ $key }}:{{ $value }}
{{- end }}

    extra_hosts:
{{- range $key, $value := .Hosts }}
      - {{ $key }}:host-gateway
{{- end }}

{{- if .Volumes }}

volumes:
{{- range $key, $value := .Volumes }}
  {{ $key }}:
{{- end }}
{{- end }}
