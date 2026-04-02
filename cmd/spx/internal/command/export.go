package command

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/cmd/spx/internal/util"
)

// ExportBuild runs a platform export with the current Godot project.
func (cmd *CmdTool) ExportBuild(platform string) error {
	fmt.Printf("Starting export: platform=%s, ProjectDir=%s\n", platform, cmd.ProjectDir)
	os.MkdirAll(filepath.Join(cmd.ProjectDir, ".builds", strings.ToLower(platform)), 0o755)
	execCmd := exec.Command(cmd.CmdPath, "--headless", "--quit", "--path", cmd.ProjectDir, "--export-debug", platform)
	err := execCmd.Run()
	if err != nil {
		fmt.Println("Error exporting to web:", err)
	}
	return err
}

// Export exports the current project for the host desktop platform.
func (cmd *CmdTool) Export() error {
	targetDir := filepath.Join(cmd.ProjectDir, ".builds", "pc")
	targetPath := filepath.Join(targetDir, PcExportName)
	platformName := ""
	if runtime.GOOS == "windows" {
		targetPath += ".exe"
		platformName = "Win"
	} else if runtime.GOOS == "darwin" {
		platformName = "Mac"
		targetPath += ".app"
	} else if runtime.GOOS == "linux" {
		platformName = "Linux"
	}

	os.Mkdir(targetDir, 0o755)
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, "--headless", "--quit", "--path", cmd.ProjectDir, "--export-debug", platformName, targetPath)
}

func (cmd *CmdTool) prepareExport() error {
	projectDir, _ := filepath.Abs(cmd.ProjectDir)
	util.CopyDir2(filepath.Join(projectDir, "..", "assets"), filepath.Join(cmd.ProjectDir, "assets"))
	return nil
}
