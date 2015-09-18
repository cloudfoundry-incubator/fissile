
# Use the following for inspiration:
# https://github.com/cloudfoundry/bosh/tree/master/stemcell_builder/stages
# 
# We don't want to use the same mechanisms as a stemcell.
# Containers should be more lightweight, and we should be able 
# to cherry pick and customize our dependencies

debs="libssl-dev lsof strace bind9-host dnsutils tcpdump iputils-arping \
curl wget libcurl3 libcurl3-dev bison libreadline6-dev \
libxml2 libxml2-dev libxslt1.1 libxslt1-dev zip unzip \
nfs-common flex psmisc apparmor-utils iptables sysstat \
rsync openssh-server traceroute libncurses5-dev quota \
libaio1 gdb libcap2-bin libcap2-dev libbz2-dev \
cmake uuid-dev libgcrypt-dev ca-certificates \
scsitools mg htop module-assistant debhelper runit parted \
anacron software-properties-common"
apt-get install -y $debs

