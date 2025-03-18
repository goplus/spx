#!/bin/bash
# Read app name from appname.txt file
appname=$(cat appname.txt)
# install cmd
go build -o $appname
mv $appname $(go env GOPATH)/bin/

cd ../igox || exit
GOOS=js GOARCH=wasm go build -o $(go env GOPATH)/bin/igdspx.wasm
cd ../gox || exit