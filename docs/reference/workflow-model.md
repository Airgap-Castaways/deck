# Workflow Model

`deck` uses a YAML workflow model so larger procedures stay reviewable. The goal is not to invent a DSL — it is to give air-gapped operational work a clearer structure than a growing shell script, where typed steps express intent and named phases show the operator what the procedure is doing before they read every detail.

## Top-level fields

- `version`: currently `v1alpha1`
- `vars`: optional variable map
- `steps`: top-level step list
- `phases`: named phase list for more structured execution

The schema allows one execution mode at a time:

- top-level `steps`
- named `phases`

Phase imports resolve from `workflows/components/`. Write component-relative paths such as `k8s/prereq.yaml`, not `../components/k8s/prereq.yaml`.

`workflows/components/` files are step fragments. They contain only `steps:` and may reference shared `vars.*`, but shared defaults should stay in `workflows/vars.yaml` or the importing scenario `vars:` block.

<!-- BEGIN GENERATED:WORKFLOW_SCHEMA_CONTRACT -->
## Workflow Schema Contract

Top-level workflow authoring reference for deck workflows.

- schema: `../../schemas/deck-workflow.schema.json`

### Example

```yaml
version: v1alpha1
steps:
  - id: write-config
    apiVersion: deck/v1alpha1
    kind: WriteFile
    spec:
      path: /etc/example.conf
      content: hello
```

### Fields

| Key | Type | Required | Default | Enum | Description | Example |
|---|---|---:|---|---|---|---|
| `phases` | `array<object>` | no | `` | `` | Ordered execution phases. Each phase can contain imports, steps, or both. | `[{name:install,steps:[...]}]` |
| `steps` | `array<object>` | no | `` | `` | Flat step list for workflows that do not need named phases. Execution normalizes these steps into an implicit `default` phase. | `[{id:configure-runtime,kind:WriteContainerdConfig,spec:{...}}]` |
| `vars` | `object` | no | `map[]` | `` |  | `map[]` |
| `version` | `string` | yes | `` | `v1alpha1` |  | `v1alpha1` |

### Validation Rules

- At least one of the top-level groups `phases` or `steps` must be present.
- Top-level `phases` and top-level `steps` cannot both be set in the same workflow.

### Notes

- A workflow must define at least one of `phases` or `steps`.
- A workflow cannot define both top-level `phases` and top-level `steps` at the same time.
- Top-level `steps` execute as an implicit phase named `default`.
- Imports are only supported under `phases[].imports` and resolve from `workflows/components/`.
- When a step omits `apiVersion`, deck resolves it from the top-level workflow `version` before schema and role checks run.
- Workflow mode is determined by command context or file location, not by an in-file `role` field.
- Each step still validates against its own kind-specific schema after the top-level workflow schema passes.
<!-- END GENERATED:WORKFLOW_SCHEMA_CONTRACT -->

## Variables

Variables, runtime values, and execution context come from distinct sources:

Static `vars` flow from four sources, in order of precedence:

1. CLI `--var` overrides
2. `vars:` block in the scenario file
3. CLI `-f, --vars-file` overlays merged into shared vars before node-scoped selection
4. `workflows/vars.yaml` shared defaults

`deck lint`, `deck prepare`, `deck plan`, and `deck apply` also support node-scoped shared variables in `workflows/vars.yaml`. When `hosts:` is present, deck detects the local hostname at execution time, applies optional `all:` values as shared defaults, and merges the matching host entry into top-level `vars`. If the hostname is not listed, execution continues with ordinary `vars.yaml` values and `all:` values.

Node-scoped `vars.yaml` example:

```yaml
all:
  kubernetesVersion: v1.35.5
  podCIDR: 10.244.0.0/16

hosts:
  k8s-cp1:
    ip: 192.168.81.211
    role: control-plane

  k8s-worker1:
    ip: 192.168.81.221
    role: worker
```

For `deck lint`, `deck prepare`, `deck plan`, and `deck apply`, deck first builds the shared vars document from `workflows/vars.yaml` plus any `-f, --vars-file` overlays supplied to `prepare`, `plan`, or `apply`. Later vars files override earlier files with the same deep-merge behavior as `vars.yaml`. The effective variable precedence is:

1. ordinary shared vars values, excluding `all` and `hosts` when `hosts:` is present
2. shared vars `all:` values
3. selected host values from shared vars `hosts.<hostname>`
4. selected workflow or scenario `vars:` values
5. CLI `--var` overrides

Hostname matching tries the detected hostname first, then the short hostname before the first `.`. Missing host entries are not fatal. Workflows that branch on host-specific fields should provide safe defaults in `all:`, such as `role: ""`, then use conditions such as `vars.role == "control-plane"`.

Runtime values flow separately through `register` outputs and built-in runtime facts such as `runtime.host`. Execution context values under `context.*` are deck-supplied metadata resolved at command runtime, such as the command name, workflow source, workflow path, bundle root, output root, and state file.

**`workflows/vars.yaml`** — define shared defaults once:

```yaml
clusterName: prod-k8s
```

**Scenario `vars:` block** — override or extend for the specific scenario:

```yaml
version: v1alpha1
vars:
  clusterName: staging-k8s   # overrides vars.yaml
```

**CLI vars files** — overlay site or node values without editing `workflows/vars.yaml`:

```bash
deck apply --scenario apply -f vars/site.yaml -f vars/cp1.yaml
```

Vars file paths are relative to the same `workflows/` location that contains `vars.yaml`.

- For local workflows, `-f vars/site.yaml` resolves to `workflows/vars/site.yaml`.
- For remote workflows, the same relative path is resolved from the remote `workflows/` URL.
- Later files override earlier files with the same deep-merge behavior as `vars.yaml`.
- Deck extracts `all:` and `hosts:` node-scoped values after vars-file overlays are merged.
- `--var` remains the final override with the highest precedence.

**Template interpolation** — use `{{ .vars.NAME }}` inside string fields:

```yaml
- id: write-hostname
  kind: WriteFile
  spec:
    path: /etc/hostname
    content: "{{ .vars.clusterName }}\n"
```

**CEL expressions** — use `vars.NAME`, `runtime.NAME`, and `context.NAME` (no braces) in `when:` conditions:

```yaml
- id: install-rhel-packages
  kind: InstallPackage
  spec:
    packages: [kubeadm, kubelet, kubectl]
  when: runtime.host.os.family == "rhel"
```

Node-scoped vars are static inputs selected before planning and state hashing; `runtime.host` remains the runtime fact namespace for detected OS, architecture, and kernel data.

<!-- BEGIN GENERATED:SYSTEM_VARIABLES -->
### Built-In Runtime Fields

`runtime.host` is a built-in reserved runtime namespace in both prepare and apply. Use it for detected host facts such as OS family, distro ID, version, architecture, and kernel release. Do not model detected local host facts as static `vars` values.

| Field | Type | Description |
|---|---|---|
| `runtime.host.os.name` | `string` | Operating system name reported by the Go runtime. |
| `runtime.host.os.id` | `string` | Distribution ID from `/etc/os-release` `ID`, lowercased. |
| `runtime.host.os.family` | `string` | Inferred distribution family such as `debian` or `rhel`, or empty when unknown. |
| `runtime.host.os.version` | `string` | Distribution version from `/etc/os-release` `VERSION`. |
| `runtime.host.os.versionId` | `string` | Distribution version ID from `/etc/os-release` `VERSION_ID`. |
| `runtime.host.os.release` | `string` | Alias of `runtime.host.os.versionId` retained for existing workflows. |
| `runtime.host.os.idLike` | `string` | Distribution compatibility IDs from `/etc/os-release` `ID_LIKE`, lowercased. |
| `runtime.host.arch` | `string` | Normalized host architecture such as `amd64` or `arm64`. |
| `runtime.host.kernel.release` | `string` | Kernel release from `/proc/sys/kernel/osrelease`. |

### Execution Context Fields

`context` is available in both `when` expressions and templates. Canonical fields are:

| Field | Type | Prepare | Apply | Description |
|---|---|---:|---:|---|
| `context.command` | `string` | yes | yes | Current command, `prepare` or `apply`. |
| `context.workflow.source` | `string` | yes | yes | Workflow source, `filesystem` or `server`. |
| `context.workflow.isServer` | `boolean` | yes | yes | Boolean convenience value derived from `context.workflow.source == "server"`. |
| `context.workflow.path` | `string` | yes | yes | Resolved workflow file path or URL. |
| `context.workflow.scenario` | `string` | no | yes | Scenario name when apply resolved a scenario. |
| `context.paths.bundleRoot` | `string` | yes | yes | Prepared output root during prepare; selected bundle root during apply. |
| `context.paths.outputRoot` | `string` | yes | no | Prepared output root. |
| `context.paths.stateFile` | `string` | no | yes | Apply state file path. |

Legacy aliases remain available for existing templates: `context.bundleRoot` maps to `context.paths.bundleRoot`, and `context.stateFile` maps to `context.paths.stateFile`.

When apply state keys are computed, deck includes a fingerprint of the execution context except fields derived from other context values, such as `context.workflow.isServer`, and `context.paths.stateFile`, which is derived from the state key itself.
<!-- END GENERATED:SYSTEM_VARIABLES -->

## Minimal workflow

```yaml
version: v1alpha1
steps:
  - id: prepare-state-dir
    kind: EnsureDirectory
    spec:
      path: /var/lib/deck
      mode: "0755"
```

## Minimal prepare workflow

```yaml
version: v1alpha1
steps:
  - id: fetch-kubeadm
    kind: DownloadFile
    spec:
      source:
        url: https://example.local/kubeadm
      mode: "0755"
```

When a prepare download step does not set an explicit output location, deck uses the step kind's default prepared path:

- `DownloadFile`: `files/<basename>`
- `DownloadImage`: `images/`
- `DownloadPackage`: `packages/`, or `packages/deb/<release>` and `packages/rpm/<release>` when `repo.type` is set

## Step Envelope Contract

Every workflow step uses the same outer envelope before kind-specific `spec` validation runs.

Required fields:

- `id`: stable step identifier; must match the workflow step id pattern
- `kind`: typed step name such as `WriteFile` or `CheckKubernetesCluster`
- `spec`: kind-specific payload validated against that step's schema

Optional shared fields:

- `apiVersion`: step api version; when omitted, deck resolves it from the top-level workflow `version`
- `when`: CEL expression; the step is skipped when it evaluates to false
- `parallelGroup`: consecutive steps with the same value can run in one batch inside a phase
- `retry`: retry count on failure
- `timeout`: duration string such as `30s` or `5m`
- `register`: export declared step outputs into later runtime values
- `metadata`: free-form annotation map for tooling or audit-oriented context

Shared envelope rules:

- `register` keys become available as `runtime.<name>` in CEL and `.runtime.<name>` in templates
- if a step runs inside a parallel batch, its `register` outputs become visible only after the full batch succeeds
- `spec` is always validated again against the selected step kind after the shared envelope passes

### `when` — conditional execution

`when` takes a CEL expression. Use `vars.` to reference input variables defined in `vars:` or `vars.yaml`, `runtime.` to reference step outputs registered earlier in the run plus built-in host facts under `runtime.host`, and `context.` to reference deck-supplied execution metadata.

```yaml
steps:
  - id: add-debian-repo
    kind: ConfigureRepository
    spec:
      format: deb
      repositories:
        - id: offline-base
          baseurl: file:///srv/offline-repo
          trusted: true
    when: runtime.host.os.family == "debian"

  - id: add-rhel-repo
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

Use `CheckHost` when the workflow should fail fast on host suitability checks such as `swap`, `kernelModules`, or required binaries. `CheckHost` validates those conditions, but `runtime.host` exists even when the workflow does not include a `CheckHost` step.

### `register` — capture step output

`register` maps a runtime variable name to a step output key. The exported value is available to later steps via `runtime.` in CEL and `.runtime` in templates. If the step runs inside a parallel batch, the value becomes visible after the full batch succeeds.

```yaml
steps:
  - id: get-join-cmd
    kind: InitKubeadm
    spec:
      outputJoinFile: "{{ .vars.joinFile }}"
    register:
      joinFile: joinFile

  - id: join-node
    kind: JoinKubeadm
    spec:
      joinFile: "{{ .runtime.joinFile }}"
      extraArgs: ["--cri-socket", "unix:///run/containerd/containerd.sock", "--ignore-preflight-errors=Swap,FileExisting-crictl,FileExisting-conntrack,FileExisting-socat"]
```

`register` can only export output names that the selected step kind explicitly declares. For example, `InitKubeadm` can export `joinFile`, while steps with no declared outputs reject non-empty `register` mappings during validation.

## Phases

Use phases when the procedure has natural boundaries — a host-prereqs block that must complete before a runtime block, for example. For simple apply workflows with a handful of steps, flat `steps:` is fine.

Each phase can import component fragments, include inline steps, or both. Phases are also the persisted resume boundary for `apply`.

```yaml
version: v1alpha1
phases:
  - name: host-prereqs
    imports:
      - path: k8s/prereq.yaml
      - path: repo/offline-repo.yaml
  - name: runtime
    maxParallelism: 2
    imports:
      - path: k8s/containerd-kubelet.yaml
  - name: verify
    steps:
      - id: check-node-ready
        kind: Command
        spec:
          command: [kubectl, get, nodes]
```

Import paths are relative to `workflows/components/`. Write `k8s/prereq.yaml`, not `../components/k8s/prereq.yaml`.

Top-level `steps:` are still valid. Execution normalizes them into an implicit phase named `default`.

## Parallel batches

Use `parallelGroup` when a few consecutive steps are safe to run together.

```yaml
version: v1alpha1
phases:
  - name: packages
    maxParallelism: 2
    steps:
      - id: download-ubuntu
        kind: DownloadPackage
        parallelGroup: distro-downloads
        spec:
          packages: [containerd]
          distro:
            family: debian
            release: ubuntu2204
          repo:
            type: deb-flat
          backend:
            mode: container
            runtime: docker
            image: ubuntu:22.04

      - id: download-rhel
        kind: DownloadPackage
        parallelGroup: distro-downloads
        spec:
          packages: [containerd]
          distro:
            family: rhel
            release: rhel9
          repo:
            type: rpm
          backend:
            mode: container
            runtime: docker
            image: rockylinux:9
```

Rules for the first version:

- only consecutive steps with the same `parallelGroup` value are in the same batch
- once a batch closes, the same `parallelGroup` value cannot reappear later in the phase
- phases still execute in order
- apply-time parallel batches are intentionally restricted to a small allowlist: `Command`, `CopyFile`, `EnsureDirectory`, `ExtractArchive`, `WaitForCommand`, `WaitForFile`, `WaitForMissingFile`, `WaitForService`, `WaitForTCPPort`, `WaitForMissingTCPPort`, and `WriteFile`
- same-batch apply steps cannot target the same literal output path or node path
- same-batch prepare steps cannot write to the same literal prepared root path such as the same `files/...`, `images/...`, or `packages/...` destination
- same-batch steps cannot consume each other's `register` outputs through `runtime.*`
- `register` outputs from a parallel batch become visible only to later batches or later phases

If a workflow needs one step to consume another step's runtime output, put those steps in separate batches or separate phases.

## Step kinds

Typed steps make the workflow easier to scan, validate, and evolve. Use `Command` only when no supported kind fits.

Public typed step reference is organized by task-oriented groups:

- `Host Prep`
- `Artifact Staging`
- `Filesystem and Content`
- `Package Management`
- `Runtime and Services`
- `Kubernetes Lifecycle`
- `Waits and Polling`
- `Advanced`

Use [Typed Steps](typed-steps.md) for the current group pages and exact supported kind inventory.

## Prepare semantics

`prepare` uses the same step grammar as `apply`, but command context determines which kinds are valid.

- `DownloadFile` is prepare-only and `outputPath` must stay under the canonical `files/` root
- `DownloadImage` is prepare-only and `outputDir` must stay under `images/` or an `images/...` subdirectory
- `DownloadPackage` is prepare-only and `outputDir` must stay under `packages/` or a `packages/...` subdirectory
- omit `outputPath` or `outputDir` unless you need a stable custom location for later apply steps
- container-backed `DownloadPackage` reuses a host-owned exported artifact cache after successful exports instead of bind-mounting apt/dnf package-manager cache directories
- `workflows/prepare.yaml` is the fixed entrypoint for prepare workflows

## When to use Command

Use `Command` when no supported step kind fits yet. It is the escape hatch, not the ideal authoring path. If a workflow leans heavily on `Command`, the procedure may still be too close to raw shell.

## Validation model

`deck lint` checks:

- the top-level workflow schema
- the schema for each referenced step kind
- reserved runtime keys and workflow compatibility rules

Validating before transport is one of the main reasons to use a workflow model instead of passing around shell files.

## Related references

- `../concepts/why-deck.md`
- [Workspace Layout](workspace-layout.md#component-fragment-contract)
- `bundle-layout.md`
- `../../schemas/deck-workflow.schema.json`
