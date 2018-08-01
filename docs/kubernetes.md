# Using Kubernetes with Fissile

Fissile has experimental support for generating [Kubernetes] resource
definitions.  It will create one workload (deployment, stateful set, or job) per
defined instance group, and any associated services (and volume claims) required.

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

Fissile emits a [StatefulSet] under two circumstances.

Any self-clustering instance groups (i.e. any group with the `clustered` tag)
will be a StatefulSet, in order for each pod to be addressable (so
that they can talk to each other). For example, a `doppler` group would
fall under this category.

Secondly, any instance groups tagged as `indexed`. An example of such would be
the CF instance group `nats`. These are groups which require load balancing and
need a 0-based, incremented index. To support this fissile creates a
public service (of type `LoadBalancer`) for indexed groups, providing a
single point of access to the pods for the group.

Note that both `clustered` and `indexed` instance groups can take advantage of
volume claim templates for local storage.

__Attention__: The automatic emission of StatefulSet for instance groups which
have volume specifications has been removed. All instance groups now have to be
explicitly tagged as described above.

[StatefulSet]: https://kubernetes.io/docs/resources-reference/v1.6/#statefulset-v1beta1-apps

### Deployment
All instance groups without the above constraints will be generated as deployments.

## Services

Each instance group may have attached services generated as necessary.  There are three
general conditions:

- Each StatefulSet will have a headless service (e.g. `nats-set`); this is used
  to manage the StatefulSet (a Kubernetes requirement), and to allow discovery
  of pods within a instance group via DNS.
- A instance group may have a service for its public ports, if any port is public.
- A instance group may have a service for its private ports, if any ports are defined.
  Public ports will also be listed to ease communication across instance groups (not
  having to use different names depending on whether a port is public).
