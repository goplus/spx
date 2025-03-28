@echo off
setlocal enabledelayedexpansion

go mod tidy
:: Read app name from appname.txt file
set /p appname=<appname.txt
:: Build and install the app
go build -o %appname%.exe
move %appname%.exe "%GOPATH%\bin\"

if "%~1" == "--web" (
    rem Build WebAssembly file
    cd ..\igox
    set GOOS=js
    set GOARCH=wasm
    go build -o "%GOPATH%\bin\igdspx.wasm"
    cd ..\gox
)
