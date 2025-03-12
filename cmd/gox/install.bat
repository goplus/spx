@echo off
setlocal enabledelayedexpansion

:: Read app name from appname.txt file
set /p appname=<appname.txt
go build -o %appname%.exe
move %appname%.exe "%GOPATH%\bin\"
