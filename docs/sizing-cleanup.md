# Issue

Parsing the `sizing` section of `values.yaml` is difficult because it
contains a mix of per-role descriptions and other things: HA, memory,
cpu.

The extra tuneables should be moved somewhere else.

Example of an old variable setting: `--set sizing.HA=true`

# Implemented change

Moved the non-role-specific settings to the new key `config`.

For example `sizing.HA` becomes `config.HA`.

The keys affected by this change are:

   * `sizing.HA`
   * `sizing.cpu.limits`
   * `sizing.cpu.requests`
   * `sizing.memory.limits`
   * `sizing.memory.requests`

The only keys left under `sizing` are the per-role descriptions.

Further, to prevent users from accidentally using the old names in
their overide yaml files, all templates are extended to contain
guarding statements of the form

```
    {{- if .Values.FOO }}
    _moved(FOO): {{ fail "Bad use of moved variable FOO. The new name to use is [FOO]" }}
    {{- end }}
```

where `FOO` is one of the keys above, `(FOO)` that key changed to not
be nested (`.` --> `_`), and `[FOO]` the new key for `FOO`.

Example: `sizing.cpu.limits` to `sizing_cpu_limits`

Fuller example:

```
    {{- if .Values.sizing.cpu.limits }}
    _moved_sizing_cpu_limits: {{ fail "Bad use of moved variable sizing.cpu.limits. The new name to use is config.cpu.limits" }}
    {{- end }}
```
