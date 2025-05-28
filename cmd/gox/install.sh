#!/bin/bash
# Read app name from appname.txt file

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

go build -o $appname
if [ "$OS" = "Windows_NT" ]; then
    IFS=';' read -r first_gopath _ <<< "$(go env GOPATH)"
    GOPATH="$first_gopath"
else
    IFS=':' read -r first_gopath _ <<< "$(go env GOPATH)"
    GOPATH="$first_gopath"
fi

mv $appname $GOPATH/bin/

go env -w GOFLAGS="-buildvcs=false"
if [ "$1" = "--web" ]; then
    cd ../igox || exit
    ./build.sh "$2"
    cp gdspx.wasm $GOPATH/bin/gdspx.wasm
    cd ../gox || exit
fi