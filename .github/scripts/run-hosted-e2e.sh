#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE_DIR="${DECK_HOSTED_E2E_FIXTURE_DIR:-${ROOT_DIR}/.github/e2e/hosted/fixtures/smoke}"
IMAGE="${DECK_HOSTED_E2E_IMAGE:-ubuntu:24.04}"
WORKDIR="${DECK_HOSTED_E2E_WORKDIR:-$(mktemp -d "${TMPDIR:-/tmp}/deck-hosted-e2e.XXXXXX")}"
KEEP_WORKDIR="${DECK_HOSTED_E2E_KEEP_WORKDIR:-}"
RUNTIME="${DECK_HOSTED_E2E_RUNTIME:-docker}"
INSTALL_PACKAGES="${DECK_HOSTED_E2E_INSTALL_PACKAGES:-true}"

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

run_workspace_flow() {
  local repo_root="$1"
  local workspace="$2"
  local install_packages="$3"
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
  ./deck apply --var "installPackages=${install_packages}"

  test -f .deck-hosted-e2e/input.txt
  test "$(cat .deck-hosted-e2e/input.txt)" = "hosted-e2e bundle seed"
  test -f .deck-hosted-e2e/config.env
  grep -qxF "MESSAGE=hosted-e2e-ok" .deck-hosted-e2e/config.env
  test -L .deck-hosted-e2e/latest-input.txt
  test "$(readlink .deck-hosted-e2e/latest-input.txt)" = "input.txt"
  if [[ "${install_packages}" == "true" ]]; then
    jq --version
  fi

  first_mtime="$(mtime_seconds .deck-hosted-e2e/config.env)"
  sleep 1
  ./deck apply --var "installPackages=${install_packages}"
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

python3 - <<'PY' "${WORKDIR}/workflows/prepare.yaml" "${WORKDIR}"
from pathlib import Path
import sys

prepare_path = Path(sys.argv[1])
workdir = Path(sys.argv[2]).as_posix()
content = prepare_path.read_text(encoding="utf-8")
content = content.replace("__FIXTURE_ROOT__", workdir)
prepare_path.write_text(content, encoding="utf-8")
PY

mkdir -p "${WORKDIR}/home"

if [[ "${RUNTIME}" == "docker" ]]; then
  docker run --rm \
    --volume "${ROOT_DIR}:/repo:ro" \
    --volume "${WORKDIR}:/workspace" \
    --workdir /workspace \
    --env DECK_HOSTED_E2E_INSTALL_PACKAGES="${INSTALL_PACKAGES}" \
    "${IMAGE}" \
    bash -lc "$(declare -f mtime_seconds)
$(declare -f extract_bundle)
$(declare -f run_workspace_flow)
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y --no-install-recommends ca-certificates tar

run_workspace_flow /repo /workspace \"\${DECK_HOSTED_E2E_INSTALL_PACKAGES}\"
"
else
  run_workspace_flow "${ROOT_DIR}" "${WORKDIR}" "${INSTALL_PACKAGES}"
fi
