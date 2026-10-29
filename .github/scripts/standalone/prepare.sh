#!/usr/bin/env bash
set -euo pipefail

source "$(dirname -- "${BASH_SOURCE[0]}")/platform.sh"
: "${BUILDCTL:?}" "${GITHUB_OUTPUT:?}"
export GOWORK=off
setup_args=(host)
if [ -n "${ENGINE_ASSET_DIR:-}" ]; then
  setup_args+=(--asset-dir "$ENGINE_ASSET_DIR")
else
  setup_args+=(--published-runtime)
fi
"$BUILDCTL" setup "${setup_args[@]}"
spx_bin="$(go_env_spx_path "$(go_env_goexe)")"
test -f "$spx_bin"
printf 'spx-bin=%s\n' "$spx_bin" >> "$GITHUB_OUTPUT"
