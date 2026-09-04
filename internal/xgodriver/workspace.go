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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

func isolateGoGraph(ctx context.Context, cfg Config, baseEnv []string) (Config, func(), error) {
	if cfg.GoWork != "" && cfg.GoWork != "off" {
		return isolateGoWorkspace(ctx, cfg, baseEnv)
	}
	if containsGraphFlag(cfg.GraphFlags, "-mod=mod") {
		return isolateGoMod(ctx, cfg, baseEnv)
	}
	return cfg, func() {}, nil
}

func isolateGoWorkspace(ctx context.Context, cfg Config, baseEnv []string) (Config, func(), error) {
	workIdentity, workData, err := readGraphFileData(graphPath{path: cfg.GoWork, required: true})
	if err != nil {
		return Config{}, nil, err
	}
	sumPath := cfg.GoWork + ".sum"
	sumIdentity, sumData, err := readGraphFileData(graphPath{path: sumPath})
	if err != nil {
		return Config{}, nil, err
	}
	expectedFiles := []graphFile{workIdentity, sumIdentity}

	rewritten, err := rewriteGoWork(cfg.GoWork, workData)
	if err != nil {
		return Config{}, nil, fmt.Errorf("xgodriver: rewrite private Go workspace: %w", err)
	}

	privateDir, err := os.MkdirTemp("", "spx-xgodriver-work-")
	if err != nil {
		return Config{}, nil, fmt.Errorf("xgodriver: create private Go workspace: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(privateDir) }
	removeOnReturn := true
	defer func() {
		if removeOnReturn {
			cleanup()
		}
	}()
	privateWork := filepath.Join(privateDir, "go.work")
	if err := os.WriteFile(privateWork, rewritten, 0o600); err != nil {
		return Config{}, nil, fmt.Errorf("xgodriver: write private Go workspace: %w", err)
	}
	if sumIdentity.present {
		if err := os.WriteFile(privateWork+".sum", sumData, 0o600); err != nil {
			return Config{}, nil, fmt.Errorf("xgodriver: write private Go workspace sum: %w", err)
		}
	}

	isolated := cfg
	isolated.commandGraph = &goCommandGraph{goWork: privateWork, flags: cfg.GraphFlags}
	_, err = queryModuleGraph(ctx, isolated, hostGoEnv(isolated, baseEnv))
	if err != nil {
		return Config{}, nil, fmt.Errorf("xgodriver: validate private Go workspace: %w", err)
	}

	currentWork, _, err := readGraphFileData(graphPath{path: cfg.GoWork, required: true})
	if err != nil {
		return Config{}, nil, err
	}
	currentSum, _, err := readGraphFileData(graphPath{path: sumPath})
	if err != nil {
		return Config{}, nil, err
	}
	if change := describeGraphFileChange(expectedFiles, []graphFile{currentWork, currentSum}); change != "" {
		return Config{}, nil, fmt.Errorf("xgodriver: Go workspace changed while creating private snapshot: %s", change)
	}
	removeOnReturn = false
	return isolated, cleanup, nil
}

func isolateGoMod(ctx context.Context, cfg Config, baseEnv []string) (Config, func(), error) {
	env := hostGoEnv(cfg, baseEnv)
	modPath, moduleRoot, err := activeGoModule(ctx, cfg, env)
	if err != nil {
		return Config{}, nil, err
	}
	modIdentity, modData, err := readGraphFileData(graphPath{path: modPath, required: true})
	if err != nil {
		return Config{}, nil, err
	}
	expectedFiles := []graphFile{modIdentity}

	sumPath := goModSumPath(modPath)
	var sumIdentity graphFile
	var sumData []byte
	if sumPath != "" {
		sumIdentity, sumData, err = readGraphFileData(graphPath{path: sumPath})
		if err != nil {
			return Config{}, nil, err
		}
		expectedFiles = append(expectedFiles, sumIdentity)
	}

	rewritten, err := rewriteGoMod(modPath, moduleRoot, modData)
	if err != nil {
		return Config{}, nil, fmt.Errorf("xgodriver: rewrite private Go module: %w", err)
	}
	privateDir, err := os.MkdirTemp("", "spx-xgodriver-mod-")
	if err != nil {
		return Config{}, nil, fmt.Errorf("xgodriver: create private Go module: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(privateDir) }
	removeOnReturn := true
	defer func() {
		if removeOnReturn {
			cleanup()
		}
	}()

	privateMod := filepath.Join(privateDir, "graph.mod")
	if err := os.WriteFile(privateMod, rewritten, 0o600); err != nil {
		return Config{}, nil, fmt.Errorf("xgodriver: write private Go module: %w", err)
	}
	if sumIdentity.present {
		if err := os.WriteFile(goModSumPath(privateMod), sumData, 0o600); err != nil {
			return Config{}, nil, fmt.Errorf("xgodriver: write private Go module sum: %w", err)
		}
	}

	isolated := cfg
	isolated.commandGraph = &goCommandGraph{
		goWork:         cfg.GoWork,
		flags:          withGraphModfile(cfg.GraphFlags, privateMod),
		privateModFile: privateMod,
	}
	if _, err := queryModuleGraph(ctx, isolated, hostGoEnv(isolated, baseEnv)); err != nil {
		return Config{}, nil, fmt.Errorf("xgodriver: validate private Go module: %w", err)
	}

	currentFiles := make([]graphFile, 0, len(expectedFiles))
	currentMod, _, err := readGraphFileData(graphPath{path: modPath, required: true})
	if err != nil {
		return Config{}, nil, err
	}
	currentFiles = append(currentFiles, currentMod)
	if sumPath != "" {
		currentSum, _, err := readGraphFileData(graphPath{path: sumPath})
		if err != nil {
			return Config{}, nil, err
		}
		currentFiles = append(currentFiles, currentSum)
	}
	if change := describeGraphFileChange(expectedFiles, currentFiles); change != "" {
		return Config{}, nil, fmt.Errorf("xgodriver: Go module changed while creating private snapshot: %s", change)
	}
	removeOnReturn = false
	return isolated, cleanup, nil
}

func activeGoMod(ctx context.Context, cfg Config, env []string) (string, error) {
	if path, ok := explicitGoMod(cfg.GraphFlags); ok {
		return path, nil
	}
	return defaultGoMod(ctx, cfg, env)
}

func activeGoModule(ctx context.Context, cfg Config, env []string) (modFile, moduleRoot string, err error) {
	goMod, err := defaultGoMod(ctx, cfg, env)
	if err != nil {
		return "", "", err
	}
	modFile = goMod
	if path, ok := explicitGoMod(cfg.GraphFlags); ok {
		modFile = path
	}
	return modFile, filepath.Dir(goMod), nil
}

func explicitGoMod(flags []string) (string, bool) {
	for _, flag := range flags {
		if path, ok := graphModfilePath(flag); ok {
			return path, true
		}
	}
	return "", false
}

func defaultGoMod(ctx context.Context, cfg Config, env []string) (string, error) {
	path, err := queryGoEnv(ctx, cfg, env, "GOMOD")
	if err != nil {
		return "", err
	}
	if path == "" || path == os.DevNull {
		return "", fmt.Errorf("xgodriver: active Go module file is unavailable")
	}
	return path, nil
}

func rewriteGoMod(path, moduleRoot string, data []byte) ([]byte, error) {
	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, err
	}
	for _, replacement := range file.Replace {
		if replacement.New.Version != "" || filepath.IsAbs(replacement.New.Path) {
			continue
		}
		if len(replacement.Syntax.Token) == 0 {
			return nil, fmt.Errorf("replacement for %s@%s has no syntax token", replacement.Old.Path, replacement.Old.Version)
		}
		absolute := filepath.Clean(filepath.Join(moduleRoot, replacement.New.Path))
		replacement.New.Path = absolute
		replacement.Syntax.Token[len(replacement.Syntax.Token)-1] = modfile.AutoQuote(absolute)
	}
	return modfile.Format(file.Syntax), nil
}

func containsGraphFlag(flags []string, want string) bool {
	for _, flag := range flags {
		if flag == want {
			return true
		}
	}
	return false
}

func withGraphModfile(flags []string, path string) []string {
	result := append([]string(nil), flags...)
	for i, flag := range result {
		if _, ok := graphModfilePath(flag); ok {
			result[i] = "-modfile=" + path
			return result
		}
	}
	return append(result, "-modfile="+path)
}

func goModSumPath(path string) string {
	if !strings.HasSuffix(path, ".mod") {
		return ""
	}
	return strings.TrimSuffix(path, ".mod") + ".sum"
}

func rewriteGoWork(path string, data []byte) ([]byte, error) {
	work, err := modfile.ParseWork(path, data, nil)
	if err != nil {
		return nil, err
	}
	base := filepath.Dir(path)
	for _, use := range work.Use {
		absolute := absoluteWorkspacePath(base, use.Path)
		if len(use.Syntax.Token) == 0 {
			return nil, fmt.Errorf("workspace use path %q has no syntax token", use.Path)
		}
		use.Path = absolute
		use.Syntax.Token[len(use.Syntax.Token)-1] = modfile.AutoQuote(absolute)
	}

	for _, replacement := range work.Replace {
		if replacement.New.Version != "" {
			continue
		}
		if len(replacement.Syntax.Token) == 0 {
			return nil, fmt.Errorf("workspace replacement for %s@%s has no syntax token", replacement.Old.Path, replacement.Old.Version)
		}
		absolute := absoluteWorkspacePath(base, replacement.New.Path)
		replacement.New.Path = absolute
		replacement.Syntax.Token[len(replacement.Syntax.Token)-1] = modfile.AutoQuote(absolute)
	}
	return modfile.Format(work.Syntax), nil
}

func absoluteWorkspacePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}
