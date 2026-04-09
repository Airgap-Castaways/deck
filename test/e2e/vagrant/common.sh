#!/usr/bin/env bash
set -euo pipefail

COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${ROOT_DIR:-$(cd "${COMMON_DIR}/../../.." && pwd)}"
VAGRANT_DIR="${ROOT_DIR}/test/vagrant"
LIBVIRT_ENV_HELPER="${ROOT_DIR}/test/vagrant/libvirt-env.sh"
BUILD_BINARIES_HELPER="${ROOT_DIR}/test/vagrant/build-deck-binaries.sh"
SCENARIO_MANIFEST_HELPER="${ROOT_DIR}/test/e2e/vagrant/scenario-manifest.py"

TS="$(date +%Y%m%d-%H%M%S)"
SCENARIO_ID="${DECK_VAGRANT_SCENARIO:-k8s-worker-join}"
RUN_ID="${DECK_VAGRANT_RUN_ID:-local}"
CACHE_KEY="${DECK_VAGRANT_CACHE_KEY:-compat}"
RUN_STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
SCENARIO_ID_SANITIZED="${SCENARIO_ID//\//-}"
RUN_ID_SANITIZED="${RUN_ID//\//-}"
ART_DIR_REL=""
ART_DIR_ABS=""
CHECKPOINT_DIR=""
RUN_LOG_DIR=""
RUN_REPORT_DIR=""
RUN_RENDERED_WORKFLOWS_DIR=""
RUN_BUNDLE_SOURCE_FILE=""
CACHE_BUNDLES_ROOT_REL=""
PREPARED_BUNDLE_REL=""
PREPARED_BUNDLE_ABS=""
PREPARED_BUNDLE_TAR_REL=""
PREPARED_BUNDLE_TAR_ABS=""
PREPARED_BUNDLE_STAMP=""
PREPARED_BUNDLE_WORK_REL=""
PREPARED_BUNDLE_WORK_ABS=""
PREPARED_BUNDLE_PACK_ROOT=""
PREPARED_BUNDLE_WORKFLOW_DIR=""
PREPARED_BUNDLE_FRAGMENT_DIR=""
PREPARED_BUNDLE_TAR=""
PREPARED_BUNDLE_STAGE_ABS=""
CONTROL_PLANE_RSYNC_STAGE_REL=""
CONTROL_PLANE_RSYNC_STAGE_ABS=""
CONTROL_PLANE_RSYNC_STAGE_STAGE_ABS=""
CONTROL_PLANE_RSYNC_STAGE_STAMP=""
WORKER_RSYNC_STAGE_REL=""
WORKER_RSYNC_STAGE_ABS=""
WORKER_RSYNC_STAGE_STAGE_ABS=""
WORKER_RSYNC_STAGE_STAMP=""

DECK_VAGRANT_PROVIDER="libvirt"
DECK_VAGRANT_SYNC_TYPE="${DECK_VAGRANT_SYNC_TYPE:-rsync}"
DECK_VAGRANT_BOX_CONTROL_PLANE="${DECK_VAGRANT_BOX_CONTROL_PLANE:-${DECK_VAGRANT_BOX:-generic/ubuntu2204}}"
DECK_VAGRANT_BOX_WORKER="${DECK_VAGRANT_BOX_WORKER:-bento/ubuntu-24.04}"
DECK_VAGRANT_BOX_WORKER_2="${DECK_VAGRANT_BOX_WORKER_2:-generic/rocky9}"
DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX:-${DECK_VAGRANT_BOX_CONTROL_PLANE}}"
DECK_VAGRANT_VM_PREFIX_FROM_ENV=0
if [[ -n "${DECK_VAGRANT_VM_PREFIX:-}" ]]; then
  DECK_VAGRANT_VM_PREFIX_FROM_ENV=1
fi
DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX:-deck-${SCENARIO_ID_SANITIZED}-${RUN_ID_SANITIZED}}"
DECK_VAGRANT_SKIP_CLEANUP="${DECK_VAGRANT_SKIP_CLEANUP:-1}"
DECK_VAGRANT_SKIP_COLLECT="${DECK_VAGRANT_SKIP_COLLECT:-0}"
DECK_VAGRANT_COLLECT_PARALLEL="${DECK_VAGRANT_COLLECT_PARALLEL:-3}"
DECK_VAGRANT_HELPER_ROOT_REL="${DECK_VAGRANT_HELPER_ROOT_REL:-test/vagrant}"
DECK_VAGRANT_CONTROL_PLANE_IP="${DECK_VAGRANT_CONTROL_PLANE_IP:-192.168.57.10}"
DECK_VAGRANT_WORKER_IP="${DECK_VAGRANT_WORKER_IP:-192.168.57.11}"
DECK_VAGRANT_WORKER_2_IP="${DECK_VAGRANT_WORKER_2_IP:-192.168.57.12}"

IN_VAGRANT_DIR=0
LIBVIRT_ENV_INITIALIZED=0
FRESH=0
FRESH_CACHE=0

HOST_BIN=""
HOST_BACKEND_RUNTIME=""
HOST_ARCH=""
SERVER_IP=""
SERVER_URL=""
SCENARIO_METADATA_LOADED=0
SCENARIO_METADATA_NODES=""
SCENARIO_METADATA_USES_WORKERS=""
SCENARIO_METADATA_KUBERNETES_VERSION=""
SCENARIO_METADATA_UPGRADE_KUBERNETES_VERSION=""
SCENARIO_METADATA_VERIFY_STAGE_DEFAULT=""
BUNDLE_KUBERNETES_VERSION=""
BUNDLE_UPGRADE_KUBERNETES_VERSION=""

scenario_basename() {
  local scenario_id="${1:-}"
  case "${scenario_id}" in
    k8s-*)
      printf '%s\n' "${scenario_id#k8s-}"
      ;;
    *)
      printf '%s\n' "${scenario_id}"
      ;;
  esac
}

load_scenario_metadata() {
  local resolved=""
  SCENARIO_METADATA_LOADED=0
  SCENARIO_METADATA_NODES=""
  SCENARIO_METADATA_USES_WORKERS=""
  SCENARIO_METADATA_KUBERNETES_VERSION=""
  SCENARIO_METADATA_UPGRADE_KUBERNETES_VERSION=""
  SCENARIO_METADATA_VERIFY_STAGE_DEFAULT=""
  resolved="$(python3 "${SCENARIO_MANIFEST_HELPER}" "${ROOT_DIR}" "${SCENARIO_ID}" metadata)" || return 1
  while IFS='=' read -r key value; do
    case "${key}" in
      NODES)
        SCENARIO_METADATA_NODES="${value}"
        ;;
      KUBERNETES_VERSION)
        SCENARIO_METADATA_KUBERNETES_VERSION="${value:-v1.30.1}"
        ;;
      UPGRADE_KUBERNETES_VERSION)
        SCENARIO_METADATA_UPGRADE_KUBERNETES_VERSION="${value}"
        ;;
      VERIFY_STAGE_DEFAULT)
        SCENARIO_METADATA_VERIFY_STAGE_DEFAULT="${value}"
        ;;
    esac
  done <<< "${resolved}"
  if [[ -n "${SCENARIO_METADATA_NODES}" ]]; then
    if [[ "${SCENARIO_METADATA_NODES}" == *" "* ]]; then
      SCENARIO_METADATA_USES_WORKERS="1"
    else
      SCENARIO_METADATA_USES_WORKERS="0"
    fi
    SCENARIO_METADATA_LOADED=1
    return 0
  fi
  return 1
}

ensure_scenario_metadata_loaded() {
  if [[ "${SCENARIO_METADATA_LOADED}" == "1" ]]; then
    return 0
  fi
  load_scenario_metadata
}

scenario_nodes() {
  if ! ensure_scenario_metadata_loaded; then
    return 1
  fi
  local node
  for node in ${SCENARIO_METADATA_NODES}; do
    printf '%s\n' "${node}"
  done
}

scenario_requires_workers() {
  if ! ensure_scenario_metadata_loaded; then
    return 1
  fi
  [[ "${SCENARIO_METADATA_USES_WORKERS}" == "1" ]]
}

scenario_kubernetes_version() {
  if ! ensure_scenario_metadata_loaded; then
    return 1
  fi
  printf '%s\n' "${SCENARIO_METADATA_KUBERNETES_VERSION:-v1.30.1}"
}

scenario_upgrade_kubernetes_version() {
  if ! ensure_scenario_metadata_loaded; then
    return 1
  fi
  printf '%s\n' "${SCENARIO_METADATA_UPGRADE_KUBERNETES_VERSION:-}"
}

scenario_verify_stage_default() {
  if ! ensure_scenario_metadata_loaded; then
    return 1
  fi
  printf '%s\n' "${SCENARIO_METADATA_VERIFY_STAGE_DEFAULT:-}"
}

resolve_shared_bundle_versions() {
  local metadata_root="${ROOT_DIR}/test/e2e/scenarios"
  local default_kubernetes_version="v1.30.1"
  local versions=""
  local upgrade_versions=""

  if [[ ! -d "${metadata_root}" ]]; then
    BUNDLE_KUBERNETES_VERSION="${default_kubernetes_version}"
    BUNDLE_UPGRADE_KUBERNETES_VERSION=""
    return 0
  fi

  local resolved
  resolved="$(python3 - <<'PY' "${metadata_root}" "${default_kubernetes_version}"
import json
from pathlib import Path
import sys

root = Path(sys.argv[1])
default = sys.argv[2]
base_versions = set()
upgrade_versions = set()

for path in sorted(root.glob('*.json')):
    values = json.loads(path.read_text(encoding='utf-8'))
    base_versions.add(values.get('kubernetesVersion', default) or default)
    upgrade = values.get('upgradeKubernetesVersion', '')
    if upgrade:
        upgrade_versions.add(upgrade)

if len(base_versions) != 1:
    raise SystemExit('multiple base Kubernetes versions for shared bundle: ' + ', '.join(sorted(base_versions)))
if len(upgrade_versions) > 1:
    raise SystemExit('multiple upgrade Kubernetes versions for shared bundle: ' + ', '.join(sorted(upgrade_versions)))

print(next(iter(base_versions)))
print(next(iter(upgrade_versions)) if upgrade_versions else '')
PY
)" || {
    echo "[deck] failed to resolve shared bundle versions"
    exit 1
  }

  BUNDLE_KUBERNETES_VERSION="$(printf '%s\n' "${resolved}" | sed -n '1p')"
  BUNDLE_UPGRADE_KUBERNETES_VERSION="$(printf '%s\n' "${resolved}" | sed -n '2p')"
}

active_nodes() { scenario_nodes; }

deck_vagrant_usage() {
  local entrypoint="${DECK_VAGRANT_ENTRYPOINT:-test/e2e/vagrant/run-scenario.sh}"
  cat <<EOF
Usage: ${entrypoint} [options]

Options:
  --scenario <name>    Override the scenario cache/artifact namespace.
  --fresh             Recreate VMs and rerun from a clean local state.
  --fresh-cache       Remove run artifacts and scenario cache, then rerun.
  --art-dir <path>    Reuse artifact directory (absolute or workspace-relative).
  --skip-cleanup      Keep VMs after scenario for debugging.
  --cleanup           Destroy VMs at the end of the run.
  --skip-collect      Do not fetch artifacts back from VMs.

Steps:
  $(IFS=,; echo "${STEPS[*]}")
EOF
}

refresh_layout_contracts() {
  SCENARIO_ID_SANITIZED="${SCENARIO_ID//\//-}"
  RUN_ID_SANITIZED="${RUN_ID//\//-}"
  ART_DIR_REL="${DECK_VAGRANT_ART_DIR:-test/artifacts/runs/${SCENARIO_ID}/${RUN_ID}}"
  ART_DIR_ABS="${ROOT_DIR}/${ART_DIR_REL}"
  CACHE_BUNDLES_ROOT_REL="test/artifacts/cache/bundles/shared/${CACHE_KEY}"
  PREPARED_BUNDLE_REL="${CACHE_BUNDLES_ROOT_REL}/bundle"
  PREPARED_BUNDLE_ABS="${ROOT_DIR}/${PREPARED_BUNDLE_REL}"
  PREPARED_BUNDLE_TAR_REL="${CACHE_BUNDLES_ROOT_REL}/prepared-bundle.tar"
  PREPARED_BUNDLE_TAR_ABS="${ROOT_DIR}/${PREPARED_BUNDLE_TAR_REL}"
  PREPARED_BUNDLE_STAMP="${PREPARED_BUNDLE_ABS}/.deck-cache-key"
  PREPARED_BUNDLE_WORK_REL="test/artifacts/cache/staging/shared/${CACHE_KEY}"
  PREPARED_BUNDLE_WORK_ABS="${ROOT_DIR}/${PREPARED_BUNDLE_WORK_REL}"
  PREPARED_BUNDLE_PACK_ROOT="${PREPARED_BUNDLE_WORK_ABS}/host-pack"
  PREPARED_BUNDLE_WORKFLOW_DIR="${PREPARED_BUNDLE_PACK_ROOT}/workflows"
  PREPARED_BUNDLE_FRAGMENT_DIR="${PREPARED_BUNDLE_WORKFLOW_DIR}/scenarios"
  PREPARED_BUNDLE_TAR="${PREPARED_BUNDLE_WORK_ABS}/prepared-bundle.tar"
  PREPARED_BUNDLE_STAGE_ABS="${PREPARED_BUNDLE_WORK_ABS}/prepared-bundle.stage"
  CONTROL_PLANE_RSYNC_STAGE_REL="test/artifacts/cache/vagrant/shared/${CACHE_KEY}/control-plane-rsync-root"
  CONTROL_PLANE_RSYNC_STAGE_ABS="${ROOT_DIR}/${CONTROL_PLANE_RSYNC_STAGE_REL}"
  CONTROL_PLANE_RSYNC_STAGE_STAGE_ABS="${ROOT_DIR}/test/artifacts/cache/vagrant/shared/${CACHE_KEY}/control-plane-rsync-root.stage"
  CONTROL_PLANE_RSYNC_STAGE_STAMP="${CONTROL_PLANE_RSYNC_STAGE_ABS}/.deck-rsync-key"
  WORKER_RSYNC_STAGE_REL="test/artifacts/cache/vagrant/shared/${CACHE_KEY}/worker-rsync-root"
  WORKER_RSYNC_STAGE_ABS="${ROOT_DIR}/${WORKER_RSYNC_STAGE_REL}"
  WORKER_RSYNC_STAGE_STAGE_ABS="${ROOT_DIR}/test/artifacts/cache/vagrant/shared/${CACHE_KEY}/worker-rsync-root.stage"
  WORKER_RSYNC_STAGE_STAMP="${WORKER_RSYNC_STAGE_ABS}/.deck-rsync-key"
  if [[ "${DECK_VAGRANT_VM_PREFIX_FROM_ENV}" != "1" ]]; then
    DECK_VAGRANT_VM_PREFIX="deck-${SCENARIO_ID_SANITIZED}-${RUN_ID_SANITIZED}"
  fi
  load_scenario_metadata || true
}

normalize_art_dir() {
  local path="$1"
  if [[ -z "${path}" ]]; then
    return 0
  fi
  if [[ "${path}" = /* ]]; then
    ART_DIR_ABS="${path}"
    ART_DIR_REL="${path#${ROOT_DIR}/}"
  else
    ART_DIR_REL="${path}"
    ART_DIR_ABS="${ROOT_DIR}/${path}"
  fi
  export DECK_VAGRANT_ART_DIR="${ART_DIR_REL}"
}

parse_args() {
  refresh_layout_contracts
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --scenario)
        SCENARIO_ID="${2:?--scenario requires value}"
        export DECK_VAGRANT_SCENARIO="${SCENARIO_ID}"
        refresh_layout_contracts
        shift 2
        ;;
      --fresh)
        FRESH=1
        DECK_VAGRANT_SKIP_CLEANUP="0"
        shift
        ;;
      --fresh-cache)
        FRESH=1
        FRESH_CACHE=1
        DECK_VAGRANT_SKIP_CLEANUP="0"
        shift
        ;;
      --art-dir)
        normalize_art_dir "${2:?--art-dir requires value}"
        shift 2
        ;;
      --skip-cleanup)
        DECK_VAGRANT_SKIP_CLEANUP="1"
        shift
        ;;
      --cleanup)
        DECK_VAGRANT_SKIP_CLEANUP="0"
        shift
        ;;
      --skip-collect)
        DECK_VAGRANT_SKIP_COLLECT="1"
        shift
        ;;
      --help|-h)
        deck_vagrant_usage
        exit 0
        ;;
      *)
        echo "[deck] unknown argument: $1"
        deck_vagrant_usage
        exit 1
        ;;
    esac
  done

  CHECKPOINT_DIR="${ART_DIR_ABS}/checkpoints"
  RUN_LOG_DIR="${ART_DIR_ABS}/logs"
  RUN_REPORT_DIR="${ART_DIR_ABS}/reports"
  RUN_RENDERED_WORKFLOWS_DIR="${ART_DIR_ABS}/rendered-workflows"
  RUN_BUNDLE_SOURCE_FILE="${ART_DIR_ABS}/bundle-source.txt"
}

prepare_local_run_state() {
  if [[ ${FRESH} -eq 1 ]]; then
    rm -rf "${ART_DIR_ABS}"
    SERVER_IP=""
    SERVER_URL=""
    if [[ ${FRESH_CACHE} -eq 1 ]]; then
      rm -rf "${CACHE_BUNDLES_ROOT_REL:+${ROOT_DIR}/${CACHE_BUNDLES_ROOT_REL}}"
      rm -rf "${PREPARED_BUNDLE_WORK_ABS}"
      rm -rf "$(dirname "${CONTROL_PLANE_RSYNC_STAGE_ABS}")"
    fi
    return 0
  fi
}

initialize_run_contract() {
  mkdir -p "${ART_DIR_ABS}" "${CHECKPOINT_DIR}" "${RUN_LOG_DIR}" "${RUN_REPORT_DIR}" "${RUN_RENDERED_WORKFLOWS_DIR}"
  if [[ ! -f "${RUN_BUNDLE_SOURCE_FILE}" ]]; then
    printf '%s\n' "pending" > "${RUN_BUNDLE_SOURCE_FILE}"
  fi
}

resolve_step_range() {
  return 0
}

ensure_libvirt_environment() {
  if [[ "${LIBVIRT_ENV_INITIALIZED}" == "1" ]]; then
    return 0
  fi
  source "${LIBVIRT_ENV_HELPER}"
  prepare_libvirt_environment
  LIBVIRT_ENV_INITIALIZED=1
}

resolve_host_build_context() {
  HOST_BIN="${ROOT_DIR}/test/artifacts/bin/deck-host"
  HOST_BACKEND_RUNTIME="podman"
  HOST_ARCH="amd64"

  if ! command -v podman >/dev/null 2>&1 && command -v docker >/dev/null 2>&1; then
    HOST_BACKEND_RUNTIME="docker"
  fi
  case "$(uname -m)" in
    x86_64)
      HOST_ARCH="amd64"
      ;;
    aarch64|arm64)
      HOST_ARCH="arm64"
      ;;
  esac
}

compute_prepared_bundle_cache_key() {
  local host_bin="$1"
  local workflow_root="$2"
  local helper_root="$3"
  local backend_runtime="$4"
  local arch="$5"
  local include_legacy_workflows="$6"
  local vm_scenario_script="${7:-}"
  local vm_dispatcher_script="${8:-}"
  local kubernetes_version
  local upgrade_kubernetes_version
  resolve_shared_bundle_versions
  kubernetes_version="${BUNDLE_KUBERNETES_VERSION}"
  upgrade_kubernetes_version="${BUNDLE_UPGRADE_KUBERNETES_VERSION}"
  python3 - <<'PY' "${ROOT_DIR}" "${host_bin}" "${workflow_root}" "${helper_root}" "${backend_runtime}" "${arch}" "${include_legacy_workflows}" "${vm_scenario_script}" "${vm_dispatcher_script}" "${kubernetes_version}" "${upgrade_kubernetes_version}"
import hashlib
from pathlib import Path
import sys

root_dir = Path(sys.argv[1])
host_bin = Path(sys.argv[2])
workflow_root = Path(sys.argv[3])
helper_root = Path(sys.argv[4])
backend_runtime = sys.argv[5]
arch = sys.argv[6]
include_legacy_workflows = sys.argv[7] == "1"
vm_scenario_script = Path(sys.argv[8]) if sys.argv[8] else None
vm_dispatcher_script = Path(sys.argv[9]) if sys.argv[9] else None
kubernetes_version = sys.argv[10]
upgrade_kubernetes_version = sys.argv[11]

paths = [host_bin]
paths.extend(sorted(p for p in workflow_root.rglob('*') if p.is_file()))
for candidate in sorted(p for p in helper_root.rglob('*') if p.is_file()):
    if not include_legacy_workflows and candidate.is_relative_to(root_dir / 'test/vagrant/workflows'):
        continue
    paths.append(candidate)

for extra_root in (root_dir / 'test/workflows',):
    if extra_root.exists():
        paths.extend(sorted(p for p in extra_root.rglob('*') if p.is_file()))

for candidate in (
    root_dir / 'test/e2e/vagrant/common.sh',
    root_dir / 'test/e2e/vagrant/run-scenario.sh',
    root_dir / 'test/e2e/vagrant/run-scenario-vm.sh',
    root_dir / 'test/e2e/vagrant/run-scenario-vm-scenario.sh',
    vm_scenario_script,
    vm_dispatcher_script,
):
    if candidate and candidate.is_file():
        paths.append(candidate)

digest = hashlib.sha256()
digest.update(f'backendRuntime={backend_runtime}\n'.encode())
digest.update(f'arch={arch}\n'.encode())
digest.update(f'kubernetesVersion={kubernetes_version}\n'.encode())
digest.update(f'upgradeKubernetesVersion={upgrade_kubernetes_version}\n'.encode())
seen = set()
for path in paths:
    if path in seen:
        continue
    seen.add(path)
    digest.update(path.relative_to(root_dir).as_posix().encode())
    digest.update(b'\0')
    digest.update(path.read_bytes())
    digest.update(b'\0')
print(digest.hexdigest())
PY
}

prepare_shared_bundle_cache() {
  local host_bin="$1"
  local backend_runtime="$2"
  local arch="$3"
  local workflow_root_abs="${ROOT_DIR}/${DECK_VAGRANT_WORKFLOW_ROOT_REL}"
  local helper_root_abs="${ROOT_DIR}/${DECK_VAGRANT_HELPER_ROOT_REL}"
  local cache_key=""
  local kubernetes_version=""
  local upgrade_kubernetes_version=""
  local -a prepare_args=()

  resolve_shared_bundle_versions
  kubernetes_version="${BUNDLE_KUBERNETES_VERSION}"
  upgrade_kubernetes_version="${BUNDLE_UPGRADE_KUBERNETES_VERSION}"

  cache_key="$(compute_prepared_bundle_cache_key "${host_bin}" "${workflow_root_abs}" "${helper_root_abs}" "${backend_runtime}" "${arch}" "0" "${DECK_VAGRANT_VM_SCENARIO_SCRIPT:-}" "${DECK_VAGRANT_VM_DISPATCHER_SCRIPT:-}")"
  CACHE_KEY="${cache_key}"
  refresh_layout_contracts
  if [[ -f "${PREPARED_BUNDLE_STAMP}" ]] && [[ -f "${PREPARED_BUNDLE_ABS}/.deck/manifest.json" ]] && [[ -f "${PREPARED_BUNDLE_TAR_ABS}" ]] && [[ "$(cat "${PREPARED_BUNDLE_STAMP}" 2>/dev/null || true)" == "${cache_key}" ]]; then
    echo "[deck] reusing shared prepared bundle cache"
    printf '%s\n' "cache-hit:${PREPARED_BUNDLE_TAR_REL}" > "${RUN_BUNDLE_SOURCE_FILE}"
    return 0
  fi

  echo "[deck] rebuilding shared prepared bundle cache"
  rm -rf "${PREPARED_BUNDLE_WORK_ABS}" "${PREPARED_BUNDLE_STAGE_ABS}"
  mkdir -p "${PREPARED_BUNDLE_WORKFLOW_DIR}" "${PREPARED_BUNDLE_FRAGMENT_DIR}"
  deck_vagrant_prepare_workflow_bundle
  prepare_args=(prepare --root outputs
    --bundle-binary-source local
    --bundle-binary-dir "${ROOT_DIR}/test/artifacts/bin"
    --bundle-binary linux/amd64
    --bundle-binary linux/arm64
    --var "kubernetesVersion=${kubernetes_version}"
    --var "arch=${arch}"
    --var "backendRuntime=${backend_runtime}")
  if [[ -n "${upgrade_kubernetes_version}" ]]; then
    prepare_args+=(--var "upgradeKubernetesVersion=${upgrade_kubernetes_version}")
  fi
  (cd "${PREPARED_BUNDLE_PACK_ROOT}" && "${host_bin}" "${prepare_args[@]}")
  (cd "${PREPARED_BUNDLE_PACK_ROOT}" && "${host_bin}" bundle build --root . --out "${PREPARED_BUNDLE_TAR}")

  mkdir -p "${PREPARED_BUNDLE_STAGE_ABS}"
  tar -xf "${PREPARED_BUNDLE_TAR}" -C "${PREPARED_BUNDLE_STAGE_ABS}" --strip-components=1
  printf '%s\n' "${cache_key}" > "${PREPARED_BUNDLE_STAGE_ABS}/.deck-cache-key"

  rm -rf "${CACHE_BUNDLES_ROOT_REL:+${ROOT_DIR}/${CACHE_BUNDLES_ROOT_REL}}"
  mkdir -p "$(dirname "${PREPARED_BUNDLE_ABS}")"
  cp "${PREPARED_BUNDLE_TAR}" "${PREPARED_BUNDLE_TAR_ABS}"
  mv "${PREPARED_BUNDLE_STAGE_ABS}" "${PREPARED_BUNDLE_ABS}"
  printf '%s\n' "cache-rebuild:${PREPARED_BUNDLE_TAR_REL}" > "${RUN_BUNDLE_SOURCE_FILE}"
}

prepare_rsync_stage_root_for_role() {
  local role="$1"
  local stage_abs="$2"
  local stage_stage_abs="$3"
  local stage_stamp="$4"
  local include_bundle_tar="$5"
  local vm_stage_path="${DECK_VAGRANT_VM_STAGED_PATH:-test/e2e/vagrant/run-scenario-vm.sh}"
  local dispatcher_source="${ROOT_DIR}/test/e2e/vagrant/run-scenario-vm.sh"
  local dispatcher_stage_path="${DECK_VAGRANT_VM_DISPATCHER_STAGED_PATH:-test/e2e/vagrant/run-scenario-vm.sh}"
  local dispatcher_scenario_helper_source="${ROOT_DIR}/test/e2e/vagrant/run-scenario-vm-scenario.sh"
  local dispatcher_scenario_helper_stage_path="test/e2e/vagrant/run-scenario-vm-scenario.sh"
  local rsync_key=""

  rsync_key="$(python3 - <<'PY' "${ROOT_DIR}" "${DECK_VAGRANT_VM_SCENARIO_SCRIPT}" "${dispatcher_source}" "${dispatcher_scenario_helper_source}" "${PREPARED_BUNDLE_STAMP}" "${include_bundle_tar}"
import hashlib
from pathlib import Path
import sys

root = Path(sys.argv[1])
scenario_script = Path(sys.argv[2])
dispatcher_script = Path(sys.argv[3]) if sys.argv[3] else None
dispatcher_scenario_helper = Path(sys.argv[4]) if sys.argv[4] else None
bundle_stamp = Path(sys.argv[5])
include_bundle_tar = sys.argv[6] == '1'

paths = [scenario_script]
if dispatcher_script:
    paths.append(dispatcher_script)
if dispatcher_scenario_helper and dispatcher_scenario_helper.is_file():
    paths.append(dispatcher_scenario_helper)
if include_bundle_tar and bundle_stamp.is_file():
    paths.append(bundle_stamp)

digest = hashlib.sha256()
digest.update(f'includeBundleTar={int(include_bundle_tar)}\n'.encode())
for path in paths:
    digest.update(path.relative_to(root).as_posix().encode())
    digest.update(b'\0')
    digest.update(path.read_bytes())
    digest.update(b'\0')
print(digest.hexdigest())
PY
)"

  if [[ -f "${stage_stamp}" ]] && [[ "$(cat "${stage_stamp}" 2>/dev/null || true)" == "${rsync_key}" ]]; then
    echo "[deck] reusing ${role} rsync stage cache"
    return 0
  fi

  rm -rf "${stage_stage_abs}" "${stage_abs}"
  mkdir -p "${stage_stage_abs}/$(dirname "${vm_stage_path}")"
  cp "${DECK_VAGRANT_VM_SCENARIO_SCRIPT}" "${stage_stage_abs}/${vm_stage_path}"
  if [[ -n "${dispatcher_source}" ]] && [[ -n "${dispatcher_stage_path}" ]]; then
    mkdir -p "${stage_stage_abs}/$(dirname "${dispatcher_stage_path}")"
    cp "${dispatcher_source}" "${stage_stage_abs}/${dispatcher_stage_path}"
    if ! cmp -s "${dispatcher_source}" "${stage_stage_abs}/${dispatcher_stage_path}"; then
      echo "[deck] staged dispatcher mismatch: ${dispatcher_source} != ${stage_stage_abs}/${dispatcher_stage_path}"
      exit 1
    fi
  fi
  if [[ -f "${dispatcher_scenario_helper_source}" ]]; then
    mkdir -p "${stage_stage_abs}/$(dirname "${dispatcher_scenario_helper_stage_path}")"
    cp "${dispatcher_scenario_helper_source}" "${stage_stage_abs}/${dispatcher_scenario_helper_stage_path}"
  fi
  if [[ "${include_bundle_tar}" == "1" ]]; then
    mkdir -p "${stage_stage_abs}/$(dirname "${PREPARED_BUNDLE_TAR_REL}")"
    cp "${PREPARED_BUNDLE_TAR_ABS}" "${stage_stage_abs}/${PREPARED_BUNDLE_TAR_REL}"
  fi
  printf '%s\n' "${rsync_key}" > "${stage_stage_abs}/.deck-rsync-key"
  mv "${stage_stage_abs}" "${stage_abs}"
}

prepare_rsync_stage_roots() {
  prepare_rsync_stage_root_for_role "control-plane" "${CONTROL_PLANE_RSYNC_STAGE_ABS}" "${CONTROL_PLANE_RSYNC_STAGE_STAGE_ABS}" "${CONTROL_PLANE_RSYNC_STAGE_STAMP}" "1"
  prepare_rsync_stage_root_for_role "worker" "${WORKER_RSYNC_STAGE_ABS}" "${WORKER_RSYNC_STAGE_STAGE_ABS}" "${WORKER_RSYNC_STAGE_STAMP}" "0"
}

ensure_rsync_sync_source() {
  resolve_host_build_context
  if [[ ! -x "${HOST_BIN}" ]] || [[ ! -f "${ROOT_DIR}/test/artifacts/bin/deck-linux-${HOST_ARCH}" ]]; then
    "${BUILD_BINARIES_HELPER}" "${ROOT_DIR}"
  fi
  prepare_shared_bundle_cache "${HOST_BIN}" "${HOST_BACKEND_RUNTIME}" "${HOST_ARCH}"
  prepare_rsync_stage_roots
}

fetch_vm_artifacts() {
  local node="$1"
  local bundle_tgz="${ART_DIR_ABS}/vm-artifacts-${node}.tgz"

  DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX}" DECK_VAGRANT_PROVIDER="${DECK_VAGRANT_PROVIDER}" DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX}" \
    vagrant ssh "${node}" -c "if [[ -d /workspace/${ART_DIR_REL} ]]; then tar -czf - -C /workspace ${ART_DIR_REL}; fi" > "${bundle_tgz}" 2>/dev/null || true

  if [[ -s "${bundle_tgz}" ]]; then
    tar -xzf "${bundle_tgz}" -C "${ROOT_DIR}" || true
  fi
}

should_fetch_vm_artifacts() {
  if [[ "${DECK_VAGRANT_SKIP_COLLECT}" == "1" ]]; then
    return 1
  fi
  if [[ -f "${ART_DIR_ABS}/pass.txt" && -f "${ART_DIR_ABS}/reports/cluster-nodes.txt" && "${DECK_VAGRANT_SYNC_TYPE}" != "rsync" ]]; then
    echo "[deck] artifacts already visible on host via shared workspace; skipping VM fetch"
    return 1
  fi
  return 0
}

fetch_vm_artifacts_parallel() {
  local -a nodes=()
  local -a pids=()
  local node=""
  local pid=""
  local rc=0
  mapfile -t nodes < <(active_nodes)
  for node in "${nodes[@]}"; do
    fetch_vm_artifacts "${node}" &
    pids+=("$!")
  done
  for pid in "${pids[@]}"; do
    if ! wait "${pid}"; then
      rc=1
    fi
  done
  return ${rc}
}

fetch_vm_artifacts_serial() {
  local -a nodes=()
  local node=""
  mapfile -t nodes < <(active_nodes)
  for node in "${nodes[@]}"; do
    fetch_vm_artifacts "${node}"
  done
}

scenario_result_spec() {
  python3 "${SCENARIO_MANIFEST_HELPER}" "${ROOT_DIR}" "${SCENARIO_ID}" result
}

validate_collected_artifacts() {
  local spec_json=""
  spec_json="$(scenario_result_spec)" || return 1
  python3 - <<'PY' "${ART_DIR_ABS}" "${spec_json}"
import json
from pathlib import Path
import sys

art_dir = Path(sys.argv[1])
spec = json.loads(sys.argv[2])
missing = []
content_errors = []

for item in spec.get("requiredArtifacts", []):
    path = art_dir / item["path"]
    if not path.is_file():
        missing.append(item["path"])
        continue
    text = path.read_text(encoding="utf-8", errors="ignore")
    for want in item.get("contains", []):
        if want not in text:
            content_errors.append(f"{item['path']}: missing {want}")

if missing or content_errors:
    for item in missing:
        print(f"missing artifact: {item}")
    for item in content_errors:
        print(item)
    raise SystemExit(1)
PY
}

write_result_contract() {
  local spec_json=""
  local finished_at=""
  spec_json="$(scenario_result_spec)" || return 1
  finished_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  python3 - <<'PY' "${ART_DIR_ABS}" "${SCENARIO_ID}" "${RUN_ID}" "${DECK_VAGRANT_PROVIDER}" "${CACHE_KEY}" "${RUN_STARTED_AT}" "${finished_at}" "${SERVER_URL}" "${spec_json}"
import json
from pathlib import Path
import sys

art_dir = Path(sys.argv[1])
scenario = sys.argv[2]
run_id = sys.argv[3]
provider = sys.argv[4]
cache_key = sys.argv[5]
started_at = sys.argv[6]
finished_at = sys.argv[7]
server_url = sys.argv[8]
spec = json.loads(sys.argv[9])

evidence = {"server": server_url}
for key, value in spec.get("resultEvidenceFiles", {}).items():
    evidence[key] = value

payload = {
    "scenario": scenario,
    "result": "PASS",
    "runId": run_id,
    "provider": provider,
    "cacheKey": cache_key,
    "startedAt": started_at,
    "finishedAt": finished_at,
    "evidence": evidence,
}

(art_dir / "result.json").write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
(art_dir / "pass.txt").write_text("PASS\n", encoding="utf-8")
PY
}

delete_stale_volume() {
  local node="$1"
  local -a candidates=(
    "${DECK_VAGRANT_VM_PREFIX}${node}.img"
    "${DECK_VAGRANT_VM_PREFIX}-${node}.img"
    "${DECK_VAGRANT_VM_PREFIX}_${node}.img"
  )
  local vol_name
  for vol_name in "${candidates[@]}"; do
    virsh -c "${DECK_LIBVIRT_URI}" vol-delete --pool "${DECK_LIBVIRT_POOL_NAME}" "${vol_name}" >/dev/null 2>&1 || true
  done
}

run_vagrant_ssh() {
  local node="$1"
  local cmd="$2"
  local attempts="${3:-4}"
  local delay_sec="${4:-5}"
  local i rc
  local result=1

  pushd "${VAGRANT_DIR}" >/dev/null
  for ((i=1; i<=attempts; i++)); do
    set +e
    DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX}" DECK_VAGRANT_PROVIDER="${DECK_VAGRANT_PROVIDER}" DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX}" \
      vagrant ssh "${node}" -c "${cmd}"
    rc=$?
    set -e
    if [[ ${rc} -eq 0 ]]; then
      result=0
      break
    fi
    sleep "${delay_sec}"
  done
  popd >/dev/null
  return ${result}
}

cleanup() {
  set +e
  if [[ "${IN_VAGRANT_DIR}" == "1" ]]; then
    popd >/dev/null || true
    IN_VAGRANT_DIR=0
  fi
}

validate_box_provider() {
  local box
  for box in "${DECK_VAGRANT_BOX_CONTROL_PLANE}" "${DECK_VAGRANT_BOX_WORKER}" "${DECK_VAGRANT_BOX_WORKER_2}"; do
    if [[ "${box}" == "manrala/ubuntu24" ]]; then
      echo "[deck] box/provider mismatch: ${box} does not support libvirt"
      exit 1
    fi
  done
}

check_provider_available() {
  local status_out=""
  local status_rc=0

  pushd "${VAGRANT_DIR}" >/dev/null
  set +e
  status_out="$(DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX}" DECK_VAGRANT_PROVIDER="libvirt" DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX}" vagrant status 2>&1)"
  status_rc=$?
  set -e
  popd >/dev/null

  if [[ ${status_rc} -ne 0 ]]; then
    echo "[deck] required provider 'libvirt' is unavailable"
    echo "${status_out}"
    exit 1
  fi
}

step_prepare_host() {
  mkdir -p "${ART_DIR_ABS}"
  ensure_libvirt_environment
  "${BUILD_BINARIES_HELPER}" "${ROOT_DIR}"
  resolve_host_build_context
  prepare_shared_bundle_cache "${HOST_BIN}" "${HOST_BACKEND_RUNTIME}" "${HOST_ARCH}"
  prepare_rsync_stage_roots
}

step_up_vms() {
  local up_rc=0
  local sync_source_env="${DECK_VAGRANT_SYNC_SOURCE:-${ROOT_DIR}}"
  local control_plane_sync_source_env="${DECK_VAGRANT_SYNC_SOURCE_CONTROL_PLANE:-${sync_source_env}}"
  local worker_sync_source_env="${DECK_VAGRANT_SYNC_SOURCE_WORKER:-${sync_source_env}}"
  local -a nodes=()
  local node=""
  mapfile -t nodes < <(active_nodes)
  ensure_libvirt_environment
  if [[ "${DECK_VAGRANT_SYNC_TYPE}" == "rsync" ]]; then
    ensure_rsync_sync_source
    control_plane_sync_source_env="${CONTROL_PLANE_RSYNC_STAGE_ABS}"
    worker_sync_source_env="${WORKER_RSYNC_STAGE_ABS}"
  fi
  pushd "${VAGRANT_DIR}" >/dev/null
  IN_VAGRANT_DIR=1
  if [[ "${DECK_VAGRANT_SKIP_CLEANUP}" != "1" ]]; then
    DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX}" DECK_VAGRANT_PROVIDER="${DECK_VAGRANT_PROVIDER}" DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX}" vagrant destroy -f || true
    for node in "${nodes[@]}"; do
      delete_stale_volume "${node}"
    done
  fi
  set +e
  DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX}" DECK_VAGRANT_PROVIDER="${DECK_VAGRANT_PROVIDER}" DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX}" DECK_VAGRANT_SYNC_TYPE="${DECK_VAGRANT_SYNC_TYPE}" DECK_VAGRANT_SYNC_SOURCE="${sync_source_env}" DECK_VAGRANT_SYNC_SOURCE_CONTROL_PLANE="${control_plane_sync_source_env}" DECK_VAGRANT_SYNC_SOURCE_WORKER="${worker_sync_source_env}" DECK_VAGRANT_SYNC_SOURCE_WORKER_2="${worker_sync_source_env}" vagrant up "${nodes[@]}" --provider "${DECK_VAGRANT_PROVIDER}"
  up_rc=$?
  set -e
  if [[ ${up_rc} -ne 0 && "${DECK_VAGRANT_SYNC_TYPE}" == "9p" ]]; then
    echo "[deck] 9p shared folders are unavailable on this host; retrying with rsync"
    DECK_VAGRANT_SYNC_TYPE="rsync"
    export DECK_VAGRANT_SYNC_TYPE
    ensure_rsync_sync_source
    DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX}" DECK_VAGRANT_PROVIDER="${DECK_VAGRANT_PROVIDER}" DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX}" DECK_VAGRANT_SYNC_TYPE="${DECK_VAGRANT_SYNC_TYPE}" vagrant destroy -f >/dev/null 2>&1 || true
    for node in "${nodes[@]}"; do
      delete_stale_volume "${node}"
    done
    DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX}" DECK_VAGRANT_PROVIDER="${DECK_VAGRANT_PROVIDER}" DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX}" DECK_VAGRANT_SYNC_TYPE="${DECK_VAGRANT_SYNC_TYPE}" DECK_VAGRANT_SYNC_SOURCE="${sync_source_env}" DECK_VAGRANT_SYNC_SOURCE_CONTROL_PLANE="${CONTROL_PLANE_RSYNC_STAGE_ABS}" DECK_VAGRANT_SYNC_SOURCE_WORKER="${WORKER_RSYNC_STAGE_ABS}" DECK_VAGRANT_SYNC_SOURCE_WORKER_2="${WORKER_RSYNC_STAGE_ABS}" vagrant up "${nodes[@]}" --provider "${DECK_VAGRANT_PROVIDER}"
  elif [[ ${up_rc} -ne 0 ]]; then
    exit ${up_rc}
  fi
  SERVER_IP="${DECK_VAGRANT_CONTROL_PLANE_IP}"
  if [[ -z "${SERVER_IP}" ]]; then
    echo "[deck] failed to resolve control-plane IPv4 address"
    exit 1
  fi
  SERVER_URL="http://${SERVER_IP}:18080"
  cat > "${ART_DIR_ABS}/vm-ips.txt" <<EOF
control-plane=${SERVER_IP}
worker=${DECK_VAGRANT_WORKER_IP}
worker-2=${DECK_VAGRANT_WORKER_2_IP}
EOF
  DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX}" DECK_VAGRANT_PROVIDER="${DECK_VAGRANT_PROVIDER}" DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX}" vagrant status > "${ART_DIR_ABS}/vagrant-status.txt"
  popd >/dev/null
  IN_VAGRANT_DIR=0
}

step_collect() {
  if should_fetch_vm_artifacts; then
    pushd "${VAGRANT_DIR}" >/dev/null
    IN_VAGRANT_DIR=1
    if [[ "${DECK_VAGRANT_COLLECT_PARALLEL}" -gt 1 ]]; then
      fetch_vm_artifacts_parallel || true
    fi
  if [[ ! -f "${ART_DIR_ABS}/pass.txt" || ! -f "${ART_DIR_ABS}/reports/cluster-nodes.txt" ]]; then
    fetch_vm_artifacts_serial
  fi
    popd >/dev/null
    IN_VAGRANT_DIR=0
  else
    echo "[deck] collect fetch skipped"
  fi

  if ! validate_collected_artifacts; then
    echo "[deck] collected artifacts failed validation"
    exit 1
  fi
  if ! write_result_contract; then
    echo "[deck] failed to write result contract"
    exit 1
  fi
}

step_cleanup() {
  pushd "${VAGRANT_DIR}" >/dev/null
  IN_VAGRANT_DIR=1
  if [[ "${DECK_VAGRANT_SKIP_CLEANUP}" != "1" ]]; then
    DECK_VAGRANT_BOX="${DECK_VAGRANT_BOX}" DECK_VAGRANT_PROVIDER="${DECK_VAGRANT_PROVIDER}" DECK_VAGRANT_VM_PREFIX="${DECK_VAGRANT_VM_PREFIX}" vagrant destroy -f || true
  else
    echo "[deck] skip cleanup enabled (DECK_VAGRANT_SKIP_CLEANUP=1): keeping VMs"
  fi
  popd >/dev/null
  IN_VAGRANT_DIR=0
}

run_step() {
  local step_name="$1"
  local err_file="${RUN_LOG_DIR}/error-${step_name}.log"
  local step_log="${RUN_LOG_DIR}/step-${step_name}.log"
  echo "[deck] step=${step_name} start"
  rm -f "${step_log}" "${err_file}"
  if ! "step_${step_name//-/_}" > >(tee "${step_log}") 2> >(tee "${err_file}" >&2); then
    echo "[deck] step failed: ${step_name}"
    printf 'failed_step=%s\n' "${step_name}" >> "${RUN_REPORT_DIR}/run-summary.txt"
    cp "${RUN_REPORT_DIR}/run-summary.txt" "${ART_DIR_ABS}/run-summary.txt" 2>/dev/null || true
    exit 1
  fi
  echo "[deck] step=${step_name} done"
}

deck_vagrant_main() {
  parse_args "$@"
  prepare_local_run_state
  resolve_step_range

  trap cleanup EXIT INT TERM

  export VAGRANT_DEFAULT_PROVIDER="libvirt"
  export DECK_VAGRANT_MANAGEMENT_NETWORK_NAME="${DECK_VAGRANT_MANAGEMENT_NETWORK_NAME:-default}"
  export DECK_VAGRANT_MANAGEMENT_NETWORK_ADDRESS="${DECK_VAGRANT_MANAGEMENT_NETWORK_ADDRESS:-192.168.122.0/24}"
  export DECK_VAGRANT_IP_ADDRESS_TIMEOUT="${DECK_VAGRANT_IP_ADDRESS_TIMEOUT:-300}"
  export DECK_VAGRANT_QEMU_USE_AGENT="${DECK_VAGRANT_QEMU_USE_AGENT:-0}"
  export DECK_VAGRANT_ENABLE_PRIVATE_NETWORK="${DECK_VAGRANT_ENABLE_PRIVATE_NETWORK:-1}"
  export DECK_VAGRANT_MGMT_ATTACH="${DECK_VAGRANT_MGMT_ATTACH:-1}"
  export DECK_VAGRANT_SYNC_TYPE
  export DECK_VAGRANT_BOX_CONTROL_PLANE
  export DECK_VAGRANT_BOX_WORKER
  export DECK_VAGRANT_BOX_WORKER_2

  for p in "${VAGRANT_DIR}/Vagrantfile" "${DECK_VAGRANT_VM_SCENARIO_SCRIPT}" "${LIBVIRT_ENV_HELPER}" "${BUILD_BINARIES_HELPER}" "${SCENARIO_MANIFEST_HELPER}"; do
    if [[ ! -e "${p}" ]]; then
      echo "[deck] missing required path: ${p}"
      exit 1
    fi
  done
  if [[ -n "${DECK_VAGRANT_VM_DISPATCHER_SCRIPT:-}" ]] && [[ ! -e "${DECK_VAGRANT_VM_DISPATCHER_SCRIPT}" ]]; then
    echo "[deck] missing required path: ${DECK_VAGRANT_VM_DISPATCHER_SCRIPT}"
    exit 1
  fi

  validate_box_provider
  ensure_libvirt_environment
  check_provider_available
  initialize_run_contract

  local step_name
  for step_name in "${STEPS[@]}"; do
    run_step "${step_name}"
  done

  trap - EXIT INT TERM
  echo "[deck] ${SCENARIO_ID} artifacts: ${ART_DIR_ABS}"
}
