# bosh-ntp-release
BOSH release for the NTP (time) Server

## How To

### 0. Install BOSH and BOSH CLI
BOSH runs in a special VM which will need to be deployed prior to deploying this NTP release. You will also need to have installed the BOSH CLI on your local workstation (i.e. the *bosh_cli* Ruby gem)

### 1. Target BOSH and login
We assume you're using [BOSH Lite](https://github.com/cloudfoundry/bosh-lite) (*BOSH* under VirtualBox); however, if you have already deployed a *MicroBOSH* or full *BOSH*, then substitute the correct IP address/hostname and credentials below.

Target the IP address (defaults to 192.168.50.4) and log in with the default account and password (admin/admin):

```
bosh target 192.168.50.4
bosh login admin admin
```

### 2. Clone and *cd* to this repo
```
git clone https://github.com/cunnie/bosh-ntp-release.git
cd bosh-ntp-release
```

### 3. Download and upload the stemcells to BOSH
```
mkdir stemcells
pushd stemcells
curl -OL https://s3.amazonaws.com/bosh-warden-stemcells/bosh-stemcell-2776-warden-boshlite-centos-go_agent.tgz
popd
bosh upload stemcell stemcells/bosh-stemcell-2776-warden-boshlite-centos-go_agent.tgz
```

### 4. Create and upload the BOSH Release
```
bosh create release --force
    Please enter development release name: ntp
bosh upload release dev_releases/ntp/ntp-0+dev.1.yml
```
If you iterate through several releases, remember to increment the release number when uploading (e.g. "...9-0+dev.2.yml").

### 5. Create Manifest from Example
We copy the manifest template and set its UUID to our BOSH's UUID.

If you're not using *BOSH Lite*, edit the manifest to change the network information and IP addresses:

```
cp examples/ntp-bosh-lite.yml config/
perl -pi -e "s/PLACEHOLDER-DIRECTOR-UUID/$(bosh status --uuid)/" config/ntp-bosh-lite.yml
```

### 6. Deploy and Test
If you're not using *BOSH Lite*, then substite the correct IP address when you use the *nslookup* command. The IP address is available from your deployment manifest or by typing `bosh vms`.

```
bosh deployment config/ntp-bosh-lite.yml
bosh -n deploy
# if you're using BOSH Lite, you'll probably need
# to add a route similar to something like this
sudo route add -net 10.244.0.0/24 192.168.50.4
#  attempt the lookup
ntpdate -q 10.244.0.66 # os x, old linux
```

### 7. [Optional] Comment-out `ntpdate` cronjob

Our stemcell had an `ntpdate` cronjob; we commented it out: `ntpdate` is a coarse timekeeper and not necessary when `ntpd` is running. We ssh'ed into our VM and did the following:

```
sudo crontab -e
    #0,15,30,45 * * * * /var/vcap/bosh/bin/ntpdate
```

### Stemcells

This has been tested with the following stemcells:

* bosh-aws-xen-hvm-centos-7-go_agent version 2962

### Bugs

Certain stemcells include their own ntpd (BOSH Lite, IIRC)
