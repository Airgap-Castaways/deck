## GitHub-hosted deck E2E

This directory holds GitHub-hosted end-to-end fixtures that are intended to run on standard GitHub-hosted runners.

- keep these scenarios narrower than the Vagrant regression harness under `test/e2e/vagrant/`
- prefer portable Linux/container-safe steps over virtualization, systemd, or kernel-level assumptions
- validate real `deck` workflow execution, bundle creation, and bundled `apply` behavior

The primary runner entrypoint is `.github/scripts/run-hosted-e2e.sh`, and the scheduled/manual workflow is `.github/workflows/hosted-e2e-nightly.yml`.
