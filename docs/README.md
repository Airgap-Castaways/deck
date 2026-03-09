# deck documentation

This directory is organized around the default `deck` operator path: prepare a bundle, carry it into the site, and run the maintenance session locally.

Site-assisted execution is documented here too, but it is a secondary, explicit choice for teams that want temporary shared visibility or a site-local content source inside the air gap.

## Start here

- New to `deck`: `tutorials/quick-start.md`
- Building an offline Kubernetes maintenance flow: `tutorials/offline-kubernetes.md`
- Looking for commands: `reference/cli.md`
- Looking for YAML structure: `reference/workflow-model.md`
- Looking for bundle contents: `reference/bundle-layout.md`
- Looking for schemas: `reference/schema-reference.md` and `schemas/README.md`
- Looking for examples: `examples/README.md`

## When to use deck

- You are preparing work outside the air gap and executing it locally inside the site.
- You want one operator-friendly workflow that stays useful even with no network services at the destination.
- You need optional site-local assistance without changing the underlying local execution model.

## When not to use deck

- You need a remote executor, long-lived service controller, or agent platform.
- You want online-first infrastructure automation as the main operating model.
- You need a broad infrastructure provisioning suite rather than a bounded offline maintenance tool.

## Tutorials

- `tutorials/quick-start.md`: create a workspace, validate it, build a bundle, and apply it locally
- `tutorials/offline-kubernetes.md`: adapt the shipped examples for Kubernetes-oriented offline maintenance sessions

## Reference

- `reference/cli.md`: command groups, default local flow, and secondary site-local helpers
- `reference/workflow-model.md`: workflow structure, step fields, and execution semantics
- `reference/bundle-layout.md`: what `pack` puts into a bundle and why it matters offline
- `reference/schema-reference.md`: workflow schema, tool schema layout, and supported step kinds
- `reference/server-audit-log.md`: audit log location and JSONL record shape for `deck serve`

## Examples and Raw Schemas

- `examples/README.md`: example workflows for local execution first, then site-specific adaptation
- `schemas/README.md`: raw schema files and validation entry points

## Archive

Older planning notes, CI runbooks, parity drafts, and superseded docs were moved under `archive/` to keep the main documentation focused on current user-facing material.
