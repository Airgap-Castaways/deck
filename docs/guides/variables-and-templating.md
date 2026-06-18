# Variables and templating

This guide explains how deck resolves variables, how to organise them across
files and CLI flags, and how to use them safely inside workflow steps.

## The four variable sources and precedence

Static `vars` come from four sources. **Later sources win over earlier ones.**
The order from lowest to highest precedence is:

1. `workflows/vars.yaml` — shared workspace defaults
2. CLI `-f, --vars-file` overlays — site or node files passed at run time
3. Scenario `vars:` block — overrides declared inside the workflow file
4. CLI `--var key=value` — per-invocation overrides

**Why this order matters:**

A value set with `--var` always beats a value in `vars.yaml` or the scenario
file. This means:

- Put stable cluster-wide defaults in `vars.yaml`.
- Use `-f` to layer site- or environment-specific files on top of those
  defaults without editing `vars.yaml`.
- Use the scenario `vars:` block to tune values for one specific scenario
  without touching shared state.
- Use `--var` for one-time overrides during debugging or staged rollouts.

When `workflows/vars.yaml` contains a `hosts:` section, deck applies an
additional selection step after the four sources above. See
[Node-scoped vars](#node-scoped-vars) below for the full ordering.

## `workflows/vars.yaml` — shared workspace defaults

Define values that every scenario in the workspace shares:

```yaml
# workflows/vars.yaml

kubernetesVersion: v1.36.1
arch: amd64

cluster:
  podCIDR: 10.244.0.0/16
  serviceCIDR: 10.96.0.0/12
  controlPlaneEndpoint: 192.0.2.10:6443

registry:
  host: 192.0.2.10:5000
  scheme: http
```

All scenarios and component fragments can reference these values with
`{{ .vars.kubernetesVersion }}`, `{{ .vars.cluster.podCIDR }}`, and so on.

## Scenario-level `vars:` block

Override or extend shared defaults for one specific scenario:

```yaml
# workflows/scenarios/staging-bootstrap.yaml

version: v1alpha1
vars:
  kubernetesVersion: v1.36.1  # pin to a specific patch for staging
  cluster:
    controlPlaneEndpoint: 192.0.2.20:6443   # staging endpoint

phases:
  - name: bootstrap
    imports:
      - path: bootstrap/kubeadm.yaml
```

The scenario `vars:` block is deep-merged on top of `vars.yaml` values, so
you only need to list the keys you want to override. All other `vars.yaml`
keys remain available.

## CLI vars files — `-f, --vars-file`

Layer site- or environment-specific values at run time without editing
`vars.yaml`:

```bash
# Apply site overrides first, then node-specific overrides on top
deck apply --root . --scenario bootstrap -f vars/site.yaml -f vars/cp1.yaml
```

File paths are relative to the `workflows/` directory:
- `-f vars/site.yaml` resolves to `workflows/vars/site.yaml`.
- Later files in the list override earlier files (same deep-merge behaviour as
  `vars.yaml`).

```yaml
# workflows/vars/site.yaml
registry:
  host: 10.0.1.5:5000
```

```yaml
# workflows/vars/cp1.yaml
cluster:
  controlPlaneEndpoint: 10.0.1.5:6443
```

## CLI `--var` — single-key overrides

Override one key without creating a file:

```bash
deck apply --root . --scenario bootstrap --var kubernetesVersion=v1.36.1
```

`--var` takes the highest precedence and is intended for one-off debugging or
staged rollouts, not for permanent configuration.

## Node-scoped vars

When `vars.yaml` contains a `hosts:` section, deck selects the entry that
matches the local hostname at execution time and deep-merges it into the
effective vars. This lets a single `vars.yaml` carry per-node config for every
node in the cluster.

### Full precedence with `hosts:`

When `hosts:` is present, the effective precedence (lowest to highest) is:

1. Ordinary top-level `vars.yaml` values (keys outside `all:` and `hosts:`)
2. `vars.yaml` `all:` defaults
3. The matching `vars.yaml` `hosts.<hostname>` entry
4. Scenario `vars:` block
5. CLI `--var` overrides

(`-f` overlays are merged into the shared vars document before step 1 above,
so they influence which values reach steps 1–3.)

### Example

```yaml
# workflows/vars.yaml

all:
  kubernetesVersion: v1.36.1
  arch: amd64
  role: ""               # safe default; prevent CEL errors on unmatched hosts

hosts:
  cp-1:
    ip: 192.0.2.10
    role: control-plane
    deckServer: true

  worker-1:
    ip: 192.0.2.21
    role: worker
    deckServer: false
```

When `deck apply` runs on a machine whose hostname is `cp-1`:

1. The top-level `vars.yaml` values are loaded.
2. `all:` defaults are applied (`kubernetesVersion`, `arch`, `role: ""`).
3. The `cp-1` host entry is merged in, so `role` becomes `"control-plane"` and
   `ip` becomes `192.0.2.10`.
4. The scenario `vars:` block (if any) is merged on top.
5. Any `--var` flags are applied last.

If the local hostname does not appear in `hosts:`, execution continues using
only the `all:` values. Provide safe defaults in `all:` for any field your
workflow branches on — for example `role: ""` — to prevent CEL evaluation
errors on unmatched hosts.

Hostname matching tries the full detected hostname first, then the short
hostname (the part before the first `.`).

## Using variables in steps: `{{ .vars.NAME }}`

Inside step `spec` string fields, use Go template syntax with double braces:

```yaml
- id: write-cluster-config
  kind: WriteFile
  spec:
    path: /etc/kubernetes/cluster.conf
    content: |
      clusterName: {{ .vars.cluster.controlPlaneEndpoint }}
      kubernetesVersion: {{ .vars.kubernetesVersion }}
      podCIDR: {{ .vars.cluster.podCIDR }}
```

Nested YAML keys use dot notation: `{{ .vars.cluster.podCIDR }}` reads the
`podCIDR` field inside the `cluster` map.

Template interpolation is available in `spec` string fields. To reference
registered runtime outputs, use `.runtime.NAME` (see
[register](../workflow-model.md#register--capture-step-output)).

### CEL expressions in `when:` use a different syntax

In `when:` conditions, use the CEL namespace without braces:

```yaml
when: vars.role == "control-plane"         # CEL — no braces
```

Not:

```yaml
when: "{{ .vars.role }} == control-plane"  # wrong — this is template syntax
```

See [Conditions with when (CEL)](conditions-and-cel.md) for the full reference.

## Inspecting resolved vars: `deck plan vars`

Before running `prepare` or `apply`, verify the effective variable snapshot:

```bash
deck plan vars
```

This prints:

- The fully merged `vars` map (showing which values survive precedence).
- Resolved `context` fields (command, workflow source, paths).
- Initial `runtime` values known before execution (including `runtime.host`
  facts detected from the local OS).
- Planned runtime keys that steps will register during execution (the values
  themselves are not predicted).

Use `deck plan vars` whenever you are debugging a precedence question or
verifying that a node-scoped host entry resolved correctly.

## Common mistakes

### Wrong precedence assumption

**Mistake:** editing `vars.yaml` to override a value set in the scenario
`vars:` block and expecting `vars.yaml` to win.

**Fix:** recall that scenario `vars:` wins over `vars.yaml`. To share a value
across all scenarios, keep it in `vars.yaml` and omit it from scenario blocks.

### Missing `all:` default for unmatched hosts

**Mistake:** branching on `vars.role` without an `all: role: ""` default.
When the local hostname does not match any `hosts:` entry, `vars.role` is
undefined and the CEL expression raises `E_CONDITION_EVAL`.

**Fix:** add a safe default to `all:`:

```yaml
all:
  role: ""   # safe default prevents CEL errors on unmatched hosts
```

### Single-brace template syntax

**Mistake:** writing `{vars.name}` or `{ .vars.name }` instead of
`{{ .vars.name }}`.

**Fix:** always use double braces. Single-brace syntax raises
`E_TEMPLATE_SINGLE_BRACE` during step validation.

```yaml
# wrong
content: "cluster: {vars.clusterName}"

# correct
content: "cluster: {{ .vars.clusterName }}"
```

## Related references

- [Workflow Model — Variables](../workflow-model.md#variables) — canonical precedence rules and node-scoped vars
- [Authoring workflows](authoring-workflows.md) — how to structure a scenario from scratch
- [Conditions with when (CEL)](conditions-and-cel.md) — using vars in CEL `when:` expressions
- [Troubleshooting](../troubleshooting.md) — diagnosing variable resolution errors
