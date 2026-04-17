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

package command

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	modenv "github.com/goplus/mod/env"
	"github.com/goplus/spx/v2/cmd/spx/internal/util"
	spxlog "github.com/goplus/spx/v2/internal/log"
	xgotoken "github.com/goplus/xgo/token"
	xgotool "github.com/goplus/xgo/tool"
	"golang.org/x/mod/module"
)

var (
	readBuildInfo = debug.ReadBuildInfo
	outputCommand = util.OutputCommand
)

func (cmd *CmdTool) BuildWasm() error {
	cmd.genGo()

	webBuildDir := path.Join(cmd.ProjectDir, ".builds/web/")
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create web build directory: %w", err)
	}
	filePath := path.Join(webBuildDir, "ispx.wasm")

	return cmd.withGoDir(func() error {
		spxlog.Debug("building WebAssembly binary: %s", filePath)
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
		spxlog.Info("building TinyGo static library for target: %s", target)
		if err := util.RunTinyGo(envVars, args...); err != nil {
			return fmt.Errorf("tinygo build failed: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	spxlog.Info("built TinyGo static library: %s", outputPath)
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

// withGoDir runs f in cmd.GoDir.
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
			spxlog.Warn("failed to restore working directory to %s: %v", rawdir, err)
		}
	}()

	return f()
}

// hideIOSFiles renames ios* files to .txt files.
func (cmd *CmdTool) hideIOSFiles() error {
	searchPattern := filepath.Join(cmd.ProjectDir, "go", "ios*")
	files, err := filepath.Glob(searchPattern)
	if err != nil {
		spxlog.Warn("glob failed for pattern %s: %v", searchPattern, err)
		return nil
	}

	for _, file := range files {
		if !strings.HasSuffix(file, ".txt") {
			newName := file + ".txt"
			if err := os.Rename(file, newName); err != nil {
				spxlog.Warn("failed to rename %s to %s: %v", file, newName, err)
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
		spxlog.Fatalf("failed to get current working directory: %v", err)
	}

	spxProjPath := filepath.Join(cmd.ProjectDir, "..")

	if err := cmd.genGoUsingXgoTool(rawdir, spxProjPath); err != nil {
		spxlog.Fatalf("code generation failed using xgo tool: %v", err)
	}

	return cmd.SafeTagArgs()
}

// genGoUsingXgoTool generates code with the embedded xgo tool package.
func (cmd *CmdTool) genGoUsingXgoTool(rawdir, spxProjPath string) error {
	if err := os.Chdir(spxProjPath); err != nil {
		return fmt.Errorf("failed to change directory to project root for xgo generation: %w", err)
	}
	defer func() {
		if err := os.Chdir(rawdir); err != nil {
			spxlog.Warn("failed to restore working directory to %s: %v", rawdir, err)
		}
	}()

	conf, err := cmd.newXGoToolConfig(spxProjPath)
	if err != nil {
		return err
	}
	if conf.CacheFile != "" {
		defer conf.UpdateCache()
	}

	if _, _, err := xgotool.GenGoEx(spxProjPath, conf, true, 0); err != nil {
		return fmt.Errorf("failed to generate xgo_autogen.go: %w", err)
	}

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

func (cmd *CmdTool) newXGoToolConfig(spxProjPath string) (*xgotool.Config, error) {
	xgoInfo, err := resolveXGoModuleInfo(spxProjPath)
	if err != nil {
		return nil, err
	}

	mod, err := xgotool.LoadMod(spxProjPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load xgo module config: %w", err)
	}

	fset := xgotoken.NewFileSet()
	imp := xgotool.NewImporter(mod, xgoInfo, fset)
	if cmd.Args.Tags != nil && *cmd.Args.Tags != "" {
		imp.SetTags(*cmd.Args.Tags)
	}

	conf := &xgotool.Config{
		XGo:      xgoInfo,
		Fset:     fset,
		Mod:      mod,
		Importer: imp,
	}
	conf.CacheFile = imp.CacheFile()
	imp.Cache().Load(conf.CacheFile)
	return conf, nil
}

func resolveXGoModuleInfo(workDir string) (*modenv.XGo, error) {
	if xgoInfo, err := resolveXGoModuleInfoFromEnv(); err == nil {
		return xgoInfo, nil
	}

	xgoInfo, err := resolveSystemXGoInfo(workDir)
	if err == nil {
		spxlog.Warn("using system xgo %s at %s", xgoInfo.Version, xgoInfo.Root)
		return xgoInfo, nil
	}

	spxlog.Warn("failed to resolve github.com/goplus/xgo from environment or system xgo; falling back to spx build info: %v", err)
	xgoInfo, fallbackErr := resolveXGoModuleInfoFromBuildInfo()
	if fallbackErr != nil {
		return nil, fmt.Errorf("failed to resolve github.com/goplus/xgo from environment or system xgo: %v; spx build info fallback failed: %w", err, fallbackErr)
	}
	return xgoInfo, nil
}

func resolveXGoModuleInfoFromEnv() (*modenv.XGo, error) {
	xgoRoot := strings.TrimSpace(os.Getenv("XGOROOT"))
	if xgoRoot == "" {
		return nil, fmt.Errorf("XGOROOT is empty")
	}
	if !isValidXGoRoot(xgoRoot) {
		return nil, fmt.Errorf("XGOROOT is invalid: %s", xgoRoot)
	}

	return &modenv.XGo{
		Version: normalizeXGoVersion(os.Getenv("XGOVERSION"), ""),
		Root:    xgoRoot,
	}, nil
}

func resolveXGoModuleInfoFromBuildInfo() (*modenv.XGo, error) {
	info, ok := readBuildInfo()
	if !ok || info == nil {
		return nil, fmt.Errorf("build info unavailable")
	}

	goModCache, err := resolveGoModCacheDir()
	if err != nil {
		return nil, err
	}

	return resolveXGoModuleInfoFromBuildData(info, goModCache)
}

func resolveSystemXGoInfo(workDir string) (*modenv.XGo, error) {
	output, err := outputCommand(
		util.CommandOptions{Dir: workDir},
		"xgo", "env", "XGOROOT", "XGOVERSION",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query system xgo: %w", err)
	}

	return parseXGoInfoOutput(output, "xgo env returned an empty XGOROOT")
}

func resolveXGoModuleInfoFromBuildData(info *debug.BuildInfo, goModCache string) (*modenv.XGo, error) {
	dep := findBuildInfoDep(info, "github.com/goplus/xgo")
	if dep == nil {
		return nil, fmt.Errorf("github.com/goplus/xgo not found in build info")
	}

	if dep.Replace != nil && isLocalModulePath(dep.Replace.Path) {
		root, err := filepath.Abs(dep.Replace.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve replaced xgo path %q: %w", dep.Replace.Path, err)
		}
		if !isValidXGoRoot(root) {
			return nil, fmt.Errorf("replaced xgo root is invalid: %s", root)
		}
		return &modenv.XGo{
			Version: normalizeXGoVersion(dep.Replace.Version, dep.Version),
			Root:    root,
		}, nil
	}

	modPath := dep.Path
	modVersion := dep.Version
	if dep.Replace != nil {
		modPath = dep.Replace.Path
		modVersion = normalizeXGoVersion(dep.Replace.Version, dep.Version)
	}
	if modVersion == "(devel)" {
		return nil, fmt.Errorf("xgo build info version is unavailable")
	}
	if goModCache == "" {
		return nil, fmt.Errorf("GOMODCACHE is empty")
	}

	root, err := resolveModuleCacheDir(goModCache, modPath, modVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve xgo module cache path: %w", err)
	}
	if !isValidXGoRoot(root) {
		return nil, fmt.Errorf("xgo module cache root is invalid or missing: %s", root)
	}

	return &modenv.XGo{
		Version: modVersion,
		Root:    root,
	}, nil
}

func findBuildInfoDep(info *debug.BuildInfo, modulePath string) *debug.Module {
	for _, dep := range info.Deps {
		if dep != nil && dep.Path == modulePath {
			return dep
		}
	}
	return nil
}

func resolveModuleCacheDir(goModCache, modPath, modVersion string) (string, error) {
	escapedPath, err := module.EscapePath(modPath)
	if err != nil {
		return "", err
	}
	escapedVersion, err := module.EscapeVersion(modVersion)
	if err != nil {
		return "", err
	}
	return filepath.Join(goModCache, escapedPath+"@"+escapedVersion), nil
}

func resolveGoModCacheDir() (string, error) {
	if goModCache := strings.TrimSpace(os.Getenv("GOMODCACHE")); goModCache != "" {
		return goModCache, nil
	}

	output, err := outputCommand(util.CommandOptions{}, "go", "env", "GOMODCACHE")
	if err != nil {
		return "", fmt.Errorf("failed to resolve GOMODCACHE: %w", err)
	}

	goModCache := strings.TrimSpace(string(output))
	if goModCache == "" {
		return "", fmt.Errorf("GOMODCACHE is empty")
	}
	return goModCache, nil
}

func isLocalModulePath(modulePath string) bool {
	return filepath.IsAbs(modulePath) || strings.HasPrefix(modulePath, ".")
}

func isValidXGoRoot(root string) bool {
	if root == "" {
		return false
	}
	if info, err := os.Stat(filepath.Join(root, "cmd", "xgo")); err != nil || !info.IsDir() {
		return false
	}
	if info, err := os.Stat(filepath.Join(root, "go.mod")); err != nil || info.IsDir() {
		return false
	}
	return true
}

func normalizeXGoVersion(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "(devel)"
}

func parseXGoInfoOutput(output []byte, emptyRootErr string) (*modenv.XGo, error) {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, fmt.Errorf("%s", emptyRootErr)
	}

	return &modenv.XGo{
		Version: normalizeXGoVersion("", valueAt(lines, 1)),
		Root:    strings.TrimSpace(lines[0]),
	}, nil
}

func valueAt(lines []string, idx int) string {
	if idx < 0 || idx >= len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[idx])
}

// executeDllBuild runs a multi-arch C-shared build.
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

		spxlog.Debug("building shared library: envs=%s, args=%s", envs, currentArgs)
		util.RunGolang(envs, currentArgs...)
	}
	return nil
}
