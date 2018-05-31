# The `sizing` hierarchy of `values.yaml`.

The non-role-specific settings are under the (new) key `config`.

For example the old `sizing.HA` becomes `config.HA`.

The affected keys are:

   * `sizing.HA`
   * `sizing.cpu.limits`
   * `sizing.cpu.requests`
   * `sizing.memory.limits`
   * `sizing.memory.requests`

The `sizing` hierarchy contains only the per-role descriptions.

To prevent users from accidentally using the old names in their
overide yaml files, all templates contain guarding statements of the
form

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
