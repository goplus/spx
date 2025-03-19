#!/bin/bash

go mod tidy
cd cmd/gox || exit
./install.sh
cd ../../ || exit

if [ ! -d "gdspx" ]; then
	git clone https://github.com/realdream-ai/gdspx.git
    cd gdspx/cmd/gdspx/ || exit
    go install .
    cd ../../../ || exit
fi

cd gdspx || exit
make pc
cd .. || exit

echo "init succ"

