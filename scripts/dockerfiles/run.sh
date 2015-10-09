#!/bin/bash
set -e

consul_address=$1
if [[ -z $consul_address ]]; then
  consul_address="{{ index . "default_consul_address" }}"
fi

config_store_prefix=$2
if [[ -z $config_store_prefix ]]; then
  config_store_prefix="{{ index . "default_config_store_prefix" }}"
fi

role_instance_index=$3
if [[ -z $role_instance_index ]]; then
  role_instance_index=0
fi

# Process templates
{{ with $role := index . "role" }}
{{ range $i, $job := .Jobs}}
# ============================================================================
#         Templates for job {{ $job.Name }}
# ============================================================================
{{ range $j, $template := $job.Templates }}
/opt/hcf/configgin/configgin \
  --data '{"job": { "name": "{{ $job.Name }}" }, "index": ${role_instance_index}, "parameters": {} }' \
  --output  "/var/vcap/jobs/{{ $job.Name }}/{{$template.DestinationPath}}" \
  --consul  "${consul_address}" \
  --prefix  "${config_store_prefix}" \
  --role    "{{$role.Name}}" \
  --job     "{{$job.Name}}" \
  "/var/vcap/jobs-src/{{ $job.Name }}/templates/{{ $template.SourcePath }}"
# =====================================================
{{ end }}
{{ end }}

# Process monitrc.erb template
/opt/hcf/configgin/configgin \
  --data '{"job": { "name": "hcf-monit-master" }, "index": ${role_instance_index}, "parameters": {} }' \
  --output  "/etc/monitrc" \
  --consul  "${consul_address}" \
  --prefix  "${config_store_prefix}" \
  --role    "{{$role.Name}}" \
  --job     "hcf-monit-master" \
  "/opt/hcf/monitrc.erb"

{{ end }}

# Run
monit -vI