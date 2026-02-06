#!/bin/bash
set -e

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
    echo "Usage: $0 <goenv-dir> [platform]"
    exit 1
fi

ROOT="$1"
PLATFORM="${2:-}"

if [ ! -d "$ROOT" ]; then
    echo "Error: goenv directory not found: $ROOT"
    exit 1
fi

ROOT="$(cd "$ROOT" && pwd)"
BIN_DIR="$ROOT/go/bin"
SHARE_DIR="$ROOT/go/share"
LIB_DIR="$ROOT/go/lib"
TOOLCHAIN_DIR="$ROOT/gotoolchain/go"

require_dir() {
    local dir="$1"
    if [ ! -d "$dir" ]; then
        echo "Error: missing directory: $dir"
        exit 1
    fi
}

require_file() {
    local file="$1"
    if [ ! -f "$file" ]; then
        echo "Error: missing file: $file"
        exit 1
    fi
}

exe_suffix=""
if [ "$PLATFORM" = "windows" ]; then
    exe_suffix=".exe"
fi

echo "Verifying goenv layout at: $ROOT"

require_dir "$BIN_DIR"
require_dir "$SHARE_DIR"
require_dir "$LIB_DIR"
require_dir "$TOOLCHAIN_DIR"

require_file "$BIN_DIR/spx$exe_suffix"
require_file "$BIN_DIR/spxrun$exe_suffix"
require_file "$TOOLCHAIN_DIR/bin/go$exe_suffix"
require_file "$SHARE_DIR/runtime.gdextension"

ENGINE_DIR="$SHARE_DIR/engines"
require_dir "$ENGINE_DIR"

found_pair=0
for pck in "$ENGINE_DIR"/gdspxrt*.pck; do
    if [ ! -f "$pck" ]; then
        continue
    fi
    base="$(basename "$pck" .pck)"
    runtime_bin="$ENGINE_DIR/$base$exe_suffix"
    if [ ! -f "$runtime_bin" ]; then
        echo "Error: runtime binary missing for $pck: $runtime_bin"
        exit 1
    fi
    found_pair=1
done

if [ "$found_pair" -ne 1 ]; then
    echo "Error: no runtime pck found in $ENGINE_DIR"
    exit 1
fi

require_dir "$SHARE_DIR/ispx"
require_file "$SHARE_DIR/ispx/ispx.wasm"
require_file "$SHARE_DIR/wasm_exec.js"

TEMPLATES_DIR="$SHARE_DIR/templates/normal"
require_dir "$TEMPLATES_DIR"

shopt -s nullglob
lib_matches=("$LIB_DIR"/gdspx-*)
shopt -u nullglob
if [ "${#lib_matches[@]}" -eq 0 ]; then
    echo "Error: no native runtime library found in $LIB_DIR"
    exit 1
fi

echo "goenv verification completed successfully"
