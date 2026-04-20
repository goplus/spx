#!/bin/bash
set -euo pipefail

# Read app name from appname.txt file
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Pin Go toolchain version
export GOTOOLCHAIN=go1.25.8

target_font_dir=./template/project/engine/fonts/
mkdir -p "$target_font_dir"
font_path=$target_font_dir/CnFont.ttf
if [ ! -f "$font_path" ]; then
    curl -L https://github.com/goplus/godot/releases/download/spx2.0.14/CnFont.ttf -o "$font_path"
fi

if [ ! -f "$font_path" ]; then
    echo "can not find font or download it, please checkout your network " $font_path
    exit 1
fi

appname=$(cat appname.txt)
os_name="${OS:-}"
GOPATH="$(go env GOPATH)"
install_web=0
embed_runtime="${SPX_EMBED_RUNTIME:-1}"

for arg in "$@"; do
    case "$arg" in
        --web)
            install_web=1
            ;;
        --embed-runtime)
            embed_runtime=1
            ;;
        --no-embed-runtime)
            embed_runtime=0
            ;;
        --opt)
            ;;
        *)
            echo "warning: ignoring unknown install.sh flag: $arg"
            ;;
    esac
done

is_truthy() {
    case "${1:-}" in
        1|true|TRUE|yes|YES|on|ON)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

runtime_asset_dir="./internal/runtimeasset/assets"
runtime_asset_placeholder="$runtime_asset_dir/placeholder.txt"

cleanup_embedded_runtime_assets() {
    mkdir -p "$runtime_asset_dir"
    find "$runtime_asset_dir" -maxdepth 1 -type f ! -name "$(basename "$runtime_asset_placeholder")" -delete
}

ensure_runtime_assets_for_embedding() {
    local runtime_version runtime_name runtime_pack goexe

    runtime_version="$(GOFLAGS="-buildvcs=false" go run ../../.github/scripts/runtime_version.go | tr -d '\r\n')"
    goexe="$(go env GOEXE | tr -d '\r\n')"
    runtime_name="gdspxrt${runtime_version}${goexe}"
    runtime_pack="gdspxrt${runtime_version}.pck"

    if [ -f "$GOPATH/bin/$runtime_name" ] && [ -f "$GOPATH/bin/$runtime_pack" ]; then
        return 0
    fi

    echo "Preparing runtime assets for embedded spx build..."
    GOFLAGS="-buildvcs=false" go run ../../internal/cmd/buildctl engine download --runtime
}

build_ispxnative() {
    ( cd ../ispxnative && ./build.sh )
}

stage_embedded_runtime_assets() {
    local runtime_version runtime_name runtime_pack goexe copied

    runtime_version="$(GOFLAGS="-buildvcs=false" go run ../../.github/scripts/runtime_version.go | tr -d '\r\n')"
    goexe="$(go env GOEXE | tr -d '\r\n')"
    runtime_name="gdspxrt${runtime_version}${goexe}"
    runtime_pack="gdspxrt${runtime_version}.pck"

    if [ ! -f "$GOPATH/bin/$runtime_name" ]; then
        echo "Error: runtime executable not found: $GOPATH/bin/$runtime_name"
        exit 1
    fi
    if [ ! -f "$GOPATH/bin/$runtime_pack" ]; then
        echo "Error: runtime pack not found: $GOPATH/bin/$runtime_pack"
        exit 1
    fi

    cleanup_embedded_runtime_assets
    cp "$GOPATH/bin/$runtime_name" "$runtime_asset_dir/"
    cp "$GOPATH/bin/$runtime_pack" "$runtime_asset_dir/"

    copied=0
    shopt -s nullglob
    for lib in ../ispxnative/gdspx-*; do
        cp "$lib" "$runtime_asset_dir/"
        copied=1
    done
    shopt -u nullglob
    if [ "$copied" -eq 0 ]; then
        echo "Error: no gdspx shared libraries found under ../ispxnative"
        exit 1
    fi
}

if is_truthy "$embed_runtime"; then
    cleanup_embedded_runtime_assets
    trap cleanup_embedded_runtime_assets EXIT
    ensure_runtime_assets_for_embedding
    build_ispxnative
    stage_embedded_runtime_assets
fi

# install cmd
if [ "$os_name" = "Windows_NT" ]; then
   appname="${appname}.exe"
fi

if [ "$os_name" = "Windows_NT" ]; then
   # Fix for Windows MinGW linker duplicate symbol errors with Go 1.24
   go build -ldflags="-extldflags=-Wl,--allow-multiple-definition" -o "$appname"
else
   go build -o "$appname"
fi 


if [ ! -f "$appname" ]; then
    echo "Error: $appname not found"
    exit 1
fi

mv "$appname" "$GOPATH/bin/"

# build and install ispx
echo "Building ispx..."

if [ "$install_web" -eq 1 ]; then
    ( cd ../ispx && GOFLAGS="-buildvcs=false" ./build.sh )
    cp ../ispx/ispx.wasm "$GOPATH/bin/"

    # Install ispx web runtime
    echo "Installing ispx web runtime..."
    rm -rf "$GOPATH/bin/ispx"
    mkdir -p "$GOPATH/bin/ispx"
    cp ../ispx/web/* "$GOPATH/bin/ispx/"
    echo "ispx web runtime installed to $GOPATH/bin/ispx/"
fi

if ! is_truthy "$embed_runtime"; then
    build_ispxnative
fi
cp ../ispxnative/gdspx-* "$GOPATH/bin/"
