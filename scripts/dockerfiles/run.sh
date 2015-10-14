#!/bin/bash
set -e

if [[ "$1" == "--help" ]]; then
cat <<EOL
Usage: run.sh [<consul_address>] [<config_store_prefix>] [<role_instance_index>] [<ip_address>] [<dns_record_name>]
EOL
fi

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

ip_address=$4
if [[ -z $ip_address ]]; then
  ip_address="127.0.0.1"
fi

dns_record_name=$5
if [[ -z $dns_record_name ]]; then
  dns_record_name="localhost"
fi

# Process templates
{{ with $role := index . "role" }}
{{ range $i, $job := .Jobs}}
# ============================================================================
#         Templates for job {{ $job.Name }}
# ============================================================================
{{ range $j, $template := $job.Templates }}
/opt/hcf/configgin/configgin \
  --data '{"job": { "name": "{{ $job.Name }}" }, "index": '"${role_instance_index}"', "parameters": {}, "networks": { "default":{ "ip":"'"${ip_address}"'", "dns_record_name":"'"${dns_record_name}"'"}}}' \
  --output  "/var/vcap/jobs/{{ $job.Name }}/{{$template.DestinationPath}}" \
  --consul  "${consul_address}" \
  --prefix  "${config_store_prefix}" \
  --role    "{{$role.Name}}" \
  --job     "{{$job.Name}}" \
  "/var/vcap/jobs-src/{{ $job.Name }}/templates/{{ $template.SourcePath }}"
# =====================================================
{{ end }}
# ============================================================================
#         Templates for job {{ $job.Name }}
# ============================================================================
/opt/hcf/configgin/configgin \
  --data '{"job": { "name": "{{ $job.Name }}" }, "index": '"${role_instance_index}"', "parameters": {}, "networks": { "default":{ "ip":"'"${ip_address}"'", "dns_record_name":"'"${dns_record_name}"'"}}}' \
  --output  "/var/vcap/monit/{{ $job.Name }}.monitrc" \
  --consul  "${consul_address}" \
  --prefix  "${config_store_prefix}" \
  --role    "{{$role.Name}}" \
  --job     "{{$job.Name}}" \
  "/var/vcap/jobs-src/{{ $job.Name }}/monit"
# =====================================================
{{ end }}

# Process monitrc.erb template
/opt/hcf/configgin/configgin \
  --data '{"job": { "name": "hcf-monit-master" }, "index": '"${role_instance_index}"', "parameters": {}, "networks": { "default":{ "ip":"'"${ip_address}"'", "dns_record_name":"'"${dns_record_name}"'"}}}' \
  --output  "/etc/monitrc" \
  --consul  "${consul_address}" \
  --prefix  "${config_store_prefix}" \
  --role    "{{$role.Name}}" \
  --job     "hcf-monit-master" \
  "/opt/hcf/monitrc.erb"

{{ end }}

chmod 0600 /etc/monitrc

# Create run dir
mkdir -p /var/vcap/sys/run

# Start rsyslog
service rsyslog start

# Run
monit -vI
