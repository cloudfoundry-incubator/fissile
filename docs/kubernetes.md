# Using Kubernetes with Fissile

Fissile has experimental support for generating [Kubernetes] resource
definitions.  It will create one workload (deployment, stateful set, or job) per
defined role, and any associated services (and volume claims) required.

[Kubernetes]: https://kubernetes.io/

## Prerequisites
Currently, the resource definitions generated assumes you have a [StorageClass]
named `persistent`.  This is used to support persistent volume claims used for
ephemeral local storage.  A second [StorageClass] named `shared` is used for
shared volumes; it must support `ReadWriteMany`.

[StorageClass]: https://kubernetes.io/docs/resources-reference/v1.6/#storageclass-v1-storage

## Generating Kubernetes Definitions
Kubernetes resource definitions may be created via the subcommand
[`fissile build kube`].  Please refer to the generated documentation for
available arguments.

[`fissile build kube`]: ./generated/fissile_build_kube.md

## Workload Types
There are three workload types that fissile will emit:

### Job
Fissile emits a [Job] for BOSH tasks (such as running smoke tests).  They are
generated in the `bosh-tasks/` subdirectory, as it's likely that you do not want
to run them to deploy the cluster.  For example, the
[Cloud Foundry Acceptance Tests] are destructive and is not suitable to run on a
cluster that is needed for other purposes.

[Job]: https://kubernetes.io/docs/resources-reference/v1.6/#job-v1-batch
[Cloud Foundry Acceptance Tests]: https://github.com/cloudfoundry/cf-acceptance-tests

### StatefulSet
Fissile emits a [StatefulSet] under two circumstances.  Any self-clustering
roles (i.e. any role with the `clustered` tag) will be a StatefulSet, in order
for each pod to be addressable (so that they can talk to each other).  For
example, a NATS role would fall under this category.  Secondly, any roles which
require local storage will be a StatefulSet to take advantage of volume claim
templates.

[StatefulSet]: https://kubernetes.io/docs/resources-reference/v1.6/#statefulset-v1beta1-apps

### Deployment
All roles without the above constraints will be generated as deployments.

## Services

Each role may have attached services generated as necessary.  There are three
general conditions:

- Each StatefulSet will have a headless service (e.g. `nats-set`); this is used
  to manage the StatefulSet (a Kubernetes requirement), and to allow discovery
  of pods within a role via DNS.
- A role may have a service for its public ports, if any port is public.
- A role may have a service for its private ports, if any ports are defined.
  Public ports will also be listed to ease communication across roles (not
  having to use different names depending on whether a port is public).
