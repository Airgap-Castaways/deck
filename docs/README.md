# deck documentation

`deck` is a workflow tool for air-gapped and operationally constrained environments. Write a workflow, validate it, bundle what the site needs, and run it locally on the target machine.

## Start here

- **[Quick Start](quick-start.md)**: Create a workspace, lint it, prepare artifacts, build a bundle, and apply locally.
- **[Using deck ask](ask.md)**: Configure and use `deck ask`, including plan mode and diagnostics.
- **[Examples](examples/README.md)**: Start from concrete workflow files that can be adapted for site procedures.

## Author Workflows

- **[Workflow Model](workflow-model.md)**: Workflow structure, shared step fields, variables, phases, and validation rules.
- **[Step Kinds](step-kinds.md)**: Phase and task-oriented reference for choosing and authoring workflow step kinds.
- **[Workspace Layout](workspace-layout.md)**: Workspace structure and component fragment contracts.

## Operate

- **[CLI Reference](cli.md)**: Command-line usage and flags.
- **[Apply State](apply-state.md)**: Phase-based apply resume and `--fresh` behavior.
- **[Bundle Layout](bundle-layout.md)**: Self-contained bundle format.
- **[Server Audit Log](server-audit-log.md)**: Server audit log record shape.

## Supporting sections

- **[Core Concepts](core-concepts/README.md)**: Why deck exists and how the architecture fits together.
- **[Contributing](contributing/README.md)**: Development process, style, release, and compatibility notes.

## Common paths

- New to deck: start with [Quick Start](quick-start.md)
- Planning an offline Kubernetes workflow: read [Offline Kubernetes Tutorial](offline-kubernetes.md)
- Writing workflows: use [Workflow Model](workflow-model.md)
- Looking up exact command syntax: use [CLI Reference](cli.md)
