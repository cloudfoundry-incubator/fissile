#!/usr/bin/env bash

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

set -o errexit -o nounset

# Check that monit thinks everything is ready
/var/vcap/bosh/bin/monit summary | gawk '
    BEGIN                                 { status = 0 }
    $1 == "Process" && $3 != "running"    { print ; status = 1 }
    $1 == "File"    && $3 != "accessible" { print ; status = 1 }
    END                                   { exit status }
    '

# Check that any additional readiness checks are ready
for command in "${@}" ; do
    /usr/bin/env bash -c "${command}"
done

# If this is an active/passive role, do that check
if test -n "${FISSILE_ACTIVE_PASSIVE_PROBE:-}" ; then

    update_readiness() {
        local svcacct=/var/run/secrets/kubernetes.io/serviceaccount
        curl --silent \
            --cacert "${svcacct}/ca.crt" \
            -H "Authorization: bearer $(cat "${svcacct}/token")" \
            -H 'Content-Type: application/merge-patch+json' \
            -X 'PATCH' \
            "https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}/api/v1/namespaces/$(cat "${svcacct}/namespace")/pods/${HOSTNAME}" \
            --data '{
                "metadata": {
                    "labels": {
                        "skiff-role-active": "'"${1}"'"
                    }
                }
            }'
        return $?
    }

    if eval ${FISSILE_ACTIVE_PASSIVE_PROBE} ; then
        update_readiness true
    else
        update_readiness false
    fi
fi
