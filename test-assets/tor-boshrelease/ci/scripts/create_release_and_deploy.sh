#!/bin/bash

# change to root of bosh release
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd $DIR/../..

cat > ~/.bosh_config << EOF
---
aliases:
  target:
    bosh-lite: ${bosh_target}
auth:
  ${bosh_target}:
    username: ${bosh_username}
    password: ${bosh_password}
EOF
bosh target ${bosh_target}

_bosh() {
  bosh -n $@
}

set -e
git submodule update --init --recursive --force

_bosh delete deployment ${bosh_deployment_name} --force || echo "Continuing..."
_bosh create release
set +e
_bosh upload release --rebase || echo "Continuing..."
set -e

./templates/make_manifest warden

_bosh deploy
