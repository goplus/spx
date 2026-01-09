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

	"github.com/goplus/spx/v2/cmd/gox/pkg/gengo"
	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

// withGoDir executes a function f inside the pself.GoDir and ensures
// the original working directory is restored via defer.
func (pself *CmdTool) withGoDir(f func() error) error {
	rawdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	if err := os.Chdir(pself.GoDir); err != nil {
		return fmt.Errorf("failed to change directory to GoDir %s: %w", pself.GoDir, err)
	}

	// Defer restoration of the original directory
	defer func() {
		if err := os.Chdir(rawdir); err != nil {
			log.Printf("Warning: Failed to restore working directory to %s: %v", rawdir, err)
		}
	}()

	return f()
}

// restoreFiles undoes renaming of files matching 'ios*' in the 'go' subdirectory.
func (pself *CmdTool) restoreFiles() error {
	searchPattern := filepath.Join(pself.ProjectDir, "go", "ios*")
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

// determineTargetArchs calculates the list of architectures to build for.
func (pself *CmdTool) determineTargetArchs() ([]string, error) {
	// If running on darwin, unconditionally build for both amd64 and arm64, ignoring Args.Arch.
	if runtime.GOOS == "darwin" {
		return []string{"amd64", "arm64"}, nil
	}

	tarArch := *pself.Args.Arch
	if tarArch == "" {
		// If no target arch specified, use the current runtime arch.
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

	// Check if the explicitly provided target arch is valid for the current OS.
	if slices.Contains(validArchs, tarArch) {
		return []string{tarArch}, nil
	}

	return nil, fmt.Errorf("invalid arch %s. Valid archs for %s: %s",
		tarArch, runtime.GOOS, strings.Join(validArchs, ","))
}

// =================================================================
// Generate Go
// =================================================================

func (pself *CmdTool) genGo() string {
	rawdir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	spxProjPath := filepath.Join(pself.ProjectDir, "..")

	if pself.UseXgobuildForCodegen {
		if err := pself.genGoUsingXgobuild(rawdir, spxProjPath); err != nil {
			log.Fatalf("Code generation failed using xgobuild: %v", err)
		}
	} else {
		if err := pself.genGoUsingXgoCLI(rawdir, spxProjPath); err != nil {
			log.Fatalf("Code generation failed using xgo CLI: %v", err)
		}
	}

	// Return tags string for subsequent build steps, common to both methods
	return pself.SafeTagArgs()
}

// genGoUsingXgobuild generates Go code using xgobuild library (new method)
func (pself *CmdTool) genGoUsingXgobuild(rawdir, spxProjPath string) error {
	if err := os.MkdirAll(pself.GoDir, 0755); err != nil {
		return fmt.Errorf("failed to create GoDir: %w", err)
	}
	outputPath := path.Join(pself.GoDir, "main.go")

	fsys := gengo.NewDirFS(spxProjPath)
	if err := gengo.GenGoFromFS(fsys, outputPath); err != nil {
		return fmt.Errorf("failed to generate Go code using xgobuild: %w", err)
	}

	if err := os.Chdir(spxProjPath); err != nil {
		return fmt.Errorf("failed to change directory to project root for mod tidy: %w", err)
	}
	util.RunGolang(nil, "mod", "tidy")

	if err := os.Chdir(rawdir); err != nil {
		// Log as non-fatal but return error
		return fmt.Errorf("failed to restore original directory: %w", err)
	}

	return nil
}

// genGoUsingXgoCLI generates Go code using xgo CLI (old method)
func (pself *CmdTool) genGoUsingXgoCLI(rawdir, spxProjPath string) error {
	if err := os.Chdir(spxProjPath); err != nil {
		return fmt.Errorf("failed to change directory to project root for XGo: %w", err)
	}

	tagStr := pself.SafeTagArgs()
	log.Printf("genGo tagStr: %s", tagStr)
	envVars := []string{""}

	args := []string{"go"}
	if tagStr != "" {
		args = append(args, tagStr)
	}
	util.RunXGo(envVars, args...)

	if err := os.MkdirAll(pself.GoDir, 0755); err != nil {
		return fmt.Errorf("failed to create GoDir: %w", err)
	}

	sourceFile := path.Join(spxProjPath, "xgo_autogen.go")
	destFile := path.Join(pself.GoDir, "main.go")

	if err := os.Rename(sourceFile, destFile); err != nil {
		return fmt.Errorf("failed to rename/move generated file %s to %s: %w", sourceFile, destFile, err)
	}

	util.RunGolang(nil, "mod", "tidy")

	if err := os.Chdir(rawdir); err != nil {
		return fmt.Errorf("failed to restore original directory: %w", err)
	}

	return nil
}

// =================================================================
// Build Functions
// =================================================================

func (pself *CmdTool) BuildWasm() error {
	pself.genGo()

	// 1. Prepare output directory
	webBuildDir := path.Join(pself.ProjectDir, ".builds/web/")
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create web build directory: %w", err)
	}
	filePath := path.Join(webBuildDir, "gdspx.wasm")

	// 2. Execute build inside GoDir
	return pself.withGoDir(func() error {
		log.Printf("Building WebAssembly binary: %s", filePath)
		envVars := []string{"GOOS=js", "GOARCH=wasm"}

		util.RunGolang(envVars, "build", "-o", filePath)
		return nil
	})
}

// BuildTinyGoLib builds static library using TinyGo for ESP32 or other targets.
func (pself *CmdTool) BuildTinyGoLib() error {
	pself.genGo()

	// 1. Determine target board
	target := *pself.Args.Target
	if target == "" || target == "esp32" {
		target = "esp32-coreboard-v2"
	}

	// 2. Prepare output directory
	tinyGoBuildDir := path.Join(pself.ProjectDir, ".builds/tinygo/")
	if err := os.MkdirAll(tinyGoBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create TinyGo build directory: %w", err)
	}
	outputPath := path.Join(tinyGoBuildDir, "golib.o")

	// 3. Define build arguments
	args := []string{
		"build",
		"-o", outputPath,
		"-target=" + target,
		"-no-debug",
		"-opt=2",
		"-gc=leaking",
		"-scheduler=none",
	}
	if tags := *pself.Args.Tags; tags != "" {
		args = append(args, "-tags="+tags)
	}
	args = append(args, ".")

	// 4. Set environment variables
	envVars := []string{"GODEBUG=gotypesalias=0"}

	// 5. Execute build inside GoDir
	if err := pself.withGoDir(func() error {
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

func (pself *CmdTool) BuildDll() error {
	// 1. Restore original files (undoing a potential previous step)
	if err := pself.restoreFiles(); err != nil {
		return err
	}

	// 2. Determine the list of target architectures.
	targetArchs, err := pself.determineTargetArchs()
	if err != nil {
		return err
	}

	// 3. Generate Go code and get tags
	tagStr := pself.genGo()

	// 4. Execute the build for each target architecture inside GoDir.
	return pself.withGoDir(func() error {
		if err := pself.executeDllBuild(targetArchs, tagStr); err != nil {
			return err
		}
		// 5. Final check: ensure the resulting library path is set.
		if pself.LibPath == "" {
			return fmt.Errorf("build error: cannot find matched dylib for runtime arch %s", runtime.GOARCH)
		}
		return nil
	})
}

// executeDllBuild performs the multi-arch C-shared build.
func (pself *CmdTool) executeDllBuild(archs []string, tagStr string) error {
	rawPath := filepath.Base(pself.LibPath)
	rawDir := filepath.Dir(pself.LibPath)

	pself.LibPath = ""
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
			pself.LibPath = newPath
		}

		envs := append(baseEnvs, "GOARCH="+arch)
		currentArgs := append(buildArgs, "-o", newPath)

		log.Printf("Building shared library: envs=%s, args=%s", envs, currentArgs)
		util.RunGolang(envs, currentArgs...)
	}
	return nil
}
