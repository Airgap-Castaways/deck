# Authoring workflows

This guide walks you through writing a deck workflow from scratch, from the
files that `deck init` creates to a multi-phase scenario that imports component
fragments and branches on host type.

## Start from `deck init`

Run `deck init` to scaffold a workspace:

```bash
deck init --out ./my-workspace
cd ./my-workspace
```

You get the following layout:

```text
my-workspace/
├── workflows/
│   ├── prepare.yaml          # entry workflow for deck prepare
│   ├── vars.yaml             # shared variable defaults
│   ├── scenarios/
│   │   └── apply.yaml        # entry scenario for deck apply
│   └── components/
│       └── example-apply.yaml
└── outputs/
    ├── files/
    ├── packages/
    └── images/
```

**Which file to edit first?**

- Start with `workflows/vars.yaml` to define shared defaults that every
  scenario will read (Kubernetes version, cluster IPs, role flags).
- Edit `workflows/scenarios/apply.yaml` to describe the steps that run on the
  target node.
- Add step fragments under `workflows/components/` when you want to reuse
  groups of steps across scenarios.
- Edit `workflows/prepare.yaml` only when you need to stage artifacts
  (binaries, packages, images) for offline delivery.

## Anatomy of a scenario

A scenario is a complete workflow file under `workflows/scenarios/`. It must
contain `version` and at least one of `steps` or `phases`.

### Minimal annotated example

```yaml
# workflows/scenarios/apply.yaml

version: v1alpha1          # required; currently always v1alpha1

vars:                      # optional; scenario-level overrides for vars.yaml values
  clusterName: k8s-lab

steps:
  - id: write-motd         # required; stable, unique step identifier
    kind: WriteFile        # required; typed step kind (see step-kinds index)
    # apiVersion: deck/v1alpha1   # optional; deck infers it from version above
    spec:                  # required; kind-specific payload
      path: /etc/motd
      content: |
        deck maintenance session in progress, {{ .vars.clusterName }}
    when: ""               # optional; CEL expression, step is skipped when false
    register: {}           # optional; export step outputs as runtime.NAME
    timeout: 30s           # optional; duration string
    metadata:              # optional; free-form annotation map
      owner: platform-team
```

### The step envelope

Every step, regardless of kind, shares the same outer envelope:

| Field | Required | Purpose |
|---|---|---|
| `id` | yes | Stable step identifier; must be unique across the workflow |
| `kind` | yes | Typed step name (`WriteFile`, `InstallPackage`, …) |
| `spec` | yes | Kind-specific payload |
| `apiVersion` | no | Deck infers from `version` when omitted |
| `when` | no | CEL expression; step is skipped (not an error) when it evaluates to `false` |
| `register` | no | Maps runtime variable names to step output keys |
| `timeout` | no | Duration string such as `30s` or `5m` |
| `retry` | no | Retry count on failure |
| `parallelGroup` | no | Consecutive steps with the same value run in one batch |
| `metadata` | no | Free-form annotation map for tooling or audit context |

## Choosing typed steps over `Command`

`deck` provides purpose-built step kinds for common operations. Prefer them
over `Command` because:

- They are validated against a schema before execution.
- They express intent clearly in `deck plan` output.
- They are easier to lint and review than inline shell.

**Use a typed step when:**

| You want to … | Use |
|---|---|
| Write a file | `WriteFile` |
| Copy a prepared artifact | `CopyFile` |
| Install packages | `InstallPackage` / `InstallAptPackage` / `InstallDnfPackage` |
| Set up a systemd service | `WriteSystemdUnit` + `ManageService` |
| Bootstrap a Kubernetes control plane | `InitKubeadm` |
| Wait for a condition | `WaitForService`, `WaitForCommand`, `WaitForTCPPort`, … |
| Check host suitability | `CheckHost` |

**Use `Command` when:**

No typed step models the action: for example, a vendor-specific CLI call or a
one-off probe that deck does not yet support directly. `Command` is the escape
hatch, not the default choice.

See [Step Kinds](../step-kinds.md) for the full index organised by phase and
group.

## Structuring a multi-phase scenario

Use `phases` when the procedure has natural boundaries. Phases are also the
resume boundary for `deck apply`: if a run stops, it resumes from the last
incomplete phase.

### Importing component fragments

Components live under `workflows/components/`. They contain only a `steps:`
list. Import them into a phase using `phases[].imports`. Paths are relative to
`workflows/components/`, write `k8s/prereq.yaml`, not
`../components/k8s/prereq.yaml`.

```yaml
# workflows/scenarios/bootstrap.yaml

version: v1alpha1
phases:
  - name: host-prereqs          # human-readable phase label
    imports:
      - path: host-prereqs.yaml         # resolves from workflows/components/
      - path: repo/offline-repo.yaml

  - name: runtime
    imports:
      - path: runtime/containerd.yaml
      - path: runtime/kubelet.yaml

  - name: verify
    steps:
      - id: check-cluster-ready         # inline step alongside imports
        kind: CheckKubernetesCluster
        spec:
          timeout: 10m
          interval: 10s
          nodes:
            total: 1
          kubeSystem:
            readyPrefixes:
              - etcd-
              - kube-apiserver-
              - kube-controller-manager-
              - kube-scheduler-
```

**Component fragment** (`workflows/components/host-prereqs.yaml`):

```yaml
steps:
  - id: disable-swap
    kind: Swap
    spec:
      disable: true
      persist: true

  - id: load-overlay-module
    kind: KernelModule
    spec:
      names: [overlay, br_netfilter]
      load: true
      persist: true
      persistFile: /etc/modules-load.d/kubernetes.conf
```

A component fragment has no `version` or `vars`; those belong to the
importing scenario.

## Branching with `when`

Use the `when` field to skip a step on hosts where it is not applicable. The
value is a CEL expression. A step is skipped (not an error) when `when`
evaluates to `false`.

### OS-family branching

```yaml
steps:
  - id: configure-apt-repo
    kind: ConfigureRepository
    spec:
      format: deb
      repositories:
        - id: offline-base
          baseurl: file:///srv/offline-repo
          trusted: true
    when: runtime.host.os.family == "debian"   # skipped on RHEL nodes

  - id: configure-yum-repo
    kind: ConfigureRepository
    spec:
      format: rpm
      repositories:
        - id: offline-base
          name: offline-base
          baseurl: file:///srv/offline-repo
          enabled: true
          gpgcheck: false
    when: runtime.host.os.family == "rhel"     # skipped on Debian nodes
```

### Role-based branching

When the `vars.yaml` uses `hosts:` to assign a `role` per node, `when` lets
you target only control-plane or worker steps:

```yaml
  - id: kubeadm-init
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

For more patterns, including conditional imports and register-gated
conditions, see [Conditions with when (CEL)](conditions-and-cel.md).

## Validate before committing

Always lint before packaging or transporting a workflow:

```bash
# lint the default scenario
deck lint

# lint a specific workflow file
deck lint --workflow ./workflows/scenarios/bootstrap.yaml
```

`deck lint` checks:

- the top-level workflow schema (`version`, `phases`/`steps` mutual exclusion,
  required fields)
- the schema for each typed step kind
- reserved runtime keys and workflow compatibility rules

**Preview execution order without running anything:**

```bash
deck plan
deck plan vars   # show the fully-resolved variable snapshot
```

`deck plan` prints the phases and steps that would run, with conditions
evaluated where possible. `deck plan vars` prints the effective `vars`,
resolved `context`, and initial `runtime` values. Useful for verifying that
node-scoped vars resolved correctly before you run `apply`.

## Related references

- [Workflow Model](../workflow-model.md): authoritative schema and field reference
- [Step Kinds](../step-kinds.md): full index by phase and group
- [Variables and templating](variables-and-templating.md): vars precedence and `{{ .vars.NAME }}` syntax
- [Conditions with when (CEL)](conditions-and-cel.md): full `when` reference
- [Workspace Layout](../workspace-layout.md): directory contract for components and scenarios
