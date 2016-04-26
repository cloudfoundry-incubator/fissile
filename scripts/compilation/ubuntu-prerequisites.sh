set -e # exit immediately if a simple command exits with a non-zero status
set -u # report the usage of uninitialized variables

# Use the following for inspiration:
# https://github.com/cloudfoundry/bosh/tree/master/stemcell_builder/stages
# 
# We don't want to use the same mechanisms as a stemcell.
# Containers should be more lightweight, and we should be able 
# to cherry pick and customize our dependencies

# TODO installation of libyaml should be part of the ruby installation, 
# or it should be a package of its own; here we install 0.1.4, 
# upstream installs 0.1.6

debs="build-essential libssl-dev lsof strace bind9-host \
dnsutils tcpdump iputils-arping \
curl wget libcurl3 libcurl3-dev bison libreadline6-dev \
libxml2 libxml2-dev libxslt1.1 libxslt1-dev zip unzip \
nfs-common flex psmisc apparmor-utils iptables sysstat \
rsync openssh-server traceroute libncurses5-dev quota \
libaio1 gdb libcap2-bin libcap2-dev libbz2-dev \
cmake uuid-dev libgcrypt-dev ca-certificates \
scsitools mg htop module-assistant debhelper runit parted \
anacron software-properties-common libyaml-dev gettext git"

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -o Dpkg::Options::="--force-confnew" -f -y --force-yes --no-install-recommends $debs

# Add the vcap:vcap user to match CF
useradd -m --comment 'hcf user' vcap
