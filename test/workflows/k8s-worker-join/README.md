# k8s-worker-join

This scenario installs the shared Kubernetes prerequisites on a worker, downloads the bootstrap-generated join command, and runs `kubeadm join`.

## Purpose

- Reuse the shared prereq, repo, runtime, binary, containerd, kubelet, and CNI fragments for a worker node.
- Fetch `join.txt` from the local deck server.
- Join the worker to the existing control-plane with `KubeadmJoin`.

## Key inputs

- Profile: `profile/worker.yaml`
- Vars: `vars/worker.yaml`
- Scenario vars such as `serverURL`, `osFamily`, `release`, and `joinFile`
- Bootstrap-published join file at `http://<serverURL>/files/cluster/join.txt`

## Key outputs and evidence

- Local join command file at the configured `joinFile`, default `/tmp/deck/join.txt`
- Joined worker state proven by cluster-level evidence such as `cluster-nodes.txt`
- Scenario runner artifact copy of `join.txt` when the acceptance path collects reports

## Not covered

- Control-plane bootstrap
- Resetting or rejoining an already joined node
- Any readiness check beyond the downstream cluster convergence evidence
