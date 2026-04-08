#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
source "${ROOT_DIR}/test/e2e/vagrant/common.sh"

CONTRACT_WORKFLOW_COMPONENTS_ROOT="test/workflows/components"
CONTRACT_WORKFLOW_SCENARIOS_ROOT="test/workflows/scenarios"
CONTRACT_E2E_VAGRANT_ROOT="test/e2e/vagrant"
DECK_VAGRANT_WORKFLOW_ROOT_REL="test/workflows"
DECK_VAGRANT_VM_SCENARIO_SCRIPT="${ROOT_DIR}/test/e2e/vagrant/run-scenario-vm.sh"
DECK_VAGRANT_VM_STAGED_PATH="test/e2e/vagrant/run-scenario-vm.sh"
DECK_VAGRANT_VM_DISPATCHER_SCRIPT="${ROOT_DIR}/test/e2e/vagrant/run-scenario-vm.sh"
DECK_VAGRANT_VM_DISPATCHER_STAGED_PATH="test/e2e/vagrant/run-scenario-vm.sh"

STEPS=(
  prepare-host
  up-vms
  prepare-bundle
  apply-scenario
  verify-scenario
  collect
  cleanup
)

deck_vagrant_usage() {
  local entrypoint="${DECK_VAGRANT_ENTRYPOINT:-test/e2e/vagrant/run-scenario.sh}"
  cat <<EOF
Usage: ${entrypoint} [options]

Options:
  --scenario <name>    Override the scenario cache/artifact namespace.
  --step <name>       Run only one step.
  --from-step <name>  Start from step.
  --to-step <name>    End at step.
  --resume            Skip completed checkpoints and continue.
  --fresh             Recreate VMs and rerun from a clean local state.
  --fresh-cache       Remove run artifacts and scenario cache, then rerun.
  --art-dir <path>    Reuse artifact directory (absolute or workspace-relative).
  --skip-cleanup      Keep VMs after scenario for debugging.
  --cleanup           Destroy VMs at the end of the run.
  --skip-collect      Do not fetch artifacts back from VMs.

Steps:
  prepare-host, up-vms, prepare-bundle, apply-scenario,
  verify-scenario, collect, cleanup
EOF
}

deck_vagrant_prepare_workflow_bundle() {
  "${ROOT_DIR}/test/e2e/vagrant/render-workflows.sh" "${ROOT_DIR}" "${PREPARED_BUNDLE_WORKFLOW_DIR}"
}

guest_vm_action_command() {
  local role="$1"
  local action="$2"
  local stage="${3:-}"
  local cmd="set -euo pipefail; exec bash /workspace/${DECK_VAGRANT_VM_STAGED_PATH} ${role} ${action}"
  if [[ -n "${stage}" ]]; then
    cmd+=" ${stage}"
  fi
  printf '%s' "bash -lc '${cmd}'"
}

control_plane_action() {
  run_role_action "control-plane" "$@"
}

step_prepare_bundle() { control_plane_action "prepare-bundle"; }

manifest_actions() {
  local phase="$1"
  local stage="${2:-}"
  python3 "${SCENARIO_MANIFEST_HELPER}" "${ROOT_DIR}" "${SCENARIO_ID}" actions "${phase}" "${stage}"
}

decode_manifest_action() {
  local action_json="$1"
  python3 - <<'PY' "${action_json}"
import json
import sys

action = json.loads(sys.argv[1])
fields = [
    action["id"],
    action["role"],
    action["workflow"],
]
print("\t".join(fields))
PY
}

run_role_workflow_action() {
  local role="$1"
  local action_name="$2"
  local workflow_rel="$3"

  load_state_env

  echo "[deck] role=${role} workflow=${workflow_rel} action=${action_name} scenario=${SCENARIO_ID}"
  run_vagrant_ssh "${role}" "ART_DIR_REL=${ART_DIR_REL} SERVER_URL=${SERVER_URL} DECK_PREPARED_BUNDLE_REL=${PREPARED_BUNDLE_REL:-} DECK_PREPARED_BUNDLE_TAR_REL=${PREPARED_BUNDLE_TAR_REL:-} DECK_E2E_ACTION_NAME=${action_name} DECK_E2E_WORKFLOW_REL=${workflow_rel} DECK_E2E_SCENARIO=${SCENARIO_ID} DECK_E2E_RUN_ID=${RUN_ID} DECK_E2E_PROVIDER=${DECK_VAGRANT_PROVIDER} DECK_E2E_CACHE_KEY=${CACHE_KEY} DECK_E2E_STARTED_AT=${RUN_STARTED_AT} $(guest_vm_action_command "${role}" run-workflow)"
}

run_role_action() {
  local role="$1"
  local action="$2"
  local stage="${3:-}"
  load_state_env

  echo "[deck] role=${role} action=${action} scenario=${SCENARIO_ID}"
  run_vagrant_ssh "${role}" "ART_DIR_REL=${ART_DIR_REL} SERVER_URL=${SERVER_URL} DECK_PREPARED_BUNDLE_REL=${PREPARED_BUNDLE_REL:-} DECK_PREPARED_BUNDLE_TAR_REL=${PREPARED_BUNDLE_TAR_REL:-} DECK_E2E_SCENARIO=${SCENARIO_ID} DECK_E2E_RUN_ID=${RUN_ID} DECK_E2E_PROVIDER=${DECK_VAGRANT_PROVIDER} DECK_E2E_CACHE_KEY=${CACHE_KEY} DECK_E2E_STARTED_AT=${RUN_STARTED_AT} $(guest_vm_action_command "${role}" "${action}" "${stage}")"
}

step_apply_scenario() {
  local -a actions=()
  local action_json=""
  local decoded=""
  local role=""
  local action_name=""
  local workflow_rel=""

  mapfile -t actions < <(manifest_actions apply)
  for action_json in "${actions[@]}"; do
    [[ -n "${action_json}" ]] || continue
    decoded="$(decode_manifest_action "${action_json}")"
    IFS=$'\t' read -r action_name role workflow_rel <<< "${decoded}"
    run_role_workflow_action "${role}" "${action_name}" "${workflow_rel}"
  done
}

step_verify_scenario() {
  local stage=""
  local -a actions=()
  local action_json=""
  local decoded=""
  local role=""
  local action_name=""
  local workflow_rel=""

  stage="$(scenario_verify_stage_default || true)"
  mapfile -t actions < <(manifest_actions verify "${stage}")
  for action_json in "${actions[@]}"; do
    [[ -n "${action_json}" ]] || continue
    decoded="$(decode_manifest_action "${action_json}")"
    IFS=$'\t' read -r action_name role workflow_rel <<< "${decoded}"
    run_role_workflow_action "${role}" "${action_name}" "${workflow_rel}"
  done
}

deck_vagrant_main "$@"
