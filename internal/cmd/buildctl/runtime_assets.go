package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goplus/spx/v2/internal/releasemeta"
)

const runtimeIndexJSON = `{"map":{"width":480,"height":360}}`

type runtimeWorkspace struct {
	repoRoot   string
	workDir    string
	goBinDir   string
	version    string
	outputPack string
}

func exportPackRuntime(runner scriptRunner) error {
	workspace, cleanup, err := prepareRuntimeWorkspace(runner.repoRootDir(), true)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runner.runCommand(workspace.workDir, "spx", "export"); err != nil {
		return err
	}

	exportedPack, err := findExportedPack(workspace.workDir)
	if err != nil {
		return err
	}
	if err := copyFile(exportedPack, workspace.outputPack); err != nil {
		return err
	}

	runtimeExtension := filepath.Join(workspace.goBinDir, "runtime.gdextension")
	if !fileExists(runtimeExtension) {
		return fmt.Errorf("runtime extension not found at %s", runtimeExtension)
	}

	dstZip := filepath.Join(workspace.goBinDir, fmt.Sprintf("gdspxrt.pck.%s.zip", workspace.version))
	return writeNamedZip(dstZip, map[string]string{
		"gdspxrt.pck":         workspace.outputPack,
		"runtime.gdextension": runtimeExtension,
	})
}

func exportWebRuntime(cfg runtimeExportWebConfig, runner scriptRunner) error {
	if err := installTools(toolInstallConfig{web: true, opt: true}, runner); err != nil {
		return err
	}

	spxCommand, err := webModeSPXCommand(cfg.mode)
	if err != nil {
		return err
	}
	outputZip, err := webModeOutputZip(cfg.mode)
	if err != nil {
		return err
	}

	workspace, cleanup, err := prepareRuntimeWorkspace(runner.repoRootDir(), false)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runner.runCommand(workspace.workDir, "spx", spxCommand); err != nil {
		return err
	}

	return zipDirectory(filepath.Join(workspace.workDir, "project", ".builds", "web"), filepath.Join(workspace.repoRoot, outputZip))
}

func exportWebTemplateRuntime(mode string, runner scriptRunner) error {
	if err := validateWebMode(mode); err != nil {
		return err
	}

	workspace, cleanup, err := prepareRuntimeWorkspace(runner.repoRootDir(), true)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runner.runCommand(workspace.workDir, "spx", "exporttemplateweb"); err != nil {
		return err
	}

	srcDir := filepath.Join(workspace.workDir, "project", ".builds", "webi")
	dstDir := filepath.Join(workspace.goBinDir, fmt.Sprintf("gdspxrt%s_web%s", workspace.version, mode))
	if err := os.RemoveAll(dstDir); err != nil {
		return err
	}
	if err := copyDir(srcDir, dstDir); err != nil {
		return err
	}

	enginePack := filepath.Join(dstDir, "engine.pck")
	engineZip := filepath.Join(dstDir, "engine.zip")
	if !fileExists(enginePack) {
		return fmt.Errorf("web runtime engine pack not found at %s", enginePack)
	}
	if err := os.Rename(enginePack, engineZip); err != nil {
		return err
	}

	engineJS := filepath.Join(dstDir, "engine.js")
	content, err := os.ReadFile(engineJS)
	if err != nil {
		return err
	}
	prefix := fmt.Sprintf("var EnginePackMode = '%s';\n", mode)
	return os.WriteFile(engineJS, append([]byte(prefix), content...), 0o644)
}

func prepareRuntimeWorkspace(repoRoot string, includeRuntimeExtension bool) (runtimeWorkspace, func(), error) {
	version, err := defaultRuntimeVersion()
	if err != nil {
		return runtimeWorkspace{}, nil, err
	}
	goPath, err := ensureGoPath()
	if err != nil {
		return runtimeWorkspace{}, nil, err
	}

	tempRoot := filepath.Join(repoRoot, ".tmp")
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	workDir, err := os.MkdirTemp(tempRoot, "runtime-*")
	if err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if err := os.MkdirAll(filepath.Join(workDir, "assets"), 0o755); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if err := os.MkdirAll(filepath.Join(goPath, "bin"), 0o755); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if err := os.WriteFile(filepath.Join(workDir, "assets", "index.json"), []byte(runtimeIndexJSON), 0o644); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if err := os.WriteFile(filepath.Join(workDir, "main.spx"), nil, 0o644); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if err := os.RemoveAll(filepath.Join(workDir, "project", ".builds")); err != nil {
		return runtimeWorkspace{}, nil, err
	}

	if includeRuntimeExtension {
		src := filepath.Join(repoRoot, "cmd", "spx", "template", "project", "runtime.gdextension.txt")
		dst := filepath.Join(goPath, "bin", "runtime.gdextension")
		if err := copyFile(src, dst); err != nil {
			return runtimeWorkspace{}, nil, err
		}
	}

	cleanup := func() {
		_ = os.RemoveAll(workDir)
	}
	return runtimeWorkspace{
		repoRoot:   repoRoot,
		workDir:    workDir,
		goBinDir:   filepath.Join(goPath, "bin"),
		version:    version,
		outputPack: filepath.Join(goPath, "bin", fmt.Sprintf("gdspxrt%s.pck", version)),
	}, cleanup, nil
}

func ensureGoPath() (string, error) {
	if goPath := os.Getenv("GOPATH"); goPath != "" {
		return goPath, nil
	}

	output, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return "", err
	}
	goPath := strings.TrimSpace(string(output))
	if goPath == "" {
		return "", fmt.Errorf("missing GOPATH")
	}
	return goPath, nil
}

func defaultRuntimeVersion() (string, error) {
	version := releasemeta.DefaultReleaseMeta().Runtime.Version
	if version == "" {
		return "", fmt.Errorf("releasemeta: Runtime.Version is empty")
	}
	return version, nil
}

func webModeOutputZip(mode string) (string, error) {
	if err := validateWebMode(mode); err != nil {
		return "", err
	}
	switch mode {
	case "normal":
		return "spx_web.zip", nil
	case "worker":
		return "spx_web_worker.zip", nil
	case "minigame":
		return "spx_web_minigame.zip", nil
	case "miniprogram":
		return "spx_web_miniprogram.zip", nil
	default:
		return "", fmt.Errorf("unsupported web-mode: %s", mode)
	}
}

func webModeSPXCommand(mode string) (string, error) {
	if err := validateWebMode(mode); err != nil {
		return "", err
	}
	switch mode {
	case "normal":
		return "exportweb", nil
	case "worker":
		return "exportwebworker", nil
	case "minigame":
		return "exportminigame", nil
	case "miniprogram":
		return "exportminiprogram", nil
	default:
		return "", fmt.Errorf("unsupported web-mode: %s", mode)
	}
}

func findExportedPack(workDir string) (string, error) {
	pcPack := filepath.Join(workDir, "project", ".builds", "pc", "gdexport.pck")
	if fileExists(pcPack) {
		return pcPack, nil
	}

	appResources, err := filepath.Glob(filepath.Join(workDir, "project", ".builds", "pc", "gdexport.app", "Contents", "Resources", "*.pck"))
	if err != nil {
		return "", err
	}
	sort.Strings(appResources)
	if len(appResources) > 0 {
		return appResources[0], nil
	}
	return "", fmt.Errorf("exported runtime pack not found in %s", workDir)
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	info, err := input.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

func writeNamedZip(dst string, namedFiles map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)

	names := make([]string, 0, len(namedFiles))
	for name := range namedFiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		src := namedFiles[name]
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate

		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		input, err := os.Open(src)
		if err != nil {
			return err
		}
		if _, err := io.Copy(entry, input); err != nil {
			input.Close()
			return err
		}
		input.Close()
	}
	return writer.Close()
}

func zipDirectory(srcDir, dstZip string) error {
	if !fileExists(srcDir) {
		return fmt.Errorf("source directory does not exist: %s", srcDir)
	}
	if err := os.MkdirAll(filepath.Dir(dstZip), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dstZip); err != nil {
		return err
	}

	var files []string
	if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(files)

	file, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)

	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate

		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(entry, input); err != nil {
			input.Close()
			return err
		}
		input.Close()
	}
	return writer.Close()
}
