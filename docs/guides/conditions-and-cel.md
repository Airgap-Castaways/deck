# Conditions with when (CEL)

This guide explains how to use the `when:` field to control which steps run on
a given host, and how to write correct CEL expressions using the available
namespaces.

## What `when:` does

Every step in a workflow accepts an optional `when:` field. Its value is a
[CEL](https://cel.dev) expression that deck evaluates before the step runs:

- When `when:` evaluates to `true` (or is absent), the step runs normally.
- When `when:` evaluates to `false`, the step is **skipped**: it is not
  treated as an error, and execution continues with the next step.
- When `when:` fails to evaluate (for example, because a referenced variable
  is the wrong type), deck reports `E_CONDITION_EVAL` and aborts.

Use `when:` to express legitimate optionality, steps that should run only on
certain host types, roles, or after certain conditions are met. Do not use it
to mask prerequisite failures; use `CheckHost` for hard suitability gates.

## Available namespaces

CEL expressions can reference three namespaces: `vars.*`, `runtime.*`, and
`context.*`. Use these without braces, `when:` is not Go template syntax.

### `vars.*`: static input variables

References the fully merged variable map built from `vars.yaml`, `-f`
overlays, the scenario `vars:` block, and `--var` flags (in increasing
precedence order). Node-scoped `hosts:` selection has already run by the time
`when:` is evaluated, so `vars.role` reflects the current node's role.

```yaml
- id: init-control-plane
  kind: InitKubeadm
  spec:
    outputJoinFile: "{{ .vars.join.file }}"
  when: vars.role == "control-plane"

- id: join-worker
  kind: JoinKubeadm
  spec:
    joinFile: "{{ .vars.join.file }}"
  when: vars.role == "worker"
```

```yaml
- id: start-deck-server
  kind: ManageService
  spec:
    name: deck-server
    state: started
    enabled: true
  when: vars.deckServer == true
```

### `runtime.*`: detected facts and registered outputs

`runtime.*` provides two kinds of values:

**Built-in host facts under `runtime.host`**: populated automatically from
the local OS before any step runs. No `CheckHost` step is required to
populate these; they are always available.

| Field | Example value |
|---|---|
| `runtime.host.os.name` | `"linux"` |
| `runtime.host.os.id` | `"ubuntu"` |
| `runtime.host.os.family` | `"debian"` or `"rhel"` |
| `runtime.host.os.version` | `"Ubuntu 24.04.2 LTS"` |
| `runtime.host.os.versionId` | `"24.04"` |
| `runtime.host.os.release` | `"24.04"` (alias of `versionId`) |
| `runtime.host.os.idLike` | `"debian"` |
| `runtime.host.arch` | `"amd64"` or `"arm64"` |
| `runtime.host.kernel.release` | `"6.8.0-60-generic"` |

```yaml
- id: configure-apt-repo
  kind: ConfigureRepository
  spec:
    format: deb
    repositories:
      - id: offline-base
        baseurl: file:///srv/offline-repo
        trusted: true
  when: runtime.host.os.family == "debian"

- id: configure-dnf-repo
  kind: ConfigureRepository
  spec:
    format: rpm
    repositories:
      - id: offline-base
        name: offline-base
        baseurl: file:///srv/offline-repo
        enabled: true
        gpgcheck: false
  when: runtime.host.os.family == "rhel"
```

**Registered step outputs under `runtime.<name>`**: populated when an
earlier step uses `register:` to export an output. The registered value is
available to all later steps in the same phase, or in later phases. If the
producing step is inside a `parallelGroup`, the value becomes visible only
after the entire batch succeeds.

```yaml
steps:
  - id: collect-join-input
    kind: Input
    register:
      joinCipher: value      # runtime.joinCipher is available to later steps
    spec:
      message: "Paste the encrypted join block"
      required: true

  - id: use-join-value
    kind: Command
    spec:
      command: [echo, "got a join block"]
    when: runtime.joinCipher != ""   # gate on the registered value
```

### `context.*`: deck execution metadata

`context.*` carries deck-supplied metadata about the current invocation.
These values are resolved when the command starts and do not change during
execution.

| Field | Available in | Description |
|---|---|---|
| `context.command` | prepare, apply | Current command: `"prepare"` or `"apply"` |
| `context.workflow.source` | prepare, apply | `"filesystem"` or `"server"` |
| `context.workflow.isServer` | prepare, apply | `true` when source is `"server"` |
| `context.workflow.path` | prepare, apply | Resolved workflow file path or URL |
| `context.workflow.scenario` | apply only | Scenario name when apply resolved a scenario |
| `context.paths.bundleRoot` | prepare, apply | Prepared output root (prepare) or bundle root (apply) |
| `context.paths.outputRoot` | prepare only | Prepared output root |
| `context.paths.stateFile` | apply only | Apply state file path |

```yaml
- id: announce-server-mode
  kind: Message
  spec:
    message: "Running from deck server, fetching artifacts remotely"
  when: context.workflow.isServer == true
```

## Common patterns

### OS-family branching

The most common `when:` pattern. Use `runtime.host.os.family` to target
Debian-family or RHEL-family nodes:

```yaml
phases:
  - name: host-prereqs
    imports:
      - path: repo/offline-repo-debian.yaml
        when: runtime.host.os.family == "debian"
      - path: repo/offline-repo-rhel.yaml
        when: runtime.host.os.family == "rhel"
```

### Role-based steps

Use `vars.role` when your `vars.yaml` assigns a role per node via `hosts:`:

```yaml
- id: init-first-control-plane
  kind: InitKubeadm
  spec:
    outputJoinFile: "{{ .vars.join.file }}"
  when: vars.role == "control-plane"

- id: join-cluster
  kind: JoinKubeadm
  spec:
    joinFile: "{{ .vars.join.file }}"
  when: vars.role == "worker"
```

Always provide a safe default in `vars.yaml`'s `all:` section so that
unmatched hosts do not cause `E_CONDITION_EVAL`:

```yaml
all:
  role: ""   # skips both steps above on hosts not in hosts:
```

### Gating on a registered value

A step can gate on a value registered by an earlier step. The registered value
is visible from the next step in the same phase onward (or after the full
parallel batch, if the producer is in a batch):

```yaml
steps:
  - id: ask-passphrase
    kind: Input
    register:
      passphrase: value
    spec:
      message: "Enter the join passphrase (leave blank to skip join)"
      secret: true

  - id: decrypt-join-command
    kind: Command
    spec:
      env:
        DECK_JOIN_PASS: "{{ .runtime.passphrase }}"
      command: [bash, -lc, "openssl enc -d -aes-256-cbc ..."]
    when: runtime.passphrase != ""
```

## Conditional imports

`phases[].imports` entries accept an optional `when:` field. Deck AND-combines
the import's condition with every step in the imported file:

- If the import has `when:` but a step has none, the step inherits the import
  condition.
- If both have `when:`, the effective condition is
  `(import-when) && (step-when)`.
- If the import has no `when:`, steps keep their own conditions unchanged.

```yaml
phases:
  - name: host-prereqs
    imports:
      - path: repo/offline-repo-debian.yaml
        when: runtime.host.os.family == "debian"
        # Every step in offline-repo-debian.yaml is now gated on this condition.
        # If a step inside also has its own when:, both conditions must be true.

      - path: host-prereqs.yaml
        # No import-level when:, steps inside keep their own conditions.
```

This means you can apply a broad guard (OS family, role) at the import level
and use narrower guards at the step level without duplicating the broad guard
in every step.

## Previewing conditions with `deck plan`

`deck plan` shows the phases and steps that would execute, including which
steps have `when:` conditions. Run it before `apply` to catch typos and
wrong-namespace references:

```bash
deck plan
```

For variables used in conditions, also run:

```bash
deck plan vars
```

This shows the effective `vars` and initial `runtime` values, confirm that
`vars.role` resolved to the expected value and that `runtime.host.os.family`
matches the target host's OS.

When a `when:` expression fails at runtime, deck reports `E_CONDITION_EVAL`
and stops the run. See the error codes reference for the full list of
condition-related codes.

## Limitations and gotchas

CEL is a typed expression language, not a general scripting language.
It does not support shell expansions, arithmetic on strings, or function calls
that are not built into CEL. Keep conditions simple: equality checks,
comparisons, boolean and/or, and string containment (`has()`).

Type rules are strict. `vars.deckServer == true` works when `deckServer`
is a boolean in `vars.yaml`. If it is the string `"true"`, the comparison
fails silently and the step is skipped. Check `deck plan vars` to confirm the
Go type that deck resolved.

Undefined variable handling. If `vars.role` is not set at all, because
`all:` has no default and the host is not in `hosts:`, the CEL expression
raises `E_CONDITION_EVAL`. Always provide `all:` defaults for every field you
branch on.

`runtime.*` values from `register` are not available before the producing
step runs.** If step B gates on `runtime.joinCipher` and step A registers
`joinCipher`, but A is skipped or has not run yet, the value is undefined and
causes `E_CONDITION_EVAL`. Use phases or serial ordering to guarantee the
producer runs first.

`when:` in component fragments is evaluated in the context of the
importing scenario. Fragments can reference `vars.*` and `runtime.*` freely,
but they do not have their own variable scope.

## Related references

- [Workflow Model, `when`](../workflow-model.md#when--conditional-execution): canonical `when` and conditional imports reference
- [Workflow Model, `register`](../workflow-model.md#register--capture-step-output): how to export step outputs to `runtime.*`
- [Workflow Model, Built-In Runtime Fields](../workflow-model.md#built-in-runtime-fields): full `runtime.host` field table
- [Variables and templating](variables-and-templating.md): how `vars.*` is built and how precedence works
- [Authoring workflows](authoring-workflows.md): putting `when:` into a full scenario
- [Troubleshooting](../troubleshooting.md): diagnosing `E_CONDITION_EVAL` and related errors
