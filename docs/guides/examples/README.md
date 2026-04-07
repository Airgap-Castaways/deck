# Examples

The files in this directory are starting points for real procedures. They show how typed steps express operational intent and how phases keep larger workflows scannable.

## How to use these examples

- Start from them when you want a concrete workflow to adapt.
- Keep the overall structure clear before adding more details.
- Replace repetitive shell with typed steps when a step kind already fits.
- Validate the result before packaging or transport.

## Command policy

- These examples intentionally stay typed-first and avoid `Command` when a built-in step already models the action.
- Reserve `Command` for vendor tools, custom probes, or one-off local commands that deck does not model directly.
- If a workflow needs service lifecycle changes, file operations, archive extraction, sysctl changes, swap control, kernel modules, or symlink management, prefer the dedicated typed steps instead.

## Files

- `offline-k8s-control-plane.yaml`: kubeadm-based control-plane bootstrap example
- `offline-k8s-worker.yaml`: kubeadm worker join example
- `offline-pull-control-plane.yaml`: pull-based control-plane bootstrap example
- `offline-pull-worker.yaml`: pull-based worker join example
- `offline-repo-preinstall.yaml`: prepare package repository configuration on the target host
- `offline-containerd-mirror.yaml`: point containerd at an internal registry or mirror path
- `offline-verify-images.yaml`: verify required images exist in the local runtime
- `vagrant-smoke-install.yaml`: Vagrant-oriented smoke workflow

For walkthrough-oriented context, start with [Quick Start](../quick-start.md) and [Offline Kubernetes Tutorial](../offline-kubernetes.md).

## Validation

Use `deck lint` for schema-level checks:

```bash
deck lint --file docs/guides/examples/offline-k8s-control-plane.yaml
```

`cases.tsv` is the lightweight example index used by repository maintainers and CI.
