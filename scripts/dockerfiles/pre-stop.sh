#! /bin/bash

# This script is executed as the Kubernetes pre-stop hook, which will in turn
# excute the BOSH drain scripts

# See https://bosh.io/docs/job-lifecycle.html#stop
# https://bosh.io/docs/drain.html

# Pre-Stop scripts don't have the normal stdout / stderr
# Redirect everything to the ones that pid 1 uses
exec 1> /proc/1/fd/1
exec 2> /proc/1/fd/2

set -o errexit

echo "Running pre-stop script..."

{{ if ne .role.Type "bosh-task" }}
    processes=($(/var/vcap/bosh/bin/monit summary | awk '$1 == "Process" { print $2 }' | tr -d "'"))

    # Lifecycle: Stop: 1. `monit unmonitor` is called for each process
    echo "${processes[@]}" | xargs -n1 /var/vcap/bosh/bin/monit unmonitor

    # Lifecycle: Stop: 2. Drain scripts
    drain_job() {
        if ! [ -x "/var/vcap/jobs/$1/bin/drain" ] ; then
            return 0
        fi
        printf "Running drain script for %s\n" "$1" >&2

        while true ; do
            # Tee the output to main container logs too, so we can see issues
            output="$("/var/vcap/jobs/$1/bin/drain" > >(tee /proc/1/fd/1))"
            result="$?"
            if test "${result}" -ne 0 ; then
                # drain script exited with non-zero; abort with that code
                printf "Pre-stop script for %s terminated with %s\n" "$1" "${result}" >&2
                exit "${result}"
            fi
            # stdout is expected to be a number, possibly followed by a new line
            # If it is >= 0, wait that many seconds and go to next script
            # If it is < 0, sleep for that many seconds, then retry
            if test "${output}" -lt 0 ; then
                sleep $(( 0 - output ))
            else
                sleep "${output}"
                break
            fi
        done
    }
    {{ range $job := .role.RoleJobs }}
        drain_job {{ $job.Name }}
    {{ end }}

    # Lifecycle: Stop: 3. `monit stop` is called for each process
    echo "${processes[@]}" | xargs -n1 /var/vcap/bosh/bin/monit stop
{{ end }}

echo "Pre-stop: All scripts completed successfully" >&2
exit 0
