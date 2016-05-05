#!/bin/bash
set -e

if [[ "$1" == "--help" ]]; then
cat <<EOL
Usage: run.sh
EOL
exit 0
fi

# When the container gets restarted, processes may end up with different pids
find /run -name "*.pid" -delete
if [ -d /var/vcap/sys/run ]; then
    find /var/vcap/sys/run -name "*.pid" -delete
fi

export IP_ADDRESS=$(/bin/hostname -i | awk '{print $1}')
export DNS_RECORD_NAME=$(/bin/hostname)

# Usage: run_configin <job> <input>  <output>
#                     name  template destination
function run_configgin()
{
	job_name="$1"
	template_file="$2"
	output_file="$3"
	/opt/hcf/configgin/configgin \
	--input-erb ${template_file} \
	--output ${output_file} \
	--base /var/vcap/jobs-src/${job_name}/config_spec.json \
	--env2conf /opt/hcf/env2conf.yml
}

# Run custom role scripts
{{ with $role := index . "role" }}
{{ range $i, $script := .Scripts}}
bash /opt/hcf/startup/{{ $script }}
{{ end }}
{{ end }}

# Process templates
{{ with $role := index . "role" }}
# =====================================================
{{ range $i, $job := .Jobs}}
# ============================================================================
#         Templates for job {{ $job.Name }}
# ============================================================================
{{ range $j, $template := $job.Templates }}
run_configgin "{{$job.Name}}" \
    "/var/vcap/jobs-src/{{$job.Name}}/templates/{{ $template.SourcePath }}" \
    "/var/vcap/jobs/{{$job.Name}}/{{$template.DestinationPath}}"
# =====================================================
{{ end }}
{{ if not (eq $role.Type "bosh-task") }}
# ============================================================================
#         Monit templates for job {{ $job.Name }}
# ============================================================================
run_configgin "{{$job.Name}}" \
    "/var/vcap/jobs-src/{{$job.Name}}/monit" \
    "/var/vcap/monit/{{$job.Name}}.monitrc"
# =====================================================
{{ end }}
{{ end }}
{{ if not (eq .Type "bosh-task") }}
# Process monitrc.erb template
run_configgin "{{with $l := index $role.JobNameList 0}}{{$l.Name}}{{end}}" \
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

# Run custom post config role scripts
{{ with $role := index . "role" }}
{{ range $i, $script := .PostConfigScripts}}
bash /opt/hcf/startup/{{ $script }}
{{ end }}
{{ end }}

# Run
{{ with $role := index . "role" }}
{{ if eq .Type "bosh-task" }}
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
