#!/usr/bin/env bash

# Log duration of readiness probes.
START="$(date +%s)"
trap runtime EXIT
runtime () {
    STOP="$(date +%s)"
    echo "TxPROBE,${KUBERNETES_NAMESPACE},${HOSTNAME},${START},${STOP},$(expr $STOP - $START)" >> rp-duration-stats.csv
    return $?
}

# This is the default readiness probe, which will look at all monit monitored
# processes and check that they are ready.

# It may optionally be launched with other scripts as arguments; for each
# argument, it will run it as a command, and report not ready if any one returns
# a non-zero exit status.

# If the enviroment variable `FISSILE_ACTIVE_CHECK` is set, that is assumed to
# be a command which, when run, will report if this pod should be placed in the
# set of pods accepting traffic.  An exit status of zero in that case is assumed
# to mean the pod is ready for traffic.

###

set -o errexit -o nounset -o pipefail

# Set up the readiness flag ahead of time, so if we error out we mark this pod
# as not ready
readiness=false
update_readiness() {
    local svcacct=/var/run/secrets/kubernetes.io/serviceaccount
    curl --silent \
        --cacert "${svcacct}/ca.crt" \
        --header "Authorization: bearer $(cat "${svcacct}/token")" \
        --header 'Content-Type: application/merge-patch+json' \
        --output /dev/null \
        --request 'PATCH' \
        "https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}/api/v1/namespaces/$(cat "${svcacct}/namespace")/pods/${HOSTNAME}" \
        --data '{
            "metadata": {
                "labels": {
                    "skiff-role-active": "'"${readiness}"'"
                }
            }
        }'
    runtime
    return $?
}
if test -n "${FISSILE_ACTIVE_PASSIVE_PROBE:-}" ; then
    trap update_readiness EXIT
fi

if [ ! -r /etc/monitrc ] ; then
    echo "Waiting for monit to start"
    exit 1
fi

# Grab monit port
monit_port=$(awk '/httpd port/ { print $4 }' /etc/monitrc)

# Check that monit thinks everything is ready
curl -s -u admin:"${MONIT_PASSWORD}" http://127.0.0.1:"${monit_port}"/_status | gawk '
    $1 == "status" && $2 != "running" && $2 != "accessible"   { print "Waiting for monit to be ready"; exit 1 }
    '

# Check that any additional readiness checks are ready
for command in "${@}" ; do
    /usr/bin/env bash -c "${command}"
done

# If this is an active/passive role, do that check
if test -n "${FISSILE_ACTIVE_PASSIVE_PROBE:-}" ; then
    if eval "${FISSILE_ACTIVE_PASSIVE_PROBE}" ; then
        readiness=true
    fi
fi
