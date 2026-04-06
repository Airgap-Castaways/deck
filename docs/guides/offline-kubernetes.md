# Offline Kubernetes Tutorial

This tutorial shows how `deck` fits a Kubernetes maintenance session in the environment it is built for: no internet, no SSH-driven orchestration, and a local operator on each node.

Kubernetes host-prep and bootstrap procedures grow quickly. Raw shell becomes hard to review at that scale. `deck` keeps the procedure structured, validates it before transport, and bundles everything the nodes need into a single archive.

## Goal

Build a portable bundle in the connected environment, move it into the air gap, then run Kubernetes-oriented workflows locally on each target node.

## 1. Start from the shipped examples

Start from the bundled workflow tree and adapt it to your site. When you need exact workflow and step contracts, cross-check with the reference docs before transport.

## 2. Keep the two jobs separate

The mental model is straightforward:

```text
prepare artifacts -> build bundle -> transfer bundle -> run locally on each node
```

`prepare` gathers what the site needs before transport. `apply` executes the procedure locally on the node. This separation means the operator on the far side of the air gap only needs the bundle: a root `deck` launcher plus the matching runtime binaries under `outputs/bin/` - no external dependencies at run time.

## 3. Model the procedure clearly

Use steps and phases to show what the procedure is doing. Typical boundaries in Kubernetes workflows:

- host preparation
- package or image setup
- runtime configuration
- kubeadm bootstrap or join
- verification

Prefer typed steps where possible. `Command` is available for the edges that are not modeled yet. For shared step fields such as `when`, `parallelGroup`, `register`, `metadata`, `retry`, and `timeout`, use the [Step Envelope Contract](../reference/workflow-model.md#step-envelope-contract).

Useful group entrypoints for Kubernetes workflows:

- [Host Prep](../reference/groups/host-prep.md)
- [Artifact Staging](../reference/groups/artifact-staging.md)
- [Runtime and Services](../reference/groups/runtime-services.md)
- [Kubernetes Lifecycle](../reference/groups/kubernetes-lifecycle.md)
- [Waits and Polling](../reference/groups/waits-polling.md)

## 4. Prepare the bundle in the connected environment

Author a `prepare` workflow that gathers packages, container images, files, and templates. Then build the bundle:

```bash
deck prepare
deck bundle build --out ./bundle.tar
```

During local testing, use repeatable `--var key=value` overrides instead of editing shared vars files for every site-specific change:

```bash
deck prepare --var kubernetesVersion=v1.30.1 --var registryHost=mirror.local
```

The bundle includes the canonical workspace inputs: `outputs/packages/`, `outputs/images/`, `outputs/files/`, `outputs/bin/`, `workflows/`, the root `deck` launcher, and `.deck/manifest.json` checksums.

## 5. Move the bundle into the offline site

Transfer `bundle.tar` through the approved path for your environment - removable media, controlled gateway, or another site-approved handoff. `deck` does not require a remote control service for this step.

## 6. Run workflows locally on the target nodes

At the offline site, execute on the target machine itself:

```bash
deck apply
```

Use the control-plane and worker workflows in your workspace as starting points for kubeadm-based bootstrap and follow-on maintenance.

## 7. Add site assistance only when it solves a real problem

Some sites benefit from a temporary shared bundle source inside the air gap. That can help when several nodes need the same release inside the same air gap.

Examples:

```bash
deck server up --root ./bundle --addr :8080
deck server up --root ./bundle --addr :8443 --tls-self-signed
deck server up --root ./bundle --addr :8080 --daemon --unit deck-server
```

Keep that choice explicit and secondary. The core workflow centers on local `deck` execution on each node.

## 8. Validate before transport and execution

```bash
deck lint
deck lint --file ./workflows/scenarios/apply.yaml
```

For planning and diagnostics, also review:

- [Workflow model](../reference/workflow-model.md)
- [Typed Steps](../reference/typed-steps.md)
- [Workspace Layout](../reference/workspace-layout.md)
- [Server audit log](../reference/server-audit-log.md)
- [CLI Reference](../reference/cli.md)
