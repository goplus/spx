

if [ "$1" != "" ]; then
    exit 1
fi

./build.sh

python -m http.server 13511


