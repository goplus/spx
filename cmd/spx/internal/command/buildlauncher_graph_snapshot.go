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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
)

type launcherGraphQuery struct {
	goCommand  string
	workDir    string
	goWork     string
	graphFlags []string
	env        []string
	files      []string
}

type launcherModuleIdentity struct {
	Path, Version, Sum, GoModSum                             string
	ReplacePath, ReplaceVersion, ReplaceSum, ReplaceGoModSum string
	LocalDir                                                 string
	Main                                                     bool
}

type launcherFileIdentity struct {
	path   string
	sha256 string
}

type launcherGraphIdentity struct {
	module    launcherModuleIdentity
	selection string
	files     []launcherFileIdentity
}

type launcherGraphVerifier struct {
	query    launcherGraphQuery
	expected launcherGraphIdentity
}

func resolveLauncherGraph(ctx context.Context, query launcherGraphQuery) (listedModule, launcherGraphVerifier, error) {
	query.graphFlags = append([]string(nil), query.graphFlags...)
	query.env = append([]string(nil), query.env...)
	query.files = append([]string(nil), query.files...)
	identity, module, err := snapshotLauncherGraph(ctx, query)
	if err != nil {
		return listedModule{}, launcherGraphVerifier{}, err
	}
	return module, launcherGraphVerifier{query: query, expected: identity}, nil
}

func (v launcherGraphVerifier) verify(ctx context.Context) error {
	current, _, err := snapshotLauncherGraph(ctx, v.query)
	if err != nil {
		return err
	}
	if current.module != v.expected.module || current.selection != v.expected.selection {
		return fmt.Errorf("module selection changed")
	}
	if !reflect.DeepEqual(current.files, v.expected.files) {
		return fmt.Errorf("module input changed")
	}
	return nil
}

func (v launcherGraphVerifier) files() []string {
	files := make([]string, len(v.expected.files))
	for i, file := range v.expected.files {
		files[i] = file.path
	}
	return files
}

func snapshotLauncherGraph(ctx context.Context, query launcherGraphQuery) (launcherGraphIdentity, listedModule, error) {
	module, err := queryListedModule(ctx, query)
	if err != nil {
		return launcherGraphIdentity{}, listedModule{}, err
	}
	files := append([]string(nil), query.files...)
	if query.goWork != "" && query.goWork != "off" {
		files = append(files, query.goWork)
	}
	for _, flag := range query.graphFlags {
		if path, ok := launcherModfilePath(flag); ok {
			files = append(files, path)
		}
	}
	vendorGraph := false
	if vendorFile, required := launcherVendorModulesFile(query); vendorFile != "" {
		if _, err := os.Stat(vendorFile); err == nil {
			files = append(files, vendorFile)
			vendorGraph = true
		} else if required || !os.IsNotExist(err) {
			return launcherGraphIdentity{}, listedModule{}, fmt.Errorf("vendor graph: %w", err)
		}
	}
	moduleIdentity, localGoMod, err := normalizeLauncherModule(module)
	if err != nil {
		return launcherGraphIdentity{}, listedModule{}, err
	}
	if localGoMod != "" {
		files = append(files, localGoMod)
	}
	selection := ""
	if !vendorGraph {
		selection, err = queryModuleSelection(ctx, query)
		if err != nil {
			return launcherGraphIdentity{}, listedModule{}, err
		}
	}
	fileIdentity, err := snapshotLauncherFiles(files)
	if err != nil {
		return launcherGraphIdentity{}, listedModule{}, err
	}
	return launcherGraphIdentity{module: moduleIdentity, selection: selection, files: fileIdentity}, module, nil
}

func launcherVendorModulesFile(query launcherGraphQuery) (string, bool) {
	mode := ""
	for _, flag := range query.graphFlags {
		if strings.HasPrefix(flag, "-mod=") {
			mode = strings.TrimPrefix(flag, "-mod=")
		}
	}
	if mode != "vendor" && mode != "" {
		return "", false
	}
	root := ""
	if query.goWork != "" && query.goWork != "off" {
		root = filepath.Dir(query.goWork)
	} else if len(query.files) > 0 {
		root = filepath.Dir(query.files[0])
	}
	if root == "" {
		return "", mode == "vendor"
	}
	return filepath.Join(root, "vendor", "modules.txt"), mode == "vendor"
}

func queryModuleSelection(ctx context.Context, query launcherGraphQuery) (string, error) {
	const format = `{{.Path}}\t{{.Version}}\t{{.Main}}{{with .Replace}}\t{{.Path}}\t{{.Version}}{{end}}`
	args := append([]string{"list", "-m"}, query.graphFlags...)
	args = append(args, "-f="+format, "all")
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, query.goCommand, args...)
	command.Dir, command.Env, command.Stderr = query.workDir, query.env, &stderr
	output, err := command.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("go list module graph: %w: %s", err, message)
		}
		return "", fmt.Errorf("go list module graph: %w", err)
	}
	digest := sha256.Sum256(output)
	return hex.EncodeToString(digest[:]), nil
}

func queryListedModule(ctx context.Context, query launcherGraphQuery) (listedModule, error) {
	args := append([]string{"list", "-m", "-json"}, query.graphFlags...)
	args = append(args, spxModulePath)
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, query.goCommand, args...)
	command.Dir, command.Env, command.Stderr = query.workDir, query.env, &stderr
	output, err := command.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return listedModule{}, fmt.Errorf("go list module: %w: %s", err, message)
		}
		return listedModule{}, fmt.Errorf("go list module: %w", err)
	}
	var module listedModule
	if err := json.Unmarshal(output, &module); err != nil {
		return listedModule{}, fmt.Errorf("decode go list module: %w", err)
	}
	if module.Path != spxModulePath {
		return listedModule{}, fmt.Errorf("go list returned module %q, want %q", module.Path, spxModulePath)
	}
	return module, nil
}

func normalizeLauncherModule(module listedModule) (launcherModuleIdentity, string, error) {
	identity := launcherModuleIdentity{
		Path: module.Path, Version: module.Version, Sum: module.Sum,
		GoModSum: module.GoModSum, Main: module.Main,
	}
	effective := module
	if module.Replace != nil {
		effective = *module.Replace
		identity.ReplacePath = effective.Path
		identity.ReplaceVersion = effective.Version
		identity.ReplaceSum = effective.Sum
		identity.ReplaceGoModSum = effective.GoModSum
	}
	if !module.Main && (module.Replace == nil || effective.Version != "") {
		return identity, "", nil
	}
	dir, err := canonicalDirectory(effective.Dir)
	if err != nil {
		return launcherModuleIdentity{}, "", fmt.Errorf("local module %q: %w", module.Path, err)
	}
	identity.LocalDir = dir
	if effective.GoMod == "" {
		return launcherModuleIdentity{}, "", fmt.Errorf("local module %q has no go.mod", module.Path)
	}
	return identity, effective.GoMod, nil
}

func snapshotLauncherFiles(paths []string) ([]launcherFileIdentity, error) {
	byPath := make(map[string]string, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		canonical, err := canonicalFile(path)
		if err != nil {
			return nil, fmt.Errorf("graph input %q: %w", path, err)
		}
		data, err := os.ReadFile(canonical)
		if err != nil {
			return nil, fmt.Errorf("read graph input %q: %w", canonical, err)
		}
		digest := sha256.Sum256(data)
		byPath[canonical] = hex.EncodeToString(digest[:])
	}
	files := make([]launcherFileIdentity, 0, len(byPath))
	for path, digest := range byPath {
		files = append(files, launcherFileIdentity{path: path, sha256: digest})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}

func launcherGraphEnvironment(base []string, goWork string) []string {
	env := make([]string, 0, len(base)+5)
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok && (key == "GOFLAGS" || key == "GOWORK" || key == "GOOS" || key == "GOARCH" || key == "CGO_ENABLED") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GOFLAGS=", "GOWORK="+goWork, "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
}

func launcherEnvValue(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		key, value, ok := strings.Cut(env[i], "=")
		if ok && key == name {
			return value
		}
	}
	return ""
}
