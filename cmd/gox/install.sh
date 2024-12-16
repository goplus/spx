#!/bin/bash

appname="spx"
# install cmd
go build -o $appname
mv $appname $(go env GOPATH)/bin/

# install igox
spx setupweb