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
	"io"
	"io/fs"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// portableConfigFS replaces the project .config with the validated snapshot.
// When the snapshot is absent, it hides a project .config instead.
type portableConfigFS struct {
	base    fs.FS
	present bool
	data    []byte
}

func newPortableConfigFS(base fs.FS, overlay *portableConfigOverlay) *portableConfigFS {
	return &portableConfigFS{base: base, present: overlay.present, data: overlay.data}
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
	mu        sync.Mutex
	entries   []fs.DirEntry
	err       error
	offset    int
}

func (f *portableConfigRoot) ReadDir(n int) ([]fs.DirEntry, error) {
	f.once.Do(func() {
		if f.directory == nil {
			f.err = &fs.PathError{Op: "readdir", Path: ".", Err: fs.ErrInvalid}
		} else {
			entries, err := f.directory.ReadDir(-1)
			if err != nil {
				f.err = err
			} else {
				f.entries = f.overlay.overlayRootEntries(entries)
			}
		}
	})
	if f.err != nil {
		return nil, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
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
	return &portableConfigFile{Reader: bytes.NewReader(data), info: portableConfigInfo{size: int64(len(data))}}
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
