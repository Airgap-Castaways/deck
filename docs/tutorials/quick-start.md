# Quick Start

This tutorial shows the default `deck` path: initialize a workspace, validate it, build a bundle, carry it into the site, then run `diff`, `doctor`, and `apply` locally.

If you later add site-assisted execution, treat that as an explicit extension of the same local workflow, not a different product mode.

## 1. Create a workspace

```bash
deck init --out ./demo
```

This creates:

- `./demo/workflows/pack.yaml`
- `./demo/workflows/apply.yaml`
- `./demo/workflows/vars.yaml`

## 2. Add or edit steps

`deck init` starts with empty workflow files. Add the preparation work to `pack.yaml` and the maintenance steps to `apply.yaml`.

Start with typed step kinds when they fit the job. Keep shell commands for edge cases only.

Minimal example:

```yaml
role: apply
version: v1alpha1
steps:
  - id: write-motd
    apiVersion: deck/v1alpha1
    kind: InstallFile
    spec:
      path: /etc/motd
      content: |
        deck maintenance session in progress
```

Use `vars.yaml` or inline `vars` to keep site-specific values out of the steps themselves.

## 3. Validate before you package

```bash
deck validate --file ./demo/workflows/apply.yaml
deck validate --file ./demo/workflows/pack.yaml
```

Validation checks the workflow structure and the schema for each supported step kind.

## 4. Build an offline bundle

Run `pack` from a directory that contains `workflows/pack.yaml`, `workflows/apply.yaml`, and `workflows/vars.yaml`.

```bash
cd ./demo
deck pack --out ./bundle.tar
```

The resulting bundle is designed to be self-contained for offline transport.

## 5. Run the local maintenance flow at the target site

```bash
tar -xf ./bundle.tar
cd ./bundle
deck diff --file ./workflows/apply.yaml
deck doctor --file ./workflows/apply.yaml --out ./reports/doctor.json
deck apply --file ./workflows/apply.yaml
```

That is the base `deck` story: prepare outside the air gap, move the bundle in, inspect drift with `diff`, confirm local readiness with `doctor`, then run `apply` on the target machine.

## 6. Optional: add site-assisted execution

Use a site-assisted path only when you explicitly want a temporary site-local server, shared bundle source, or session visibility inside the air gap.

That choice is additive. Operators still run `deck diff`, `deck doctor`, and `deck apply` locally on the nodes that need the work.

`RunCommand` is still supported, but keep it as a last resort when a clearer step kind does not fit yet.

## What to read next

- `../tutorials/offline-kubernetes.md`
- `../reference/workflow-model.md`
- `../reference/bundle-layout.md`
