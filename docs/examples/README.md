---
sidebar_label: "Examples"
---

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

## offline-kubernetes/ workspace

`offline-kubernetes/` is a complete multi-node workspace that shows the full offline Kubernetes lifecycle. It is the primary example of how to structure a real air-gap deployment with deck.

### Layout

```
offline-kubernetes/
  workflows/
    vars.yaml                       # shared variables (versions, IPs, paths)
    prepare.yaml                    # prepare workflow: packages, binaries, images
    scenarios/
      bootstrap.yaml                # control-plane bootstrap
      join.yaml                     # worker join
      reset.yaml                    # node reset
    components/                     # reusable phase fragments imported by scenarios
      bootstrap/
        announce-join.yaml          # encrypted join block output
        kubeadm.yaml
      host-prereqs.yaml
      join/
        input-join.yaml             # operator pastes ciphertext + passphrase
        join-node.yaml
      kube/user-access.yaml
      node/verify-selection.yaml
      repo/offline-repo-debian.yaml
      reset/
        artifacts.yaml
        cri.yaml
        kubeadm.yaml
        network.yaml
        scope-notice.yaml           # reset scope documentation step
      runtime/
        containerd.yaml
        k8s-binaries.yaml
        kubelet.yaml
        packages-debian.yaml
        registry-mirror.yaml
      verify/
        bootstrap-cluster.yaml
        node.yaml
```

### Install flow

1. **Prepare**: in the connected environment, run `deck prepare` against `prepare.yaml`. This downloads Debian packages, Kubernetes binaries (kubelet, kubeadm, kubectl, containerd, runc, CNI plugins), kubeadm images, and Calico CNI images into `outputs/`.
2. **Build**: `deck bundle build` packages everything into a portable `bundle.tar`.
3. **Transfer**: move `bundle.tar` through the approved path into the air-gapped site.
4. **Bootstrap**: on the control-plane node (e.g. `cp-1`, `192.0.2.10`), unpack the bundle and run:
   ```bash
   ./deck apply scenarios/bootstrap.yaml
   ```
   During bootstrap, the operator is prompted for a passphrase. The kubeadm join command is then printed to the log as an **encrypted block** only, no plaintext join file is served. Copy the text between `BEGIN ENCRYPTED JOIN` and `END ENCRYPTED JOIN`.
5. **Join**: on each worker node, run:
   ```bash
   ./deck apply scenarios/join.yaml --server 192.0.2.10:5000
   ```
   Paste the ciphertext as a single line when prompted, then enter the passphrase chosen at bootstrap. The worker decrypts the join command locally and never writes it to logs or apply state.
6. **Verify**: both scenarios include a verify phase that checks cluster and node readiness.

### Encrypted-join pattern {#encrypted-join-pattern}

Bootstrap prints the join command only in encrypted form via `openssl enc -aes-256-cbc -pbkdf2`. The passphrase is entered interactively by the bootstrap operator and is never stored. The worker operator pastes the ciphertext (`Input` step) and enters the passphrase (`Input`, secret) separately. Decryption happens locally on the worker and the plaintext is written to a temp file used by `JoinKubeadm`, then removed. No plaintext join credentials traverse the network.

### Calico CNI

Calico images (`quay.io/calico/*`, `quay.io/tigera/operator`) are downloaded by `prepare.yaml` and served from the bundle's image store. The CNI manifest (Tigera operator or `calico.yaml`) is applied **out-of-band** after bootstrap, typically with `kubectl apply -f` from a connected terminal or by placing the manifest in the bundle and running it manually.

### Reset scope

`scenarios/reset.yaml` returns a node to a **re-joinable** state. It runs `kubeadm reset`, stops the CRI, clears network state and artifacts. It does **not**:

- Remove the Node object from the cluster API. To drain and delete the node, run on a control-plane node:
  ```bash
  kubectl drain <node> --ignore-daemonsets --delete-emptydir-data
  kubectl delete node <node>
  ```
- Uninstall the container runtime or Kubernetes binaries (kept for fast rejoin).
- Re-enable swap, remove the offline repo, or uninstall OS packages.

See `offline-kubernetes/workflows/components/reset/scope-notice.yaml` for the in-workflow operator notice.

### Validation

```bash
deck lint --root docs/examples/offline-kubernetes
```

For walkthrough-oriented context, start with [Quick Start](../quick-start.md) and [Offline Kubernetes Tutorial](../offline-kubernetes.md).
