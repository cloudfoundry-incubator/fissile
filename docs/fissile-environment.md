## local environment setup

If you plan to setup a local environment to be able to build docker images with
Cloud Foundry components, you need to create a directory where you should store
specific release source code, dark-opinions.yml, opinions.yml, role-manifest.yml
and files with environment variables.

From this document you will gain knowledge on how to build a docker image for
`nats`. You will then be able to apply the procedure for other Cloud Foundry
components.

### prerequisites

#### fissile

Make sure you have `fissile` in your path. You can build it yourself from
[source](https://github.com/SUSE/fissile) or grab a binary from the releases
page [here](https://github.com/SUSE/fissile/releases).

#### yaml to json converter

Install `y2j` converter

```
docker run --rm wildducktheories/y2j y2j.sh installer /usr/local/bin | sudo bash
```

### create build directory

Create some base directory (`mkdir cf-build`) where you can store the following files:

- role-manifest.yml

  ```yaml
  roles:
  - name: nats
    jobs:
    - name: nats
      release_name: nats
    - name: nats_stream_forwarder
      release_name: nats
    run:    
      scaling:
        min: 1
        max: 3    
      memory: 256    
      virtual-cpus: 4    
      exposed-ports:
      - name: nats
        protocol: TCP      
        external: 4222   
        internal: 4222   
        public: false    
      - name: nats-routes      
        protocol: TCP      
        external: 4223      
        internal: 4223      
        public: false

  configuration:
    templates:
      index: '0'
      networks.default.dns_record_name: '"((DNS_RECORD_NAME))"'
      networks.default.ip: '"((IP_ADDRESS))"'
      properties.nats.user: '"((NATS_USER))"'
      properties.nats.password: '"((NATS_PASSWORD))"'
  ```

- opinions.yml

  ```yaml
  properties:
    nats:
      debug: false
      monitor_port: 0
      port: 4222
      prof_port: 0
      trace: false
  ```

- dark-opinions.yml

  ```yaml
  properties:
    nats:
      password: ""
  ```

- fissilerc

  ```shell
  # The Docker repository name used for images
  export FISSILE_REPOSITORY=fissile

  # This is a comma separated list of paths to the local repositories
  # of all the releases
  export FISSILE_RELEASE="releases/nats-release"

  # Path to a role manifest
  export FISSILE_ROLE_MANIFEST="role-manifest.yml"

  # Path to a BOSH deployment manifest that contains light opinions
  export FISSILE_LIGHT_OPINIONS="opinions.yml"

  # Path to a BOSH deployment manifest that contains dark opinions
  export FISSILE_DARK_OPINIONS="dark-opinions.yml"

  # Path to a location where all fissile output is stored
  export FISSILE_WORK_DIR="output/fissile"

  # This is the location of the local BOSH cache
  # You shouldn't need to override this
  # This will be ~/.bosh/cache in vagrant
  export FISSILE_CACHE_DIR="${HOME}/.bosh/cache"

  # Those variables are used to create BOSH releases for nats
  export ROOT=$(pwd)

  export release_path=releases/nats-release

  export release_name=nats
  ```

### get nats release

Create an additional directory for releases and clone the `nats` release:

```
mkdir releases
cd releases
git clone https://github.com/cloudfoundry/nats-release.git
```

Make sure you initialize submodules by running the following:

```
cd nats-release
git submodule sync --recursive
git submodule update --init  --recursive
```

### building

```
# Go to the base directory
cd ../..

# You need to source fissilerc
source fissilerc
```

Create a BOSH dev release for nats. All you need to do is run the
following command. Things will run in a docker container and we'll get a cache
of BOSH objects (jobs and packages) that are used by fissile to build the image
we want.

```
rm -rf releases/nats-release/dev-releases/nats

docker run \
  --interactive \
  --rm \
  --volume ${HOME}/.bosh:/root/.bosh \
  --volume $ROOT/:$ROOT/ \
  --env RBENV_VERSION="2.2.3" \
  helioncf/hcf-pipeline-ruby-bosh \
  bash -l -c "rm -rf ${ROOT}/${release_path}/dev_releases && bosh --parallel 10 create release --dir ${ROOT}/${release_path} --force --name ${release_name}"
```

Finally, use fissile to build the image and Kubernetes configs

```
# Create the needed Docker layers
fissile build layer compilation
fissile build layer stemcell

# Compile packages from the nats release
fissile build packages

# Build the nats docker image
fissile build images

# (optional) tag it to something more human readable
docker tag $(fissile show image) fissile-nats:latest

# Build kubernetes deployment yaml
fissile build kube
```
