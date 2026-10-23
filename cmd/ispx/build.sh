#!/bin/bash
set -euo pipefail

usage() {
    echo "Usage: $0 [--base|--ai]"
    echo "Build ispx.wasm; SPX_ISPX_PROFILE selects base or ai (default: ai)."
}

profile="${SPX_ISPX_PROFILE:-ai}"
if [ "$#" -gt 1 ]; then
    usage >&2
    exit 2
fi
if [ "$#" -eq 1 ]; then
    case "$1" in
        --base|--ai) profile="${1#--}" ;;
        -h|--help) usage; exit 0 ;;
        *) usage >&2; exit 2 ;;
    esac
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
case "$profile" in
    base) module_dir="$script_dir" ;;
    ai) module_dir="$script_dir/ai" ;;
    *) echo "Invalid SPX_ISPX_PROFILE: $profile (expected base or ai)" >&2; exit 2 ;;
esac
cd -- "$module_dir"

GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w" -o "$script_dir/ispx.wasm" .
