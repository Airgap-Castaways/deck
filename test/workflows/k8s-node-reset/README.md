# k8s-node-reset

This scenario performs the worker reset portion used by the node-reset acceptance flow and writes a reset-state report for later assertions.

## Purpose

- Run `kubeadm reset -f` only when `allowDestructive` is set to `"true"`.
- Remove kubelet and control-plane leftovers that should not survive a reset on the target node.
- Confirm containerd is back, static pod manifests are gone, and record reset-state evidence.

## Key inputs

- Profile: `profile/node-reset.yaml`
- Vars: `vars/node-reset.yaml`
- Scenario vars such as `allowDestructive`, `resetReason`, and `resetStatePath`

## Key outputs and evidence

- Reset-state report at the configured `resetStatePath`, default `reports/reset-state.txt` in the Vagrant scenario runner
- Final reset proof is recorded in `reports/reset-state.txt` and must show service recovery markers including `containerd=active` and `kubeletService=active`
- Follow-up rejoin health is also captured separately by the runner in `reports/rejoin-kubelet.txt`, which records `kubeletServiceAfterRejoin=active`
- Downstream node-reset acceptance markers `worker-reset-done.txt`, `worker-rejoin-done.txt`, and `cluster-nodes.txt` from the scenario runner after the separate rejoin flow completes

## Not covered

- Fetching or generating a new `join.txt`
- Control-plane or second-worker apply paths
- Upgrade, reprovisioning, or etcd recovery work
