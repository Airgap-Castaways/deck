# Tool Schemas

This directory contains JSON Schemas for typed workflow steps.

The files are not all equal from a user-authoring point of view.

## Current schemas

- `inspection.schema.json`
- `artifacts.schema.json`
- `packages.schema.json`
- `directory.schema.json`
- `symlink.schema.json`
- `systemd-unit.schema.json`
- `containerd.schema.json`
- `repository.schema.json`
- `package-cache.schema.json`
- `swap.schema.json`
- `kernel-module.schema.json`
- `service.schema.json`
- `sysctl.schema.json`
- `file.schema.json`
- `image.schema.json`
- `wait.schema.json`
- `kubeadm.schema.json`
- `command.schema.json`

Legacy/internal prepare fetch schemas have been removed. Prepare artifact planning now lowers to noun families with `action: download`.

## Metadata

Each schema root now carries:

- `description`: what the step is for
- `x-deck-visibility`: one of `public`, `advanced`, or `legacy-prepare`

Use `../../reference/schema-reference.md` for the curated reference view.
