#!/bin/bash
#set -e
# Cannot use this generally. Interferes with the check via `monit summary`.
# I.e. when things are ready the failure of grep to match aborts us.

# Check for post-start scripts to run. We may not have any.
#
# * `flock` is used to guard against monit starting this script
#   multiple times when our dependency checking and/or the invoked
#   scripts are taking too long.
#
# * `monit summary` is used to check if the __other__ jobs are up.
#   Nothing is done while they are not up yet. We know that we are
#   `post-start`, important to exclude ourselves from the check
#
# Doing our own dependency checking works around issues in monit.
# This can be shifted to monit itself ('depends on') when we reach use
# of monit v5.15+ where the issues are fixed.

(
  flock -n 9 || exit 1

  notyet=$(monit summary | tail -n+3 | grep -v post-start | grep -v 'Accessible\|Running')
  if [ -z "$notyet" ]
  then
      scripts="$(find /var/vcap/jobs/*/bin -name post-start)"
      set -e
      for fname in ${scripts}
      do
	  echo bash $fname
	  bash $fname
      done
      touch /var/vcap/monit/ready
  fi
) 9> /var/vcap/monit/ready.lock
