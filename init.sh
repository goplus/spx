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
./tools/init.sh
if [ "$1" == "-a" ] || [ "$1" == "-w" ]; then
    ./tools/init_web.sh 
fi
cd .. || exit

echo "init succ"

