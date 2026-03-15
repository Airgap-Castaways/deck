#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="${1:?root dir required}"
TARGET_DIR="${2:?target dir required}"
SCENARIO_ID="${3:-${DECK_VAGRANT_SCENARIO:-k8s-worker-join}}"

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

CANONICAL_ROOT="${ROOT_DIR}/test/workflows"
COMPAT_ROOT="${ROOT_DIR}/test/vagrant/workflows/offline-multinode"

mkdir -p "${TARGET_DIR}"

if [[ -d "${CANONICAL_ROOT}" ]]; then
  cp -a "${CANONICAL_ROOT}/." "${TARGET_DIR}/"
fi

if [[ "${SCENARIO_ID}" == "offline-multinode" ]] && [[ -d "${COMPAT_ROOT}" ]]; then
  mkdir -p "${TARGET_DIR}/offline-multinode"
  cp -a "${COMPAT_ROOT}/." "${TARGET_DIR}/offline-multinode/"
fi
