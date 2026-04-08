#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE_DIR="${DECK_HOSTED_E2E_FIXTURE_DIR:-${ROOT_DIR}/.github/e2e/hosted/fixtures/smoke}"
IMAGE="${DECK_HOSTED_E2E_IMAGE:-ubuntu:24.04}"
WORKDIR="${DECK_HOSTED_E2E_WORKDIR:-$(mktemp -d "${TMPDIR:-/tmp}/deck-hosted-e2e.XXXXXX")}"
KEEP_WORKDIR="${DECK_HOSTED_E2E_KEEP_WORKDIR:-}"
RUNTIME="${DECK_HOSTED_E2E_RUNTIME:-docker}"
INSTALL_PACKAGES="${DECK_HOSTED_E2E_INSTALL_PACKAGES:-true}"
PACKAGE_MANAGER="${DECK_HOSTED_E2E_PACKAGE_MANAGER:-apt}"
PACKAGE_NAME="${DECK_HOSTED_E2E_PACKAGE_NAME:-jq}"

resolve_realpath() {
  python3 - <<'PY' "$1"
from pathlib import Path
import sys

print(Path(sys.argv[1]).resolve())
PY
}

mtime_seconds() {
  if stat -c %Y "$1" >/dev/null 2>&1; then
    stat -c %Y "$1"
    return
  fi
  stat -f %m "$1"
}

extract_bundle() {
  local archive_path="$1"
  local output_dir="$2"

  mkdir -p "${output_dir}"
  tar -m -xf "${archive_path}" -C "${output_dir}"
}

prepare_fixture_assets() {
  local workspace="$1"

  mkdir -p "${workspace}/seed/files/archive-src"
  cat >"${workspace}/seed/files/archive-src/message.txt" <<'EOF'
hosted-e2e archive payload
EOF
  tar -czf "${workspace}/seed/files/archive.tgz" -C "${workspace}/seed/files/archive-src" .
}

container_bootstrap() {
  local image="$1"

  case "${image}" in
    ubuntu:*|debian:*)
      apt-get update
      apt-get install -y --no-install-recommends ca-certificates tar gzip
      ;;
    rockylinux:*|quay.io/rockylinux/rockylinux:*)
      dnf install -y ca-certificates tar gzip
      update-ca-trust
      ;;
    *)
      printf 'unsupported hosted e2e container image bootstrap: %s\n' "${image}" >&2
      exit 1
      ;;
  esac
}

run_workspace_flow() {
  local repo_root="$1"
  local workspace="$2"
  local install_packages="$3"
  local package_manager="$4"
  local package_name="$5"
  local first_mtime
  local second_mtime

  export HOME="${workspace}/home"
  mkdir -p "${HOME}"

  pushd "${workspace}" >/dev/null
  "${repo_root}/bin/deck" lint --root "${workspace}"
  "${repo_root}/bin/deck" prepare --root "${workspace}/outputs" --bundle-binary-source local
  "${repo_root}/bin/deck" bundle verify --file "${workspace}"
  "${repo_root}/bin/deck" bundle build --root "${workspace}" --out "${workspace}/bundle.tar"
  popd >/dev/null

  extract_bundle "${workspace}/bundle.tar" "${workspace}/unpacked"

  pushd "${workspace}/unpacked/bundle" >/dev/null
  ./deck apply \
    --var "installPackages=${install_packages}" \
    --var "packageManager=${package_manager}" \
    --var "packageName=${package_name}"

  test -f .deck-hosted-e2e/input.txt
  test "$(cat .deck-hosted-e2e/input.txt)" = "hosted-e2e bundle seed"
  test -f .deck-hosted-e2e/extracted/message.txt
  test "$(cat .deck-hosted-e2e/extracted/message.txt)" = "hosted-e2e archive payload"
  test -f .deck-hosted-e2e/config.env
  grep -qxF "PACKAGE=${package_name}" .deck-hosted-e2e/config.env
  grep -qxF "PACKAGE_MANAGER=${package_manager}" .deck-hosted-e2e/config.env
  grep -qxF "MESSAGE=hosted-e2e-ok" .deck-hosted-e2e/config.env
  test -L .deck-hosted-e2e/latest-input.txt
  test "$(readlink .deck-hosted-e2e/latest-input.txt)" = "input.txt"
  if [[ "${install_packages}" == "true" ]]; then
    "${package_name}" --version
  fi

  first_mtime="$(mtime_seconds .deck-hosted-e2e/config.env)"
  sleep 1
  ./deck apply \
    --var "installPackages=${install_packages}" \
    --var "packageManager=${package_manager}" \
    --var "packageName=${package_name}"
  second_mtime="$(mtime_seconds .deck-hosted-e2e/config.env)"
  test "${first_mtime}" = "${second_mtime}"
  popd >/dev/null
}

safe_reset_workdir() {
  local candidate="$1"
  local resolved

  resolved="$(resolve_realpath "${candidate}")"
  case "${resolved}" in
    /|.)
      printf 'refusing to reset unsafe hosted e2e workdir: %s\n' "${resolved}" >&2
      exit 1
      ;;
  esac
  if [[ "${resolved}" == "${ROOT_DIR}" ]]; then
    printf 'refusing to reset repository root as hosted e2e workdir: %s\n' "${resolved}" >&2
    exit 1
  fi

  rm -rf "${resolved}"
  mkdir -p "${resolved}"
  WORKDIR="${resolved}"
}

cleanup() {
  if [[ -z "${KEEP_WORKDIR}" ]]; then
    rm -rf "${WORKDIR}"
  fi
}
trap cleanup EXIT

if [[ ! -d "${FIXTURE_DIR}" ]]; then
  printf 'missing hosted e2e fixture: %s\n' "${FIXTURE_DIR}" >&2
  exit 1
fi
if [[ ! -x "${ROOT_DIR}/bin/deck" ]]; then
  printf 'missing deck binary at %s; run make build first\n' "${ROOT_DIR}/bin/deck" >&2
  exit 1
fi
case "${RUNTIME}" in
  docker|host)
    ;;
  *)
    printf 'unsupported hosted e2e runtime: %s\n' "${RUNTIME}" >&2
    exit 1
    ;;
esac

if [[ "${RUNTIME}" == "docker" ]] && ! command -v docker >/dev/null 2>&1; then
  printf 'docker is required for hosted e2e smoke when DECK_HOSTED_E2E_RUNTIME=docker\n' >&2
  exit 1
fi

safe_reset_workdir "${WORKDIR}"
cp -R "${FIXTURE_DIR}/." "${WORKDIR}/"
prepare_fixture_assets "${WORKDIR}"

mkdir -p "${WORKDIR}/home"

if [[ "${RUNTIME}" == "docker" ]]; then
  docker run --rm \
    --volume "${ROOT_DIR}:/repo:ro" \
    --volume "${WORKDIR}:/workspace" \
    --workdir /workspace \
    --env DECK_HOSTED_E2E_INSTALL_PACKAGES="${INSTALL_PACKAGES}" \
    --env DECK_HOSTED_E2E_PACKAGE_MANAGER="${PACKAGE_MANAGER}" \
    --env DECK_HOSTED_E2E_PACKAGE_NAME="${PACKAGE_NAME}" \
    "${IMAGE}" \
    bash -lc "$(declare -f mtime_seconds)
$(declare -f extract_bundle)
$(declare -f container_bootstrap)
$(declare -f run_workspace_flow)
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

container_bootstrap '${IMAGE}'

run_workspace_flow /repo /workspace \
  \"\${DECK_HOSTED_E2E_INSTALL_PACKAGES}\" \
  \"\${DECK_HOSTED_E2E_PACKAGE_MANAGER}\" \
  \"\${DECK_HOSTED_E2E_PACKAGE_NAME}\"
"
else
  run_workspace_flow "${ROOT_DIR}" "${WORKDIR}" "${INSTALL_PACKAGES}" "${PACKAGE_MANAGER}" "${PACKAGE_NAME}"
fi
