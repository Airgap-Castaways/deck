# Workflow test trees

`test/workflows/` is the canonical home for the scenario-owned workflow content used by the Kubernetes regression layout.

## Tree

- `_shared/`, reusable fragments shared across scenarios
- `k8s-control-plane-bootstrap/`, single-node control-plane bootstrap scenario
- `k8s-worker-join/`, worker join scenario that consumes the published join file
- `k8s-node-reset/`, node reset scenario used before rejoin proof

## Expected scenario shape

Each canonical scenario tree keeps scenario meaning in workflow files instead of the Vagrant harness:

- `profile/`, entrypoint workflow definitions passed to `deck validate` and the scenario runner
- `prepare/`, reserved for scenario-scoped prepare assets when a scenario needs them
- `apply/`, scenario execution steps and imports
- `vars/`, scenario defaults and shared variable inputs
- `README.md`, maintainer-facing notes about purpose, inputs, and evidence

`_shared/k8s/` follows the same layout for reusable Kubernetes fragments, including shared `prepare/`, `apply/`, and `vars/` content.
