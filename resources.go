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

package spx

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	spxfs "github.com/goplus/spx/v4/fs"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

type engineResourceDir struct {
	spxfs.Dir
	assetDir string
}

type engineResourceDirWithReadDir struct {
	*engineResourceDir
	spxfs.ReadDirer
}

func (d *engineResourceDir) GetPath() string { return d.assetDir }

func (d *engineResourceDir) Open(name string) (io.ReadCloser, error) {
	if spxfs.IsAbsResourcePath(name) {
		return nil, fmt.Errorf("resource path %q must be relative", name)
	}
	if d.usesEngineFiles() {
		for _, candidate := range engineLookupPaths(d.assetDir, name) {
			if engine.HasFile(candidate) {
				return io.NopCloser(strings.NewReader(engine.ReadAllText(candidate))), nil
			}
		}
	}
	return d.Dir.Open(name)
}

// VerifyFile checks binary assets without reading them through the text bridge.
func (d *engineResourceDir) VerifyFile(name string) error {
	if spxfs.IsAbsResourcePath(name) {
		return fmt.Errorf("resource path %q must be relative", name)
	}
	if d.usesEngineFiles() {
		return d.verifyEngineFile(name)
	}
	if verifier, ok := d.Dir.(spxfs.FileVerifier); ok {
		return verifier.VerifyFile(name)
	}
	file, err := d.Dir.Open(name)
	if err == nil {
		return file.Close()
	}
	// Interpreted Web projects contain source and JSON; their binaries remain
	// in the engine's mounted project data under the same logical asset path.
	if d.verifyEngineFile(name) == nil {
		return nil
	}
	return err
}

// ListDirectories supplies directory-only discovery without claiming the full
// ReadDirer capability. ReadDir, when available, still delegates to the source.
func (d *engineResourceDir) ListDirectories(name string) ([]string, error) {
	if d.usesEngineFiles() {
		var names []string
		encoded := engine.ListDirectories(d.assetDir + "/" + name)
		if err := json.Unmarshal([]byte(encoded), &names); err != nil {
			return nil, fmt.Errorf("list directories through engine: %w", err)
		}
		return names, nil
	}
	return spxfs.ListDirectories(d.Dir, name)
}

func (d *engineResourceDir) ValidateResourcePath(name string) error {
	return spxfs.ValidateResourcePath(d.Dir, name)
}

func (d *engineResourceDir) AllowsLiteralResPrefix() bool {
	return !d.usesEngineFiles() && spxfs.AllowsLiteralResPrefix(d.Dir)
}

func (d *engineResourceDir) usesEngineFiles() bool {
	schema, _ := spxfs.SplitSchema(d.assetDir)
	return schema != ""
}

func (d *engineResourceDir) verifyEngineFile(name string) error {
	if slices.ContainsFunc(engineLookupPaths(d.assetDir, name), engine.HasFile) {
		return nil
	}
	return os.ErrNotExist
}

// openGameResources installs engine access beneath the packed configuration overlay.
// The returned directory, or the core loader on failure, owns closing the source.
func openGameResources(resource any, conf *coreproject.Config) (coreproject.OpenedBuilderResources, error) {
	assetDir := coreproject.AssetDirFromResource(resource)
	dir, err := coreproject.ResourceDir(resource)
	if err != nil {
		return coreproject.OpenedBuilderResources{}, err
	}
	opened, err := coreproject.OpenBuilderResources(withEngineAssets(dir), conf)
	if err != nil {
		return coreproject.OpenedBuilderResources{}, err
	}
	opened.AssetDir = assetDir
	return opened, nil
}

func withEngineAssets(dir spxfs.Dir) spxfs.Dir {
	switch dir.(type) {
	case *engineResourceDir, *engineResourceDirWithReadDir:
		return dir
	}
	gdDir, ok := dir.(spxfs.GdDir)
	if !ok {
		return dir
	}
	assetDir := strings.TrimSuffix(gdDir.GetPath(), "/")
	if assetDir == "" {
		return dir
	}
	adapted := &engineResourceDir{Dir: dir, assetDir: assetDir}
	if reader, ok := dir.(spxfs.ReadDirer); ok {
		return &engineResourceDirWithReadDir{engineResourceDir: adapted, ReadDirer: reader}
	}
	return adapted
}

func engineLookupPaths(assetDir, file string) []string {
	// Godot treats backslashes as separators. Only the source filesystem
	// can look up a POSIX resource name containing a literal backslash.
	assetDir = strings.TrimSuffix(filepath.ToSlash(assetDir), "/")
	if strings.ContainsRune(file, '\\') || strings.ContainsRune(assetDir, '\\') {
		return nil
	}
	file = path.Clean(file)
	if assetDir == "" {
		return []string{file}
	}
	paths := []string{assetDir + "/" + file}
	if schema, _ := spxfs.SplitSchema(assetDir); schema == "" {
		paths = append(paths, "res://"+strings.TrimPrefix(assetDir, "/")+"/"+file)
	}
	// Preparation runs before SetAssetDir. A fallback through ToAssetPath
	// would read the previous project's files when this source is missing one.
	return paths
}
