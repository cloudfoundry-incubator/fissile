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
{{ range $script := .role.Scripts}}
    bash {{ if not (is_abs $script) }}/opt/hcf/startup/{{ end }}{{ $script }}
{{ end }}

# Process templates
{{ with $role := .role }}
    # =====================================================
    {{ range $job := $role.Jobs}}
    # ============================================================================
    #         Templates for job {{ $job.Name }}
    # ============================================================================
        {{ range $template := $job.Templates }}
            run_configgin "{{$job.Name}}" \
                "/var/vcap/jobs-src/{{$job.Name}}/templates/{{$template.SourcePath}}" \
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
    {{ if not (eq $role.Type "bosh-task") }}
        # Process monitrc.erb template
        run_configgin "{{(index $role.JobNameList 0).Name}}" \
            "/opt/hcf/monitrc.erb" \
            "/etc/monitrc"
        chmod 0600 /etc/monitrc
    {{ end }}
{{ end }}

# Create run dir
mkdir -p /var/vcap/sys/run
chown root:vcap /var/vcap/sys/run
chmod 775 /var/vcap/sys/run

# Start rsyslog and cron
service rsyslog start
cron

# Run custom post config role scripts
# Run any custom scripts other than pre-start
{{ range $script := .role.PostConfigScripts}}
{{ if not (is_pre_start $script) }}
    echo bash {{ if not (is_abs $script) }}/opt/hcf/startup/{{ end }}{{ $script }}
    bash {{ if not (is_abs $script) }}/opt/hcf/startup/{{ end }}{{ $script }}
{{ end }}
{{ end }}

# Run all the scripts called pre-start, but ensure consul_agent/bin/pre-start is run before others.
# None of the other pre-start scripts appear to have any dependencies on one another.
# Use Perl to sort consul_agent/bin/pre-start before others.
# (Perl's sort is stable, not that it matters for `find' output.)
function sorted-pre-start-paths()
{
    find /var/vcap/jobs/*/bin -name pre-start |
    perl -e 'my ($path, @files, $ptn);
             while ($path = <>) { chomp $path; push(@files, $path); }
             $ptn=qr{/consul_agent/bin/pre-start$};
             print join("\n", sort { 
                                     $a =~ $ptn ? -1 :
                                     $b =~ $ptn ?  1 : 0 } @files) . "\n";'
}

for fname in $(sorted-pre-start-paths) ; do
    echo bash $fname
    bash $fname
done

# Run
{{ if eq .role.Type "bosh-task" }}
    {{ range $job := .role.Jobs}}
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
