/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/envutil"
	gomodule "golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

type launcherSource struct {
	main             bool
	sourceMode       bool
	root             string
	effectivePath    string
	selectedVersion  string
	effectiveVersion string
	goCommand        string
	workDir          string
	goWork           string
	graphFlags       []string
	env              []string
	protectedFiles   []string
	verifyGraph      func(context.Context) error
}

type listedModule struct {
	Path     string
	Version  string
	Main     bool
	Dir      string
	GoMod    string
	Sum      string
	GoModSum string
	Replace  *listedModule
}

func resolveLauncherInputs(ctx context.Context, projectDir, goCommand string) (launcherProject, launcherSource, error) {
	env := append([]string(nil), os.Environ()...)
	flags, err := effectiveGraphFlags(ctx, goCommand, projectDir, env)
	if err != nil {
		return launcherProject{}, launcherSource{}, err
	}
	goMod, err := queryGoEnv(ctx, goCommand, projectDir, env, "GOMOD")
	if err != nil {
		return launcherProject{}, launcherSource{}, fmt.Errorf("buildlauncher: resolve Go module file: %w", err)
	}
	if goMod == "" || goMod == os.DevNull {
		return launcherProject{}, launcherSource{}, fmt.Errorf("buildlauncher: project path is outside a Go module")
	}
	goMod, err = canonicalFile(goMod)
	if err != nil {
		return launcherProject{}, launcherSource{}, fmt.Errorf("buildlauncher: Go module file: %w", err)
	}
	moduleRoot := filepath.Dir(goMod)
	if !launcherPathWithin(moduleRoot, projectDir) {
		return launcherProject{}, launcherSource{}, fmt.Errorf("buildlauncher: project path %q is outside the active Go module %q", projectDir, moduleRoot)
	}
	project, err := loadLauncherProject(moduleRoot, projectDir)
	if err != nil {
		return launcherProject{}, launcherSource{}, err
	}
	goWork, err := queryGoEnv(ctx, goCommand, projectDir, env, "GOWORK")
	if err != nil {
		return launcherProject{}, launcherSource{}, fmt.Errorf("buildlauncher: resolve Go workspace: %w", err)
	}
	if goWork == "" {
		goWork = "off"
	} else if goWork != "off" {
		goWork, err = canonicalFile(goWork)
		if err != nil {
			return launcherProject{}, launcherSource{}, fmt.Errorf("buildlauncher: Go workspace file: %w", err)
		}
	}
	graphEnv := launcherGraphEnvironment(env, goWork)
	spx, verifier, err := resolveLauncherGraph(ctx, launcherGraphQuery{
		goCommand: goCommand, workDir: projectDir, goWork: goWork,
		graphFlags: flags, env: graphEnv, files: []string{goMod, project.metadataFile},
	})
	if err != nil {
		return launcherProject{}, launcherSource{}, fmt.Errorf("buildlauncher: resolve module graph: %w", err)
	}
	root, selectedVersion, effectiveVersion, sourceMode, err := resolveSPXSource(spx)
	if err != nil {
		return launcherProject{}, launcherSource{}, err
	}
	effectivePath := spxModulePath
	if spx.Replace != nil && spx.Replace.Path != "" {
		effectivePath = spx.Replace.Path
	}
	return project, launcherSource{
		main: spx.Main, sourceMode: sourceMode, root: root, effectivePath: effectivePath,
		selectedVersion: selectedVersion, effectiveVersion: effectiveVersion,
		goCommand: goCommand, workDir: projectDir, goWork: goWork, graphFlags: flags,
		env: graphEnv, protectedFiles: launcherGraphProtectedFiles(verifier.files(), flags),
		verifyGraph: verifier.verify,
	}, nil
}

func queryGoEnv(ctx context.Context, goCommand, workDir string, env []string, name string) (string, error) {
	var stderr strings.Builder
	command := exec.CommandContext(ctx, goCommand, "env", name)
	command.Dir = workDir
	command.Env = env
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("%w: %s", err, message)
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func resolveSPXSource(module listedModule) (root, selectedVersion, effectiveVersion string, sourceMode bool, err error) {
	if module.Path != spxModulePath {
		return "", "", "", false, fmt.Errorf("buildlauncher: Go graph selected %q for SPX, want %q", module.Path, spxModulePath)
	}
	sourceMode = module.Main || module.Replace != nil && module.Replace.Version == ""
	if !sourceMode {
		if module.Replace != nil {
			return "", "", "", false, fmt.Errorf("buildlauncher: published SPX module must not use a versioned replacement")
		}
		if !semver.IsValid(module.Version) || semver.Canonical(module.Version) != module.Version || gomodule.IsPseudoVersion(module.Version) {
			return "", "", "", false, fmt.Errorf("buildlauncher: published SPX requires an exact canonical release version, got %q", module.Version)
		}
		return "", module.Version, module.Version, false, nil
	}
	effective := module
	if module.Replace != nil {
		effective = *module.Replace
	}
	if effective.Dir == "" {
		return "", "", "", false, fmt.Errorf("buildlauncher: local SPX module has no source directory")
	}
	root, err = canonicalDirectory(effective.Dir)
	if err != nil {
		return "", "", "", false, fmt.Errorf("buildlauncher: SPX module root: %w", err)
	}
	bridge := filepath.Join(root, "cmd", "ispxnative", "main.go")
	info, statErr := os.Lstat(bridge)
	if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		if statErr != nil {
			return "", "", "", false, fmt.Errorf("buildlauncher: SPX module %q has no cmd/ispxnative package: %w", root, statErr)
		}
		return "", "", "", false, fmt.Errorf("buildlauncher: SPX module %q has no regular cmd/ispxnative package", root)
	}
	return root, module.Version, effective.Version, sourceMode, nil
}

func resolveGoCommand() (string, error) {
	path, err := exec.LookPath("go")
	if err != nil {
		return "", err
	}
	return canonicalFile(path)
}

func graphFlagsFromEnv(workDir string) ([]string, error) {
	goCommand, err := resolveGoCommand()
	if err != nil {
		return nil, err
	}
	return effectiveGraphFlags(context.Background(), goCommand, workDir, os.Environ())
}

func effectiveGraphFlags(ctx context.Context, goCommand, workDir string, env []string) ([]string, error) {
	value, found, duplicate := envutil.Lookup(env, "GOFLAGS")
	if duplicate {
		return nil, fmt.Errorf("buildlauncher: GOFLAGS is repeated")
	}
	if !found || value == "" {
		var err error
		value, err = queryGoEnv(ctx, goCommand, workDir, env, "GOFLAGS")
		if err != nil {
			return nil, fmt.Errorf("buildlauncher: resolve GOFLAGS: %w", err)
		}
	}
	return parseGraphFlags(workDir, value)
}

func parseGraphFlags(workDir, value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	fields, err := splitGOFLAGS(value)
	if err != nil {
		return nil, fmt.Errorf("buildlauncher: parse GOFLAGS: %w", err)
	}
	flags := make([]string, 0, len(fields))
	for _, flag := range fields {
		path, modfile := launcherModfilePath(flag)
		if modfile {
			if !filepath.IsAbs(path) {
				path = filepath.Join(workDir, path)
			}
			canonical, err := canonicalFile(filepath.Clean(path))
			if err != nil {
				return nil, fmt.Errorf("buildlauncher: GOFLAGS modfile: %w", err)
			}
			flag = "-modfile=" + canonical
		}
		flags = append(flags, flag)
	}
	return flags, nil
}

// splitGOFLAGS mirrors Go's cmd/internal/quoted parser.
func splitGOFLAGS(value string) ([]string, error) {
	var fields []string
	for len(value) > 0 {
		for len(value) > 0 && isGOFLAGSFieldSpace(value[0]) {
			value = value[1:]
		}
		if value == "" {
			break
		}
		if value[0] == '\'' || value[0] == '"' {
			quote := value[0]
			value = value[1:]
			end := strings.IndexByte(value, quote)
			if end < 0 {
				return nil, fmt.Errorf("unterminated %c string", quote)
			}
			fields = append(fields, value[:end])
			value = value[end+1:]
			continue
		}
		end := 0
		for end < len(value) && !isGOFLAGSFieldSpace(value[end]) {
			end++
		}
		fields = append(fields, value[:end])
		value = value[end:]
	}
	return fields, nil
}

func isGOFLAGSFieldSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r'
}

func canonicalDirectory(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a regular directory", absolute)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("%q is not a canonical directory: %w", absolute, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func canonicalFile(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%q is not a regular file", absolute)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("%q is not a canonical file: %w", absolute, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func launcherPathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
