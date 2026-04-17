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
web_mode="${1:-}"
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
GOPATH="$(go env GOPATH)"


if [ ! -f "$appname" ]; then
    echo "Error: $appname not found"
    exit 1
fi

mv "$appname" "$GOPATH/bin/"

# build and install ispx
echo "Building ispx..."

if [ "$web_mode" = "--web" ]; then
    ( cd ../ispx && GOFLAGS="-buildvcs=false" ./build.sh )
    cp ../ispx/ispx.wasm "$GOPATH/bin/"

    # Install ispx web runtime
    echo "Installing ispx web runtime..."
    rm -rf "$GOPATH/bin/ispx"
    mkdir -p "$GOPATH/bin/ispx"
    cp ../ispx/web/* "$GOPATH/bin/ispx/"
    echo "ispx web runtime installed to $GOPATH/bin/ispx/"
fi

( cd ../ispxnative && ./build.sh )
cp ../ispxnative/gdspx-* "$GOPATH/bin/"
