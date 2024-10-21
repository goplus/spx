package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"godot-ext/gdspx/cmd/gdspx/pkg/impl"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	_ "embed"

	cp "github.com/otiai10/copy"
)

var (
	//go:embed template/engine/*
	engineFiles embed.FS

	//go:embed template/go.mod.txt
	go_mode_txt string

	//go:embed template/gop.mod.txt
	gop_mod_txt string

	//go:embed template/gitignore.txt
	gitignore string

	//go:embed template/main.go
	main_go string
)

func main() {
	impl.ReplaceTemplate(go_mode_txt, main_go, gitignore)
	impl.CheckPresetEnvironment()
	impl.TargetDir = "."
	if len(os.Args) > 2 {
		impl.TargetDir = os.Args[2]
	}
	if len(os.Args) <= 1 {
		showHelpInfo()
		return
	}
	switch os.Args[1] {
	case "help", "version":
		showHelpInfo()
		return
	case "clear":
		impl.StopWebServer()
		if impl.IsFileExist(path.Join(impl.TargetDir, ".godot")) {
			clearProject(impl.TargetDir)
		}
		return
	case "stopweb":
		impl.StopWebServer()
		return
	case "init":
		prepareGoEnv()
	}

	if !impl.IsFileExist(path.Join(impl.TargetDir, "go.mod")) {
		prepareGoEnv()
	}

	if err := execCmds(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func execCmds() error {
	impl.CopyEmbed(engineFiles, "template/engine", filepath.Join(impl.TargetDir, "engine"))
	return impl.ExecCmds(buildDll)
}

func showHelpInfo() {
	impl.ShowHelpInfo("spx")
}

func buildDll(project, outputPath string) {
	os.Remove(path.Join(project, "main.go"))
	rawdir, _ := os.Getwd()
	os.Chdir(project)
	envVars := []string{""}
	impl.RunGoplus(envVars, "build")
	os.Chdir(rawdir)
	os.Rename(path.Join(project, "gop_autogen.go"), path.Join(project, "main.go"))
	os.Remove(path.Join(project, "gdspx-demo.exe"))
	impl.BuildDll(project, outputPath)

}

type projctConfig struct {
	ExtAsset string `json:"extasset"`
}

const (
	extassetDir = "extasset"
)

func prepareGoEnv() {
	impl.PrepareGoEnv()
	impl.SetupFile(false, impl.TargetDir+"/gop.mod", gop_mod_txt)

	configPath := path.Join(impl.TargetDir, ".config")
	if impl.IsFileExist(configPath) && !impl.IsFileExist(path.Join(impl.TargetDir, extassetDir)) {
		file, err := os.Open(configPath)
		defer file.Close()
		ctx, err := io.ReadAll(file)
		if err != nil {
			log.Fatalf("read config error:" + err.Error())
		}
		var config projctConfig
		err = json.Unmarshal(ctx, &config)
		if err != nil {
			log.Fatalf("read config error:" + string(ctx) + err.Error())
		}
		println("src dir ", path.Join(impl.TargetDir, config.ExtAsset))
		err = cp.Copy(path.Join(impl.TargetDir, config.ExtAsset), path.Join(impl.TargetDir, extassetDir))
		if err != nil {
			log.Fatalf("Error copying directory: %v", err)
		}
	}
}

func clearProject(dir string) {
	deleteFilesAndDirs(dir)
	deleteImportFiles(dir)
}
func deleteFilesAndDirs(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		fullPath := filepath.Join(dir, file.Name())
		if file.Name() == "assets" || file.Name() == "res" || file.Name() == ".config" || strings.HasSuffix(fullPath, ".spx") {
			continue
		}

		if file.IsDir() {
			err = os.RemoveAll(fullPath)
			if err != nil {
				println(err.Error())
				return err
			}
		} else {
			err = os.Remove(fullPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
func deleteImportFiles(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".import") {
			err = os.Remove(path)
			if err != nil {
				return fmt.Errorf("failed to delete file: %v", err)
			}
		}

		return nil
	})
}
