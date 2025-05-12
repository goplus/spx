#!/bin/bash
# Read app name from appname.txt file

go mod tidy
./setup_font.sh
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

if [ "$1" = "--web" ]; then
    cd ../igox || exit
    go mod tidy
    GOOS=js GOARCH=wasm go build  -trimpath -ldflags "-s -w"  -o $GOPATH/bin/igdspx_raw.wasm
    wasm-opt -Oz --enable-bulk-memory -o $GOPATH/bin/igdspx.wasm $GOPATH/bin/igdspx_raw.wasm
    cd ../gox || exit
fi