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
)

const (
	// PckVersion is the pck file version
	PckVersion = "2.0.30"

	// RuntimeURLBase is the base URL for downloading runtime executable
	// Format: https://github.com/goplus/godot/releases/download/spx{VERSION}/{platform}-{arch}.zip
	RuntimeURLBase = "https://github.com/goplus/godot/releases/download/"

	// PckURLBase is the base URL for downloading pck file
	// Format: https://github.com/goplus/spx/releases/download/v2.0.0-pre.30/gdspxrt.pck.{PCK_VERSION}.zip
	PckURLBase = "https://github.com/goplus/spx/releases/download/v2.0.0-pre.30/"

	// RuntimeTag is the tag name for runtime files
	RuntimeTag = "gdspxrt"

	// GDExtensionTemplate is the template for runtime.gdextension file
	GDExtensionTemplate = `[configuration]

entry_symbol = "loadExtension"
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
}

// New creates a new Runner for the given project path
func New(projectPath string) (*Runner, error) {
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

	r := &Runner{
		ProjectDir: absPath,
		GoDir:      filepath.Join(absPath, "project", "go"),
		LibDir:     filepath.Join(absPath, "project", "lib"),
		TempDir:    filepath.Join(absPath, ".temp"),
		GoBinPath:  goBinPath,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
	}

	// Setup runtime paths
	binPostfix := ""
	if runtime.GOOS == "windows" {
		binPostfix = ".exe"
	}

	tagName := RuntimeTag + Version()
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
	if err := r.runWithRuntime(); err != nil {
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
		fmt.Println("Downloading runtime pck...")
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

	// URL: https://github.com/goplus/godot/releases/download/spx{VERSION}/{platform}-{arch}.zip
	zipName := fmt.Sprintf("%s-%s.zip", urlPlatform, urlArch)
	url := RuntimeURLBase + "spx" + Version() + "/" + zipName

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

// downloadRuntimePck downloads the Godot runtime pck file from spx releases
// URL format: https://github.com/goplus/spx/releases/download/v2.0.0-pre.30/gdspxrt.pck.{PCK_VERSION}.zip
func (r *Runner) downloadRuntimePck() error {
	// URL: https://github.com/goplus/spx/releases/download/v2.0.0-pre.30/gdspxrt.pck.{PCK_VERSION}.zip
	zipName := fmt.Sprintf("gdspxrt.pck.%s.zip", PckVersion)
	url := PckURLBase + zipName

	// Download to temp file
	tmpZip := filepath.Join(r.GoBinPath, zipName)
	fmt.Printf("Downloading pck from: %s\n", url)

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

	// Ensure go.mod exists in go directory
	if err := r.ensureGoMod(); err != nil {
		return fmt.Errorf("failed to ensure go.mod: %w", err)
	}

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

// runWithRuntime runs the project using Godot runtime
func (r *Runner) runWithRuntime() error {
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

	// Run Godot runtime
	args := []string{
		"--path", r.TempDir,
		"--gdextpath", extensionDst,
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

// ensureGoMod ensures go.mod exists in the go directory
func (r *Runner) ensureGoMod() error {
	goModPath := filepath.Join(r.GoDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		fmt.Println("Creating go.mod in project/go...")

		// Determine module name from project directory name
		moduleName := filepath.Base(r.ProjectDir)
		if moduleName == "." || moduleName == "" {
			moduleName = "spxproject"
		}

		// Create minimal go.mod without dependencies
		content := fmt.Sprintf(`module %s

go 1.24.0
`, moduleName)
		if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create go.mod: %w", err)
		}

		// Use 'go get' to add the latest spx/v2 version automatically
		fmt.Println("Getting latest spx/v2...")
		getCmd := exec.Command("go", "get", SpxModule+"@latest")
		getCmd.Dir = r.GoDir
		getCmd.Stdout = os.Stdout
		getCmd.Stderr = os.Stderr
		if err := getCmd.Run(); err != nil {
			return fmt.Errorf("go get spx failed: %w", err)
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
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0755)
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
