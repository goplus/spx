#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname -- "${BASH_SOURCE[0]}")/../../.."
export GOWORK=off
source ./cmd/internal/macos_go_toolchain.sh
configure_macos_go_toolchain

buildctl_path="$(pwd)/.bin/buildctl$(go env GOEXE)"
mkdir -p ./.bin
# Go's build cache reuses unchanged packages without trusting an inherited
# executable that may come from a different checkout or toolchain.
macos_go_toolchain_go_build -o "$buildctl_path" ./internal/cmd/buildctl
printf 'path=%s\n' "$buildctl_path" >> "$GITHUB_OUTPUT"
