package main

import (
	"embed"
	"strings"

	"github.com/goplus/spx/v2/cmd/gox/pkg/command"
)

var (
	//go:embed template/platform/*
	platformFS embed.FS

	//go:embed template/project/*
	projectFS embed.FS

	//go:embed template/version
	version string

	//go:embed template/.gitignore.txt
	gitignoreTxt string

	//go:embed template/go.mod.template
	gomodtemplate string

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
	cmd.PlatformFS = platformFS
	cmd.Version = strings.TrimSpace(version)
	cmd.GitignoreTxt = gitignoreTxt
	cmd.RunSh = runSh
	cmd.MainSh = mainSh
	cmd.GoModTemplate = gomodtemplate

	// Initialize the Args field if not already initialized
	err := cmd.RunCmd(appName, appName, cmd.Version, projectFS, "template/project", "project")
	if err != nil {
		println("failed to run cmd:", err)
		return
	}
}
