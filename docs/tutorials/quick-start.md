# Quick Start

This tutorial shows the smallest useful `deck` workflow: initialize a workspace, validate it, build a bundle, and run it locally.

## 1. Create a workspace

```bash
deck init --out ./demo
```

This creates:

- `./demo/workflows/pack.yaml`
- `./demo/workflows/apply.yaml`
- `./demo/workflows/vars.yaml`

## 2. Add or edit steps

`deck init` starts with empty workflow files. Add the steps you need to the `pack` and `apply` workflows.

Minimal example:

```yaml
role: apply
version: v1alpha1
steps:
  - id: disable-swap
    apiVersion: deck/v1alpha1
    kind: RunCommand
    spec:
      command: ["swapoff", "-a"]
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

## 5. Apply locally at the target site

```bash
deck apply
```

`apply` executes the `apply` workflow locally. This is the core `deck` promise: no SSH, no PXE, no BMC, and no online control plane required.

## 6. Optional: expose a bundle over HTTP

If you want an internal repo-server style flow for packages, files, images, or workflow discovery:

```bash
deck serve --root ./bundle --addr :8080
deck source set --server http://127.0.0.1:8080
deck list
deck health
```

## What to read next

- `../tutorials/offline-kubernetes.md`
- `../reference/workflow-model.md`
- `../reference/bundle-layout.md`
