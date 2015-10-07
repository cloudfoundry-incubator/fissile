
![fissile-logo](https://region-b.geo-1.objects.hpcloudsvc.com/v1/10990308817909/pelerinul/fissile-logo.png)

Fissile converts an existing BOSH release (packaged, like the ones you download from bosh.io) into docker images.
It’s supposed to do this using just the release itself, without a BOSH deployment, CPIs or a BOSH agent.


At a high level, fissile helps you go through the following process:

![fissile-highlevel](https://docs.google.com/drawings/d/1cu3H9UHKH6qD4JNQtMoCpMlBDdiIj0KYWQ4IT5yJS3Q/export/png)

## Configuration base

The following configuration stores are required:

- The descriptions for all keys: `/<prefix>/descriptions/<key-path>/`

- Default sets
 - The default values from the job specs, per job: `/<prefix>/spec/<job-name>/<key-path>/`
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
