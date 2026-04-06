#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
OUTPUT_DIR="${ROOT_DIR}/test/artifacts/cache/vagrant/manual"
OUTPUT_ENV="${OUTPUT_DIR}/rsync-sources.env"

export ROOT_DIR
export DECK_VAGRANT_WORKFLOW_ROOT_REL="test/workflows"
export DECK_VAGRANT_VM_SCENARIO_SCRIPT="${ROOT_DIR}/test/e2e/vagrant/run-scenario-vm.sh"
export DECK_VAGRANT_VM_DISPATCHER_SCRIPT="${ROOT_DIR}/test/e2e/vagrant/run-scenario-vm.sh"
export DECK_VAGRANT_VM_STAGED_PATH="test/e2e/vagrant/run-scenario-vm.sh"
export DECK_VAGRANT_VM_DISPATCHER_STAGED_PATH="test/e2e/vagrant/run-scenario-vm.sh"

source "${ROOT_DIR}/test/e2e/vagrant/common.sh"
source "${ROOT_DIR}/test/vagrant/libvirt-env.sh"

deck_vagrant_prepare_workflow_bundle() {
  "${ROOT_DIR}/test/e2e/vagrant/render-workflows.sh" "${ROOT_DIR}" "${PREPARED_BUNDLE_WORKFLOW_DIR}"
}

refresh_layout_contracts
CHECKPOINT_DIR="${ART_DIR_ABS}/checkpoints"
RUN_LOG_DIR="${ART_DIR_ABS}/logs"
RUN_REPORT_DIR="${ART_DIR_ABS}/reports"
RUN_RENDERED_WORKFLOWS_DIR="${ART_DIR_ABS}/rendered-workflows"
RUN_BUNDLE_SOURCE_FILE="${ART_DIR_ABS}/bundle-source.txt"
STATE_ENV_PATH="${CHECKPOINT_DIR}/state.env"
initialize_run_contract
prepare_local_run_state
prepare_libvirt_environment
"${BUILD_BINARIES_HELPER}" "${ROOT_DIR}"
resolve_host_build_context
prepare_shared_bundle_cache "${HOST_BIN}" "${HOST_BACKEND_RUNTIME}" "${HOST_ARCH}"
prepare_rsync_stage_roots

mkdir -p "${OUTPUT_DIR}"
cat >"${OUTPUT_ENV}" <<EOF
DECK_VAGRANT_SYNC_SOURCE_CONTROL_PLANE=${CONTROL_PLANE_RSYNC_STAGE_ABS}
DECK_VAGRANT_SYNC_SOURCE_WORKER=${WORKER_RSYNC_STAGE_ABS}
DECK_VAGRANT_SYNC_SOURCE_WORKER_2=${WORKER_RSYNC_STAGE_ABS}
DECK_VAGRANT_SHARED_CACHE_KEY=${CACHE_KEY}
EOF

printf '[deck] prepared minimal rsync sources in %s\n' "${OUTPUT_ENV}"
