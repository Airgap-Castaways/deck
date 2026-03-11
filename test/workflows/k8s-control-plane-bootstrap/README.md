# k8s-control-plane-bootstrap

This scenario boots a single control-plane node from the canonical workflow fragments under `test/workflows/_shared/k8s/` and the scenario steps in `apply/bootstrap.yaml`.

## Purpose

- Prepare a clean control-plane host for offline Kubernetes bootstrap.
- Run `kubeadm init` with the generated config and publish `/tmp/deck/join.txt` to the local deck server.
- Verify that the cluster reaches exactly one Ready control-plane node.

## Key inputs

- Profile: `profile/control-plane.yaml`
- Vars: `vars/control-plane.yaml`
- Shared imports for prereqs, offline repo, runtime deps, binaries, containerd, kubelet, and CNI
- Scenario vars such as `serverURL`, `registryHost`, and `kubernetesVersion`

## Key outputs and evidence

- `/tmp/deck/join.txt`, copied to `/tmp/deck/server-root/files/cluster/join.txt` for later worker joins
- `/tmp/deck/reports/bootstrap-nodes.txt` from the in-workflow readiness check
- Scenario runner evidence such as `join.txt` and `cluster-nodes.txt` when the Vagrant acceptance path collects artifacts

## Not covered

- Joining worker nodes
- Multi-node cluster convergence checks
- Node reset or rejoin behavior
