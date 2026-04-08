#!/usr/bin/env bash
set -euo pipefail

contains_unit() {
  local list="${1:-}"
  local unit="${2:-}"
  [[ -n "${unit}" ]] || return 1
  if [[ ",${list}," == *",${unit},"* ]]; then
    return 0
  fi
  if [[ "${unit}" == *.service ]]; then
    local base="${unit%.service}"
    [[ ",${list}," == *",${base},"* ]]
    return
  fi
  [[ ",${list}," == *",${unit}.service,"* ]]
}

cmd="${1:-}"
case "${cmd}" in
  is-enabled)
    if contains_unit "${SYSTEMCTL_ENABLED_UNITS:-}" "${2:-}"; then
      exit 0
    fi
    exit 1
    ;;
  is-active)
    unit="${2:-}"
    if [[ "${unit}" == "--quiet" ]]; then
      unit="${3:-}"
    fi
    if contains_unit "${SYSTEMCTL_ACTIVE_UNITS:-}" "${unit}"; then
      exit 0
    fi
    exit 1
    ;;
  list-unit-files)
    if contains_unit "${SYSTEMCTL_EXISTING_UNITS:-}" "${2:-}"; then
      printf '%s enabled\n' "${2:-}"
      exit 0
    fi
    exit 1
    ;;
  daemon-reload)
    printf '%s\n' "$*" >> "__LOG_PATH__"
    exit 0
    ;;
  enable|disable|start|stop|restart|reload)
    if contains_unit "${SYSTEMCTL_MISSING_UNITS:-}" "${2:-}"; then
      printf 'Unit %s not found.\n' "${2:-}" >&2
      exit 1
    fi
    ;;
esac

printf '%s\n' "$*" >> "__LOG_PATH__"
