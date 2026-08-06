#!/bin/bash
set -euo pipefail

# Read app name from appname.txt file
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Pin Go toolchain version
export GOTOOLCHAIN=go1.25.8

. "$SCRIPT_DIR/../internal/macos_go_toolchain.sh"
configure_macos_go_toolchain

appname=$(cat appname.txt)
os_name="${OS:-}"
GOPATH="$(go env GOPATH)"
gopath_bin_dir="$GOPATH/bin"
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
            echo "Warning: ignoring unknown install.sh flag: $arg"
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
runtime_asset_manifest="$runtime_asset_dir/manifest.json"
runtime_version=""
runtime_name=""
runtime_pack=""
runtime_goos=""
runtime_goarch=""
runtime_lib_name=""

cleanup_embedded_runtime_assets() {
    mkdir -p "$runtime_asset_dir"
    find "$runtime_asset_dir" -maxdepth 1 -type f ! -name "$(basename "$runtime_asset_placeholder")" -delete
}

require_file() {
    local path="$1"
    local description="$2"

    if [ ! -f "$path" ]; then
        echo "Error: $description not found: $path"
        exit 1
    fi
}

resolve_runtime_asset_names() {
    local goexe libext

    runtime_version="$(GOFLAGS="-buildvcs=false" go run ../../.github/scripts/runtime_version.go | tr -d '\r\n')"
    runtime_goos="$(go env GOOS | tr -d '\r\n')"
    runtime_goarch="$(go env GOARCH | tr -d '\r\n')"
    goexe="$(go env GOEXE | tr -d '\r\n')"
    runtime_name="gdspxrt${runtime_version}${goexe}"
    runtime_pack="gdspxrt${runtime_version}.pck"

    case "$runtime_goos" in
        darwin)
            libext="dylib"
            ;;
        linux)
            libext="so"
            ;;
        windows)
            libext="dll"
            ;;
        *)
            echo "Error: unsupported runtime OS: $runtime_goos"
            exit 1
            ;;
    esac
    runtime_lib_name="gdspx-${runtime_goos}-${runtime_goarch}.${libext}"
}

ensure_runtime_assets_for_embedding() {
    resolve_runtime_asset_names

    if [ -f "$gopath_bin_dir/$runtime_name" ] && [ -f "$gopath_bin_dir/$runtime_pack" ]; then
        return 0
    fi

    echo "Preparing runtime assets for embedded SPX build..."
    GOFLAGS="-buildvcs=false" go run ../../internal/cmd/buildctl engine download --runtime
}

build_ispxnative() {
    ( cd ../ispxnative && ./build.sh )
}

copy_ispxnative_runtime_lib() {
    local destination_dir="$1"

    resolve_runtime_asset_names
    mkdir -p "$destination_dir"
    require_file "../ispxnative/$runtime_lib_name" "runtime shared library"
    cp "../ispxnative/$runtime_lib_name" "$destination_dir/"
}

write_runtime_asset_manifest() {
    GOFLAGS="-buildvcs=false" go run ../../.github/scripts/runtime_asset_manifest.go \
        "$runtime_name=$gopath_bin_dir/$runtime_name" \
        "$runtime_pack=$gopath_bin_dir/$runtime_pack" \
        "$runtime_lib_name=../ispxnative/$runtime_lib_name" > "$runtime_asset_manifest"
}

stage_embedded_runtime_assets() {
    resolve_runtime_asset_names

    require_file "$gopath_bin_dir/$runtime_name" "runtime executable"
    require_file "$gopath_bin_dir/$runtime_pack" "runtime pack"
    require_file "../ispxnative/$runtime_lib_name" "runtime shared library"

    cleanup_embedded_runtime_assets
    cp "$gopath_bin_dir/$runtime_name" "$runtime_asset_dir/"
    cp "$gopath_bin_dir/$runtime_pack" "$runtime_asset_dir/"
    copy_ispxnative_runtime_lib "$runtime_asset_dir"
    write_runtime_asset_manifest
}

build_spx_binary() {
    if [ "$os_name" = "Windows_NT" ]; then
       # Fix for Windows MinGW linker duplicate symbol errors with Go 1.24
       go build -ldflags="-extldflags=-Wl,--allow-multiple-definition" -o "$appname"
    else
       go build -o "$appname"
    fi
}

install_web_runtime() {
    echo "Installing ispx web runtime..."
    rm -rf "$gopath_bin_dir/ispx"
    mkdir -p "$gopath_bin_dir/ispx"
    cp ../ispx/web/* "$gopath_bin_dir/ispx/"
    echo "Installed ispx web runtime to $gopath_bin_dir/ispx/"
}

if is_truthy "$embed_runtime"; then
    cleanup_embedded_runtime_assets
    trap cleanup_embedded_runtime_assets EXIT
    ensure_runtime_assets_for_embedding
fi

build_ispxnative

if is_truthy "$embed_runtime"; then
    stage_embedded_runtime_assets
fi

# install cmd
if [ "$os_name" = "Windows_NT" ]; then
   appname="${appname}.exe"
fi

build_spx_binary
require_file "$appname" "built spx binary"
mv "$appname" "$gopath_bin_dir/"

# build and install ispx
echo "Building ispx..."

if [ "$install_web" -eq 1 ]; then
    ( cd ../ispx && GOFLAGS="-buildvcs=false" ./build.sh )
    cp ../ispx/ispx.wasm "$gopath_bin_dir/"
    # A non-optimized build must not leave a compressed artifact from an older
    # runtime behind. Optimized workflows recreate it after installation.
    rm -f "$gopath_bin_dir/ispx.wasm.br"

    install_web_runtime
fi

copy_ispxnative_runtime_lib "$gopath_bin_dir"
