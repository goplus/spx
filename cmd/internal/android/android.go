package android

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/qiniu/x/log"

	"github.com/goplus/spx/cmd/internal/base"
)

const runandroidsh string = `
docker pull mpl7/go4droid
mkdir -p $HOME/.gradle
docker run --rm -v "$PWD":/home/gopher/project -v $HOME/.gradle:/home/gopher/.gradle -w /home/gopher/project --name go4droid -i -t mpl7/go4droid /bin/bash -c "gomobile build --tags canvas  -target=android"
`

const (
	autoGenFile = "gop_autogen.go"
)

// -----------------------------------------------------------------------------

// Cmd - gop build
var Cmd = &base.Command{
	UsageLine: "gopspx android [-v] <gopSrcDir>",
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
