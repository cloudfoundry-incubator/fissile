#!/bin/bash

set -e

if [[ "$1" == "--help" ]]; then
cat <<EOL
Usage: run.sh
EOL
exit 0
fi

# Compatibility code for use with older stemcells. Move all files
# provided in the old location to the new. And fix the references
# they have.

if [ -d /opt/hcf ] ; then
    mkdir -p /opt/scf
    mv       /opt/hcf/* /opt/scf/
    rmdir    /opt/hcf
    sed < /opt/scf/monitrc.erb > $$ -e 's|/hcf|/scf|'
    mv $$ /opt/scf/monitrc.erb
fi

# Make BOSH installed binaries available
export PATH=/var/vcap/bosh/bin:$PATH

# Load RVM
source /usr/local/rvm/scripts/rvm

# Taken from https://github.com/cloudfoundry/bosh-linux-stemcell-builder/blob/95aa0de0fe734547b2dd9241685c31c5f6d61a83/stemcell_builder/lib/prelude_apply.bash
# To be used by scripts that are run or sourced by this file.
function get_os_type {
  centos_file=$chroot/etc/centos-release
  rhel_file=$chroot/etc/redhat-release
  ubuntu_file=$chroot/etc/lsb-release
  photonos_file=$chroot/etc/photon-release
  opensuse_file=$chroot/etc/SuSE-release

  os_type=''
  if [ -f $photonos_file ]
  then
    os_type='photonos'
  elif [ -f $ubuntu_file ]
  then
    os_type='ubuntu'
  elif [ -f $centos_file ]
  then
    os_type='centos'
  elif [ -f $rhel_file ]
  then
    os_type='rhel'
  elif [ -f $opensuse_file ]
  then
    os_type='opensuse'
  fi

  echo $os_type
}
export -f get_os_type

# Unmark the role. We may have this file from a previous run of the
# role, i.e. this may be a restart. Ensure that we are not seen as
# ready yet.
rm -f /var/vcap/monit/ready /var/vcap/monit/ready.lock

# When the container gets restarted, processes may end up with different pids
find /run -name "*.pid" -delete
if [ -d /var/vcap/sys/run ]; then
    find /var/vcap/sys/run -name "*.pid" -delete
fi

# Note, any changes to this list of variables have to be replicated in
# --> model/mustache.go, func builtins
export IP_ADDRESS=$(ip route | grep -v ^default | awk '{print $NF;exit}')
export DNS_RECORD_NAME=$(/bin/hostname)
export KUBE_COMPONENT_INDEX="${HOSTNAME##*-}"
if test -n "${KUBE_COMPONENT_INDEX//[0-9]/}" ; then
  # The instance id wasn't a valid number; make it one
  # We use the gawk expression to ensure we have a unique instance id across all
  # active containers.  The name was generated via
  # https://github.com/kubernetes/kubernetes/blob/v1.7.0/pkg/api/v1/generate.go#L59
  # https://github.com/kubernetes/apimachinery/blob/b166f81f/pkg/util/rand/rand.go#L73
  export KUBE_COMPONENT_INDEX="$(
    echo -n ${HOSTNAME##*-} \
    | gawk -vRS=".|" ' BEGIN { chars="bcdfghjklmnpqrstvwxz0123456789" } { n = n * length(chars) + index(chars, RT) - 1 } END { print n }'
  )"
fi
if test -z "${KUBE_SERVICE_DOMAIN_SUFFIX:-}" && grep -E --quiet '^search' /etc/resolv.conf ; then
  export KUBE_SERVICE_DOMAIN_SUFFIX="$(awk '/^search/ { print $2 }' /etc/resolv.conf)"
fi

# Write a couple of identification files for the stemcell
mkdir -p /var/vcap/instance
echo {{ .role.Name }} > /var/vcap/instance/name
echo "${KUBE_COMPONENT_INDEX}" > /var/vcap/instance/id

# Run custom environment scripts (that are sourced)
{{ range $script := .role.EnvironScripts }}
    source {{ if not (is_abs $script) }}/opt/scf/startup/{{ end }}{{ $script }}
{{ end }}
# Run custom role scripts
{{ range $script := .role.Scripts}}
    bash {{ if not (is_abs $script) }}/opt/scf/startup/{{ end }}{{ $script }}
{{ end }}

configgin \
	--jobs /opt/scf/job_config.json \
	--env2conf /opt/scf/env2conf.yml

if [ -e /etc/monitrc ]
then
  chmod 0600 /etc/monitrc
fi

# Create run dir
mkdir -p /var/vcap/sys/run
chown root:vcap /var/vcap/sys/run
chmod 775 /var/vcap/sys/run

# Fix permissions
chmod 640 /var/log/messages
if [ -d /var/spool/cron/tabs ]
then
  chmod 1730 /var/spool/cron/tabs/
fi

{{ if eq .role.Type "bosh-task" }}
    # Start rsyslog and cron
    /usr/sbin/rsyslogd
    cron
{{ else }}
    # rsyslog and cron are started via monit
{{ end }}

# Run custom post config role scripts
# Run any custom scripts other than pre-start
{{ range $script := .role.PostConfigScripts}}
{{ if not (is_pre_start $script) }}
    echo bash {{ if not (is_abs $script) }}/opt/scf/startup/{{ end }}{{ $script }}
    bash {{ if not (is_abs $script) }}/opt/scf/startup/{{ end }}{{ $script }}
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
    {{ range $job := .role.RoleJobs}}
        /var/vcap/jobs/{{ $job.Name }}/bin/run
    {{ end }}
{{ else }}

  killer() {
    # Wait for all monit services to be stopped
    echo "Received SIGTERM. Will run 'monit stop all'."

    total_services=$(monit summary | grep -c "^Process")

    monit stop all

    echo "Ran 'monit stop all'."

    while [ $total_services != $(monit summary | grep "^Process" | grep -c "Not monitored") ] ; do
       sleep 1
    done

    echo "All monit processes have been stopped."
    monit summary
    monit quit
  }

  trap killer SIGTERM

  if [[ "${LOG_LEVEL}" == "debug"* || -n "${LOG_DEBUG}" ]]; then
    # monit -v without the -I would fork a child, but then we can't wait on it,
    # so it's not very useful.
    monit -vI &
  else
    monit -I &
  fi
  child=$!
  wait "$child"
{{ end }}
