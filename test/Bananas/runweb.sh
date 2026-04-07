
mkdir  res/
GOEXPERIMENT=noregabi GOOS=js GOARCH=wasm go build --tags canvas -o test.wasm
cp -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" ./go.wasm.exec.js
cp -f "$(go env GOROOT)/lib/wasm/wasm_exec.html" ./go.wasm.exec.html
cp -f -p ../shared-assets/* ./res/


echo '// test.go
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
)

var (
	listen = flag.String("listen", ":8080", "listen address")
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
rm -rf go.wasm.exec.js
rm -rf go.wasm.exec.html
rm -rf server.go
rm -rf res/
rm -rf *.wasm
