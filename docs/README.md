# deck documentation

This directory is organized for operators who want to use `deck` immediately, then go deeper into the workflow model and supported schema.

## Start here

- New to `deck`: `tutorials/quick-start.md`
- Building an offline Kubernetes workflow: `tutorials/offline-kubernetes.md`
- Looking for commands: `reference/cli.md`
- Looking for YAML structure: `reference/workflow-model.md`
- Looking for bundle contents: `reference/bundle-layout.md`
- Looking for schemas: `reference/schema-reference.md` and `schemas/README.md`
- Looking for examples: `examples/README.md`

## Tutorials

- `tutorials/quick-start.md`: create a workspace, validate, pack, and apply
- `tutorials/offline-kubernetes.md`: shape an end-to-end offline Kubernetes workflow around the shipped examples

## Reference

- `reference/cli.md`: current command surface and when to use each command
- `reference/workflow-model.md`: workflow structure, step fields, and execution semantics
- `reference/bundle-layout.md`: what `pack` puts into a bundle and why it matters offline
- `reference/schema-reference.md`: workflow schema, tool schema layout, and supported step kinds
- `reference/server-audit-log.md`: audit log location and JSONL record shape for `deck serve`

## Examples and Raw Schemas

- `examples/README.md`: runnable example workflows in this repository
- `schemas/README.md`: raw schema files and validation entry points

## Archive

Older planning notes, CI runbooks, parity drafts, and superseded docs were moved under `archive/` to keep the main documentation focused on user-facing material.
