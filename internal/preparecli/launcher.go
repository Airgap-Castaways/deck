package preparecli

import "strings"

func renderLauncherScript() string {
	return strings.TrimSpace(`#!/bin/sh
set -eu

log_error() {
	level="error"
	component="launcher"
	event="$1"
	shift
	printf 'level=%s component=%s event=%s' "$level" "$component" "$event" >&2
	while [ "$#" -gt 1 ]; do
		escaped_value=$(printf '%s' "$2" | sed 's/["\\]/\\&/g')
		printf ' %s="%s"' "$1" "$escaped_value" >&2
		shift 2
	done
	printf '\n' >&2
}

os_name="$(uname -s)"
arch_name="$(uname -m)"

case "$os_name" in
	Linux) deck_os="linux" ;;
	Darwin) deck_os="darwin" ;;
	*)
		log_error unsupported_os os "$os_name"
		exit 1
		;;
esac

case "$arch_name" in
	x86_64|amd64) deck_arch="amd64" ;;
	aarch64|arm64) deck_arch="arm64" ;;
	*)
		log_error unsupported_arch architecture "$arch_name"
		exit 1
		;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
runtime_bin="$script_dir/outputs/bin/$deck_os/$deck_arch/deck"

if [ ! -x "$runtime_bin" ]; then
	if [ -e "$runtime_bin" ]; then
		log_error runtime_binary_not_executable path "outputs/bin/$deck_os/$deck_arch/deck"
	else
		log_error runtime_binary_missing os "$deck_os" architecture "$deck_arch" path "outputs/bin/$deck_os/$deck_arch/deck"
	fi
	exit 1
fi

exec "$runtime_bin" "$@"`)
}
