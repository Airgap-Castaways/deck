#!/usr/bin/env bash

upgrade_apply_workflow() {
  local workflow_url="${SERVER_URL}/workflows/scenarios/upgrade.yaml"
  local server_no_scheme="${SERVER_URL#http://}"
  server_no_scheme="${server_no_scheme#https://}"
  if [[ -z "${KUBERNETES_UPGRADE_VERSION}" ]]; then
    echo "[deck] missing upgrade target version for upgrade scenario"
    exit 1
  fi
  sudo -n "${DECK_BIN}" apply --workflow "${workflow_url}" \
    --var "serverURL=${server_no_scheme}" \
    --var "registryHost=${server_no_scheme}" \
    --var "release=${OFFLINE_RELEASE_CONTROL_PLANE}" \
    --var "kubernetesVersion=${KUBERNETES_VERSION}" \
    --var "upgradeKubernetesVersion=${KUBERNETES_UPGRADE_VERSION}" > "${CASE_DIR}/04-upgrade-control-plane.log" 2>&1
}

upgrade_prepare() {
  apply_offline_guard
}

upgrade_apply() {
  if [[ "${ROLE}" != "control-plane" ]]; then
    echo "[deck] unsupported role for upgrade scenario: ${ROLE}"
    exit 1
  fi
  upgrade_apply_workflow
}

upgrade_verify() {
  local stage="$1"
  case "${stage}" in
    upgrade|cluster)
      mkdir -p "${REPORT_DIR}"
      sudo -n cp /tmp/deck/reports/upgrade-version.txt "${REPORT_DIR}/upgrade-version.txt"
      sudo -n cp /tmp/deck/reports/upgrade-nodes.txt "${REPORT_DIR}/upgrade-nodes.txt"
      sudo -n cp /tmp/deck/reports/cluster-nodes.txt "${REPORT_DIR}/cluster-nodes.txt"
      sudo -n cp /tmp/deck/reports/kube-system-pods.txt "${REPORT_DIR}/kube-system-pods.txt"
      finalize_result_contract
      ;;
    *)
      echo "[deck] unsupported verify stage for upgrade scenario: ${stage}"
      exit 1
      ;;
  esac
}
