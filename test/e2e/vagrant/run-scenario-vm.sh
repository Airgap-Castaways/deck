#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: test/e2e/vagrant/run-scenario-vm.sh <role> <action> [stage]

Actions:
  prepare-bundle   control-plane only; prepare bundle, render workflows, start server, apply offline guard
  run-workflow     run a manifest-selected workflow
  collect          ensure artifact directory exists
  cleanup          stop server and offline guard, restore ownership

Roles:
  control-plane | worker | worker-2

Examples:
  bash test/e2e/vagrant/run-scenario-vm.sh control-plane prepare-bundle
  bash test/e2e/vagrant/run-scenario-vm.sh control-plane run-workflow
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

ROLE="${1:?role required}"
ACTION="${2:?action required}"
SCENARIO_STAGE="${3:-}"
ART_DIR_REL="${ART_DIR_REL:?ART_DIR_REL is required}"
ART_DIR="/workspace/${ART_DIR_REL}"
CASE_DIR="${ART_DIR}/cases"
REPORT_DIR="${ART_DIR}/reports"
RENDERED_WORKFLOWS_DIR="${ART_DIR}/rendered-workflows"
SERVER_URL="${SERVER_URL:?SERVER_URL is required}"
SERVER_BIND_ADDR="${DECK_SERVER_BIND_ADDR:-0.0.0.0:18080}"
SERVER_ENDPOINT="${SERVER_URL#http://}"
SERVER_ENDPOINT="${SERVER_ENDPOINT#https://}"
SERVER_HOST="${SERVER_ENDPOINT%%:*}"
KUBEADM_ADVERTISE_ADDRESS="${DECK_KUBEADM_ADVERTISE_ADDRESS:-${SERVER_HOST}}"
PREPARED_BUNDLE_REL="${DECK_PREPARED_BUNDLE_REL:-}"
PREPARED_BUNDLE_TAR_REL="${DECK_PREPARED_BUNDLE_TAR_REL:-}"
E2E_SCENARIO="${DECK_E2E_SCENARIO:-k8s-worker-join}"
E2E_RUN_ID="${DECK_E2E_RUN_ID:-local}"
E2E_PROVIDER="${DECK_E2E_PROVIDER:-libvirt}"
E2E_CACHE_KEY="${DECK_E2E_CACHE_KEY:-compat}"
E2E_STARTED_AT="${DECK_E2E_STARTED_AT:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
SERVER_ROOT="/tmp/deck/server-root"
DECK_BIN="/tmp/deck/deck"
DECK_BIN_STAMP_FILE="/tmp/deck/deck.cache-key"
SERVER_PID=""
REPO_TYPE="apt-flat"
OFFLINE_GUARD_ACTIVE=0
KEEP_PROCESSES=0
SERVER_PID_FILE="/tmp/deck/offline-server.pid"
CONTROL_PLANE_WORKFLOW_URL="${SERVER_URL}/workflows/scenarios/control-plane-bootstrap.yaml"
SCENARIO_HELPERS="/workspace/test/e2e/vagrant/run-scenario-vm-scenario.sh"
WORKFLOW_REL="${DECK_E2E_WORKFLOW_REL:-}"
ACTION_NAME="${DECK_E2E_ACTION_NAME:-${ACTION}}"

if [[ ! -f "${SCENARIO_HELPERS}" ]]; then
  echo "[deck] missing scenario helper script: ${SCENARIO_HELPERS}"
  exit 1
fi
source "${SCENARIO_HELPERS}"

if [[ "${ROLE}" == "control-plane" && "${ACTION}" == "prepare-bundle" ]]; then
  rm -rf "${ART_DIR}"
fi

mkdir -p "${ART_DIR}" "${CASE_DIR}" "${REPORT_DIR}" "${RENDERED_WORKFLOWS_DIR}" /tmp/deck
mkdir -p "${SERVER_ROOT}"

ARCH_RAW="$(uname -m)"
ARCH=""
case "${ARCH_RAW}" in
  x86_64)
    ARCH="amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    ;;
  *)
    echo "[deck] unsupported VM architecture: ${ARCH_RAW}"
    exit 1
    ;;
esac

cd /workspace

if [[ -d /etc/yum.repos.d ]]; then
  REPO_TYPE="yum"
fi

cleanup() {
  set +e
  if [[ "${KEEP_PROCESSES}" == "1" ]]; then
    chown -R vagrant:vagrant "${ART_DIR}" /tmp/deck >/dev/null 2>&1 || true
    set -e
    return
  fi
  if [[ -n "${SERVER_PID}" ]]; then
    sudo -n kill "${SERVER_PID}" >/dev/null 2>&1 || true
    SERVER_PID=""
  fi
  if [[ -f "${SERVER_PID_FILE}" ]]; then
    sudo -n kill "$(cat "${SERVER_PID_FILE}")" >/dev/null 2>&1 || true
    rm -f "${SERVER_PID_FILE}"
  fi
  if [[ "${OFFLINE_GUARD_ACTIVE}" == "1" ]]; then
    sudo -n iptables -D OUTPUT -j DECK_OFFLINE_GUARD >/dev/null 2>&1 || true
    sudo -n iptables -F DECK_OFFLINE_GUARD >/dev/null 2>&1 || true
    sudo -n iptables -X DECK_OFFLINE_GUARD >/dev/null 2>&1 || true
    if command -v ip6tables >/dev/null 2>&1; then
      sudo -n ip6tables -D OUTPUT -j DECK_OFFLINE_GUARD6 >/dev/null 2>&1 || true
      sudo -n ip6tables -F DECK_OFFLINE_GUARD6 >/dev/null 2>&1 || true
      sudo -n ip6tables -X DECK_OFFLINE_GUARD6 >/dev/null 2>&1 || true
    fi
  fi
  chown -R vagrant:vagrant "${ART_DIR}" /tmp/deck >/dev/null 2>&1 || true
  set -e
}

trap cleanup EXIT INT TERM

wait_server_health() {
  local i
  for ((i=1; i<=180; i++)); do
    if curl -fsS --max-time 5 "${SERVER_URL}/healthz" > "${ART_DIR}/server-health.json" 2>/dev/null; then
      return 0
    fi
    sleep 2
  done
  return 1
}

detect_local_ipv4() {
  local candidate=""
  candidate="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i=1; i<=NF; i++) if ($i=="src") {print $(i+1); exit}}')"
  if [[ -n "${candidate}" ]]; then
    printf '%s\n' "${candidate}"
    return 0
  fi
  candidate="$(ip -4 -o addr show scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | head -n1)"
  if [[ -n "${candidate}" ]]; then
    printf '%s\n' "${candidate}"
    return 0
  fi
  return 1
}

ensure_advertise_address() {
  if ip -4 -o addr show scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | grep -Fx "${KUBEADM_ADVERTISE_ADDRESS}" >/dev/null 2>&1; then
    return 0
  fi
  local detected=""
  detected="$(detect_local_ipv4 || true)"
  if [[ -n "${detected}" ]]; then
    KUBEADM_ADVERTISE_ADDRESS="${detected}"
  fi
}

control_plane_bundle_deck_path() {
  printf '%s\n' "${SERVER_ROOT}/outputs/bin/linux/${ARCH}/deck"
}

download_worker_deck_binary() {
  local deck_url="${SERVER_URL}/bin/linux/${ARCH}/deck"
  local i
  for ((i=1; i<=180; i++)); do
    if curl -fsS --max-time 10 "${deck_url}" -o "${DECK_BIN}.tmp"; then
      chmod +x "${DECK_BIN}.tmp"
      mv "${DECK_BIN}.tmp" "${DECK_BIN}"
      return 0
    fi
    sleep 2
  done
  rm -f "${DECK_BIN}.tmp"
  echo "[deck] failed to download deck runtime binary from ${deck_url}"
  exit 1
}

ensure_deck_runtime_binary() {
  if [[ -x "${DECK_BIN}" ]] && [[ -f "${DECK_BIN_STAMP_FILE}" ]] && [[ "$(cat "${DECK_BIN_STAMP_FILE}" 2>/dev/null || true)" == "${E2E_CACHE_KEY}" ]]; then
    return 0
  fi
  if [[ "${ROLE}" == "control-plane" ]]; then
    local source_path
    source_path="$(control_plane_bundle_deck_path)"
    if [[ ! -f "${source_path}" ]]; then
      echo "[deck] missing control-plane runtime binary: ${source_path}"
      exit 1
    fi
    cp "${source_path}" "${DECK_BIN}"
    chmod +x "${DECK_BIN}"
    printf '%s\n' "${E2E_CACHE_KEY}" > "${DECK_BIN_STAMP_FILE}"
    return 0
  fi
  download_worker_deck_binary
  printf '%s\n' "${E2E_CACHE_KEY}" > "${DECK_BIN_STAMP_FILE}"
}

prepare_server_bundle() {
  if [[ -n "${PREPARED_BUNDLE_TAR_REL}" ]] && [[ -f "/workspace/${PREPARED_BUNDLE_TAR_REL}" ]]; then
    sudo -n rm -rf "${SERVER_ROOT}"
    mkdir -p "${SERVER_ROOT}"
    tar -xf "/workspace/${PREPARED_BUNDLE_TAR_REL}" -C "${SERVER_ROOT}" --strip-components=1
    printf 'prepared-bundle-tar=%s\n' "${PREPARED_BUNDLE_TAR_REL}" > "${CASE_DIR}/01-prepare.log"
    return 0
  fi

  echo "[deck] prepared bundle tar missing; host step prepare-host must publish DECK_PREPARED_BUNDLE_TAR_REL" | tee -a "${CASE_DIR}/01-prepare.log"
  exit 1
}

write_runtime_workflows() {
  local workflow_dir="${SERVER_ROOT}/workflows"
  ensure_advertise_address
  if [[ ! -d "${workflow_dir}" ]]; then
    echo "[deck] missing rendered workflows in extracted bundle: ${workflow_dir}"
    exit 1
  fi
  CONTROL_PLANE_WORKFLOW_URL="${SERVER_URL}/workflows/scenarios/control-plane-bootstrap.yaml"
  rm -rf "${RENDERED_WORKFLOWS_DIR}"
  mkdir -p "${RENDERED_WORKFLOWS_DIR}"
  cp -a "${workflow_dir}/." "${RENDERED_WORKFLOWS_DIR}/"
}

start_server_background() {
  if [[ -f "${SERVER_PID_FILE}" ]]; then
    local existing
    existing="$(cat "${SERVER_PID_FILE}")"
    if sudo -n kill -0 "${existing}" >/dev/null 2>&1; then
      SERVER_PID="${existing}"
      return 0
    fi
  fi
  echo "[deck] starting server ${SERVER_BIND_ADDR}"
  rm -f "${SERVER_PID_FILE}"
  sudo -n bash -c "nohup \"${DECK_BIN}\" server up --root \"${SERVER_ROOT}\" --addr \"${SERVER_BIND_ADDR}\" > \"${CASE_DIR}/02-server.log\" 2>&1 < /dev/null & echo \$! > \"${SERVER_PID_FILE}\""
  SERVER_PID="$(cat "${SERVER_PID_FILE}")"
  if ! sudo -n kill -0 "${SERVER_PID}" >/dev/null 2>&1; then
    echo "[deck] server failed to stay running after start"
    exit 1
  fi
}

clear_install_state() {
  sudo -n rm -f /root/.deck/state/*.json /root/.local/state/deck/state/*.json
}

require_control_plane() {
  if [[ "${ROLE}" != "control-plane" ]]; then
    echo "[deck] unsupported role/action: role=${ROLE} action=${ACTION}"
    exit 1
  fi
}

action_prepare_bundle() {
  require_control_plane
  prepare_server_bundle
  ensure_deck_runtime_binary
  write_runtime_workflows
  start_server_background
  if ! wait_server_health; then
    echo "[deck] server health check failed" | tee "${CASE_DIR}/06-assertions.log"
    exit 1
  fi
  if ! apply_offline_guard; then
    echo "[deck] offline guard setup failed" | tee -a "${CASE_DIR}/06-assertions.log"
    exit 1
  fi
}

run_workflow_action() {
  local workflow_rel="${WORKFLOW_REL:?DECK_E2E_WORKFLOW_REL is required}"
  local workflow_url="${SERVER_URL}/workflows/${workflow_rel}"
  local log_path="${CASE_DIR}/${ACTION_NAME}.log"
  local -a apply_args=(apply --workflow "${workflow_url}" --fresh)

  clear_install_state
  ensure_deck_runtime_binary
  sudo -n "${DECK_BIN}" "${apply_args[@]}" > "${log_path}" 2>&1
  mkdir -p "${REPORT_DIR}"
  sudo -n cp -a /tmp/deck/reports/. "${REPORT_DIR}/" >/dev/null 2>&1 || true
}

if [[ "${ACTION}" != "cleanup" ]]; then
  KEEP_PROCESSES=1
fi

case "${ACTION}" in
  prepare-bundle)
    action_prepare_bundle
    ;;
  run-workflow|apply-scenario|verify-scenario)
    run_workflow_action
    ;;
  collect)
    mkdir -p "${ART_DIR}"
    ;;
  cleanup)
    KEEP_PROCESSES=0
    cleanup
    exit 0
    ;;
  *)
    echo "[deck] unsupported role/action: role=${ROLE} action=${ACTION}"
    exit 1
    ;;
esac
