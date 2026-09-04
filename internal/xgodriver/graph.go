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

package xgodriver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type graphFile struct {
	path, digest string
	present      bool
}

type graphIdentity struct {
	selectionDigest string
	files           []graphFile
}

type graphVerifier struct {
	cfg      Config
	env      []string
	expected graphIdentity
}

func newGraphVerifier(ctx context.Context, cfg Config, baseEnv []string) (graphVerifier, error) {
	env := hostGoEnv(cfg, baseEnv)
	identity, err := snapshotGraph(ctx, cfg, env)
	if err != nil {
		return graphVerifier{}, err
	}
	return graphVerifier{cfg: cfg, env: env, expected: identity}, nil
}

func (v graphVerifier) verify(ctx context.Context) error {
	current, err := snapshotGraph(ctx, v.cfg, v.env)
	if err != nil {
		return err
	}
	if current.selectionDigest != v.expected.selectionDigest {
		return fmt.Errorf("module selection changed")
	}
	if change := describeGraphFileChange(v.expected.files, current.files); change != "" {
		return fmt.Errorf("graph input changed: %s", change)
	}
	return nil
}

func snapshotGraph(ctx context.Context, cfg Config, env []string) (graphIdentity, error) {
	modules, err := queryModuleGraph(ctx, cfg, env)
	if err != nil {
		return graphIdentity{}, err
	}
	files, err := snapshotGraphFiles(ctx, cfg, env, modules.localFiles)
	if err != nil {
		return graphIdentity{}, err
	}
	return graphIdentity{selectionDigest: modules.selectionDigest, files: files}, nil
}

type graphModuleSelection struct {
	Path    string
	Version string
	Main    bool
	Dir     string
	GoMod   string
	Replace *graphModuleSelection
}

type moduleGraph struct {
	selectionDigest string
	localFiles      []graphPath
}

func queryModuleGraph(ctx context.Context, cfg Config, env []string) (moduleGraph, error) {
	args := append([]string{"list", "-m"}, cfg.graphFlagsForCommand()...)
	args = append(args, "-json", "all")
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, cfg.GoCommand, args...)
	command.Dir, command.Env, command.Stderr = cfg.GraphWorkDir, env, &stderr
	output, err := command.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return moduleGraph{}, fmt.Errorf("xgodriver: query module graph: %w: %s", err, message)
		}
		return moduleGraph{}, fmt.Errorf("xgodriver: query module graph: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	modules := make([]listedModule, 0)
	for {
		var module listedModule
		if err := decoder.Decode(&module); err == io.EOF {
			break
		} else if err != nil {
			return moduleGraph{}, fmt.Errorf("xgodriver: decode module graph: %w", err)
		}
		modules = append(modules, module)
	}
	return summarizeModuleGraph(modules)
}

func summarizeModuleGraph(modules []listedModule) (moduleGraph, error) {
	if len(modules) == 0 {
		return moduleGraph{}, fmt.Errorf("xgodriver: query module graph returned no modules")
	}
	selection := make([]graphModuleSelection, len(modules))
	localFiles := make([]graphPath, 0, len(modules))
	addLocalFile := func(module *listedModule, label string) error {
		file, err := localModuleGraphFile(module, label)
		if err != nil {
			return err
		}
		localFiles = append(localFiles, file)
		return nil
	}
	for i := range modules {
		module := &modules[i]
		selection[i] = graphSelectionForSelectedModule(module)
		if module.Main {
			if err := addLocalFile(module, module.Path); err != nil {
				return moduleGraph{}, err
			}
		}
		if module.Replace != nil && module.Replace.Version == "" {
			if err := addLocalFile(module.Replace, module.Path+" replacement"); err != nil {
				return moduleGraph{}, err
			}
		}
	}
	sort.Slice(selection, func(i, j int) bool {
		if selection[i].Path != selection[j].Path {
			return selection[i].Path < selection[j].Path
		}
		return selection[i].Version < selection[j].Version
	})
	identity, err := json.Marshal(selection)
	if err != nil {
		return moduleGraph{}, fmt.Errorf("xgodriver: encode module graph identity: %w", err)
	}
	digest := sha256.Sum256(identity)
	return moduleGraph{selectionDigest: hex.EncodeToString(digest[:]), localFiles: localFiles}, nil
}

func graphSelectionForSelectedModule(module *listedModule) graphModuleSelection {
	selection := graphModuleSelection{Path: module.Path, Version: module.Version, Main: module.Main}
	if module.Main {
		selection.Dir, selection.GoMod = module.Dir, module.GoMod
	}
	if module.Replace != nil {
		selection.Replace = graphSelectionForReplacement(module.Replace)
	}
	return selection
}

func graphSelectionForReplacement(module *listedModule) *graphModuleSelection {
	selection := &graphModuleSelection{Path: module.Path, Version: module.Version, Main: module.Main}
	if module.Version == "" {
		selection.Path = module.Dir
		selection.Dir, selection.GoMod = module.Dir, module.GoMod
	}
	if module.Replace != nil {
		selection.Replace = graphSelectionForReplacement(module.Replace)
	}
	return selection
}

func localModuleGraphFile(module *listedModule, label string) (graphPath, error) {
	if module.GoMod == "" || module.GoMod == os.DevNull || !filepath.IsAbs(module.GoMod) {
		return graphPath{}, fmt.Errorf("xgodriver: local module %q has invalid GoMod path %q", label, module.GoMod)
	}
	return graphPath{path: module.GoMod, required: true}, nil
}

type graphPath struct {
	path     string
	required bool
}

func snapshotGraphFiles(ctx context.Context, cfg Config, env []string, moduleFiles []graphPath) ([]graphFile, error) {
	paths := make(graphPathSet, len(moduleFiles)*2+6)
	privateModFile := cfg.privateModFileForCommand()
	for _, file := range moduleFiles {
		if privateModFile != "" && normalizeGraphPath(file.path) == normalizeGraphPath(privateModFile) {
			continue
		}
		paths.addMod(file.path, file.required)
	}
	paths.add(cfg.TargetModFile.Path, true)
	mod, err := activeGoMod(ctx, cfg, env)
	if err != nil {
		return nil, err
	}
	paths.addMod(mod, true)
	if cfg.GoWork != "" && cfg.GoWork != "off" {
		paths.addWork(cfg.GoWork)
	}
	if effective := cfg.DriverOrigin.Effective(); cfg.DriverOrigin.IsLocal() && effective.GoMod != "" {
		paths.addMod(effective.GoMod, true)
	}
	files, err := readGraphFiles(paths.sorted())
	if err != nil {
		return nil, err
	}
	if cfg.TargetModFile.Path != "" {
		target := normalizeGraphPath(cfg.TargetModFile.Path)
		for _, file := range files {
			if file.path == target && file.digest != cfg.TargetModFile.SHA256 {
				return nil, fmt.Errorf("xgodriver: target modfile %q changed after XGo resolution", target)
			}
		}
	}
	return files, nil
}

func queryGoEnv(ctx context.Context, cfg Config, env []string, name string) (string, error) {
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, cfg.GoCommand, "env", name)
	command.Dir, command.Env, command.Stderr = cfg.GraphWorkDir, env, &stderr
	output, err := command.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("xgodriver: query Go env %s: %w: %s", name, err, message)
		}
		return "", fmt.Errorf("xgodriver: query Go env %s: %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func graphModfilePath(flag string) (string, bool) {
	if path, ok := strings.CutPrefix(flag, "-modfile="); ok {
		return path, path != ""
	}
	if path, ok := strings.CutPrefix(flag, "--modfile="); ok {
		return path, path != ""
	}
	return "", false
}

type graphPathSet map[string]bool

func normalizeGraphPath(path string) string {
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		if absolute, err := filepath.Abs(path); err == nil {
			path = absolute
		}
	}
	return path
}

func (s graphPathSet) add(path string, required bool) {
	s.addNormalized(normalizeGraphPath(path), required)
}

func (s graphPathSet) addNormalized(path string, required bool) {
	if path != "" {
		s[path] = s[path] || required
	}
}

func (s graphPathSet) addMod(path string, required bool) {
	path = normalizeGraphPath(path)
	s.addNormalized(path, required)
	if sumPath := goModSumPath(path); sumPath != "" {
		s.addNormalized(sumPath, false)
	}
}

func (s graphPathSet) addWork(path string) {
	path = normalizeGraphPath(path)
	s.addNormalized(path, true)
	if path != "" {
		s.addNormalized(path+".sum", false)
	}
}

func (s graphPathSet) sorted() []graphPath {
	items := make([]graphPath, 0, len(s))
	for path, required := range s {
		items = append(items, graphPath{path: path, required: required})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].path < items[j].path })
	return items
}

func readGraphFiles(paths []graphPath) ([]graphFile, error) {
	files := make([]graphFile, 0, len(paths))
	for _, item := range paths {
		file, err := readGraphFile(item, graphFileAccess{lstat: os.Lstat, open: os.Open})
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

const maxGraphFileBytes = int64(64 << 20)

type graphFileAccess struct {
	lstat func(string) (os.FileInfo, error)
	open  func(string) (*os.File, error)
}

func readGraphFile(item graphPath, access graphFileAccess) (result graphFile, err error) {
	result, _, err = readGraphFileContent(item, access, false)
	return result, err
}

func readGraphFileData(item graphPath) (graphFile, []byte, error) {
	return readGraphFileContent(item, graphFileAccess{lstat: os.Lstat, open: os.Open}, true)
}

func readGraphFileContent(item graphPath, access graphFileAccess, capture bool) (result graphFile, data []byte, err error) {
	result.path = item.path
	before, err := access.lstat(item.path)
	if os.IsNotExist(err) {
		if item.required {
			return graphFile{}, nil, fmt.Errorf("xgodriver: graph input %q is missing", item.path)
		}
		return result, nil, nil
	}
	if err != nil {
		return graphFile{}, nil, fmt.Errorf("xgodriver: inspect graph input %q: %w", item.path, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return graphFile{}, nil, fmt.Errorf("xgodriver: graph input %q is not a regular non-symlink file", item.path)
	}
	if before.Size() < 0 || before.Size() > maxGraphFileBytes {
		return graphFile{}, nil, fmt.Errorf("xgodriver: graph input %q exceeds %d bytes", item.path, maxGraphFileBytes)
	}

	file, err := access.open(item.path)
	if err != nil {
		return graphFile{}, nil, fmt.Errorf("xgodriver: open graph input %q: %w", item.path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("xgodriver: close graph input %q: %w", item.path, closeErr)
		}
	}()
	opened, err := file.Stat()
	if err != nil {
		return graphFile{}, nil, fmt.Errorf("xgodriver: stat graph input %q: %w", item.path, err)
	}
	if !opened.Mode().IsRegular() || !sameStableGraphFile(before, opened) {
		return graphFile{}, nil, fmt.Errorf("xgodriver: graph input %q changed while opening", item.path)
	}

	hasher := sha256.New()
	writer := io.Writer(hasher)
	var content bytes.Buffer
	if capture {
		content.Grow(int(before.Size()))
		writer = io.MultiWriter(hasher, &content)
	}
	read, err := io.Copy(writer, io.LimitReader(file, maxGraphFileBytes+1))
	if err != nil {
		return graphFile{}, nil, fmt.Errorf("xgodriver: read graph input %q: %w", item.path, err)
	}
	if read > maxGraphFileBytes {
		return graphFile{}, nil, fmt.Errorf("xgodriver: graph input %q exceeds %d bytes", item.path, maxGraphFileBytes)
	}
	openedAfter, statErr := file.Stat()
	after, lstatErr := access.lstat(item.path)
	if statErr != nil || lstatErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() ||
		!sameStableGraphFile(opened, openedAfter) || !sameStableGraphFile(openedAfter, after) || read != openedAfter.Size() {
		return graphFile{}, nil, fmt.Errorf("xgodriver: graph input %q changed while reading", item.path)
	}
	result.digest = hex.EncodeToString(hasher.Sum(nil))
	result.present = true
	return result, content.Bytes(), nil
}

func sameStableGraphFile(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func describeGraphFileChange(left, right []graphFile) string {
	if len(left) != len(right) {
		return fmt.Sprintf("file set count is %d, want %d", len(right), len(left))
	}
	for i := range left {
		if left[i].path != right[i].path {
			return fmt.Sprintf("file set differs at %q and %q", left[i].path, right[i].path)
		}
		if left[i].present != right[i].present {
			return fmt.Sprintf("presence of %q changed", left[i].path)
		}
		if left[i].digest != right[i].digest {
			return fmt.Sprintf("contents of %q changed", left[i].path)
		}
	}
	return ""
}
