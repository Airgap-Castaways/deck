# Workspace Layout

This document describes the standard directory structure of a `deck` project. Use `deck init` to scaffold this layout automatically.

A `deck` workspace is organized into three main functional areas: **Workflows**, **Prepared Outputs**, and **Metadata**.

## Directory Structure

```text
.
├── workflows/
│   ├── prepare.yaml    # Prepare entry workflow
│   ├── scenarios/      # Apply scenario entry workflows
│   ├── components/     # Reusable step fragments (Component Fragments)
│   └── vars.yaml       # Shared variable definitions
├── outputs/            # Prepared artifacts and runtime binaries
│   ├── files/          # Prepared file payloads
│   ├── packages/       # Prepared package payloads
│   ├── images/         # Prepared image payloads
│   └── bin/            # Prepared runtime binaries by os/arch
└── .deck/              # (Internal) Checksums, manifest, and run history
```

## Workflows (`workflows/`)

The `workflows/` directory contains all your operational logic.

### Prepare Entrypoint (`workflows/prepare.yaml`)
`workflows/prepare.yaml` is the fixed entry workflow for `deck prepare`.

### Scenarios (`workflows/scenarios/`)
Scenarios are the primary entrypoints for `deck apply`.
- Each file here must be a complete workflow with `version` and either `steps` or `phases`.
- Typical filenames: `apply.yaml`, `bootstrap.yaml`, `worker-join.yaml`.

### Components (`workflows/components/`)
Components are **Component Fragments**—reusable sets of steps that are imported into scenarios.
- Files here follow the **Component Fragment Schema**.
- They only contain a `steps:` list.
- They are imported via `phases[].imports` in a scenario.
- **Example**: `workflows/components/k8s/runtime.yaml` is imported as `k8s/runtime.yaml`.

<!-- BEGIN GENERATED:COMPONENT_FRAGMENT_CONTRACT -->
#### Component Fragment Contract

Reference for reusable workflow component fragments located under `workflows/components/`.

- schema: `../../schemas/deck-component-fragment.schema.json`

##### Example

```yaml
steps:
  - id: write-config
    kind: WriteFile
    spec:
      path: /etc/example.conf
      content: hello
  - id: restart-service
    kind: ManageService
    spec:
      name: example
      state: restarted
```

##### Fields

| Key | Type | Required | Default | Enum | Description | Example |
|---|---|---:|---|---|---|---|
| `steps` | `array<object>` | yes | `` | `` | Ordered list of workflow steps contained in this fragment. | `[{id:write-config,kind:WriteFile,spec:{path:/etc/example.conf,content:hello}}]` |

##### Notes

- Component fragments are stored in the `workflows/components/` directory of your workspace.
- They contain only a `steps:` list and follow a restricted schema compared to full scenarios.
- Fragments are imported into a scenario phase using `phases[].imports`.
- The surrounding Workspace Layout documentation explains how component fragments fit into the standard project structure.
<!-- END GENERATED:COMPONENT_FRAGMENT_CONTRACT -->

### Variables (`workflows/vars.yaml`)
A central YAML file for shared defaults. Values defined here are available to all workflows and components via the `{{ .vars.NAME }}` syntax.

For node-specific runs, `vars.yaml` may contain `all:` defaults and `hosts:` overlays selected by local hostname. See [Workflow Model](workflow-model.md#variables) for precedence and hostname matching details.

## Prepared Outputs (`outputs/`)

These directories hold the prepared source material that `apply` consumes.
- During `deck prepare`, artifacts are gathered from workflow-declared local or remote sources and placed into canonical `outputs/files/`, `outputs/packages/`, or `outputs/images/` roots.
- Runtime binaries for offline execution are written under `outputs/bin/<os>/<arch>/deck`, while the workspace root `deck` file is a launcher script.
- Once bundled, these sources are no longer needed; the target node only sees the final bundle content.

## Internal Metadata (`.deck/`)

This directory is managed by `deck` and should not be edited manually.
- `manifest.json`: Tracks every file in the workspace for integrity and versioning.
- `runs/`: Local execution history for the workspace.

## Related References

- [Workflow Model](workflow-model.md)
- [Bundle Layout](bundle-layout.md)
- [Component Fragment Contract](#component-fragment-contract)
