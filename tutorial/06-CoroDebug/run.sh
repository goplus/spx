
rm -rf wasm_exec.js
rm -rf index.html
rm -rf server.go
rm -rf *.wasm

if [ "$1" == "-i" ]; then
	lastdir=$(pwd)
    cd ../../cmd/ispx || exit
	cp -f "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./
	./runweb.sh
	cd $lastdir || exit
	exit 0
fi

gop go 
GOEXPERIMENT=noregabi GOOS=js GOARCH=wasm go build --tags canvas -o test.wasm
cp -f "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./
cp -f "$(go env GOROOT)/misc/wasm/wasm_exec.html" ./index.html



echo '// test.go
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
)

var (
	listen = flag.String("listen", ":13511", "listen address")
	dir    = flag.String("dir", ".", "directory to serve")
)

func main() {
	flag.Parse()
	log.Printf("listening on %q...", *listen)
	log.Fatal(http.ListenAndServe(*listen, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, ".wasm") {
			resp.Header().Set("content-type", "application/wasm")
		}

		http.FileServer(http.Dir(*dir)).ServeHTTP(resp, req)
	})))
}'> server.go
go run server.go