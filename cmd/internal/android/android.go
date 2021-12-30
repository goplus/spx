package android

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/qiniu/x/log"

	"github.com/goplus/spx/cmd/internal/base"
)

const runandroidsh string = `
go build .
docker build -t go4droid . # or docker pull mpl7/go4droid
mkdir $HOME/.gradle # for caching
go get -d golang.org/x/mobile/example/bind/...
cd $GOPATH/src/golang.org/x/mobile/example/bind/android
docker run --rm -v "$PWD":/home/gopher/project -v $HOME/.gradle:/home/gopher/.gradle -w /home/gopher/project --name go4droid -i -t go4droid /bin/bash
gomobile bind -o app/hello.aar -target=android golang.org/x/mobile/example/bind/hello
gradle wrapper --gradle-version 4.4 # only needed once, to generate the gradle wrapper.
./gradlew assembleDebug
`

const (
	autoGenFile = "gop_autogen.go"
)

// -----------------------------------------------------------------------------

// Cmd - gop build
var Cmd = &base.Command{
	UsageLine: "gopspx mac [-v] <gopSrcDir>",
	Short:     "",
}

var (
	flag = &Cmd.Flag
	_    = flag.Bool("v", false, "print verbose information.")
)

func init() {
	Cmd.Run = runCmd
}

func runMac(dir string) {
	var err error
	file := filepath.Join(dir, autoGenFile)
	if _, err = os.Stat(file); err != nil {
		fmt.Printf(" %s no exist!  use ` gop build .` gen\n", file)
		return
	}

	base.RunBashCmd(dir, runandroidsh)

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
	runMac(dir)
}
