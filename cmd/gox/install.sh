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

go env -w GOFLAGS="-buildvcs=false"
if [ "$1" = "--web" ]; then
    cd ../igox || exit
    ./build.sh "$2"
    cp gdspx.wasm $GOPATH/bin/gdspx.wasm
    cd ../gox || exit
fi