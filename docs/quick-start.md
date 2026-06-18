# Quick Start

This tutorial walks through the default `deck` path:

1. create a workspace
2. express the procedure as a workflow
3. lint it
4. build the bundle
5. run it locally

## 1. Create a workspace

```bash
deck init --out ./demo
```

This creates a starter layout with two entry workflows and a prepared output tree scaffold:

- `./demo/workflows/prepare.yaml`
- `./demo/workflows/scenarios/apply.yaml`
- `./demo/workflows/vars.yaml`
- `./demo/workflows/components/example-apply.yaml`
- `./demo/outputs/files/`
- `./demo/outputs/packages/`
- `./demo/outputs/images/`

## 2. Add or edit steps

`deck init` creates two entry workflows with distinct roles: `workflows/prepare.yaml` runs in your connected environment to fetch artifacts, while `workflows/scenarios/apply.yaml` runs on the target machine to apply them. Start by editing `apply.yaml` for what the target node needs to do; edit `prepare.yaml` to control what gets downloaded into the bundle. Reusable fragments shared across scenarios live under `workflows/components/`.

Prefer typed steps. They make the procedure easier to read and lint as it grows.

When choosing a step, start with [Step Kinds](step-kinds.md). For shared step fields such as `when`, `parallelGroup`, `register`, `metadata`, `retry`, and `timeout`, use the [Step Envelope Contract](workflow-model.md#step-envelope-contract).

```yaml
version: v1alpha1
steps:
  - id: write-motd
    apiVersion: deck/v1alpha1
    kind: WriteFile
    spec:
      path: /etc/motd
      content: |
        deck maintenance session in progress
```

Use `vars.yaml` or inline `vars` to keep site-specific values out of the step definitions.

## 3. Validate before you package

```bash
deck lint
deck lint --workflow ./demo/workflows/scenarios/apply.yaml
```

`deck lint` checks the workflow structure and the schema for each typed step. Catching mistakes here is cheaper than discovering them inside the air gap.

## 4. Build an offline bundle

Run `prepare` from a workspace directory that contains `workflows/prepare.yaml`. `workflows/vars.yaml` and `workflows/scenarios/apply.yaml` are optional at this stage.

```bash
cd ./demo
deck prepare
deck bundle build --out ./bundle.tar
```

`prepare` writes generated artifacts under `./demo/outputs/`, writes a root `./demo/deck` launcher, and updates `./demo/.deck/manifest.json`. `bundle build` turns the current workspace into the archive you carry into the site.

## 5. Apply locally at the target site

```bash
deck apply
```

`apply` executes the scenario locally on the machine that needs the change. Run it from a workspace or unpacked bundle root that contains `workflows/`. No SSH, no controller, no external reach-back required.

On success you will see phase and step progress on stderr, with a final completion summary. A run that completes cleanly exits 0 and leaves saved apply state under `.deck/state/apply/` — useful for auditing what ran and for resuming at phase boundaries if the run is interrupted. If anything fails, `deck apply` exits non-zero and prints the failing step with its error. See [Troubleshooting](troubleshooting.md) if the run does not progress as expected.

## 6. Optional: add site assistance

Some sites benefit from a temporary local server inside the air gap as a shared bundle source. Use `deck server up` for this when it solves a real problem.

Typical patterns:

```bash
deck server up --root ./bundle --addr :8080
deck server up --root ./bundle --addr :8443 --tls-self-signed
deck server up --root ./bundle --addr :8080 --daemon --unit deck-server
```

Use [CLI Reference](cli.md) for TLS and daemon flags, and [Server Audit Log](server-audit-log.md) for the current audit record shape written under `.deck/logs/server-audit.log`.

That path extends the local workflow. It does not replace it.

## What to read next

- [Why deck?](core-concepts/why-deck.md)
- [Workflow model](workflow-model.md)
- [Apply State](apply-state.md)
- [Step Kinds](step-kinds.md)
- [Bundle layout](bundle-layout.md)
- [CLI Reference](cli.md)
- [Using deck ask](ask.md)
