#!/bin/bash
set -e

if [[ "$1" == "--help" ]]; then
cat <<EOL
Usage: run.sh
EOL
exit 0
fi

# Unmark the role. We may have this file from a previous run of the
# role, i.e. this may be a restart. Ensure that we are not seen as
# ready yet.
rm -f /var/vcap/monit/ready /var/vcap/monit/ready.lock

# When the container gets restarted, processes may end up with different pids
find /run -name "*.pid" -delete
if [ -d /var/vcap/sys/run ]; then
    find /var/vcap/sys/run -name "*.pid" -delete
fi

export IP_ADDRESS=$(/bin/hostname -i | awk '{print $1}')
export DNS_RECORD_NAME=$(/bin/hostname)

# Run custom environment scripts (that are sourced)
{{ range $script := .role.EnvironScripts }}
    source {{ if not (is_abs $script) }}/opt/hcf/startup/{{ end }}{{ $script }}
{{ end }}
# Run custom role scripts
{{ range $script := .role.Scripts}}
    bash {{ if not (is_abs $script) }}/opt/hcf/startup/{{ end }}{{ $script }}
{{ end }}

/opt/hcf/configgin/configgin \
	--jobs /opt/hcf/job_config.json \
	--env2conf /opt/hcf/env2conf.yml

if [ -e /etc/monitrc ]
then
  chmod 0600 /etc/monitrc
fi

# Create run dir
mkdir -p /var/vcap/sys/run
chown root:vcap /var/vcap/sys/run
chmod 775 /var/vcap/sys/run

{{ if eq .role.Type "bosh-task" }}
    # Start rsyslog and cron
    service rsyslog start
    cron
{{ else }}
    # rsyslog and cron are started via monit
{{ end }}

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
function sorted-pre-start-paths()
{
    declare -a fnames
    idx=0
    if [ -x /var/vcap/jobs/consul_agent/bin/pre-start ] ; then
	fnames[$idx]=/var/vcap/jobs/consul_agent/bin/pre-start
	idx=$((idx + 1))
    fi
    for fname in $(find /var/vcap/jobs/*/bin -name pre-start | grep -v '/consul_agent/bin/pre-start$') ; do
	fnames[$idx]=$fname
	idx=$((idx + 1))
    done
    echo ${fnames[*]}
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
    # Run all the scripts called post-start, if any.
    psp=$(find /var/vcap/jobs/*/bin -name post-start)

    if [ "X$psp" != X ] ; then
	# We have post-start scripts to run.

	# We do this by adding a job to the monit configuration
	# before invoking it. This actually trivial, we simply
	# put a .monitrc file into the directory /var/vcap/monit/job
	# and monit will pick it up on start.
	#
	# The trick is to make this new job ("PS") dependent on all
	# the existing jobs (of the role). Because the monit we have
	# has bugs in its dependency management our job's start
	# command does the checking manually, parsing the output of
	# `monit summary`.  We are also using `flock` to guard against
	# monit starting the job multiple times (due to the checking
	# itself or the called post-start scripts taking too long).
	# The start command is a script we arrange to run all the
	# post-start files. Its last action is setting the monitored
	# marker file, completing things.


	# Create the job script, runs all the post-start scripts
	# found, then sets the marker.
	cat > /opt/hcf/post-start.sh <<EOF
#!/bin/bash
set -e
(
  flock -n 9 || exit 1
  notyet=\$(monit summary | tail -n+3 |grep -v 'Accessible\|Running'|wc -l)
  if [ \$notyet -eq 1 ] ; then
    for fname in ${psp} ; do
      bash \$fname
    done
    touch /var/vcap/monit/ready
  fi
) 9> /var/vcap/monit/ready.lock
EOF
        chmod ug+x /opt/hcf/post-start.sh

	# Create the trivial configuration file for the new job. We
	# keep using the standard timeout of 10 seconds here. The
	# flock in the script guards against possible multiple
	# invokations by monit.
	cat > /var/vcap/monit/zomega.monitrc <<EOF
check file zomega path /var/vcap/monit/ready
  start program = "/opt/hcf/post-start.sh"
EOF

    fi

    # Replace bash with monit to handle both SIGTERM and SIGINT
    exec dumb-init -- monit -vI
{{ end }}
