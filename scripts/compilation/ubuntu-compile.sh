set -e # exit immediately if a simple command exits with a non-zero status

packageName=$1
packageVersion=$2

if [ -z "$packageName" ]; then
  echo "Package name not specified" >&2
fi

if [ -z "$packageVersion" ]; then
  echo "Package version not specified" >&2
fi


if [ -z "$VAR" ];

mkdir -p /var/vcap

cp -r /fissile-in/var/vcap/* /var/vcap

export BOSH_COMPILE_TARGET=/var/vcap/source
export BOSH_INSTALL_TARGET=/fissile-out
export BOSH_PACKAGE_NAME=$packageName
export BOSH_PACKAGE_VERSION=$packageVersion

cd $BOSH_COMPILE_TARGET
bash -x ./packaging
