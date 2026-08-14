# Capturing step output with register

Some steps produce values that later steps need, a kubeadm join file path, an
operator-supplied IP address, an encrypted join block. `register` is the
mechanism for forwarding those values without shell variable hacks or
hard-coded paths.

---

## The problem it solves

In a raw shell script, you might capture a command's output into a variable
and reference it later. In a deck workflow, steps are isolated typed
operations, there is no shared shell state. `register` gives you a typed,
named bridge from one step's output to a later step's input.

Without `register`:

```yaml
# BAD: join file path hard-coded in two places, fragile, easy to break.
- id: init-cluster
  kind: InitKubeadm
  spec:
    outputJoinFile: /tmp/deck/join.txt

- id: join-worker
  kind: JoinKubeadm
  spec:
    joinFile: /tmp/deck/join.txt   # duplicated literal
```

With `register`:

```yaml
- id: init-cluster
  kind: InitKubeadm
  register:
    joinFile: joinFile             # export the step's "joinFile" output as runtime.joinFile
  spec:
    outputJoinFile: "{{ .vars.join.file }}"

- id: join-worker
  kind: JoinKubeadm
  spec:
    joinFile: "{{ .runtime.joinFile }}"   # consumed via template
```

---

## Syntax

```yaml
register:
  <runtime-name>: <output-key>
```

- `<runtime-name>` is the name you choose. It becomes available as
  `runtime.<runtime-name>` in CEL expressions (`when:`) and
  `.runtime.<runtime-name>` in Go template fields (`spec:` values).
- `<output-key>` is the output name the step kind declares. You can only
  register output names that the step kind explicitly declares. Attempting to
  register an undeclared key fails validation at `deck lint` time.

### Where to find declared outputs

Each step kind's reference page lists its `outputs` in the summary header. For
example:

| Step kind | Declared outputs |
|-----------|-----------------|
| `InitKubeadm` | `joinFile` |
| `Input` | `value` |

Steps with no declared outputs reject non-empty `register` mappings during
validation.

---

## Consuming registered values

### In step `spec` fields: Go templates

Use `.runtime.<name>` inside double-brace template expressions:

```yaml
- id: announce-encrypted-join
  kind: Command
  spec:
    env:
      DECK_JOIN_PASS: "{{ .runtime.joinPass }}"
    command:
      - bash
      - -lc
      - |
        join_cmd="$(kubeadm token create --print-join-command)"
        printf '%s\n' "${join_cmd}" \
          | openssl enc -aes-256-cbc -pbkdf2 -salt -base64 -A -pass env:DECK_JOIN_PASS
```

### In `when` conditions: CEL expressions

Use `runtime.<name>` (no dot prefix, no braces) in CEL expressions:

```yaml
- id: skip-if-no-passphrase
  kind: Message
  when: runtime.joinPass != ""
  spec:
    level: info
    message: "Passphrase received; proceeding with encryption."
```

Both forms refer to the same runtime namespace; the syntax differs only
because `spec` fields use Go templates and `when` uses CEL.

---

## Secret values

When `Input` is used with `secret: true`, the captured value is:

- Never written to the apply state file (not persisted across runs).
- Not echoed on the terminal during input.
- Available in memory to later steps during the same run.

This makes `Input` + `register` + `secret: true` the right pattern for
passphrases, tokens, and anything else that should not reach disk.

```yaml
- id: join-passphrase
  kind: Input
  register:
    joinPass: value          # "value" is the sole output of Input
  spec:
    message: "Passphrase used to encrypt the join command"
    secret: true
    required: true
```

If a run is interrupted after a secret `Input` step, the value is gone, it
was never persisted. On the next run the step re-prompts the operator.

### The encrypted-join pattern

The offline-kubernetes example uses this pattern across two scenarios:

1. **Bootstrap** (`components/bootstrap/announce-join.yaml`): the operator
   chooses a passphrase (`Input`, `secret: true`, registered as `joinPass`).
   The kubeadm join command is encrypted with the passphrase and printed to
   the log only as ciphertext. The plaintext never appears in logs or state.

2. **Join** (`components/join/input-join.yaml`): on the worker node, the
   operator pastes the ciphertext (`Input`, registered as `joinCipher`) and
   re-enters the passphrase (`Input`, `secret: true`, registered as
   `joinPass`). A `Command` step decrypts the join command locally. Both
   registered values are passed as environment variables, never inlined in
   command strings or written to state.

For the full example, see
[examples/README.md](../examples/README.md#encrypted-join-pattern).

---

## Parallel batch restriction

If a step that produces a `register` value runs inside a `parallelGroup`
batch, its output is visible **only after the entire batch completes**. Steps
in the same batch start from the same runtime snapshot and cannot see each
other's `register` outputs.

```yaml
# This is invalid, both steps are in the same batch.
steps:
  - id: get-passphrase
    kind: Input
    parallelGroup: setup
    register:
      pass: value
    spec:
      message: "Enter passphrase"
      secret: true

  - id: use-passphrase
    kind: Command
    parallelGroup: setup        # WRONG: cannot consume pass from the same batch
    spec:
      env:
        PASS: "{{ .runtime.pass }}"
      command: [echo, "encrypting..."]
```

Fix: remove the `parallelGroup` from both steps, or put the producer in an
earlier batch or earlier phase.

See [Phases and parallelism](phases-and-parallelism.md) for the full parallel
batch rules.

---

## Common mistakes

### Registering an undeclared output key

```yaml
# BAD: Command does not declare an output named "stdout".
- id: get-ip
  kind: Command
  register:
    nodeIP: stdout     # Command has no declared outputs, this fails validation
  spec:
    command: [hostname, -I]
```

`deck lint` catches this before execution. Check the step kind's reference
page for its declared outputs.

### Consuming a register in the same parallel batch

As shown above, a step cannot read a `register` value that was produced in the
same batch. Place the consumer in a later step (different batch) or a later
phase.

### Expecting a secret value to survive a resume

Secret `Input` values are never persisted. If a run is interrupted after the
`Input` step completes but before a later step that consumes the value, the
value is gone on resume. Deck will re-prompt for the secret when the phase
reruns.

---

## Worked example: Input → Command pipeline

A simple workflow that asks the operator for a registry address and uses it in
a later step:

```yaml
version: v1alpha1
steps:
  # Step 1: ask the operator for the registry host.
  - id: get-registry-host
    kind: Input
    register:
      registryHost: value      # "value" is Input's only declared output
    spec:
      message: "Registry host (e.g. 192.0.2.10:5000)"
      required: true

  # Step 2: write a containerd mirror configuration using the registered value.
  - id: configure-mirror
    kind: WriteFile
    spec:
      path: /etc/containerd/certs.d/registry.k8s.io/hosts.toml
      content: |
        server = "https://registry.k8s.io"
        [host."http://{{ .runtime.registryHost }}"]
          capabilities = ["pull", "resolve"]
          skip_verify = true
```

---

## Related references

- [Workflow Model, register](../workflow-model.md#register--capture-step-output)
- [Workflow Model, Step Envelope Contract](../workflow-model.md#step-envelope-contract)
- [Phases and parallelism](phases-and-parallelism.md)
- [Input step kind](../step-kinds/input.md)
- [InitKubeadm step kind](../step-kinds/init-kubeadm.md)
- [Offline Kubernetes example](../examples/README.md)
