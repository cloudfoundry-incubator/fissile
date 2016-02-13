Testing PR checker
Testing PR checker
Testing PR checker

![fissile-logo](https://region-b.geo-1.objects.hpcloudsvc.com/v1/10990308817909/pelerinul/fissile-logo.png)


Fissile converts an existing BOSH release (packaged, like the ones you download from bosh.io) into docker images.
It’s supposed to do this using just the release itself, without a BOSH deployment, CPIs, or a BOSH agent.


At a high level, fissile helps you go through the following process:

![fissile-highlevel](https://docs.google.com/drawings/d/1cu3H9UHKH6qD4JNQtMoCpMlBDdiIj0KYWQ4IT5yJS3Q/export/png)


## Usage

### Dependencies (in what order should I run everything?)

The following diagram shows the ordering of things. The highlighted items are commands you need to get to a final build.

![fissile-command-dependencies](https://docs.google.com/drawings/d/1E7qBfIM1z_23hWE0OlIxx3O8EWY6taoxq8MWrJOuQQM/export/png)

### Commands (what can it do?)

#### `release`

- `release jobs-report`
 
 -  `--release <RELEASE_PATH>` path to a BOSH release **(not optional)**
 
 > Displays a report of all jobs in the release. The report contains the name, version and description  of each job and the total count of all jobs.
 > If there's any error reading the release, the command will terminate with exit code 1.
 > e.g.
 > ```bash
 > fissile release jobs-report \
 >     --release ~/fissile-releases/cf-release-v217
 > ```
 
- `release packages-report`

 -  `--release <RELEASE_PATH>` path to a BOSH release **(not optional)**
 
  > Displays a report of all packages in the release. The report contains the name and version of each package and the total count of all packages.
 > If there's any error reading the release, the command will terminate with exit code 1.
 > ```bash
 > fissile release packages-report \
 >     --release ~/fissile-releases/cf-release-v217
 > ```

#### `compilation`

- `compilation build-base`

 - `--base-image <DOCKER_BASE_IMAGE>` the name of the docker image to be used as a base for the compilation base image; this parameter has a default value of `ubuntu:14.04`
 - `--repository <REPOSITORY_PREFIX>` a repository prefix to be used when naming the image; the final image name will have this form: `<REPOSITORY_PREFIX>-cbase:<FISSILE_VERSION>`

 > This command creates a container with the name `<REPOSITORY_PREFIX>-cbase-<FISSILE_VERSION>` and runs compilation prerequisite scripts within. Once the prerequisites scripts complete successfully, an image named  `<REPOSITORY_PREFIX>-cbase:<FISSILE_VERSION>` is created and the created container is removed. If the prerequisites script fails, the container is not removed.
 > If the compilation base image already exists, this command does not do anything.
 > ```bash
 > fissile compilation build-base \
 >     --base-image ubuntu:14.04 \
 >     --repository fissile
 > ```

- `compilation show-base`

 - `--base-image <DOCKER_BASE_IMAGE>` the name of the docker image used as a base for the compilation image; this parameter has a default value of `ubuntu:14.04`
 - `--repository <REPOSITORY_PREFIX>` the repository prefix used when naming the compilation image base; this parameter has a default value of `fissile`
 
 > This command will show the name, ID and virtual size of the base (i.e. `ubuntu:14.04`) and the compilation image (i.e. `fissile-cbase:1.0.0`).
 > ```bash
 > fissile compilation show-base \
 >     --base-image ubuntu:14.04 \
 >     --repository fissile
 > ```

- `compilation start`

 - `--repository <REPOSITORY_PREFIX>` the repository prefix used when naming the compilation image base; this parameter has a default value of `fissile`
 - `--release <RELEASE_PATH>` path to a BOSH release **(not optional)**
 - `--target <TARGET_DIRECTORY>` path to a directory where all compilation artifacts will be written **(not optional)**
 - `--workers <WORKER_COUNT>` the number of workers (containers) to use for package compilation

 > This command will compile all packages in a BOSH release. The command will create a compilation container named `<REPOSITORY_PREFIX>-cbase-<FISSILE_VERSION>-<RELEASE_NAME>-<RELEASE_VERSION>-pkg-<PACKAGE_NAME>` for each package (e.g. `fissile-cbase-1.0.0-cf-217-pkg-nats`). All containers are removed, whether compilation is successful or not. However, if the compilation is interrupted during compilation (e.g. ending `SIGINT`), containers will most likely be left behind.
 > 
 > It's safe (and desired) to use the same target directory when compiling multiple releases, because `fissile` uses the package's fingerprint as part of the directory structure. This means that if the same package (with the same version) is used by multiple releases, it will only be compiled once.
 >
 > The target directory will have the following structure:
 > ```
 >  .
 >  └── <pkg-name>
 >     └── <pkg-fingerprint>
 > 	     ├── compiled
 > 	     ├── compiled-temp
 > 	     └── sources
 > 	         └── var
 > 	             └── vcap
 > 	                 ├── packages
 > 	                 │   └── <dependency-package>
 > 	                 └── source                  
 > ```
 > The `compiled-temp` directory is renamed to `compiled` upon success. The rest of the directories contain sources and dependencies that are used during compilation.
 > ```bash
 > fissile compilation start \
 >     --repository fissile \
 >     --release ~/fissile-releases/cf-release-v217 \
 >     --target ~/fissile-compiled/ \
 >     --workers 4
 > ```

#### `configuration`

- `configuration report`

 - `--release <RELEASE_PATH>` path to a BOSH release **(not optional)**

 > This command prints a report that contains all the unique configuration keys found in a BOSH release, together with their usage count (how many jobs reference a particular config) and their default value. The report also tries to detect if jobs define different defaults for the same configuration key.
 > ```bash
 > fissile configuration report \
 >     --release ~/fissile-releases/cf-release-v217
 > ```

- `configuration generate`

 - `--release <RELEASE_PATH>` path to BOSH release(s) - you can specify this parameter multiple times **(not optional)**
 - `--light-opinions <LIGHT_OPINIONS_YAML_PATH>` path to a BOSH YAML deployment manifest generated using the instructions found [here](https://docs.cloudfoundry.org/deploying/common/create_a_manifest.html#generate-manifest) **(not optional)** 
 - `--dark-opinions <DARK_OPINIONS_YAML_PATH>`. Normally, the path should point to an edited version of the `cf-stub` BOSH YAML deployment manifest as documented [here](https://docs.cloudfoundry.org/deploying/openstack/cf-stub.html) **(not optional)**.
 - `--target <TARGET_DIRECTORY>` path to a directory where the command will write the configuration **(not optional)**
 - `--prefix <CONFIGURATION_KEYS_PREFIX>` a prefix to be used for all the BOSH keys; defaults to `hcf`
 - `--provider <GENERATION_PROVIDER>` the provider to use when generating the configuration; defaults to `dirtree` (this is the only provider currently available)

 > This command generates an output that is used to populate `consul` or other configuration stores with BOSH configuration keys. 
 > The `dirtree` provider creates a directory structure where the directories themselves represent keys (e.g. `nats.host` is represented by a directory `<CONFiGURATION_KEYS_PREFIX>/nats/host`) and the value is stored in a file named `value.yml`. The contents of the `value.yml` file contain a value that is serialized to `yaml`.
 > ```bash
 > fissile configuration generate \
 >     --release ~/fissile-releases/cf-release-v217 \
 >     --light-opinions ~/config-opinions/cf-v217/opinions.yml \
 >     --dark-opinions ~/config-opinions/cf-v217/dark-opinions.yml \
 >     --target ~/fissile-config \
 >     --prefix hcf \
 >     --provider dirtree
 > ```

#### `templates`

- `templates report`

 - `--release <RELEASE_PATH>` path to a BOSH release **(not optional)**

 > This command reports on how many BOSH templates exist in a release, and tries to figure out how many `erb` blocks could be converted to GO's templating mechanism.
 > ```bash
 > fissile templates report \
 >     --release ~/fissile-releases/cf-release-v217
 > ```

#### `images`

- `images create-base`

 - `--target <TARGET_DIRECTORY>` a path to a directory where fissile will write the `Dockerfile` and all necessary assets **(not optional)**
 - `--configgin <CONFIGGIN_TARBALL>` a path to a tarball containing the [`configgin`](https://github.com/hpcloud/hcf-configgin) tool **(not optional)**
 - `--base-image <DOCKER_BASE_IMAGE>` the docker image to be used as a starting layer for the role base image;  this parameter has a default value of `ubuntu:14.04`
 - `--no-build` if present, the command will not build the docker image; it will only generate the `Dockerfile` and the necessary assets (this is an optional flag)
 - `--repository <REPOSITORY_PREFIX>` a repository prefix used to name the base image name; this parameter has a default value of `fissile`

 > This command creates a docker image to be used as a base layer for all role images; fissile will write a `Dockerfile` and all dependencies in `<TARGET_DIRECTORY>`. After that, if the `--no-build` is *not* present, it will build an image named `<REPOSITORY_PREFIX>-role-base:<FISSILE_VERSION>`. This is the same as changing directory to `<TARGET_DIRECTORY>` and running `docker build -t <REPOSITORY_PREFIX>-role-base:<FISSILE_VERSION> . `
 > ```bash 
 > fissile images create-base \
 >     --target ~/fissile-role-base/ \
 >     --configgin ~/configgin.tgz \
 >     --base-image ubuntu:14.04 \
 >     --repository fissile
 > ```

- `images create-roles`

 - `--target <TARGET_DIRECTORY>` a path to a directory where fissile will write the `Dockerfiles` and all necessary assets for each of the roles; a directory will be created for each role **(not optional)**
 - `--no-build` if present, the command will not build the docker images; it will only generate the `Dockerfiles` and the necessary assets (this is an optional flag)
 - `--repository <REPOSITORY_PREFIX>`  a repository prefix used to name the images; this parameter has a default value of `fissile`
 - `--release <RELEASE_PATH>` path to a BOSH release - you can specify this parameter multiple times **(not optional)**
 - `--roles-manifest <MANIFEST_PATH>` path to a roles manifest yaml file; this file details which jobs make up each role **(not optional)**
 - `--compiled-packages <COMPILED_PACKAGES_DIR>` path to a directory containing all the compiled packages; this flag is usually set to the target path used in the `fissile compilation start` command **(not optional)**
 - `--default-consul-address <CONSUL_ADDRESS>` a consul address that container images will try to connect to when run, by default; this parameter has a default value of `http://127.0.0.1:8500`
 - `--default-config-store-prefix <CONFIG_STORE_PREFIX>` configuration store prefix that container images will try to use when run, by default; this parameter has a default value of `hcf`
 - `--version <IMAGE_VERSION>` this is used as a version label when creating images and for naming them as well  **(not optional)**

 > This command goes through all role definitions inside of the manifest specified by `<MANIFEST_PATH>` and creates a directory with a `Dockerfile` and all required assets. If the `--no-build` flag is *not* specified, a docker image is built with the name  `<REPOSITORY_PREFIX>-<RELEASE_NAME>-<role_name>:<release_version>-<IMAGE_VERSION>`.
 > ```bash
 > fissile images create-roles \
 >     --target ~/fissile-roles/ \
 >     --repository fissile \
 >     --release ~/fissile-releases/cf-release-v217/ \
 >     --roles-manifest ~/roles-manifest.yml \
 >     --compiled-packages ~/fissile-compiled/ \
 >     --default-consul-address http://127.0.0.1:8500 \
 >     --default-config-store-prefix hcf \
 >     --version 3.14.15
 > ```

- `images list-roles`

 - `--repository <REPOSITORY_PREFIX>`  a repository prefix used to name the images; this parameter has a default value of `fissile`
 - `--release <RELEASE_PATH>` path to BOSH release(s) - you can specify this parameter multiple times **(not optional)**
 - `--roles-manifest <MANIFEST_PATH>` path to a roles manifest yaml file; this file details which jobs make up each role **(not optional)**
 - `--version <IMAGE_VERSION>` this is used as a version label when creating images and for naming them as well  **(not optional)**
 - `--docker-only` if this flag is set, only images that are available on docker will be displayed; this is an optional flag
 - `--with-sizes` if this flag is set, the command also displays the virtual size for each image; if this flag is set, the `--docker-only` flag must be set as well; this is an optional flag

 > This command lists all the final docker image names for all the roles defined in the manifest at `<MANIFEST_PATH>`. If the `--docker-only` flag is *not* set, this command does not connect to docker.
 > ```bash
 > fissile images list-roles \
 >     --repository fissile \
 >     --release ~/fissile-releases/cf-release-v217/ \
 >     --roles-manifest ~/roles-manifest.yml \
 >     --version 3.14.15
 >     --docker-only \
 >     --with-sizes
 > ```

## Configuration base

The following configuration stores are required:

- The descriptions for all keys: `/<prefix>/descriptions/<key-path>/`

- Default sets
 - The default values from the job specs, per job: `/<prefix>/spec/<release-name>/<job-name>/<key-path>/`
 - Opinions retrieved from generated manifest files: `/<prefix>/opinions/<key-path>/`

- User sets
 - Global properties store: `/<prefix>/user/<key-path>`
 - Per-role container store: `/<prefix>/role/<role-name>/<key-path>`

`configgin` should resolve a property in the following order:

1. User per-role
2. User global
3. Default opinions
4. Default specs

It is important to note the following:

- The default spec store is the only store that will contain all values, and that this store is not accessible to the user for editing by default
- The opinions set contains a smaller set of values (~ less than half); this store is not accessible to the user for editing by default
- The global properties store contains ~90 values (mostly credentials, certificates and various limits); these should all be set by terraform by default
- The per-role container store contains the least amount of values (~ 20); these are values that are different for each role - like the set of consul services that get registered with an agent; these cannot be automatically generated in a reliable fashion (the HCF concept of role is different than the BOSH concept of colocated jobs), so they should be set by fissile using a manifest-like config file


> BOSH allows you to group your jobs into roles on-the-fly, allowing the flexibility to have a different set of configs for each new role that you create. This ability is currently used only when deploying on multiple availability zones, and you need some config values to be different for each of them (e.g. the metron agent zone).
> The approach HCF should take for this scenario (multiple AZs) is TBD

LOL
LOL
LOL
LOL
LOL
LOL
LOL
LOL
LOL
