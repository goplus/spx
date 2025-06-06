package main

import (
	"embed"

	"github.com/goplus/spx/v2/cmd/gox/pkg/command"
)

var (
	//go:embed template/project/*
	projectFS embed.FS

	//go:embed template/version
	version string

	//go:embed template/.gitignore.txt
	gitignoreTxt string

	//go:embed appname.txt
	appName string

	mainSh string
	runSh  string
)

func main() {
	cmd := &command.CmdTool{}

	// Initialize with default values
	cmd.ServerPort = 8005
	cmd.ProjectRelPath = "/project"
	cmd.BinPostfix = ""

	// Initialize with provided values
	cmd.ProjectFS = projectFS
	cmd.Version = version
	cmd.GitignoreTxt = gitignoreTxt
	cmd.RunSh = runSh
	cmd.MainSh = mainSh

	// Initialize the Args field if not already initialized
	cmd.RunCmd(appName, appName, version, projectFS, "template/project", "project")
}
