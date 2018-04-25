
![fissile-logo](./docs/fissile-logo.png)

Fissile converts existing [BOSH] final or dev releases into docker images.

It does this using just the releases, without a BOSH deployment, CPIs, or a BOSH
agent.

[BOSH]: http://bosh.io/docs

## Getting fissile

### Prerequisites
Building fissile needs [Go 1.7] or higher and [Docker].

[Go 1.7]: https://golang.org/doc/install
[Docker]: https://www.docker.com

### Build procedure
Fissile requires generated code using additional tools, and therefore isn't
`go get`-able.

```
$ go get -d github.com/SUSE/fissile       # Download sources
$ cd $GOPATH/src/github.com/SUSE/fissile
$ make tools                              # install required tools; only needed first time
$ make docker-deps                        # pull docker images required to build
$ make all
```

Depending on your architecture you can use the fissile binary files from those directories:
`fissile/build/darwin-amd64` or `fissile/build/linux-amd64`.

## Using Fissile
Please refer to the following additional documentation:

* [Walkthrough] on configuring and using fissile to build a docker image and
corresponding Kubernetes resource definition
* Additional [Kubernetes] usage instructions and resource definition details
* Information on [stemcells] and how to build them
* Auto-generated [usage reference]

[walkthrough]: ./docs/configuration.md
[Kubernetes]: ./docs/kubernetes.md
[stemcells]: ./docs/stemcells.md
[usage reference]: ./docs/generated/fissile.md

## Developing Fissile
In general, use of the default `make` target is preferred before
making a [pull request].  This will run the unit tests, as well as
various linters.  To manually build fissile only, run
`make bindata build`.  This will run the necessary code generation
before building the binary.

[pull request]: https://github.com/SUSE/fissile/pulls

### Testing
Run tests with `make test` (or use `go test` directly if you want to filter for
specific tests, etc.)  There are environment variables that can be set to
adjust how tests are run:

Name | Value
--- | ---
`FISSILE_TEST_DOCKER_IMAGE` | the name of the default docker image for testing(e.g. `splatform/fissile-opensuse-stemcell:42.2`)

### Vendoring
Fissile uses [Godep] for vendoring required source code.  To update the vendored
source tree, please run `godep save ./...` and double-check that it has not done
anything silly.

[Godep]: https://github.com/tools/godep#godep
