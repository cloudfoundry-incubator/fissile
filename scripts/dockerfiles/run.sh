#!/bin/bash
set -e

if [[ "$1" == "--help" ]]; then
cat <<EOL
Usage: run.sh [<consul_address>] [<config_store_prefix>] [<role_instance_index>] [<dns_record_name>]
EOL
exit 0
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

dns_record_name=$4
if [[ -z $dns_record_name ]]; then
  dns_record_name="localhost"
fi

ip_address=$(/bin/hostname -i)

# Usage: run_configin <role-description> <role> <release> <job> <input>  <output>
#                     json               name   name      name  template destination
function run_configgin()
{
    role_desc="$1"
    role_name="$2"
    release_name="$3"
    job_name="$4"
    input="$5"
    output="$6"
    /opt/hcf/configgin/configgin \
	--data    "${role_desc}" \
	--output  "${output}" \
	--consul  "${consul_address}" \
	--prefix  "${config_store_prefix}" \
	--role    "${role_name}" \
	--release "${release_name}" \
	--job     "${job_name}" \
	"${input}"
}

# Process templates
{{ with $role := index . "role" }}
the_role_desc='{"job": { "name": "{{ $role.Name }}", "templates":[{{ range $iJob, $innerJob := $role.Jobs}}{{if $iJob}},{{end}}{"name":"{{$innerJob.Name}}"}{{ end }}] }, "index": '"${role_instance_index}"', "parameters": {}, "networks": { "default":{ "ip":"'"${ip_address}"'", "dns_record_name":"'"${dns_record_name}"'"}}}'
# =====================================================
{{ range $i, $job := .Jobs}}
# ============================================================================
#         Templates for job {{ $job.Name }}
# ============================================================================
{{ range $j, $template := $job.Templates }}
run_configgin "$the_role_desc" "{{$role.Name}}" "{{$job.Release.Name}}" "{{$job.Name}}" \
    "/var/vcap/jobs-src/{{$job.Name}}/templates/{{ $template.SourcePath }}" \
    "/var/vcap/jobs/{{$job.Name}}/{{$template.DestinationPath}}"
# =====================================================
{{ end }}
{{ if not $role.IsTask }}
# ============================================================================
#         Monit templates for job {{ $job.Name }}
# ============================================================================
run_configgin "$the_role_desc" "{{$role.Name}}" "{{$job.Release.Name}}" "{{$job.Name}}" \
    "/var/vcap/jobs-src/{{$job.Name}}/monit" \
    "/var/vcap/monit/{{$job.Name}}.monitrc"
# =====================================================
{{ end }}
{{ end }}
{{ if not .IsTask }}
# Process monitrc.erb template
run_configgin "$the_role_desc" "{{$role.Name}}" "{{with $l := index $role.JobNameList 0}}{{$l.ReleaseName}}{{end}}" "hcf-monit-master" \
    "/opt/hcf/monitrc.erb" \
    "/etc/monitrc"
chmod 0600 /etc/monitrc
{{ end }}
{{ end }}

# Create run dir
mkdir -p /var/vcap/sys/run

# Start rsyslog and cron
service rsyslog start
cron

# Run custom role scripts
export CONSUL_ADDRESS=$consul_address
export CONFIG_STORE_PREFIX=$config_store_prefix
export ROLE_INSTANCE_INDEX=$role_instance_index
export IP_ADDRESS=$ip_address
export DNS_RECORD_NAME=$dns_record_name
{{ with $role := index . "role" }}
{{ range $i, $script := .Scripts}}
bash /opt/hcf/startup/{{ $script }}
{{ end }}
{{ end }}

# Run
{{ with $role := index . "role" }}
{{ if .IsTask }}
{{ range $i, $job := .Jobs}}
/var/vcap/jobs/{{ $job.Name }}/bin/run
{{ end }}
{{ else }}

monit -vI &
pid=$!
echo "pid = $pid"

killer() {
  echo "killing $pid"
  kill $pid
}

trap killer INT TERM

( while $(sleep 1); do true; done )

{{ end }}
{{ end }}
