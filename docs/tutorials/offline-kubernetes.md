# Offline Kubernetes Tutorial

This tutorial shows how to use `deck` for a manual-first Kubernetes maintenance session in the environment it is built for: a site with no internet, no SSH-based orchestration, no PXE, and no BMC dependencies.

## Goal

Build a portable bundle outside the site, move it into the air gap, then run Kubernetes-oriented workflows locally on the nodes that need the change.

## 1. Start from the shipped examples

Relevant examples in this repository:

- `../examples/offline-k8s-control-plane.yaml`
- `../examples/offline-k8s-worker.yaml`
- `../examples/offline-repo-preinstall.yaml`
- `../examples/offline-containerd-mirror.yaml`
- `../examples/offline-verify-images.yaml`

These examples are intentionally YAML-first so operators can review and adapt them before carrying anything into the site.

## 2. Keep the two jobs separate

- `pack` is the preparation step outside the air gap
- `apply` is the execution step inside the air gap

The core mental model is:

```text
prepare artifacts -> pack bundle -> transfer bundle -> run locally on each node
```

## 3. Prepare the bundle in the connected environment

Author a `pack` workflow that gathers the packages, container images, files, and templates your site needs.

Then build the bundle:

```bash
deck pack --out ./bundle.tar
```

The bundle can include `packages/`, `images/`, `files/`, `workflows/`, the `deck` binary, and `.deck/manifest.json` checksums.

## 4. Move the bundle into the offline site

Transfer `bundle.tar` through the approved path for your environment: removable media, controlled gateway, or any other site-approved handoff.

`deck` assumes this transfer step is out-of-band. It does not depend on SSH automation or a remote control service.

## 5. Run workflows locally on the target nodes

At the offline site, execute on the target machine itself:

```bash
deck apply
```

Use the control-plane and worker examples as building blocks for kubeadm-based cluster bootstrap and follow-on maintenance.

## 6. Add site assistance only when it solves a real local problem

Some sites want a temporary shared source for bundle contents or a local place to collect session status. That can help when multiple nodes need the same release inside the same air gap.

Keep that choice explicit and secondary. The operator workflow still centers on local `deck` execution on each node, not remote triggering.

## 7. When to use deck here

- You are walking nodes one by one in a disconnected Kubernetes environment.
- You need deterministic local steps for bootstrap, repair, upgrade prep, or validation.
- You want optional site-local coordination without changing the local execution model.

## 8. When not to use deck here

- You want cluster-wide remote orchestration from a central controller.
- You expect unattended execution pushed from a service.
- You need a general online Kubernetes platform manager instead of an offline maintenance-session tool.

## 9. Validate the workflow shape and artifact assumptions

Before transport or execution, validate your YAML and schema compatibility:

```bash
deck validate --file ./workflows/pack.yaml
deck validate --file ./workflows/apply.yaml
```

For workflow planning and diagnostics, also review:

- `../reference/workflow-model.md`
- `../reference/schema-reference.md`
- `../reference/server-audit-log.md`
