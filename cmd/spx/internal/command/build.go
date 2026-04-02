package command

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/goplus/spx/v2/cmd/spx/internal/gengo"
	"github.com/goplus/spx/v2/cmd/spx/internal/util"
)

func (cmd *CmdTool) BuildWasm() error {
	cmd.genGo()

	webBuildDir := path.Join(cmd.ProjectDir, ".builds/web/")
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create web build directory: %w", err)
	}
	filePath := path.Join(webBuildDir, "ispx.wasm")

	return cmd.withGoDir(func() error {
		log.Printf("Building WebAssembly binary: %s", filePath)
		envVars := []string{"GOOS=js", "GOARCH=wasm"}

		util.RunGolang(envVars, "build", "-o", filePath)
		return nil
	})
}

// BuildTinyGoLib builds a TinyGo static library.
func (cmd *CmdTool) BuildTinyGoLib() error {
	cmd.genGo()

	target := *cmd.Args.Target
	if target == "" || target == "esp32" {
		target = "esp32-coreboard-v2"
	}

	tinyGoBuildDir := path.Join(cmd.ProjectDir, ".builds/tinygo/")
	if err := os.MkdirAll(tinyGoBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create TinyGo build directory: %w", err)
	}
	outputPath := path.Join(tinyGoBuildDir, "golib.o")

	args := []string{
		"build",
		"-o", outputPath,
		"-target=" + target,
		"-no-debug",
		"-opt=2",
		"-gc=leaking",
		"-scheduler=none",
	}
	if tags := *cmd.Args.Tags; tags != "" {
		args = append(args, "-tags="+tags)
	}
	args = append(args, ".")

	envVars := []string{"GODEBUG=gotypesalias=0"}

	if err := cmd.withGoDir(func() error {
		log.Printf("Building TinyGo static library for target: %s", target)
		if err := util.RunTinyGo(envVars, args...); err != nil {
			return fmt.Errorf("TinyGo build failed: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	log.Printf("TinyGo static library built successfully: %s", outputPath)
	return nil
}

func (cmd *CmdTool) BuildDll() error {
	if err := cmd.hideIOSFiles(); err != nil {
		return err
	}

	targetArchs, err := cmd.determineTargetArchs()
	if err != nil {
		return err
	}

	tagStr := cmd.genGo()

	return cmd.withGoDir(func() error {
		if err := cmd.executeDllBuild(targetArchs, tagStr); err != nil {
			return err
		}
		if cmd.LibPath == "" {
			return fmt.Errorf("build error: cannot find matched dylib for runtime arch %s", runtime.GOARCH)
		}
		return nil
	})
}

// withGoDir runs f in GoDir.
func (cmd *CmdTool) withGoDir(f func() error) error {
	rawdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	if err := os.Chdir(cmd.GoDir); err != nil {
		return fmt.Errorf("failed to change directory to GoDir %s: %w", cmd.GoDir, err)
	}

	defer func() {
		if err := os.Chdir(rawdir); err != nil {
			log.Printf("Warning: Failed to restore working directory to %s: %v", rawdir, err)
		}
	}()

	return f()
}

// hideIOSFiles renames ios* files to *.txt.
func (cmd *CmdTool) hideIOSFiles() error {
	searchPattern := filepath.Join(cmd.ProjectDir, "go", "ios*")
	files, err := filepath.Glob(searchPattern)
	if err != nil {
		log.Printf("Warning: Glob failed for pattern %s: %v", searchPattern, err)
		return nil
	}

	for _, file := range files {
		if !strings.HasSuffix(file, ".txt") {
			newName := file + ".txt"
			if err := os.Rename(file, newName); err != nil {
				log.Printf("Warning: Failed to rename %s to %s: %v", file, newName, err)
			}
		}
	}
	return nil
}

// determineTargetArchs resolves target architectures.
func (cmd *CmdTool) determineTargetArchs() ([]string, error) {
	if runtime.GOOS == "darwin" {
		return []string{"amd64", "arm64"}, nil
	}

	tarArch := *cmd.Args.Arch
	if tarArch == "" {
		return []string{runtime.GOARCH}, nil
	}

	var validArchs []string
	switch runtime.GOOS {
	case "windows":
		validArchs = []string{"amd64", "386"}
	case "darwin":
		validArchs = []string{"amd64", "arm64"}
	case "linux":
		validArchs = []string{"amd64", "arm", "arm64", "386"}
	default:
		validArchs = []string{runtime.GOARCH}
	}

	if tarArch == "all" {
		return validArchs, nil
	}

	if slices.Contains(validArchs, tarArch) {
		return []string{tarArch}, nil
	}

	return nil, fmt.Errorf("invalid arch %s. Valid archs for %s: %s",
		tarArch, runtime.GOOS, strings.Join(validArchs, ","))
}

func (cmd *CmdTool) genGo() string {
	rawdir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	spxProjPath := filepath.Join(cmd.ProjectDir, "..")

	if cmd.UseXgobuildForCodegen {
		if err := cmd.genGoUsingXgobuild(rawdir, spxProjPath); err != nil {
			log.Fatalf("Code generation failed using xgobuild: %v", err)
		}
	} else {
		if err := cmd.genGoUsingXgoCLI(rawdir, spxProjPath); err != nil {
			log.Fatalf("Code generation failed using xgo CLI: %v", err)
		}
	}

	return cmd.SafeTagArgs()
}

// genGoUsingXgobuild uses xgobuild.
func (cmd *CmdTool) genGoUsingXgobuild(rawdir, spxProjPath string) error {
	if err := os.MkdirAll(cmd.GoDir, 0755); err != nil {
		return fmt.Errorf("failed to create GoDir: %w", err)
	}
	outputPath := path.Join(cmd.GoDir, "main.go")

	fsys := gengo.NewDirFS(spxProjPath)
	if err := gengo.GenGoFromFS(fsys, outputPath); err != nil {
		return fmt.Errorf("failed to generate Go code using xgobuild: %w", err)
	}

	if err := os.Chdir(spxProjPath); err != nil {
		return fmt.Errorf("failed to change directory to project root for mod tidy: %w", err)
	}

	defer func() {
		if err := os.Chdir(rawdir); err != nil {
			log.Printf("Warning: Failed to restore working directory to %s: %v", rawdir, err)
		}
	}()

	util.RunGolang(nil, "mod", "tidy")

	return nil
}

// genGoUsingXgoCLI uses xgo.
func (cmd *CmdTool) genGoUsingXgoCLI(rawdir, spxProjPath string) error {
	if err := os.Chdir(spxProjPath); err != nil {
		return fmt.Errorf("failed to change directory to project root for XGo: %w", err)
	}
	defer func() {
		if err := os.Chdir(rawdir); err != nil {
			log.Printf("Warning: Failed to restore working directory to %s: %v", rawdir, err)
		}
	}()

	tagStr := cmd.SafeTagArgs()
	log.Printf("genGo tagStr: %s", tagStr)
	envVars := []string{""}

	args := []string{"go"}
	if tagStr != "" {
		args = append(args, tagStr)
	}
	util.RunXGo(envVars, args...)

	if err := os.MkdirAll(cmd.GoDir, 0755); err != nil {
		return fmt.Errorf("failed to create GoDir: %w", err)
	}

	sourceFile := path.Join(spxProjPath, "xgo_autogen.go")
	destFile := path.Join(cmd.GoDir, "main.go")

	if err := os.Rename(sourceFile, destFile); err != nil {
		return fmt.Errorf("failed to rename/move generated file %s to %s: %w", sourceFile, destFile, err)
	}

	util.RunGolang(nil, "mod", "tidy")

	return nil
}

// executeDllBuild runs the multi-arch C-shared build.
func (cmd *CmdTool) executeDllBuild(archs []string, tagStr string) error {
	rawPath := filepath.Base(cmd.LibPath)
	rawDir := filepath.Dir(cmd.LibPath)

	cmd.LibPath = ""
	baseEnvs := []string{"CGO_ENABLED=1"}

	buildArgs := []string{"build"}
	if tagStr != "" {
		buildArgs = append(buildArgs, tagStr)
	}
	buildArgs = append(buildArgs, "-buildmode=c-shared")

	strs := strings.Split(rawPath, "-")
	if len(strs) < 3 {
		return fmt.Errorf("unexpected library path format: %s. Expected format like base-ver-arch.ext", rawPath)
	}
	baseName := strings.Join(strs[:2], "-")

	extParts := strings.Split(strs[2], ".")
	fileExt := extParts[len(extParts)-1]

	for _, arch := range archs {
		newPath := filepath.Join(rawDir, fmt.Sprintf("%s-%s.%s", baseName, arch, fileExt))

		if arch == runtime.GOARCH {
			cmd.LibPath = newPath
		}

		envs := append(baseEnvs, "GOARCH="+arch)
		currentArgs := append(buildArgs, "-o", newPath)

		log.Printf("Building shared library: envs=%s, args=%s", envs, currentArgs)
		util.RunGolang(envs, currentArgs...)
	}
	return nil
}
