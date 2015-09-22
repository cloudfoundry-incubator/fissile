set -e # exit immediately if a simple command exits with a non-zero status

packageName=$1
packageVersion=$2

if [ -z "$packageName" ];
then
  echo "Package name not specified" 1>&2
  exit 1
fi

if [ -z "$packageVersion" ];
then
  echo "Package version not specified" 1>&2
  exit 1
fi

mkdir -p /var/vcap

cp -r /fissile-in/var/vcap/* /var/vcap

export BOSH_COMPILE_TARGET="/var/vcap/source/$packageName"
export BOSH_INSTALL_TARGET="/var/vcap/packages/$packageName"
export BOSH_PACKAGE_NAME=$packageName
export BOSH_PACKAGE_VERSION=$packageVersion

echo "Compiling to $BOSH_INSTALL_TARGET"

ln -s /fissile-out $BOSH_INSTALL_TARGET

cd $BOSH_COMPILE_TARGET
bash ./packaging

