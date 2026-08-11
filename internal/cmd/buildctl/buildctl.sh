#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
BUILDCTL_BIN="$REPO_DIR/.bin/buildctl$(go env GOEXE)"

cd "$REPO_DIR"

. "$REPO_DIR/cmd/internal/macos_go_toolchain.sh"
configure_macos_go_toolchain

buildctl_bin_is_fresh() {
    if [ ! -f "$BUILDCTL_BIN" ]; then
        return 1
    fi
    if [ "$REPO_DIR/go.mod" -nt "$BUILDCTL_BIN" ]; then
        return 1
    fi
    if [ -f "$REPO_DIR/go.sum" ] && [ "$REPO_DIR/go.sum" -nt "$BUILDCTL_BIN" ]; then
        return 1
    fi
    while IFS= read -r -d '' file; do
        if [ "$file" -nt "$BUILDCTL_BIN" ]; then
            return 1
        fi
    done < <(
        find "$REPO_DIR/cmd" "$REPO_DIR/internal" -type f \
            \( \
                -name '*.go' \
                -o -path "$REPO_DIR/cmd/internal/macos_go_toolchain.sh" \
                -o -path "$REPO_DIR/internal/release/runtime.lock.json" \
                -o -path "$REPO_DIR/internal/release/runtime_locks/*.json" \
            \) \
            ! -name '*_test.go' \
            -print0
    )
    return 0
}

if ! buildctl_bin_is_fresh; then
    mkdir -p "$REPO_DIR/.bin"
    macos_go_toolchain_go_build -o "$BUILDCTL_BIN" ./internal/cmd/buildctl
fi

exec "$BUILDCTL_BIN" "$@"
