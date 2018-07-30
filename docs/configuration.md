# Fissile Configuration
Fissile requires some configuration to work with BOSH releases.  It is necessary
to describe what docker images should be created; in general, each docker image
(termed "role") will have one or more BOSH jobs.

There are three main files for configuration: the [role manifest], [opinions],
and [dark opinions].  A set of [environment files] will also be required.

For examples of established configuration, please see [uaa-fissile-release.git]
or [scf.git].  They are, respectively, configuration for UAA and a full
(single-node) Cloud Foundry deployment.

[role manifest]: #role-manifest
[opinions]: #opinions-dark-opinions-and-environment
[dark opinions]: #opinions-dark-opinions-and-environment
[environment files]: #opinions-dark-opinions-and-environment
[uaa-fissile-release.git]: https://github.com/SUSE/uaa-fissile-release
[scf.git]: https://github.com/SUSE/scf/

The rest of this document will also, as an example, describe how to create
fissile configuration to place `nats` in a docker image.

## Role Manifest
The role manifest is the main configuration file for fissile.  It must contain a
list of roles (analogous to BOSH VMs); each role will result in one docker
image.  It can also have a configurable variables section to describe the
tunable inputs, and a configuration templates section to map those variables to
BOSH properties and related BOSHisms.  We will be working with the example role
manifest for NATS:

```yaml
instance_groups:
- name: nats                       # The name of the role
  jobs:                            # BOSH jobs this role will have
  - name: nats
    release_name: nats             # The name of the BOSH release this is from
  tags:
  - indexed                        # Mark this role as indexed (load-balanced) => StatefulSet
  run:                             # Runtime configuration
    scaling:                       # Auto-scaling limits
      min: 1
      max: 3
    memory: 256                    # Memory request for each instance (MB)
    virtual-cpus: 4                # CPU request for each instance
    exposed-ports:
    - name: nats
      protocol: TCP                # TCP or UDP
      external: 4222               # Port visible outside the container
      internal: 4222               # Port inside the container
      public: false                # Whether to expose to outside the cluster
    - name: nats-routes
      protocol: TCP
      external: 4223
      internal: 4223
      public: false

configuration:
  templates:
    networks.default.dns_record_name: '"((DNS_RECORD_NAME))"'
    networks.default.ip: '"((IP_ADDRESS))"'
    properties.nats.user: '"((NATS_USER))"' # In BOSH templates, `p('nats.user')`
    properties.nats.password: '"((NATS_PASSWORD))"'

  variables:
  - name: NATS_PASSWORD
    description: Password for NATS
    secret: true
    required: true
  - name: NATS_USER
    description: User name for NATS
    required: true
    previous_names: [NATS_USR]
```

Note that there are a few special variables that are automatically supplied to
the container (via [run.sh]).  They are:

Name | Description
-- | --
`DNS_RECORD_NAME` | Hostname of the container
`IP_ADDRESS` | Primary IP address of the container
`KUBE_COMPONENT_INDEX` | Numeric index for roles with multiple replicas
`KUBERNETES_CLUSTER_DOMAIN` | Kubernetes cluster domain, `cluster.local` by default

[run.sh]: https://github.com/SUSE/fissile/blob/master/scripts/dockerfiles/run.sh

There are also some fields not shown above (as the are not needed for NATS):

For the role:

Name | Description
-- | --
`scripts` | scripts relative to the role manifest that are executed before expanding BOSH templates and starting jobs
`environment_scripts` | scripts that are sourced in bash (and could modify environment variables); executed before `scripts` above.
`post_config_scripts` | scripts executed after BOSH templates have been expanded, before starting jobs
`type` | `bosh` or `bosh-task`; the latter will result in a Kubernetes Job

For the `run` section:

Name | Description
-- | --
`capabilities` | additional capabilities to grant the container (see `man 7 capabilities`); drop the `CAP_` prefix (e.g. use `NET_ADMIN`)
`persistent-volumes` | volumes to attach to the role
`shared-volumes` | volumes shared across all containers of the role
`healthcheck` | optional healthchecking parameters, see below
`env` | list of environment variables, as `FOO=bar`
`flight-stage` | one of `pre-flight`, `post-flight`, `manual`, or `flight` (default).  The first three are for jobs.

### Health Checking
A `run` section can optionally have health checking via [Kubernetes container
probes].  The `healthcheck` field may have `liveness` and `readiness` subfields,
each with one the following:

Name | Description
-- | --
`url` | URL to `HTTP GET`; expects a 2xx or 3xx reply. Use `container-ip` as the hostname.
`command` | Command to run; should be a list of strings.
`port` | TCP port to connect to; success is declared when the port is open

Additionally, the following options are available:

Name | Description
-- | --
`initial_delay` | wait this many seconds before performing the first check
`period` | wait this many seconds between check check
`timeout` | timeout for unresponsive checks
`success_threshold` | minimal consecutive successful checks required to be considered up
`failure_threshold` | minimal consecutive failed checks required to be considered down, after previously have been successful

For `url` type checks, a `headers` map is also available for additional HTTP
headers (for example, to set the `Accept:` header to request JSON responses).

[Kubernetes container probes]: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#container-probes

## Tagging

The NATS role above was tagged as `indexed`, causing fissile to emit
it as a [StatefulSet].

The second way of causing fissile to do that is to tag a role as
`clustered`. The main difference between `clustered` and `indexed`
roles is that fissile creates a public service (of type
`LoadBalancer`) for the latter, providing a single point of access to
the pods for the role.

Note that both `clustered` and `indexed` roles can take advantage of
volume claim templates for local storage.

Therefore the user should index roles which require load balancing and
need a 0-based, incremented index, and mark them as clustered
otherwise.

An example of a clustered role is the MYSQL database of CF. See the
example below. While mysql actually needs a load balancer for access
this role is made explicit in CF through role `mysql-proxy`.

```yaml
instance_groups:
- name: mysql
  jobs:
  - name: mysql
    release_name: cf-mysql
    provides:
      mysql: {}
  processes:
  - name: mariadb_ctrl
  - name: galera-healthcheck
  - name: gra-log-purger-executable
  tags:
  - clustered
  # No implicit LB, handled by mysql-proxy, and use of volumes.
  run:
    scaling:
      min: 1
      max: 3
      ha: 2
    capabilities: []
    volumes:
    - path: /var/vcap/store
      tag: mysql-data
      size: 20
      type: persistent
    memory: 2841
    virtual-cpus: 2
    exposed-ports:
    - name: mysql
      protocol: TCP
      internal: 3306
    [...]
    healthcheck:
      readiness:
        url: http://container-ip:9200/
  configuration:
    templates:
      [...]
```

[StatefulSet]: https://kubernetes.io/docs/resources-reference/v1.6/#statefulset-v1beta1-apps

## Opinions, Dark Opinions, and Environment

For BOSH properties that are constant across deployments, but that do not match
the upstream defaults, they can be stored in an opinions file which will be
embedded within the docker image.  An additional dark opinions file is used to
ensure that we block out anything that must be different per-cluster (for
example, passwords).  A third file is used for the variables found in last
section of the role manifest.  For the NATS role, we can use the following files:

- opinions.yml
  ```yaml
  properties:
    nats:
      port: 4222
  ```

- dark-opinions.yml

  ```yaml
  properties:
    nats:
      password: ""
  ```

- defaults.txt
  ```bash
  NATS_PASSWORD=nats_password
  ```

## Fissile command line options

All fissile options are also available as environment variables.  For that NATS
release, we are just setting the paths to the things we created above.  For
additional options, please refer to the [command documentation]:

[command documentation]: generated/fissile.md

```bash
# This is a comma separated list of paths to the local repositories
# of all the releases
export FISSILE_RELEASE="nats-release"

# Path to a role manifest
export FISSILE_ROLE_MANIFEST="role-manifest.yml"

# Path to a BOSH deployment manifest that contains light opinions
export FISSILE_LIGHT_OPINIONS="opinions.yml"

# Path to a BOSH deployment manifest that contains dark opinions
export FISSILE_DARK_OPINIONS="dark-opinions.yml"

# Path to a location where all fissile output is stored
export FISSILE_WORK_DIR="output/fissile"

# The image to use as the stemcell; note that the specific image here may be outdated
export FISSILE_STEMCELL="splatform/fissile-stemcell-opensuse:42.2-6.ga651b2d-28.33"
```

## Building the NATS Image

We can now assemble all the files necessary from the information above:

```bash
$ vim role-manifest.yml # See above for contents
$ vim opinions.yml
$ vim dark-opinions.yml
$ vim defaults.txt
$ git clone https://github.com/cloudfoundry/nats-release.git # from $FISSILE_RELEASE
$ git -C nats-release submodule update --init --recursive
```

We will also need to create a BOSH dev release:
```bash
$ rm -rf nats-release/dev_releases
$ docker run --rm \
    --volume "${HOME}/.bosh/cache:/bosh-cache" \
    --volume "${PWD}/nats-release:${PWD}/nats-release" \
    --env "RUBY_VERSION=2.2.3" \
    splatform/bosh-cli \
    /usr/local/bin/create-release.sh \
        "$(id -u)" "$(id -g)" /bosh-cache --dir "${PWD}/nats-release" --force --name "nats"
```

Finally, use fissile to build the image and Kubernetes configs
```bash
# Compile packages from the nats release
fissile build packages

# Build the nats docker image
fissile build images

# Build kubernetes deployment yaml
fissile build kube --defaults-file=defaults.txt
```
