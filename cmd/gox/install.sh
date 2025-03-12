#!/bin/bash
# Read app name from appname.txt file
appname=$(cat appname.txt)
# install cmd
go build -o $appname
mv $appname $(go env GOPATH)/bin/
