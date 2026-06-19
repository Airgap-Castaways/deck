# Glossary

Alphabetical reference of terms used throughout the deck documentation. Each entry links to the authoritative source.

---

**Air-gapped**
A deployment environment with no direct internet access, where all dependencies must be carried in advance. deck is designed around the air-gapped constraint: `prepare` fetches everything on a network-connected machine and `bundle build` packages it into an archive that can be transferred without any further reach-back. See [bundle-layout.md](bundle-layout.md).

**Apply**
The command and phase that executes a workflow on a target machine. `deck apply` reads the scenario, resolves the bundle root, verifies the bundle manifest, and runs each phase in order. Apply state is tracked so interrupted runs can resume. See [apply-state.md](apply-state.md) and the [CLI Reference](cli.md).

**Bundle**
The self-contained archive produced by `deck bundle build` that carries the workflow files, prepared artifacts (`outputs/`), and the deck launcher into an air-gapped site. A bundle is the unit of offline handoff. See [bundle-layout.md](bundle-layout.md).

**Bundle root**
The directory that serves as the base for a `deck apply` run. It must contain a `workflows/` tree and a `.deck/manifest.json`. When running from an extracted bundle or a prepared workspace, the bundle root is the directory passed via `--root` or resolved from the current directory. See [bundle-layout.md](bundle-layout.md) and [workspace-layout.md](workspace-layout.md).

**Component fragment**
A reusable YAML file under `workflows/components/` that contains only a `steps:` list. Fragments are imported into scenario phases via `phases[].imports`. They cannot contain top-level `phases` or `vars` and follow the component fragment schema rather than the full workflow schema. See [workspace-layout.md](workspace-layout.md#component-fragment-contract) and [workflow-model.md](workflow-model.md).

**Context (runtime context)**
The set of deck-supplied execution metadata available to steps via `context.*` in both CEL `when` expressions and Go templates. Fields include `context.command`, `context.workflow.source`, `context.workflow.path`, `context.workflow.scenario`, `context.paths.bundleRoot`, `context.paths.outputRoot`, and `context.paths.stateFile`. Context fields are part of the apply state key fingerprint. See [workflow-model.md](workflow-model.md#execution-context-fields).

**deck**
The CLI tool for authoring, packaging, and executing offline Kubernetes and deployment workflows. It covers the full operator flow: `init` → `lint` → `prepare` → `bundle build` → `apply`. See [quick-start.md](quick-start.md) and [CLI Reference](cli.md).

**Manifest**
The file `.deck/manifest.json` written by `deck prepare` that records the SHA-256 digest, size, and path of every artifact in `outputs/`. The manifest is the integrity baseline for `deck bundle verify` and for the apply-start verification check. A missing or empty manifest causes `E_MANIFEST_MISSING` or `E_MANIFEST_EMPTY`. See [bundle-layout.md](bundle-layout.md#apply-time-manifest-verification) and [diagnostics/error-codes.md](diagnostics/error-codes.md).

**ParallelGroup**
The shared label on a set of consecutive workflow steps that are allowed to run concurrently within a phase. Steps with the same `parallelGroup` value must be contiguous in their phase and must not read each other's `register` outputs or write to the same target path. Registered outputs from a parallel batch become visible only after the full batch succeeds. See [workflow-model.md](workflow-model.md#parallel-batches) and [apply-state.md](apply-state.md#parallel-batches-inside-a-phase).

**Phase**
A named section of a workflow that groups logically related steps and acts as the resume boundary for `deck apply`. Completed phases are skipped on resume; a failed phase reruns from its first step. Workflows may use named `phases:` for structured execution or a flat `steps:` list (which executes as an implicit phase named `default`). See [workflow-model.md](workflow-model.md#phases) and [apply-state.md](apply-state.md#phase-based-resume).

**Prepare**
The command and phase that fetches artifacts from the network, builds container-backed packages, and writes all prepared outputs under `outputs/`. Running `deck prepare` on a network-connected machine is the first step before the bundle is archived and carried into the air gap. See [workflow-model.md](workflow-model.md#prepare-semantics), [bundle-layout.md](bundle-layout.md), and the [CLI Reference](cli.md).

**Register**
The step envelope field that maps a step's declared output key to a runtime variable name. A `register:` block exports the output as `runtime.<name>`, making it available to later steps via CEL (`runtime.name`) or templates (`.runtime.name`). Only step kinds that explicitly declare outputs support `register`. See [workflow-model.md](workflow-model.md#register--capture-step-output).

**Scenario**
A complete workflow file under `workflows/scenarios/` that serves as the entry point for `deck apply`. A scenario must have a `version` field and either `phases` or `steps`. It may import component fragments and define its own `vars:` block to extend or override shared defaults. The scenario name is the filename without the `.yaml` extension. See [workspace-layout.md](workspace-layout.md#scenarios-workflowsscenarios) and [workflow-model.md](workflow-model.md).

**Source locator**
The combination of flags used to identify the workflow source and entrypoint for `plan`, `apply`, and `state` commands. The two primary source selectors are `--root <path>` (local workflow tree or bundle root) and `--server <url>` (remote workflow server); `--scenario <name>` then selects the scenario file under the chosen source. `--workflow <path-or-url>` is available as an explicit file escape hatch. See [CLI Reference](cli.md#workflow-source-locators).

**State key**
The stable identifier that deck uses to match saved apply state to a specific workflow run. It is derived from the resolved workflow bytes after imports expand, the effective vars for the run, and the apply execution context fingerprint. Changing the workflow, vars, or context produces a different state key, causing a fresh run rather than a resume. See [apply-state.md](apply-state.md#what-identifies-saved-state).

**Step envelope**
The shared outer wrapper present on every workflow step before kind-specific `spec` validation runs. It contains the required fields `id`, `kind`, and `spec`, plus optional shared fields including `apiVersion`, `when`, `parallelGroup`, `retry`, `timeout`, `register`, and `metadata`. See [workflow-model.md](workflow-model.md#step-envelope-contract).

**Step kind**
The typed name (`kind:`) that identifies what a step does. Each step kind has its own schema and a defined set of allowed roles (prepare-only, apply-only, or both). Examples include `WriteFile`, `DownloadFile`, `InstallPackage`, `InitKubeadm`, and `Command`. The full step kind inventory is organized by phase and task group in [step-kinds.md](step-kinds.md).

**Vars**
Static input variables declared in `workflows/vars.yaml`, in a scenario's `vars:` block, or supplied at the command line via `--var` or `-f`/`--vars-file`. Vars are resolved before execution and are part of the apply state key fingerprint. They are available in Go templates as `.vars.NAME` and in CEL expressions as `vars.NAME`. See [workflow-model.md](workflow-model.md#variables).

**Workspace**
The directory tree created by `deck init` that holds workflow files, prepared outputs, and deck metadata. The three main areas are `workflows/` (operational logic), `outputs/` (prepared artifacts), and `.deck/` (checksums, manifest, and state). See [workspace-layout.md](workspace-layout.md).

**XDG state root**
The base directory used by deck for user-scoped persistent state that lives outside any workspace. Follows the XDG Base Directory Specification: `$XDG_STATE_HOME/deck/` or `~/.local/state/deck/` by default. Remote workflow apply state, apply run logs, cache, and ask session data are stored here rather than in the workspace. See [apply-state.md](apply-state.md#where-state-is-stored) and [apply-runlogs.md](apply-runlogs.md).
