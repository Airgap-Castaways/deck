# Offline Kubernetes Tutorial

This tutorial shows how to use `deck` as an offline Kubernetes installer in the environment it is built for: a site with no internet, no SSH-based orchestration, no PXE, and no BMC dependencies.

## Goal

Build a portable bundle online, move it into the offline site, then run Kubernetes-oriented apply workflows locally on each node.

## 1. Start from the shipped examples

Relevant examples in this repository:

- `../examples/offline-k8s-control-plane.yaml`
- `../examples/offline-k8s-worker.yaml`
- `../examples/offline-repo-preinstall.yaml`
- `../examples/offline-containerd-mirror.yaml`
- `../examples/offline-verify-images.yaml`

These examples are intentionally YAML-first and Kubernetes-friendly so operators used to manifest-driven tooling can adapt them quickly.

## 2. Separate the two jobs clearly

- `pack` is the online-side preparation step
- `apply` is the offline-side execution step

The clean mental model is:

```text
prepare artifacts -> pack bundle -> transfer bundle -> apply locally
```

## 3. Prepare a bundle in the connected environment

Author a `pack` workflow that gathers the packages, container images, files, and templates your site needs.

Then build the bundle:

```bash
deck pack --out ./bundle.tar
```

The bundle can include `packages/`, `images/`, `files/`, `workflows/`, the `deck` binary, and `.deck/manifest.json` checksums.

## 4. Move the bundle into the offline site

Transfer `bundle.tar` through the approved path for your environment: removable media, controlled gateway, or any other site-approved offline handoff.

`deck` assumes this transfer step is out-of-band. It does not depend on an SSH automation path.

## 5. Run apply workflows locally

At the offline site, execute on the target machine itself:

```bash
deck apply
```

Use the control-plane and worker examples as building blocks for kubeadm-based cluster bootstrap.

## 6. Optional: provide an internal repo server

If the offline site benefits from a shared local source for files, packages, or workflows, serve the prepared bundle over HTTP:

```bash
deck serve --root ./bundle --addr :8080
deck source set --server http://127.0.0.1:8080
deck list
deck health
```

The repo-server flow is still local-site friendly. It exists to reduce repetition inside the air gap, not to reintroduce online-first assumptions.

## 7. Validate the workflow shape and artifact assumptions

Before transport or execution, validate your YAML and schema compatibility:

```bash
deck validate --file ./workflows/pack.yaml
deck validate --file ./workflows/apply.yaml
```

For workflow planning and diagnostics, also review:

- `../reference/workflow-model.md`
- `../reference/schema-reference.md`
- `../reference/server-audit-log.md`
