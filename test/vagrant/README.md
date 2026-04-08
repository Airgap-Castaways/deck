# Vagrant Scenario Runner

This directory contains the host-side assets used to run libvirt-based Vagrant regression tests on Linux hosts. The current maintenance entrypoints are `test/workflows/*` scenarios and `test/e2e/vagrant/run-scenario.sh`.

## Files

- `Vagrantfile`: defines the fixed three-node lab (`control-plane`, `worker`, `worker-2`) on `192.168.57.10-12`
- `build-deck-binaries.sh`: builds the test `deck` binary on the host
- `libvirt-env.sh`: prepares the libvirt pool/network and Vagrant plugin/home state

Canonical scenario execution helpers live under `test/e2e/vagrant/`.

## Prerequisites

- Linux host
- `vagrant`, `virsh`, and libvirt installed
- `vagrant-libvirt` plugin available

## Basic Usage

```bash
bash test/e2e/vagrant/run-scenario.sh --scenario k8s-control-plane-bootstrap
bash test/e2e/vagrant/run-scenario.sh --scenario k8s-worker-join
bash test/e2e/vagrant/run-scenario.sh --scenario k8s-node-reset
```

The default behavior is tuned for repeated local runs.

- Default shared folder mode: `rsync`
- If needed, override with `DECK_VAGRANT_SYNC_TYPE=9p` or `DECK_VAGRANT_SYNC_TYPE=nfs`
- Direct `vagrant up ...` also prepares the default minimal role-specific rsync tree automatically
- Before direct runs, stale `.vagrant` libvirt machine metadata for a different VM prefix is cleaned automatically
- In `rsync` mode, only the role-specific minimal execution tree is synced instead of the full repository
- The control-plane receives the prepared bundle tarball and guest helpers; workers receive guest helpers only
- NFS is fixed to `nfs_version: 4` and `nfs_udp: false`
- Default artifact path: `test/artifacts/runs/<scenario>/<run-id>/`
- Default VM prefix: `deck-<scenario>-local`
- Default cleanup behavior: keep VMs
- A repeated run restarts from the canonical `prepare-bundle` step when a prior local run already exists

Common maintenance commands:

- Bootstrap only: `bash test/e2e/vagrant/run-scenario.sh --scenario k8s-control-plane-bootstrap`
- Worker join: `bash test/e2e/vagrant/run-scenario.sh --scenario k8s-worker-join`
- Node reset: `bash test/e2e/vagrant/run-scenario.sh --scenario k8s-node-reset`
- Run a single step: `bash test/e2e/vagrant/run-scenario.sh --scenario k8s-worker-join --step up-vms`
- Resume from an existing run: `bash test/e2e/vagrant/run-scenario.sh --scenario k8s-worker-join --resume --art-dir test/artifacts/runs/k8s-worker-join/local`
- Start fresh: `bash test/e2e/vagrant/run-scenario.sh --scenario k8s-worker-join --fresh`
- Skip artifact fetch: `bash test/e2e/vagrant/run-scenario.sh --scenario k8s-worker-join --skip-collect`
- Clean up VMs at the end: `bash test/e2e/vagrant/run-scenario.sh --scenario k8s-worker-join --cleanup`

## Artifact Paths

- `test/artifacts/runs/<scenario>/<run-id>/`
- `test/artifacts/cache/bundles/shared/<cache-key>/...`
- `test/artifacts/cache/staging/shared/<cache-key>/...`
- `test/artifacts/cache/vagrant/shared/<cache-key>/...`
- `test/vagrant/.vagrant/`

Important outputs:

- `checkpoints/<step>.done`
- `error-<step>.log`
- `reports/cluster-nodes.txt`
- `result.json`
- `pass.txt`
- The shared prepared bundle cache lives under `test/artifacts/cache/bundles/shared/<cache-key>/...`, not under each run directory
- Host-side bundle staging uses `test/artifacts/cache/staging/shared/<cache-key>/...`
- In `rsync` mode, the control-plane tarball payload and worker helper-only payload are staged separately under `test/artifacts/cache/vagrant/shared/<cache-key>/...` before syncing to `/workspace`
- Vagrant machine state is kept under `test/vagrant/.vagrant/`
- When `nfs` or `9p` makes result files immediately visible on the host, collect performs validation without fetching

## Execution Model

This document is for maintaining the Vagrant regression environment. The product-facing local workflow remains the documented `plan -> doctor -> apply` path.

- The internal regression flow is: host preparation, VM startup, scenario execution, verification collection, and optional cleanup
- Scenario entrypoint workflows live under `test/workflows/scenarios/*.yaml`
- Shared fragments live under `test/workflows/components/`, and shared defaults live in `test/workflows/vars.yaml`
- Scenario topology and ordered role-level workflow orchestration are described under `test/e2e/scenarios/*.json`
- Repeated local runs reuse the same artifact path and VM prefix by default
- To narrow the scope of a rerun, use `--from-step`, `--to-step`, `--resume`, or `--art-dir`
- Changing `--art-dir` does not change the shared prepared bundle cache path
- To fully reset local state, remove `test/vagrant/.vagrant`, the relevant `test/artifacts/runs/...` directory, and the shared bundle/staging/vagrant cache trees before re-running
- Direct `vagrant up control-plane worker worker-2 --provider libvirt` automatically uses `test/vagrant/prepare-minimal-rsync.sh` to prepare the shared cache and role-specific rsync sources

## Maintenance Notes

- Treat `test/e2e/vagrant/run-scenario.sh` and `test/workflows/*` as the main maintenance surface for this test environment

## Periodic CI

- The scheduled workflow is `.github/workflows/vagrant-periodic.yml`
- The nightly default scenario set is `k8s-control-plane-bootstrap` and `k8s-worker-join`
- Manual `workflow_dispatch` can run the `full` set, including `k8s-node-reset` and `k8s-upgrade`
- The runner must carry all of the following labels: `self-hosted`, `linux`, `vagrant`, `libvirt`
- Each scenario job uploads `test/artifacts/runs/<scenario>/<run-id>/` and summarizes `result.json`, `run-summary.txt`, and `logs/error-*.log` in the step summary
