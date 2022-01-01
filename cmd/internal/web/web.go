package web

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/qiniu/x/log"

	"github.com/goplus/spx/cmd/internal/base"
)

const runwebsh string = `mkdir  res/
GOEXPERIMENT=noregabi GOOS=js GOARCH=wasm go build --tags canvas -o test.wasm
cp -f "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./
cp -f "$(go env GOROOT)/misc/wasm/wasm_exec.html" ./
cp -f -p ../res/* ./res/


echo '// server.go
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
open "http://127.0.0.1:8080/wasm_exec.html"
echo "open http://127.0.0.1:8080/wasm_exec.html"
go run server.go
rm -rf wasm_exec.js
rm -rf wasm_exec.html
rm -rf server.go
rm -rf res/
rm -rf *.wasm `

const (
	autoGenFile = "gop_autogen.go"
)

// -----------------------------------------------------------------------------

// Cmd - gop build
var Cmd = &base.Command{
	UsageLine: "gopspx web [-v] <gopSrcDir>",
	Short:     "",
}

var (
	flag = &Cmd.Flag
	_    = flag.Bool("v", false, "print verbose information.")
)

func init() {
	Cmd.Run = runCmd
}

func runWeb(dir string) {
	var err error
	file := filepath.Join(dir, autoGenFile)
	if _, err = os.Stat(file); err != nil {
		fmt.Printf(" %s no exist!  use ` gop build .` gen\n", file)
		return
	}

	base.RunBashCmd(dir, runwebsh)

}

func runCmd(_ *base.Command, args []string) {
	err := flag.Parse(args)
	if err != nil {
		log.Fatalln("parse input arguments failed:", err)
	}
	var dir string
	if flag.NArg() == 0 {
		dir = "."
	} else {
		dir = flag.Arg(0)
	}
	runWeb(dir)
}
