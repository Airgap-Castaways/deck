#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAGRANT_DIR="${SCRIPT_DIR}"
VAGRANT_STATE_DIR="${VAGRANT_DIR}/.vagrant/machines"
VM_PREFIX="${DECK_VAGRANT_VM_PREFIX:-deck}"
LIBVIRT_URI="${DECK_LIBVIRT_URI:-qemu:///system}"
NODE_NAMES="${DECK_VAGRANT_NODE_NAMES:-control-plane worker worker-2}"

if [[ ! -d "${VAGRANT_STATE_DIR}" ]] || ! command -v virsh >/dev/null 2>&1; then
  exit 0
fi

sanitize_machine_dir() {
  local machine_dir="$1"
  local id_file="${machine_dir}/id"
  local domain_uuid=""

  if [[ ! -f "${id_file}" ]]; then
    return 0
  fi

  domain_uuid="$(tr -d '[:space:]' < "${id_file}")"
  if [[ -z "${domain_uuid}" ]]; then
    rm -rf "${machine_dir%/libvirt}"
    return 0
  fi

  if ! virsh -c "${LIBVIRT_URI}" dominfo "${domain_uuid}" >/dev/null 2>&1; then
    rm -rf "${machine_dir%/libvirt}"
  fi
}

remove_stale_domain() {
  local domain_name="$1"

  if ! virsh -c "${LIBVIRT_URI}" dominfo "${domain_name}" >/dev/null 2>&1; then
    return 0
  fi

  virsh -c "${LIBVIRT_URI}" destroy "${domain_name}" >/dev/null 2>&1 || true
  virsh -c "${LIBVIRT_URI}" undefine "${domain_name}" --nvram --remove-all-storage >/dev/null 2>&1 || \
    virsh -c "${LIBVIRT_URI}" undefine "${domain_name}" --remove-all-storage >/dev/null 2>&1 || \
    virsh -c "${LIBVIRT_URI}" undefine "${domain_name}" --nvram >/dev/null 2>&1 || \
    virsh -c "${LIBVIRT_URI}" undefine "${domain_name}" >/dev/null 2>&1 || true
}

while IFS= read -r machine_dir; do
  sanitize_machine_dir "${machine_dir}"
done < <(find "${VAGRANT_STATE_DIR}" -mindepth 2 -maxdepth 2 -type d -name libvirt | sort)

for node_name in ${NODE_NAMES}; do
  machine_dir="${VAGRANT_STATE_DIR}/${node_name}/libvirt"
  if [[ ! -f "${machine_dir}/id" ]]; then
    remove_stale_domain "${VM_PREFIX}${node_name}"
  fi
done
