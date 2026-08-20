//go:build !js || !wasm

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

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goplus/spx/v3/internal/interpruntime"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

type portableConfigOverlay struct {
	present bool
	data    []byte
}

func loadPortableConfigOverlay(env []string, roots interpruntime.Roots) (*portableConfigOverlay, error) {
	configDir, found, err := interpruntime.PortableConfigDirFromEnv(env)
	if err != nil {
		return nil, err
	}
	expectedIdentity, identityFound, err := interpruntime.PortableConfigIdentityFromEnv(env)
	if err != nil {
		return nil, err
	}
	if found != identityFound {
		return nil, fmt.Errorf("ispxnative: portable config environment is incomplete")
	}
	if !found {
		return nil, nil
	}
	if err := validatePortableConfigDir(roots.SessionDir, configDir); err != nil {
		return nil, err
	}
	configRoot, err := openPortableConfigRoot(roots.SessionDir, configDir)
	if err != nil {
		return nil, err
	}
	defer configRoot.Close()
	snapshot, err := projectpolicy.SnapshotPortableConfigRoot(configRoot)
	if err != nil {
		return nil, fmt.Errorf("ispxnative: load portable config snapshot: %w", err)
	}
	identity, err := snapshot.Identity()
	if err != nil {
		return nil, fmt.Errorf("ispxnative: identify portable config snapshot: %w", err)
	}
	if identity != expectedIdentity {
		return nil, fmt.Errorf("ispxnative: portable config identity mismatch")
	}
	return &portableConfigOverlay{present: snapshot.Present(), data: snapshot.Bytes()}, nil
}

// openPortableConfigRoot pins the session directory before opening the
// provider-owned config directory. The lexical containment check above keeps
// diagnostics clear; the root handle makes a replacement race unable to
// redirect reads outside the session tree.
func openPortableConfigRoot(sessionDir, configDir string) (*os.Root, error) {
	sessionRoot, err := os.OpenRoot(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("ispxnative: open session directory: %w", err)
	}
	rel, err := filepath.Rel(sessionDir, configDir)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		_ = sessionRoot.Close()
		return nil, fmt.Errorf("ispxnative: portable config directory %q is outside session directory %q", configDir, sessionDir)
	}
	before, err := sessionRoot.Lstat(rel)
	if err != nil {
		_ = sessionRoot.Close()
		return nil, fmt.Errorf("ispxnative: inspect portable config directory: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		_ = sessionRoot.Close()
		return nil, fmt.Errorf("ispxnative: portable config directory %q must be a real directory", configDir)
	}
	configRoot, err := sessionRoot.OpenRoot(rel)
	if err != nil {
		_ = sessionRoot.Close()
		return nil, fmt.Errorf("ispxnative: open portable config directory: %w", err)
	}
	after, statErr := configRoot.Stat(".")
	pathAfter, pathErr := sessionRoot.Lstat(rel)
	if statErr != nil || pathErr != nil || !os.SameFile(before, after) || !os.SameFile(after, pathAfter) {
		_ = configRoot.Close()
		_ = sessionRoot.Close()
		if statErr != nil {
			return nil, fmt.Errorf("ispxnative: stat pinned portable config directory: %w", statErr)
		}
		if pathErr != nil {
			return nil, fmt.Errorf("ispxnative: verify portable config directory: %w", pathErr)
		}
		return nil, fmt.Errorf("ispxnative: portable config directory changed while opening")
	}
	closeErr := sessionRoot.Close()
	if closeErr != nil {
		_ = configRoot.Close()
		return nil, fmt.Errorf("ispxnative: close session directory: %w", closeErr)
	}
	return configRoot, nil
}

func validatePortableConfigDir(sessionDir, configDir string) error {
	if configDir == "" {
		return fmt.Errorf("ispxnative: portable config directory is empty")
	}
	if !filepath.IsAbs(configDir) {
		return fmt.Errorf("ispxnative: portable config directory %q is not absolute", configDir)
	}
	if clean := filepath.Clean(configDir); clean != configDir {
		return fmt.Errorf("ispxnative: portable config directory %q is not clean (want %q)", configDir, clean)
	}
	info, err := os.Lstat(configDir)
	if err != nil {
		return fmt.Errorf("ispxnative: inspect portable config directory %q: %w", configDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("ispxnative: portable config directory %q must be a real directory", configDir)
	}
	canonicalSession, err := filepath.EvalSymlinks(sessionDir)
	if err != nil {
		return fmt.Errorf("ispxnative: canonicalize session directory %q: %w", sessionDir, err)
	}
	canonicalConfig, err := filepath.EvalSymlinks(configDir)
	if err != nil {
		return fmt.Errorf("ispxnative: canonicalize portable config directory %q: %w", configDir, err)
	}
	rel, err := filepath.Rel(canonicalSession, canonicalConfig)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("ispxnative: portable config directory %q must be below session directory %q", configDir, sessionDir)
	}
	return nil
}

type portableConfigFS struct {
	base    fs.FS
	present bool
	data    []byte
}

func newPortableConfigFS(base fs.FS, overlay *portableConfigOverlay) *portableConfigFS {
	data := make([]byte, len(overlay.data))
	copy(data, overlay.data)
	return &portableConfigFS{base: base, present: overlay.present, data: data}
}

func (f *portableConfigFS) Open(name string) (fs.File, error) {
	if isPortableConfigPath(name) {
		if !isPortableConfigName(name) || !f.present {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		return newPortableConfigFile(f.data), nil
	}
	file, err := f.base.Open(name)
	if err != nil {
		return nil, err
	}
	if name == "." {
		directory, _ := file.(fs.ReadDirFile)
		return &portableConfigRoot{File: file, directory: directory, overlay: f}, nil
	}
	return file, nil
}

func (f *portableConfigFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if isPortableConfigPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	entries, err := fs.ReadDir(f.base, name)
	if err != nil {
		return nil, err
	}
	if name == "." {
		entries = f.overlayRootEntries(entries)
	}
	return entries, nil
}

func (f *portableConfigFS) overlayRootEntries(entries []fs.DirEntry) []fs.DirEntry {
	result := make([]fs.DirEntry, 0, len(entries)+1)
	for _, entry := range entries {
		if isPortableConfigName(entry.Name()) {
			continue
		}
		result = append(result, entry)
	}
	if f.present {
		result = append(result, fs.FileInfoToDirEntry(portableConfigInfo{size: int64(len(f.data))}))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name() < result[j].Name() })
	return result
}

func isPortableConfigPath(name string) bool {
	first, _, _ := strings.Cut(name, "/")
	return isPortableConfigName(first)
}

func isPortableConfigName(name string) bool {
	return name == ".config" || (runtime.GOOS == "windows" && strings.EqualFold(name, ".config"))
}

type portableConfigRoot struct {
	fs.File
	directory fs.ReadDirFile
	overlay   *portableConfigFS
	once      sync.Once
	entries   []fs.DirEntry
	err       error
	offset    int
}

func (f *portableConfigRoot) ReadDir(n int) ([]fs.DirEntry, error) {
	f.once.Do(func() {
		if f.directory == nil {
			f.err = &fs.PathError{Op: "readdir", Path: ".", Err: fs.ErrInvalid}
			return
		}
		entries, err := f.directory.ReadDir(-1)
		if err != nil {
			f.err = err
			return
		}
		f.entries = f.overlay.overlayRootEntries(entries)
	})
	if f.err != nil {
		return nil, f.err
	}
	if n <= 0 {
		entries := f.entries[f.offset:]
		f.offset = len(f.entries)
		return entries, nil
	}
	if f.offset >= len(f.entries) {
		return nil, io.EOF
	}
	end := min(f.offset+n, len(f.entries))
	entries := f.entries[f.offset:end]
	f.offset = end
	if len(entries) < n {
		return entries, io.EOF
	}
	return entries, nil
}

type portableConfigFile struct {
	*bytes.Reader
	info portableConfigInfo
}

func newPortableConfigFile(data []byte) *portableConfigFile {
	contents := make([]byte, len(data))
	copy(contents, data)
	return &portableConfigFile{Reader: bytes.NewReader(contents), info: portableConfigInfo{size: int64(len(contents))}}
}

func (f *portableConfigFile) Close() error               { return nil }
func (f *portableConfigFile) Stat() (fs.FileInfo, error) { return f.info, nil }

type portableConfigInfo struct {
	size int64
}

func (portableConfigInfo) Name() string       { return ".config" }
func (i portableConfigInfo) Size() int64      { return i.size }
func (portableConfigInfo) Mode() fs.FileMode  { return 0o444 }
func (portableConfigInfo) ModTime() time.Time { return time.Time{} }
func (portableConfigInfo) IsDir() bool        { return false }
func (portableConfigInfo) Sys() any           { return nil }
