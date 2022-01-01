package mac

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/qiniu/x/log"

	"github.com/goplus/spx/cmd/internal/base"
)

const (
	spxExecVersion = "1.0"
	autoGenFile    = "gop_autogen.go"
	autoGenExec    = "gop_spx"
)

var darwinData = map[string]string{

	"scripts/postinstall": `#!/bin/bash
echo "Fixing permissions"
find . -exec chmod ugo+r \{\} \;
find bin -exec chmod ugo+rx \{\} \;
find . -type d -exec chmod ugo+rx \{\} \;
chmod o-w .
rm -rf goplus.gopSpx.pkg
`,

	"scripts/preinstall": `#!/bin/bash
echo "Removing previous installation"
`,

	"Distribution": `<?xml version="1.0" encoding="utf-8" standalone="no"?>
<installer-script minSpecVersion="1.000000">
    <title>gopSpx</title>
    <background mime-type="image/png" file="dialog.png"/>
    <license file="LICENSE"/>
    <welcome file="WELCOME" />
    <options customize="never" allow-external-scripts="no"/>
    <domains enable_localSystem="true" />
    <installation-check script="installCheck();"/>
    <script>
function installCheck() {
    return true;
}
    </script>
    <choices-outline>
        <line choice="goplus.gopSpx.choice"/>
    </choices-outline>
    <choice id="goplus.gopSpx.choice" title="gopSpx">
        <pkg-ref id="goplus.gopSpx.pkg"/>
    </choice>
    <pkg-ref id="goplus.gopSpx.pkg" auth="Root">goplus.gopSpx.pkg</pkg-ref>
</installer-script>
`,
}

func darwinPKG(cwd string) error {

	work := filepath.Join(cwd, "darwinpkg")
	if err := os.MkdirAll(work, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(work)

	darwinDir := filepath.Join(work, "darwin")
	if err := base.WriteDataFiles(darwinData, darwinDir); err != nil {
		return err
	}

	execFile := filepath.Join(work, autoGenExec)
	base.RunGoCmd(work, "build", "-o", execFile, "..")

	assetsDir := filepath.Join(cwd, "assets")
	workassetsDir := filepath.Join(work, "assets")
	base.CpDir(workassetsDir, assetsDir)

	pkg := filepath.Join(cwd, "pkg") // known to cmd/release
	if _, err := os.Stat(pkg); err != nil {
		if err := os.Mkdir(pkg, 0755); err != nil {
			return err
		}
	}

	installlocation := filepath.Join(os.Getenv("HOME"), "spx")
	if _, err := os.Stat(installlocation); err != nil {
		if err := os.Mkdir(installlocation, 0755); err != nil {
			return err
		}
	}

	if err := base.Run(work, "pkgbuild",
		"--identifier", "org.goplus.spx",
		"--version", spxExecVersion,
		"--install-location", installlocation,
		"--scripts", "darwin/scripts",
		"--root", work,
		filepath.Join(work, "goplus.gopSpx.pkg"),
	); err != nil {
		return err
	}

	os.Remove(filepath.Join(pkg, "spx-"+spxExecVersion+".pkg"))
	err := base.Run(work, "productbuild",
		"--distribution", "darwin/Distribution",
		"--resources", "darwin/Resources",
		"--package-path", work,
		filepath.Join(pkg, "spx-"+spxExecVersion+".pkg"), // file name irrelevant
	)
	return err
}

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

	err = darwinPKG(dir)
	if err != nil {
		fmt.Println(err)
		return
	}

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
