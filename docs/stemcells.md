# Docker Stemcells

Fissile uses docker images based on [BOSH stemcells](http://bosh.cloudfoundry.org/docs/stemcell.html) when compiling packages
and as a base for all the role images that it creates.

Prior to version `5.0.0`, fissile would build its own compilation and stemcell
layers, using hardcoded scripts, that only worked for Ubuntu Trusty.

Starting with version `5.0.0`, fissile no longer builds these images. The user
is responsible with providing an image that contains the same packages and
dependencies as a regular [BOSH stemcell](https://github.com/cloudfoundry/bosh-linux-stemcell-builder/tree/master/stemcell_builder/stages), plus:
- prerequisites: `libopenssl-devel`, `gettext-tools`
- [`configgin`](https://rubygems.org/gems/configgin) - a ruby gem that processes
  BOSH erb templates (this means ruby and bundler need to be there as well)
- [`dumb-init`](https://github.com/Yelp/dumb-init) - a tool used as PID 1 for managing processes in a container
  > Installed to `/usr/bin/dumb-init`

## Implementations

- [OpenSUSE](https://github.com/SUSE/fissile-stemcell-openSUSE/blob/42.2/Dockerfile)
- [Ubuntu](https://github.com/cloudfoundry-community/fissile-stemcell-ubuntu)

Both of these are built during the same process that builds the actual BOSH
stemcells; you can find the pipeline for these [here](https://ci.from-the.cloud/teams/main/pipelines/bosh-os-images).
The CPI specific dependencies are not required for Docker Stemcells, so we use
the BOSH stemcells before they are differentiated for each supported IaaS.
