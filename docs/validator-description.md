# Checks performed by the original role manifest validator

Most of these checks are independent of the SCF project. Some are not,
these are noted when the checks are described in detail.

## Inputs to the validator

 * Role manifest
 * Light opinions
 * Dark opinions
 * Output of `fissile show properties --output yaml` (`FSP` from now on)
 * The set of BOSH releases the role manifest is for.
 * `.env` files for the local defaults (vagrant box)

Notes:

 * The `FSP` structure is a nested map of the form

   * release -> (job -> (property -> default))

   See `fissile.go`'s method `collectProperties` where it is created.

 * Manifest and opinions are YAML files.

 * Fissile might already have code for reading `.env` files. Not sure
   however.

## Checks

First an overview. The details of each check follow in subsections.

 1. All dark opinions must be configured as templates
 1. No dark opinions must have defaults in light opinions
 1. No duplicates must exist between role manifest and light opinions
 1. All role manifest properties must be defined in a BOSH release
 1. All light opinions must exist in a BOSH release
 1. All dark opinions must exist in a BOSH release
 1. All bosh properties in a release should have the same default across jobs
 1. All light opinions should differ from their defaults in the BOSH releases
 1. All vars in env files must exist in the role manifest
 1. All role manifest parameters must be sorted
 1. All role manifest parameters must be used
 1. All role manifest templates must use only declared parameters
 1. All role manifest templates must be sorted
 1. The role manifest must not contain any constants in the global section
 1. All of the scripts must be used
 1. Check clustering
 1. The run.env references of docker roles must use only declared parameters
 1. No non-docker role may declare 'run.env'

### All dark opinions must be configured as templates

All elements P found in the dark opinions must be among the templates
defined in the role manifest, i.e. have a definition for
`properties.P`.

### No dark opinions must have defaults in light opinions

For all properties P found in dark opinions, light opinions __must not__
contain a definition for `P`.

### No duplicates must exist between role manifest and light opinions

Templates in the role manifest should not have anything in the light opinions.
If the values are identical it should just be in light opinions.
If they are different, then the definition in light opinions is superflous.

### All role manifest properties must be defined in a BOSH release

For all templates `properties.P` in role manifest we must have a definition of
`P` in at least one BOSH release.

This uses the `FSP` as the origin of data about BOSH releases and the
properties they contain.

Just noticed that this recursively searches the FSP as is. It would
likely be faster if it would use the global defaults derived from the
`FSP` instead, because that structure has the relevant information
(property names) as the main key.

See the upcoming section __All properties in a BOSH release should
have the same default across jobs__ for the definition of global
defaults.

This is likely historic cruft, i.e. these checks got implemented
first, and when the global defaults-based checks were added later it
was forgotten that they could use the new structure.

__Note__: The SCF validator excludes a number of properties from this
check. __Discussion with Vlad__ is needed to determine which
exclusions could/should (not) be made generic, and if yes, how.

Details:

 * Properties with structured values (which could be mistaken for
   longer-named properties) are excluded:

   * properties.cc.security_group_definitions
   * properties.ccdb.roles
   * properties.uaadb.roles
   * properties.uaa.clients
   * properties.cc.quota_definitions

 * Properties/templates without prefix `properties.` in their name are
   excluded.

### All light opinions must exist in a BOSH release

This is the same as the previous, applied to the properties in the light opinions.

### All dark opinions must exist in a BOSH release

This is the same as the previous, applied to the properties in the light opinions.

### All properties in a BOSH release should have the same default across jobs

Note, these are actually only warnings, not errors. IOW having these
will not cause the validator to prevent further processing.

Going over the `FSP` the validator collects all defaults values for a
property in a release, across the jobs. IOW the FSP structure

 * release -> (job -> (property -> default))

is inverted and rekeyed to the nested map

 * property -> (default -> list(pair(release, job)...))

to make the check simple. This new structure is called `global
defaults`.

Warnings are issued when a property has multiple defaults in its map.
When printing the lists of jobs for each default, care is taken to
handle lists both containing only one job vs multiple jobs nicely.

### All light opinions should differ from their defaults in the BOSH releases

This check uses the global defaults as source.

Check to see if all light opinions differ from their defaults in the
BOSH releases.

Note, if a property has more than one default then the opinion
automatically differs from at least one, i.e no error is to be
generated for this. A warning is issued however. If a property has no
default at all then its light opinions is also different from that,
i.e. no error.

The note in section __All role manifest properties must be defined in a BOSH
release__, about excluded properties applies here as well too.

### All vars in env files must exist in the role manifest

All variables found in the `.env` files given to the validator must
exists under the key `configuration.variables` in the role manifest.

### All role manifest parameters must be sorted

Check that all parameters declared under `configuration.variables` are
listed in lexicographical order.

### All role manifest parameters must be used

The elements under key `configuration.variables` in the role manifest are also
called __parameters__.

Go over the properties, process the mustache templates, extract the
parameters used in them. Calls this the set `TP`.

Check that all parameters declared under `configuration.variables`
(the set `CV`) are used by at least one template. I.e. `CV - TP`
should be empty.

### All role manifest templates must use only declared parameters

This check is complementary to __All role manifest parameters must be used__.

Go over the parameters found in the mustache templates and check they
they exist under `configuration.variables`. Using the definitions for
`TP` and `CV` from the previous section, the set `TP - CV` should be
empty.

__Note__ however that a number of special parameters are ignored if
they appear in `TP - CV`. __Discussion with Vlad__ is needed on how to
handle these for generic operation of `fissile`. These are:

 * A number of variables were not defined in the role manifest and
   were injected through scripts instead. The list below is reasonably
   complete as of the date the fissile validator was written. By now
   it likely is very much out of date.

   * `*_CLUSTER_IPS`
   * `CATS_SUITES`
   * `DNS_RECORD_NAME`
   * `DONT_SKIP_CERT_VERIFY_INTERNAL`
   * `HTTPS_PROXY`
   * `HTTP_PROXY`
   * `IP_ADDRESS`
   * `JWT_SIGNING_PEM`
   * `JWT_SIGNING_PUB`
   * `KUBE_COMPONENT_INDEX`
   * `KUBE_SERVICE_DOMAIN_SUFFIX`
   * `NO_PROXY`
   * `http_proxy`
   * `https_proxy`
   * `no_proxy`

 * UAA-related variables, injected via scripts

   * `JWT_SIGNING_PUB`
   * `JWT_SIGNING_PEM`
   * `UAA_CLIENTS`
   * `UAA_USER_AUTHORITIES`

 * Variables related to role-indexing

   * SCF_BOOTSTRAP
   * SCF_ROLE_INDEX

### All role manifest templates must be sorted

Check that all properties declared under either
`configuration.templates` or inside of roles are listed in
lexicographical order.

### The role manifest must not contain any constants in the global section

Look at the templates under key `configuration.templates` and reject
any whose mustache definition does not contain any variables.

### All of the scripts must be used

This check is currently specific to SCF because it assumes a specific
location where to search for the script files used by the keys
`scripts`, `environment_scripts` and `post_config_scripts`, and for
the location of the manifest. If that can be supplied through fissile
options it should be generic.

Looking over the script search path any script found must be used by
at least one role, in any of the script keys mentioned in the previous
paragraph.

### Check clustering

This is a check specific to SCF.

Checks that all roles requiring any of the clustering parameters use
the script "scripts/configure-HA-hosts.sh" and that all roles which
don't will not.

It should not be implemented in generic `fissile`.

### The run.env references of docker roles must use only declared parameters

Check for roles of type `docker` with a key `run.env` whose elements
reference unknown parameters. We ignore the parameters provided by
the control plane, and the proxy parts, these are ok.

The `we ignore ...` actually refers to all the parameters mentioned in
the note in section __All role manifest templates must use only declared
parameters___

### No non-docker role may declare 'run.env'

Makes sure that no roles of type != `docker` have a key `run.env`.
