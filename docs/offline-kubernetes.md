---
sidebar_label: "Tutorial: Offline Kubernetes"
---

# Offline Kubernetes Tutorial

This tutorial shows how `deck` fits a Kubernetes maintenance session in the environment it is built for: no internet, no SSH-driven orchestration, and a local operator on each node.

Kubernetes host-prep and bootstrap procedures grow quickly. Raw shell becomes hard to review at that scale. `deck` keeps the procedure structured, validates it before transport, and bundles everything the nodes need into a single archive.

## Goal

Build a portable bundle in the connected environment, move it into the air gap, then run Kubernetes-oriented workflows locally on each target node.

## Workspace

The reference workspace for this tutorial is `examples/offline-kubernetes/`. It includes a prepare workflow, scenario workflows for bootstrap, join, and reset, and reusable component fragments that the scenarios import.

```
examples/offline-kubernetes/
  workflows/
    vars.yaml          # shared site variables
    prepare.yaml       # gather packages, binaries, images
    scenarios/
      bootstrap.yaml   # control-plane bootstrap
      join.yaml        # worker join
      reset.yaml       # node reset
    components/        # imported by scenarios
```

Adapt `vars.yaml` to your site. The defaults use RFC 5737 TEST-NET addresses (`192.0.2.0/24`); replace them with real IPs before use.

## 1. Prepare artifacts in the connected environment

Run the prepare workflow to download everything the nodes need:

```bash
deck prepare
```

This executes `workflows/prepare.yaml`, which downloads:

- Debian packages (conntrack, socat, iptables, and other host prerequisites)
- Kubernetes binaries: `kubelet`, `kubeadm`, `kubectl`, `crictl`, `containerd`, `runc`, CNI plugins
- kubeadm container images: `kube-apiserver`, `kube-controller-manager`, `kube-scheduler`, `kube-proxy`, `etcd`, `coredns`, `pause`
- Calico CNI images (`quay.io/calico/*`, `quay.io/tigera/operator`)

Override per-site values without editing shared files:

```bash
deck prepare -f vars/lab.yaml --var kubernetesVersion=v1.30.1 --var cluster.controlPlaneEndpoint=10.0.1.10:6443
```

## 2. Build the bundle

Package all prepared artifacts into a portable archive:

```bash
deck bundle build --out ./bundle.tar
```

The bundle includes `outputs/packages/`, `outputs/images/`, `outputs/files/`, `outputs/bin/`, the `workflows/` tree, the root `deck` launcher, and `.deck/manifest.json` checksums.

## 3. Transfer the bundle into the air-gapped site

Move `bundle.tar` through the approved path for your environment. Common mechanisms include USB or removable media handed through a physical checkpoint, a secure gateway or file-drop service that bridges the boundary, or rsync over a controlled jump host with an outbound-only transfer rule. Whichever path you use, **verify the bundle on the target side before running apply**: a truncated or corrupted transfer will produce an integrity failure mid-run rather than before it starts:

```bash
deck bundle verify --file ./bundle.tar
```

If verification fails (error code `E_BUNDLE_INTEGRITY`), re-transfer the archive and verify again before proceeding. See [diagnostics/error-codes.md](diagnostics/error-codes.md) for error details. Only after verification passes should you unpack and run `deck apply`.

## 4. Bootstrap the control-plane node

On the control-plane node (e.g. `cp-1` at `192.0.2.10`), unpack the bundle and run the bootstrap scenario:

```bash
tar -xf bundle.tar
cd bundle
./deck apply scenarios/bootstrap.yaml
```

The bootstrap scenario runs these phases in order:

1. **host-prereqs**: verifies node selection, applies offline Debian repo, installs OS packages
2. **runtime**: installs Kubernetes binaries, containerd, and kubelet
3. **image-source**: configures the containerd registry mirror pointing at the bundle server
4. **bootstrap**: runs `kubeadm init`, prints the encrypted join block, configures kubeconfig
5. **verify**: checks cluster readiness

### Encrypted join block

During the `bootstrap` phase, the operator is prompted for a passphrase. The kubeadm join command is then printed to the log **only in encrypted form**:

```
----- BEGIN ENCRYPTED JOIN -----
<base64-encoded ciphertext>
----- END ENCRYPTED JOIN -----
```

Copy the text between the `BEGIN` and `END` markers (a single line of base64). No plaintext join credentials are written to logs, apply state, or any served file.

### Calico CNI

Calico images are prepared and available in the bundle's image store. The CNI manifest (Tigera operator or `calico.yaml`) is applied **out-of-band** after bootstrap: for example, by placing the manifest in the bundle and running `kubectl apply -f` from a terminal with cluster access once `kubeconfig` is available.

## 5. Join worker nodes

On each worker node (e.g. `worker-1`), unpack the bundle and run the join scenario:

```bash
tar -xf bundle.tar
cd bundle
./deck apply scenarios/join.yaml --server 192.0.2.10:5000
```

The join scenario prompts the operator for two inputs:

1. **Ciphertext**: paste the encrypted join block as a single line (no line breaks).
2. **Passphrase**: the passphrase chosen at bootstrap; entered as a secret (not echoed).

The worker decrypts the join command locally and writes it to a temp file used by `JoinKubeadm`. The plaintext never traverses the network and is not stored in apply state or logs.

After the join phase the verify phase checks that the node has joined the cluster.

## 6. Validate before transport and execution

```bash
deck lint --root docs/examples/offline-kubernetes
```

For individual scenarios:

```bash
deck lint --workflow docs/examples/offline-kubernetes/workflows/scenarios/bootstrap.yaml
```

## 7. Site server (optional)

Some sites benefit from a temporary shared bundle source inside the air gap so that several nodes can fetch the same release without individual transfers:

```bash
deck server up --root ./bundle --addr :5000
deck server up --root ./bundle --addr :5000 --daemon --unit deck-server
```

The vars default (`server.url: 192.0.2.10:5000`) matches this pattern. Keep this choice explicit and secondary; the core workflow centers on local `deck` execution on each node.

## 8. Reset a node

To return a node to a re-joinable state, run the reset scenario:

```bash
./deck apply scenarios/reset.yaml
```

The reset scenario runs `kubeadm reset`, stops the CRI, clears network state, and removes deck artifacts. It ends with an operator-facing scope notice.

### Reset scope

Reset does **not**:

- Remove the Node object from the cluster API. To drain and delete the node, run on a control-plane node:
  ```bash
  kubectl drain <node> --ignore-daemonsets --delete-emptydir-data
  kubectl delete node <node>
  ```
- Uninstall the container runtime or Kubernetes binaries. These are kept so the node can rejoin quickly. Full teardown is manual:
  ```bash
  systemctl disable --now containerd-k8s
  rm -rf /opt/containerd-k8s /data/containerd-k8s /etc/containerd-k8s
  ```
- Re-enable swap, remove the offline repo, or uninstall OS packages.

The in-workflow reset scope notice is in `examples/offline-kubernetes/workflows/components/reset/scope-notice.yaml`.

## Step kinds used in this workspace

Useful reference entries for the step types this workspace relies on:

- [DownloadPackage](step-kinds/download-package.md)
- [DownloadImage](step-kinds/download-image.md)
- [DownloadFile](step-kinds/download-file.md)
- [CheckHost](step-kinds/check-host.md)
- [InstallPackage](step-kinds/install-package.md)
- [WriteContainerdConfig](step-kinds/write-containerd-config.md)
- [ManageService](step-kinds/manage-service.md)
- [InitKubeadm](step-kinds/init-kubeadm.md)
- [WaitForService](step-kinds/wait-for-service.md)

For shared step fields (`when`, `parallelGroup`, `register`, `metadata`, `retry`, `timeout`), see the [Step Envelope Contract](workflow-model.md#step-envelope-contract).

For planning and diagnostics, also review:

- [Workflow model](workflow-model.md)
- [Step Kinds](step-kinds.md)
- [Workspace Layout](workspace-layout.md)
- [Server audit log](server-audit-log.md)
- [CLI Reference](cli.md)
