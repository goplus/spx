/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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
	"archive/zip"
	"fmt"
	"go/build"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/internal/base/fileutil"
	"github.com/goplus/spx/v2/internal/scaffold"
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

	// Runtime paths
	GoBinPath      string // $GOPATH/bin directory
	RuntimeCmdPath string // Path to gdspxrt executable
	RuntimePckPath string // Path to gdspxrt.pck
	LibPath        string // Path to compiled dynamic library

	// Platform info
	GOOS   string
	GOARCH string

	// Runner version (same as spx since runner is a subpackage of spx)
	RunnerVersion string // Runner version (e.g., "latest", "v2.0.0")
	ReleaseMeta   ReleaseMeta
}

// New creates a new Runner for the given project path and optional version
func New(projectPath string, version ...string) (*Runner, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project path: %w", err)
	}

	// Determine GOPATH/bin
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	paths := filepath.SplitList(gopath)
	goBinPath := filepath.Join(paths[0], "bin")

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
		GoBinPath:     goBinPath,
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
		RunnerVersion: runnerVersion,
		ReleaseMeta:   ReleaseMetaForSPXVersion(runnerVersion),
	}

	// Setup runtime paths
	binPostfix := ""
	if runtime.GOOS == "windows" {
		binPostfix = ".exe"
	}

	tagName := r.ReleaseMeta.RuntimeBinaryTag()
	r.RuntimeCmdPath = filepath.Join(goBinPath, tagName+binPostfix)
	r.RuntimePckPath = filepath.Join(goBinPath, tagName+".pck")

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
		fmt.Println("Downloading runtime executable...")
		if err := r.downloadRuntime(); err != nil {
			return err
		}
	}

	// Check if pck file exists
	if _, err := os.Stat(r.RuntimePckPath); os.IsNotExist(err) {
		fmt.Println("Downloading runtime assets...")
		if err := r.downloadRuntimePck(); err != nil {
			return err
		}
	}

	// Make runtime executable
	if err := os.Chmod(r.RuntimeCmdPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod runtime: %w", err)
	}

	fmt.Printf("Runtime ready: %s\n", r.RuntimeCmdPath)
	return nil
}

// downloadRuntime downloads the Godot runtime executable from zip
// URL format: https://github.com/goplus/godot/releases/download/spx{VERSION}/{platform}-{arch}.zip
func (r *Runner) downloadRuntime() error {
	// Determine platform name for URL and binary name
	// URL uses: macos, linux, windows
	// Binary uses: macos, linuxbsd, windows
	var urlPlatform, binaryPlatform, binaryPostfix string
	switch r.GOOS {
	case "windows":
		urlPlatform = "windows"
		binaryPlatform = "windows"
		binaryPostfix = ".exe"
	case "darwin":
		urlPlatform = "macos"
		binaryPlatform = "macos"
		binaryPostfix = ""
	case "linux":
		urlPlatform = "linux"
		binaryPlatform = "linuxbsd"
		binaryPostfix = ""
	default:
		return fmt.Errorf("unsupported OS: %s", r.GOOS)
	}

	// Map Go arch names to release arch names
	// Go uses: amd64, arm64
	// Releases use: x86_64, arm64
	urlArch := r.GOARCH
	if urlArch == "amd64" {
		urlArch = "x86_64"
	}

	// Binary name inside zip
	binaryName := fmt.Sprintf("godot.%s.template_release.%s%s", binaryPlatform, urlArch, binaryPostfix)

	zipName := fmt.Sprintf("%s-%s.zip", urlPlatform, urlArch)
	url := r.ReleaseMeta.RuntimeDownloadURL(zipName)

	// Download and extract
	tmpZip := filepath.Join(r.GoBinPath, zipName)
	fmt.Printf("Downloading runtime from: %s\n", url)

	if err := downloadFile(url, tmpZip); err != nil {
		return fmt.Errorf("failed to download runtime: %w", err)
	}
	defer os.Remove(tmpZip)

	// Extract binary from zip
	if err := extractFileFromZip(tmpZip, binaryName, r.RuntimeCmdPath); err != nil {
		return fmt.Errorf("failed to extract runtime: %w", err)
	}

	fmt.Printf("Runtime executable installed: %s\n", r.RuntimeCmdPath)
	return nil
}

// downloadRuntimePck downloads the runtime asset bundle from spx releases.
// URL format: https://github.com/goplus/spx/releases/download/{spxTag}/spx-runtime-assets.zip
func (r *Runner) downloadRuntimePck() error {
	zipName := RuntimeAssetZipName
	url := r.ReleaseMeta.RuntimeAssetDownloadURL(zipName)

	// Download to temp file
	tmpZip := filepath.Join(r.GoBinPath, zipName)
	fmt.Printf("Downloading runtime assets from: %s\n", url)

	if err := downloadFile(url, tmpZip); err != nil {
		return fmt.Errorf("failed to download pck: %w", err)
	}
	defer os.Remove(tmpZip)

	// Extract gdspxrt.pck from zip and rename to gdspxrt{VERSION}.pck
	if err := extractFileFromZip(tmpZip, "gdspxrt.pck", r.RuntimePckPath); err != nil {
		return fmt.Errorf("failed to extract pck: %w", err)
	}

	fmt.Printf("Runtime pck installed: %s\n", r.RuntimePckPath)
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

	// Ensure the project root has a go.mod before generating or building code.
	// project/go only carries generated sources and should stay in the root module.
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

	// Refresh the root module after generating project/go/main.go.
	fmt.Println("Running go mod tidy...")
	if err := runCommand(r.ProjectDir, nil, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	// Build for each architecture
	for _, arch := range archs {
		libPath := r.getLibPathForArch(arch)
		fmt.Printf("Building for %s/%s -> %s\n", r.GOOS, arch, libPath)

		// Set extra environment variables for the build.
		env := []string{
			"CGO_ENABLED=1",
			"GOARCH=" + arch,
		}

		// Build command
		args := []string{
			"build",
			"-buildmode=c-shared",
			"-o", libPath,
			"./project/go",
		}

		fmt.Printf("Running: CGO_ENABLED=1 GOARCH=%s go %s\n", arch, strings.Join(args, " "))
		if err := runCommand(r.ProjectDir, env, "go", args...); err != nil {
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
		if err := os.WriteFile(extensionDst, []byte(scaffold.RuntimeGDExtension()), 0644); err != nil {
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

	return runCommand(r.TempDir, nil, r.RuntimeCmdPath, args...)
}

// ensureGoxMod ensures gox.mod exists in the project directory.
func (r *Runner) ensureGoxMod() error {
	goxModPath := filepath.Join(r.ProjectDir, "gox.mod")
	if _, err := os.Stat(goxModPath); os.IsNotExist(err) {
		fmt.Println("Creating gox.mod...")
		if err := os.WriteFile(goxModPath, []byte(GoxModTemplate), 0644); err != nil {
			return fmt.Errorf("failed to create gox.mod: %w", err)
		}
	}
	return nil
}

// SpxModule is the SPX v2 module path
const SpxModule = "github.com/goplus/spx/v2"

func applyRunnerVersionToGoModTemplate(content, version string) string {
	if version == "" || version == "latest" {
		return content
	}

	const (
		// The embedded template already carries a concrete release version.
		// Override that single require line in-place when callers request
		// an explicit runner version.
		requirePrefix = "require " + SpxModule + " "
		requireSuffix = " //xgo:class"
	)

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, requirePrefix) && strings.HasSuffix(trimmed, requireSuffix) {
			replacement := requirePrefix + version + requireSuffix
			if strings.HasSuffix(line, "\r") {
				replacement += "\r"
			}
			lines[i] = replacement
			return strings.Join(lines, "\n")
		}
	}
	return content
}

// ensureGoMod ensures the project root has a go.mod file.
func (r *Runner) ensureGoMod() error {
	rootGoModPath := filepath.Join(r.ProjectDir, "go.mod")

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
		content = applyRunnerVersionToGoModTemplate(content, r.RunnerVersion)

		if err := os.WriteFile(rootGoModPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create go.mod: %w", err)
		}

		// If version is "latest", use go get to update to actual latest version
		if r.RunnerVersion == "latest" {
			fmt.Println("Updating to latest spx version...")
			if err := runCommand(r.ProjectDir, nil, "go", "get", SpxModule+"@latest"); err != nil {
				return fmt.Errorf("go get @latest failed: %w", err)
			}
		}
	}

	return nil
}

// generateGoCode runs xgo to generate Go code from .spx files
func (r *Runner) generateGoCode() error {
	// Ensure gox.mod exists first.
	if err := r.ensureGoxMod(); err != nil {
		return err
	}

	if err := runCommand(r.ProjectDir, nil, "xgo", "go", "."); err != nil {
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

// runCommand executes a command in dir, streaming stdout/stderr to the current process.
// Any env entries are appended to the inherited environment.
func runCommand(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// progressWriter wraps an io.Writer to track and display download progress
type progressWriter struct {
	total      int64
	downloaded int64
	lastPct    int
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.downloaded += int64(n)

	if pw.total > 0 {
		pct := int(pw.downloaded * 100 / pw.total)
		if pct != pw.lastPct {
			pw.lastPct = pct
			fmt.Printf("\rDownloading: %d%% (%s / %s)", pct, formatBytes(pw.downloaded), formatBytes(pw.total))
		}
	} else {
		fmt.Printf("\rDownloading: %s", formatBytes(pw.downloaded))
	}
	return n, nil
}

// formatBytes formats bytes into human-readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
	)
	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// downloadFile downloads a file from URL to destination with progress display
func downloadFile(url, dest string) error {
	fmt.Printf("Downloading: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Create progress writer
	pw := &progressWriter{
		total: resp.ContentLength,
	}

	// Copy with progress tracking
	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	fmt.Println() // New line after progress

	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	return fileutil.CopyFile(src, dst)
}

// extractFileFromZip extracts a specific file from a zip archive using pure Go
func extractFileFromZip(zipPath, fileName, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive %s: %w", zipPath, err)
	}
	defer r.Close()

	var targetFile *zip.File
	for _, f := range r.File {
		if filepath.Base(f.Name) == fileName {
			targetFile = f
			break
		}
	}

	if targetFile == nil {
		return fmt.Errorf("file %s not found in zip archive %s", fileName, zipPath)
	}

	rc, err := targetFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open file %s in zip: %w", fileName, err)
	}
	defer rc.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, rc)
	if err != nil {
		return fmt.Errorf("failed to write to destination file %s: %w", destPath, err)
	}

	return nil
}
