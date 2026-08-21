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

package projectbundle

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
)

const (
	defaultMaxEntries            = 10_000
	defaultMaxFileBytes    int64 = 64 << 20
	defaultMaxTotalBytes   int64 = 256 << 20
	defaultMaxArchiveBytes int64 = 512 << 20
)

var (
	// ErrInvalidPath reports an absolute, traversing, or non-canonical member path.
	ErrInvalidPath = errors.New("projectbundle: invalid relative path")
	// ErrUnsafeFile reports a symlink, non-regular file, or file changed while read.
	ErrUnsafeFile = errors.New("projectbundle: unsafe file")
	// ErrCollision reports duplicate, Unicode-canonical, or case-folded entry names.
	ErrCollision = errors.New("projectbundle: entry name collision")
	// ErrLimit reports an entry-count or byte-size limit violation.
	ErrLimit = errors.New("projectbundle: limit exceeded")
)

// Config defines the complete allowlist and limits for one project archive.
type Config struct {
	ProjectDir    string
	ProjectFiles  []string
	IncludeConfig bool
	ConfigBytes   []byte
	PackDir       string
	Output        string
	FinalOutput   string
	Limits        Limits
}

// Limits bounds collection memory and ZIP output. Zero selects a default;
// negative values are invalid.
type Limits struct {
	MaxEntries      int
	MaxFileBytes    int64
	MaxTotalBytes   int64
	MaxArchiveBytes int64
}

// Digest is the SHA-256 of the complete canonical ZIP bytes.
type Digest [sha256.Size]byte

type collectedEntry struct {
	name string
	data []byte
}

// bundle is an immutable in-memory snapshot of collected source files.
type bundle struct {
	entries []collectedEntry
	limits  resolvedLimits
}

func collect(cfg Config) (*bundle, error) {
	limits, err := resolveLimits(cfg.Limits)
	if err != nil {
		return nil, err
	}
	if cfg.ProjectDir == "" {
		return nil, fmt.Errorf("%w: ProjectDir is empty", ErrInvalidPath)
	}

	projectRootObservation, err := observeRoot(cfg.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("projectbundle: observe ProjectDir: %w", err)
	}
	projectDir := projectRootObservation.path

	projectFiles := make([]string, len(cfg.ProjectFiles))
	for i, name := range cfg.ProjectFiles {
		projectFiles[i], err = validateRelativePath(name, "ProjectFiles")
		if err != nil {
			return nil, err
		}
	}

	packDir := ""
	if cfg.PackDir != "" {
		packDir, err = validateRelativePath(cfg.PackDir, "PackDir")
		if err != nil {
			return nil, err
		}
	}
	packRoot := ""
	if packDir != "" {
		packRoot = filepath.Join(projectDir, filepath.FromSlash(packDir))
	}
	if err := validateOutputs(projectDir, cfg.Output, cfg.FinalOutput, packRoot); err != nil {
		return nil, err
	}

	projectRoot, err := openSafeRoot(projectDir, projectRootObservation.info)
	if err != nil {
		return nil, fmt.Errorf("projectbundle: open ProjectDir %q: %w", projectDir, err)
	}
	defer projectRoot.Close()

	collector := newCollector(limits)
	for _, name := range projectFiles {
		if err := collector.addFile(projectRoot, name, name); err != nil {
			return nil, err
		}
	}
	if cfg.IncludeConfig {
		if cfg.ConfigBytes != nil {
			err = collector.addData(".config", cfg.ConfigBytes)
		} else {
			err = collector.addFile(projectRoot, ".config", ".config")
		}
		if err != nil {
			return nil, err
		}
	}
	if packDir != "" {
		root, err := projectRoot.OpenDir(filepath.FromSlash(packDir))
		if err != nil {
			return nil, fmt.Errorf("projectbundle: open PackDir %q: %w", packDir, err)
		}
		if err := collector.addTree(root, packDir); err != nil {
			root.Close()
			return nil, err
		}
		if err := root.Close(); err != nil {
			return nil, fmt.Errorf("projectbundle: close PackDir %q: %w", packDir, err)
		}
	}
	sort.Slice(collector.entries, func(i, j int) bool {
		return collector.entries[i].name < collector.entries[j].name
	})
	return &bundle{entries: collector.entries, limits: limits}, nil
}
