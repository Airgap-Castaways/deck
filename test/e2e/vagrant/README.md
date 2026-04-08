# Vagrant canonical E2E runner

This directory contains the canonical local Vagrant regression harness.

- `run-scenario.sh`: host-side entrypoint for scenario runs.
- `common.sh`: shared host-side helpers and step implementation.
- `run-scenario-vm.sh`: guest-side dispatcher used by the host runner.
- `run-scenario-vm-scenario.sh`: guest-side generic helper for offline-guard setup.
- `scenario-manifest.py`: host-side manifest loader for VM/workflow orchestration metadata.
- `render-workflows.sh`: copies the canonical workflow tree into the prepared bundle workspace.

The maintained path is `test/e2e/vagrant/run-scenario.sh` with workflows under `test/workflows/*`.

The canonical lab uses the fixed private-network addresses from `test/vagrant/Vagrantfile`, with the control-plane server exposed to workflows at `192.168.57.10:18080`.
