# Example Workflows

The files in `docs/examples/` are examples for the default `deck` model: prepare the bundle outside the site, then execute the workflow locally during the maintenance session.

You can also adapt them for site-assisted use inside the air gap, but that is a deliberate extension of the same local execution path.

## When to use these examples

- Start from them when you want a concrete local workflow to review and adapt.
- Use them to replace repetitive shell snippets with clearer typed steps where possible.
- Carry them into a bundle and run them on the target host or node.

## When not to use these examples

- Don't treat them as remote orchestration playbooks.
- Don't assume a shared server is required before they are useful.
- Don't use `RunCommand` first if a more specific step kind can express the change.

## Files

- `offline-k8s-control-plane.yaml`: kubeadm-based control-plane bootstrap example
- `offline-k8s-worker.yaml`: kubeadm worker join example
- `offline-repo-preinstall.yaml`: prepare package repository configuration on the target host
- `offline-containerd-mirror.yaml`: point containerd at an internal registry or mirror path
- `offline-verify-images.yaml`: verify required images exist in the local runtime
- `vagrant-smoke-install.yaml`: Vagrant-oriented smoke workflow

## Validation

Use `deck validate` for schema-level checks:

```bash
deck validate --file docs/examples/offline-k8s-control-plane.yaml
```

`cases.tsv` remains the lightweight example index used by repository maintainers.
