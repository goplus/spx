@echo off

set appname=spx.exe
go build -o %appname%
move %appname% "%GOPATH%\bin\"

call spx setupweb