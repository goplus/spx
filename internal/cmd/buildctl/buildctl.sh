#!/bin/bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
BUILDCTL_BIN="$REPO_DIR/.bin/buildctl$(go env GOEXE)"

cd "$REPO_DIR"

. "$REPO_DIR/cmd/internal/macos_go_toolchain.sh"
configure_macos_go_toolchain

make -s buildctl

exec "$BUILDCTL_BIN" "$@"
