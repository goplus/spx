#!/bin/bash
# Read app name from appname.txt file
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $SCRIPT_DIR

# Pin Go toolchain version
export GOTOOLCHAIN=go1.24.4

go mod tidy
if ! go generate pkg/gengo/embedded_pkgs.go > /dev/null 2>&1; then
    echo "Error during go generate, showing full output:"
    go generate pkg/gengo/embedded_pkgs.go
fi

go mod tidy

target_font_dir=./template/project/engine/fonts/
mkdir -p $target_font_dir
font_path=$target_font_dir/CnFont.ttf
if [ ! -f "$font_path" ]; then
    curl -L https://github.com/goplus/godot/releases/download/spx2.0.14/CnFont.ttf -o "$font_path"
fi

if [ ! -f "$font_path" ]; then
    echo "can not find font or download it, please checkout your network " $font_path
    exit 1
fi

appname=$(cat appname.txt)
# install cmd
if [ "$OS" = "Windows_NT" ]; then
   appname="${appname}.exe"
fi

if [ "$OS" = "Windows_NT" ]; then
   # Fix for Windows MinGW linker duplicate symbol errors with Go 1.24
   go build -ldflags="-checklinkname=0 -extldflags=-Wl,--allow-multiple-definition" -o $appname
else
   go build -ldflags="-checklinkname=0" -o $appname
fi 
if [ "$OS" = "Windows_NT" ]; then
    IFS=';' read -r first_gopath _ <<< "$(go env GOPATH)"
    GOPATH="$first_gopath"
else
    IFS=':' read -r first_gopath _ <<< "$(go env GOPATH)"
    GOPATH="$first_gopath"
fi


if [ ! -f "$appname" ]; then
    echo "Error: $appname not found"
    exit 1
fi

mv $appname $GOPATH/bin/

# build and install gdspx (PC platform shared library)
echo "Building gdspx..."
cd ../igox || exit

go mod tidy

# Get architecture
GOARCH=$(go env GOARCH)

if [ "$OS" = "Windows_NT" ]; then
    # Windows: build DLL with proper naming
    GOOS_NAME="windows"
    EXT=".dll"
    LIB_NAME="gdspx-${GOOS_NAME}-${GOARCH}${EXT}"
    go build -ldflags="-checklinkname=0 -extldflags=-Wl,--allow-multiple-definition" -buildmode=c-shared -o "$LIB_NAME" .
    if [ -f "$LIB_NAME" ]; then
        cp -f "$LIB_NAME" "$GOPATH/bin/"
        echo "Installed $LIB_NAME to $GOPATH/bin/"
    else
        echo "Error: $LIB_NAME build failed"
        exit 1
    fi
elif [ "$(uname)" = "Darwin" ]; then
    # macOS: build dylib with proper naming
    GOOS_NAME="darwin"
    EXT=".dylib"
    LIB_NAME="gdspx-${GOOS_NAME}-${GOARCH}${EXT}"
    go build -ldflags="-checklinkname=0" -buildmode=c-shared -o "$LIB_NAME" .
    if [ -f "$LIB_NAME" ]; then
        cp -f "$LIB_NAME" "$GOPATH/bin/"
        echo "Installed $LIB_NAME to $GOPATH/bin/"
    else
        echo "Error: $LIB_NAME build failed"
        exit 1
    fi
else
    # Linux: build so with proper naming
    GOOS_NAME="linux"
    EXT=".so"
    LIB_NAME="gdspx-${GOOS_NAME}-${GOARCH}${EXT}"
    go build -ldflags="-checklinkname=0 -extldflags=-Wl,--allow-multiple-definition" -buildmode=c-shared -o "$LIB_NAME" .
    if [ -f "$LIB_NAME" ]; then
        cp -f "$LIB_NAME" "$GOPATH/bin/"
        echo "Installed $LIB_NAME to $GOPATH/bin/"
    else
        echo "Error: $LIB_NAME build failed"
        exit 1
    fi
fi


cd ../gox || exit

# build wasm (Web platform)
if [ "$1" = "--web" ]; then
    go env -w GOFLAGS="-buildvcs=false"
    cd ../igox || exit
    ./build.sh "$2"
    cp gdspx.wasm $GOPATH/bin/gdspx.wasm
    cp -f gdspx.wasm.br $GOPATH/bin/gdspx.wasm.br

    cd ../gox || exit
fi
