#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
BUILDCTL_BIN="$REPO_DIR/.bin/buildctl$(go env GOEXE)"

cd "$REPO_DIR"

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
    done < <(find "$REPO_DIR/internal/cmd/buildctl" -type f -name '*.go' -print0)
    return 0
}

if ! buildctl_bin_is_fresh; then
    mkdir -p "$REPO_DIR/.bin"
    go build -o "$BUILDCTL_BIN" ./internal/cmd/buildctl
fi

exec "$BUILDCTL_BIN" "$@"
