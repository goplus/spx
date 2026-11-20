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

package ispx

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"runtime"
	"strings"

	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/pkg/ispx/internal/memfs"
)

// spxDir wraps a [fs.FS] to implement [spxfs.Dir].
type spxDir struct {
	fsys   fs.FS
	prefix string
}

// Open implements [spxfs.Dir].
func (d *spxDir) Open(file string) (io.ReadCloser, error) {
	if err := d.ValidateResourcePath(file); err != nil {
		return nil, err
	}
	return d.fsys.Open(d.resolve(file))
}

// Close implements [spxfs.Dir].
func (d *spxDir) Close() error {
	return nil
}

// ReadDir implements [spxfs.ReadDirer]. The web interpreter only receives
// source and JSON files, but their paths are sufficient to discover resource
// collection directories such as assets/fonts/*.
func (d *spxDir) ReadDir(name string) ([]spxfs.DirEntry, error) {
	if err := d.ValidateResourcePath(name); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(d.fsys, d.resolve(name))
	if err != nil {
		return nil, err
	}
	return spxfs.DirEntriesFromFS(entries), nil
}

// GetPath implements [spxfs.GdDir]. Binary assets are intentionally omitted
// from the interpreter filesystem and are loaded by Godot from this path.
func (d *spxDir) GetPath() string {
	return d.prefix
}

// ValidateResourcePath prevents physical project symlinks from redirecting a
// configuration reference outside the asset tree. Web sources omit binaries,
// so a missing entry is left to the engine's mounted asset filesystem.
func (d *spxDir) ValidateResourcePath(name string) error {
	if spxfs.IsAbsResourcePath(name) || strings.IndexByte(name, 0) >= 0 || !fs.ValidPath(path.Clean(name)) {
		return &fs.PathError{Op: "validate", Path: name, Err: fs.ErrInvalid}
	}
	// Memory sources have no symlinks. Mounted Web archives are checked at export.
	_, memorySource := d.fsys.(*memfs.FS)
	if memorySource || runtime.GOOS == "js" {
		return nil
	}
	current := "."
	for part := range strings.SplitSeq(d.resolve(name), "/") {
		current = path.Join(current, part)
		mode, err := d.componentMode(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if mode&fs.ModeSymlink != 0 {
			return fmt.Errorf("resource %q uses a symlink", name)
		}
	}
	return nil
}

func (d *spxDir) AllowsLiteralResPrefix() bool {
	return runtime.GOOS == "js" || spxfs.AllowsLiteralResPrefix(d.fsys)
}

func (d *spxDir) resolve(name string) string {
	if d.prefix == "" {
		return name
	}
	return path.Join(d.prefix, name)
}

func (d *spxDir) componentMode(name string) (fs.FileMode, error) {
	// Native file systems can look up the entry directly, using the volume's
	// own case and Unicode matching rules.
	if files, ok := d.fsys.(interface {
		Lstat(string) (fs.FileInfo, error)
	}); ok {
		info, err := files.Lstat(name)
		if err != nil {
			return 0, err
		}
		return info.Mode(), nil
	}
	if name == "." {
		return fs.ModeDir, nil
	}
	// An fs.FS wrapper can hide Lstat but expose symlinks through ReadDir.
	parent, base := path.Split(name)
	entries, err := fs.ReadDir(d.fsys, path.Clean(parent))
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if entry.Name() == base {
			return entry.Type(), nil
		}
	}
	// If this spelling still opens, its directory entry cannot be verified.
	if _, err := fs.Stat(d.fsys, name); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("cannot identify directory entry for resource %q", name)
}

// newSpxDir creates a new spx directory.
func newSpxDir(fsys fs.FS, prefix string) spxfs.Dir {
	return &spxDir{fsys: fsys, prefix: prefix}
}
