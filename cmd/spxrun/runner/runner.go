/*
 * Copyright (c) 2024 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package runner implements the SPX 2.0 project runner.
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// RuntimeTag is the tag name for runtime files
	RuntimeTag = "gdspxrt"

	// GDExtensionTemplate is the template for runtime.gdextension file
	GDExtensionTemplate = `[configuration]

entry_symbol = "gdspx_init"
compatibility_minimum = 4.1

[libraries]

macos.debug.x86_64 = "gdspx-darwin-amd64.dylib"
macos.release.x86_64 = "gdspx-darwin-amd64.dylib"
macos.debug.arm64 = "gdspx-darwin-arm64.dylib"
macos.release.arm64 = "gdspx-darwin-arm64.dylib"
windows.debug.x86_64 = "gdspx-windows-amd64.dll"
windows.release.x86_64 = "gdspx-windows-amd64.dll"
linux.debug.x86_64 = "gdspx-linux-amd64.so"
linux.release.x86_64 = "gdspx-linux-amd64.so"
`
)

// RuntimeOptions holds runtime configuration options
type RuntimeOptions struct {
	Fullscreen  bool   // Run in fullscreen mode
	Windowed    bool   // Run in windowed mode (opposite of fullscreen)
	Width       int    // Window width
	Height      int    // Window height
	Position    string // Window position (e.g., "100,100")
	Maximized   bool   // Start maximized
	AlwaysOnTop bool   // Keep window always on top
	Debug       bool   // Enable debug mode
}

// Runner handles the SPX project running process
type Runner struct {
	// Project paths
	ProjectDir string // SPX project directory (contains .spx files)
	GoDir      string // Generated Go code directory
	LibDir     string // Library output directory
	TempDir    string // Temporary runtime directory
	ShareDir   string // Share directory relative to spxrun executable
	EnginesDir string // Engine directory under share

	// Runtime paths
	RuntimeCmdPath string // Path to gdspxrt executable
	RuntimePckPath string // Path to gdspxrt.pck
	LibPath        string // Path to compiled dynamic library

	// Platform info
	GOOS   string
	GOARCH string

	// Runner version (same as spx since runner is a subpackage of spx)
	RunnerVersion string // Runner version (e.g., "latest", "v2.0.0")
}

// New creates a new Runner for the given project path and optional version
func New(projectPath string, version ...string) (*Runner, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project path: %w", err)
	}

	shareDir, err := getShareDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve share directory: %w", err)
	}
	enginesDir := filepath.Join(shareDir, "engines")

	// Determine runner version (default to "latest")
	runnerVersion := "latest"
	if len(version) > 0 && version[0] != "" {
		runnerVersion = version[0]
	}

	r := &Runner{
		ProjectDir:    absPath,
		GoDir:         filepath.Join(absPath, "project", "go"),
		LibDir:        filepath.Join(absPath, "project", "lib"),
		TempDir:       filepath.Join(absPath, ".temp"),
		ShareDir:      shareDir,
		EnginesDir:    enginesDir,
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
		RunnerVersion: runnerVersion,
	}

	// Setup runtime paths
	binPostfix := ""
	if runtime.GOOS == "windows" {
		binPostfix = ".exe"
	}

	runtimeVersion := PckVersion()
	if runtimeVersion == "" {
		runtimeVersion = Version()
	}
	tagName := RuntimeTag + runtimeVersion
	r.RuntimeCmdPath = filepath.Join(enginesDir, tagName+binPostfix)
	r.RuntimePckPath = filepath.Join(enginesDir, tagName+".pck")

	// Setup library path
	libName := fmt.Sprintf("gdspx-%s-%s", r.GOOS, r.GOARCH)
	switch r.GOOS {
	case "windows":
		libName += ".dll"
	case "darwin":
		libName += ".dylib"
	default:
		libName += ".so"
	}
	r.LibPath = filepath.Join(r.LibDir, libName)

	return r, nil
}

// Run executes the complete SPX project running process
func (r *Runner) Run() error {
	return r.RunWithOptions(nil)
}

// RunWithOptions executes the SPX project running process with custom runtime options
func (r *Runner) RunWithOptions(opts *RuntimeOptions) error {
	fmt.Println("=== SPX Runner ===")
	fmt.Printf("Project: %s\n", r.ProjectDir)

	// Step 1: Check and download runtime
	if err := r.ensureRuntime(); err != nil {
		return fmt.Errorf("failed to ensure runtime: %w", err)
	}

	// Step 2: Build dynamic library
	if err := r.buildLibrary(); err != nil {
		return fmt.Errorf("failed to build library: %w", err)
	}

	// Step 3: Run with Godot runtime
	if err := r.runWithRuntimeOptions(opts); err != nil {
		return fmt.Errorf("failed to run: %w", err)
	}

	return nil
}

// ensureRuntime checks and downloads the Godot runtime if needed
func (r *Runner) ensureRuntime() error {
	fmt.Println("Checking runtime...")

	// Check if runtime executable exists
	if _, err := os.Stat(r.RuntimeCmdPath); os.IsNotExist(err) {
		return fmt.Errorf("runtime executable not found at %s. Run 'make setup-engines' to download engines", r.RuntimeCmdPath)
	} else if err != nil {
		return fmt.Errorf("failed to stat runtime executable: %w", err)
	}

	// Check if pck file exists
	if _, err := os.Stat(r.RuntimePckPath); os.IsNotExist(err) {
		return fmt.Errorf("runtime pck not found at %s. Run 'make setup-engines' to download engines", r.RuntimePckPath)
	} else if err != nil {
		return fmt.Errorf("failed to stat runtime pck: %w", err)
	}

	// Make runtime executable
	if err := os.Chmod(r.RuntimeCmdPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod runtime: %w", err)
	}

	fmt.Printf("Runtime ready: %s\n", r.RuntimeCmdPath)
	return nil
}

// buildLibrary builds the Go dynamic library for the SPX project
func (r *Runner) buildLibrary() error {
	fmt.Println("Building dynamic library...")

	// Ensure lib directory exists
	if err := os.MkdirAll(r.LibDir, 0755); err != nil {
		return fmt.Errorf("failed to create lib directory: %w", err)
	}

	// Ensure go directory exists
	if err := os.MkdirAll(r.GoDir, 0755); err != nil {
		return fmt.Errorf("failed to create go directory: %w", err)
	}

	// Ensure go.mod exists in both project root and project/go directory
	// Root go.mod is needed for xgo to resolve dependencies during code generation
	// Create in project root first, then copy to project/go
	if err := r.ensureGoMod(); err != nil {
		return fmt.Errorf("failed to ensure go.mod: %w", err)
	}

	// Always regenerate Go code from .spx files (project may have changed)
	fmt.Println("Generating Go code with xgo...")
	if err := r.generateGoCode(); err != nil {
		return fmt.Errorf("failed to generate Go code: %w", err)
	}

	// Check if xgo_autogen.go was generated and move to go/main.go
	autogenPath := filepath.Join(r.ProjectDir, "xgo_autogen.go")
	mainPath := filepath.Join(r.GoDir, "main.go")

	if _, err := os.Stat(autogenPath); err == nil {
		// Copy xgo_autogen.go to go/main.go
		if err := copyFile(autogenPath, mainPath); err != nil {
			return fmt.Errorf("failed to copy autogen file: %w", err)
		}
		// Remove xgo_autogen.go from project directory to keep it clean
		os.Remove(autogenPath)
	} else {
		return fmt.Errorf("xgo failed to generate code. Check if .spx files exist in project")
	}

	// Build library for all architectures on macOS
	archs := r.determineTargetArchs()

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Change to go directory
	if err := os.Chdir(r.GoDir); err != nil {
		return fmt.Errorf("failed to change to go directory: %w", err)
	}
	defer os.Chdir(origDir)

	// Run go mod tidy first
	fmt.Println("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = r.GoDir
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	// Build for each architecture
	for _, arch := range archs {
		libPath := r.getLibPathForArch(arch)
		fmt.Printf("Building for %s/%s -> %s\n", r.GOOS, arch, libPath)

		// Set environment variables
		env := append(os.Environ(),
			"CGO_ENABLED=1",
			"GOARCH="+arch,
		)

		// Build command
		args := []string{
			"build",
			"-buildmode=c-shared",
			"-o", libPath,
		}

		cmd := exec.Command("go", args...)
		cmd.Env = env
		cmd.Dir = r.GoDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		fmt.Printf("Running: CGO_ENABLED=1 GOARCH=%s go %s\n", arch, strings.Join(args, " "))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("build failed for %s: %w", arch, err)
		}

		// Update LibPath if this is the current architecture
		if arch == r.GOARCH {
			r.LibPath = libPath
		}
	}

	fmt.Printf("Library built: %s\n", r.LibPath)
	return nil
}

// runWithRuntimeOptions runs the project using Godot runtime with custom options
func (r *Runner) runWithRuntimeOptions(opts *RuntimeOptions) error {
	fmt.Println("Running project...")

	// Ensure temp directory exists
	if err := os.MkdirAll(r.TempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Copy all built libraries to temp directory
	for _, arch := range r.determineTargetArchs() {
		libPath := r.getLibPathForArch(arch)
		dstLibPath := filepath.Join(r.TempDir, filepath.Base(libPath))
		if err := copyFile(libPath, dstLibPath); err != nil {
			return fmt.Errorf("failed to copy library %s: %w", libPath, err)
		}
	}

	// Generate or copy runtime.gdextension to temp directory
	extensionDst := filepath.Join(r.TempDir, "runtime.gdextension")
	extensionSrc := filepath.Join(r.ProjectDir, "project", "runtime.gdextension.txt")
	if _, err := os.Stat(extensionSrc); err == nil {
		// Use project's custom gdextension file if exists
		if err := copyFile(extensionSrc, extensionDst); err != nil {
			return fmt.Errorf("failed to copy runtime.gdextension: %w", err)
		}
	} else {
		// Generate default gdextension file from template
		if err := os.WriteFile(extensionDst, []byte(GDExtensionTemplate), 0644); err != nil {
			return fmt.Errorf("failed to generate runtime.gdextension: %w", err)
		}
	}

	// Build Godot runtime arguments
	args := []string{
		"--path", r.TempDir,
		"--gdextpath", extensionDst,
	}

	// Apply runtime options if provided
	if opts != nil {
		// Window mode options
		if opts.Fullscreen {
			args = append(args, "--fullscreen")
		} else if opts.Windowed {
			args = append(args, "--windowed")
		}

		// Window size
		if opts.Width > 0 && opts.Height > 0 {
			args = append(args, fmt.Sprintf("--resolution=%dx%d", opts.Width, opts.Height))
		}

		// Window position
		if opts.Position != "" {
			args = append(args, fmt.Sprintf("--position=%s", opts.Position))
		}

		// Window state
		if opts.Maximized {
			args = append(args, "--maximized")
		}

		if opts.AlwaysOnTop {
			args = append(args, "--always-on-top")
		}

		// Debug options
		if opts.Debug {
			args = append(args, "--debug")
		}
	}

	fmt.Printf("Running: %s %s\n", r.RuntimeCmdPath, strings.Join(args, " "))

	cmd := exec.Command(r.RuntimeCmdPath, args...)
	cmd.Dir = r.TempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// ensureGopMod ensures gop.mod exists in the project directory
func (r *Runner) ensureGopMod() error {
	gopModPath := filepath.Join(r.ProjectDir, "gop.mod")
	if _, err := os.Stat(gopModPath); os.IsNotExist(err) {
		fmt.Println("Creating gop.mod...")
		if err := os.WriteFile(gopModPath, []byte(GopModTemplate), 0644); err != nil {
			return fmt.Errorf("failed to create gop.mod: %w", err)
		}
	}
	return nil
}

// SpxModule is the SPX v2 module path
const SpxModule = "github.com/goplus/spx/v2"

// ensureGoMod ensures go.mod exists in both project root and project/go directory
func (r *Runner) ensureGoMod() error {
	rootGoModPath := filepath.Join(r.ProjectDir, "go.mod")
	goGoModPath := filepath.Join(r.GoDir, "go.mod")

	// Check if root go.mod already exists
	if _, err := os.Stat(rootGoModPath); os.IsNotExist(err) {
		fmt.Println("Creating go.mod in project root...")

		// Determine module name from project directory name
		moduleName := filepath.Base(r.ProjectDir)
		if moduleName == "." || moduleName == "" {
			moduleName = "spxproject"
		}

		// Use embedded template and replace placeholders
		content := GoModTemplate
		content = strings.Replace(content, "github.com/goplus/spxdemo", moduleName, 1)
		content = strings.Replace(content, "v2.0.0-pre.28", r.RunnerVersion, 1)

		if err := os.WriteFile(rootGoModPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create go.mod: %w", err)
		}

		// If version is "latest", use go get to update to actual latest version
		if r.RunnerVersion == "latest" {
			fmt.Println("Updating to latest spx version...")
			getCmd := exec.Command("go", "get", SpxModule+"@latest")
			getCmd.Dir = r.ProjectDir
			getCmd.Stdout = os.Stdout
			getCmd.Stderr = os.Stderr
			if err := getCmd.Run(); err != nil {
				return fmt.Errorf("go get @latest failed: %w", err)
			}
		}
	}

	// Copy root go.mod to project/go if it doesn't exist
	if _, err := os.Stat(goGoModPath); os.IsNotExist(err) {
		fmt.Println("Copying go.mod to project/go...")
		if err := copyFile(rootGoModPath, goGoModPath); err != nil {
			return fmt.Errorf("failed to copy go.mod: %w", err)
		}
	}

	return nil
}

// generateGoCode runs xgo to generate Go code from .spx files
func (r *Runner) generateGoCode() error {
	// Ensure gop.mod exists first
	if err := r.ensureGopMod(); err != nil {
		return err
	}

	cmd := exec.Command("xgo", "go", ".")
	cmd.Dir = r.ProjectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xgo go failed: %w (make sure xgo is installed)", err)
	}
	return nil
}

// determineTargetArchs returns the list of architectures to build for
func (r *Runner) determineTargetArchs() []string {
	// On macOS, build for both amd64 and arm64, with current arch first
	if r.GOOS == "darwin" {
		if r.GOARCH == "arm64" {
			return []string{"arm64", "amd64"}
		}
		return []string{"amd64", "arm64"}
	}
	return []string{r.GOARCH}
}

// getLibPathForArch returns the library path for a specific architecture
func (r *Runner) getLibPathForArch(arch string) string {
	libName := fmt.Sprintf("gdspx-%s-%s", r.GOOS, arch)
	switch r.GOOS {
	case "windows":
		libName += ".dll"
	case "darwin":
		libName += ".dylib"
	default:
		libName += ".so"
	}
	return filepath.Join(r.LibDir, libName)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0755)
}

// getShareDir returns the share directory path relative to the spxrun executable.
func getShareDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	binDir := filepath.Dir(exe)
	shareDir := filepath.Join(binDir, "..", "share")
	return filepath.Abs(shareDir)
}
