

![fissile-logo](./docs/fissile-logo.png)

Fissile converts existing BOSH dev releases into docker images.

It does this using just the releases, without a BOSH deployment, CPIs, or a BOSH
agent.

## Build and Install
Building fissile needs Go 1.4 or higher. You can download it from the [Golang website](https://golang.org/doc/install)
   
### Build procedure
Execute the following commands to compile fissile
   
```
$ cd $GOPATH
$ mkdir -p src/github.com/hpcloud
$ cd src/github.com/hpcloud
$ git clone https://github.com/hpcloud/fissile.git
$ cd fissile
$ git submodule sync --recursive
$ git submodule update --init  --recursive
$ make all
```

Depending on your architecture you can use the fissile binary files from those directories:
`fissile/build/darwin-amd64` or `fissile/build/linux-amd64`.

## Install procedure

The fissile is also a go-gettable package:

```
go get github.com/hpcloud/fissile
```

## Usage

You can find detailed usage documentation [here](./docs/fissile.md).

## Kubernetes

### TODO

- [ ] Implement extension of the role manifest in the fissile model
- [ ] Implement the conversion of role manifest information to kube objects in fissile
- [ ] Alter the role manifest so we no longer use HCP constructs (for service discovery)
- [ ] Deploy & test

### Generated objects

> Note: we now have _almost_ all the information we need in the role manifest to
> be able to generate the needed kube objects.
> However, if one can assume that at some point kube may be the only deployment
> mechanism for HCF, it's interesting to consider that we could maintain all the
> kube configurations for HCF manually.
> The role manifest would then be reduced to a minimal size - only the information
> required to transform the BOSH releases to docker images.

Each object can have it's own file. We should be able to concatenate them if we
need to.
We have 3 things we need in Cloud Foundry:
- stateful sets (mysql, consul, etcd)
- jobs (post-deployment-setup, sso-create-service, autoscaler-create-service)
- deployments (all other roles that are not stateful sets and are not jobs)

#### namespace

As far as I can tell, this is not _really_ required.


#### persistent volumes

We need to generate persistent volumes for the stateful sets. The deployments
should not require any persistence.


#### deployment (with pods)

Nothing special here. Most of the things we have should be deployments.


#### stateful sets (with pods)

We should be able to identify these based on the fact that they need persistent
storage.


#### jobs (with pods)

Everything that's a bosh-task should be a job. These should never be associated
with a service.


#### services

We need to create services for all exposed ports in the role manifest.
It would seem that for _our_ stateful sets we could make do with headless services
because none of them need load balancing, or an allocated clusterIP.


### Configuration

At this point the environment variables needed by each pod need to be exposed
for each pod. The user needs to configure and match all of them.
Not sure how we'll be able to make it nice for the user *yet*.

> Notes
>
> - Why aren't all the defaults for env vars in the role manifest strings?
> - The UAA auth settings should move away from the role manifest.
> - The kube client-go library is about to change structure (and move from PetSet to StatefulSet)
>   Would it make sense to start directly with that?
