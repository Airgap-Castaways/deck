# Tool Schemas

This directory contains JSON Schemas for typed workflow steps.

The files are not all equal from a user-authoring point of view.

## Public apply steps

- `public/inspection.schema.json`
- `public/containerd.schema.json`
- `public/directory.schema.json`
- `public/artifacts.schema.json`
- `public/packages.schema.json`
- `public/file.schema.json`
- `public/image.schema.json`
- `public/kernel-module.schema.json`
- `public/kubeadm.schema.json`
- `public/package-cache.schema.json`
- `public/repository.schema.json`
- `public/service.schema.json`
- `public/swap.schema.json`
- `public/symlink.schema.json`
- `public/systemd-unit.schema.json`
- `public/sysctl.schema.json`
- `public/wait.schema.json`

## Advanced steps

- `advanced/command.schema.json`

These remain user-visible, but they are not the preferred starting point when a higher-level typed step or declarative prepare model already exists.

## Legacy/internal prepare steps

Legacy/internal prepare fetch schemas have been removed. Prepare artifact planning now lowers to noun families with `action: download`.

## Metadata

Each schema root now carries:

- `description`: what the step is for
- `x-deck-visibility`: one of `public`, `advanced`, or `legacy-prepare`

Use `../../reference/schema-reference.md` for the curated reference view.
