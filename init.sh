#!/bin/bash

go mod tidy
cd cmd/spx || exit
go install .
cd ../../ || exit

if [ ! -d "gdspx" ]; then
	git clone https://github.com/realdream-ai/gdspx.git
    cd gdspx/cmd/gdspx/ || exit
    go install .
    cd ../../../ || exit
fi

cd gdspx || exit
make pc
if [ "$1" == "-a" ] || [ "$1" == "-w" ]; then
    make web
fi
cd .. || exit

echo "============= init succ =============="
echo "Now you can type 'spx run ./tutorial/00-Hello' to run the first tutorial."

