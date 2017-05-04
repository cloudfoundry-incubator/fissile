#!/usr/bin/env bash
set -o errexit -o nounset

usage() {
  echo "Package ${1} not specified" >&2
  echo "Usage: ${0} <package> <version> [buildroot]" >&2
  exit 1
}

packageName="${1:-}"
packageVersion="${2:-}"
if test -z "${packageName}" ; then
  usage name
fi
if test -z "${packageVersion}" ; then
  usage version
fi

if test -d "/fissile-in" ; then
  mkdir -p "/var/vcap"
  cp -r /fissile-in/var/vcap/* /var/vcap
else
  # Running new in new mount ns
  buildroot="${3:-}"
  if test -z "${buildroot}" ; then
    usage "build root"
  fi
  mkdir -p /var/vcap
  mount --bind "${buildroot}/${packageVersion}/sources/var/vcap" /var/vcap
fi

export BOSH_COMPILE_TARGET="/var/vcap/source/$packageName"
export BOSH_INSTALL_TARGET="/var/vcap/packages/$packageName"
export BOSH_PACKAGE_NAME="${packageName}"
export BOSH_PACKAGE_VERSION="${packageVersion}"

echo "Compiling to ${BOSH_INSTALL_TARGET}"

if test -d "/fissile-out" ; then
  ln -s /fissile-out "${BOSH_INSTALL_TARGET}"
else
  rm -rf "${buildroot}/${packageVersion}/compiled-temp"
  mkdir -p "${buildroot}/${packageVersion}/compiled-temp"
  mkdir -p "${BOSH_INSTALL_TARGET}"
  mount --bind "${buildroot}/${packageVersion}/compiled-temp" "${BOSH_INSTALL_TARGET}"
fi

cd "${BOSH_COMPILE_TARGET}"
bash ./packaging

chown -R "${HOST_USERID}:${HOST_USERGID}" "$(readlink --canonicalize "${BOSH_INSTALL_TARGET}")" 2>/dev/null \
  || echo "Warning - could not change ownership of compiled artifacts" 1>&2
