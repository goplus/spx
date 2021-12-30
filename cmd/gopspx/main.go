package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/goplus/spx/cmd/internal/base"
	"github.com/goplus/spx/cmd/internal/help"
	"github.com/goplus/spx/cmd/internal/mac"
	"github.com/goplus/spx/cmd/internal/web"
	"github.com/qiniu/x/log"
)

func mainUsage() {
	help.PrintUsage(os.Stderr, base.GopSpx)
	os.Exit(2)
}

func init() {
	base.Usage = mainUsage
	base.GopSpx.Commands = []*base.Command{
		web.Cmd,
		mac.Cmd,
	}
}

func main() {
	flag.Usage = base.Usage
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		base.Usage()
	}
	log.SetFlags(log.Ldefault &^ log.LstdFlags)

	base.CmdName = args[0] // for error messages
	if args[0] == "help" {
		help.Help(os.Stderr, args[1:])
		return
	}

BigCmdLoop:
	for bigCmd := base.GopSpx; ; {
		for _, cmd := range bigCmd.Commands {
			if cmd.Name() != args[0] {
				continue
			}
			args = args[1:]
			if len(cmd.Commands) > 0 {
				bigCmd = cmd
				if len(args) == 0 {
					help.PrintUsage(os.Stderr, bigCmd)
					os.Exit(2)
				}
				if args[0] == "help" {
					help.Help(os.Stderr, append(strings.Split(base.CmdName, " "), args[1:]...))
					return
				}
				base.CmdName += " " + args[0]
				continue BigCmdLoop
			}
			if !cmd.Runnable() {
				continue
			}
			cmd.Run(cmd, args)
			return
		}
		helpArg := ""
		if i := strings.LastIndex(base.CmdName, " "); i >= 0 {
			helpArg = " " + base.CmdName[:i]
		}
		fmt.Fprintf(os.Stderr, "gop %s: unknown command\nRun 'gop help%s' for usage.\n", base.CmdName, helpArg)
		os.Exit(2)
	}
}
