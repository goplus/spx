//go:build !android && !ios

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

package asset_test

import (
	"errors"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	spxfs "github.com/goplus/spx/v3/fs"
	"github.com/goplus/spx/v3/fs/asset"
)

func TestOpenRegistersDefaultSchema(t *testing.T) {
	root := t.TempDir()
	writeAssetFile(t, root, "work/assets/local.txt", "local contents")
	writeAssetFile(t, root, "assets/local.txt", "parent contents")
	writeAssetFile(t, root, "assets/nested/parent.txt", "parent-only contents")
	t.Chdir(filepath.Join(root, "work"))

	dir, err := spxfs.Open("assets")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dir.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	assets, ok := dir.(*asset.FS)
	if !ok {
		t.Fatalf("default schema opened %T, want *asset.FS", dir)
	}
	if got := assets.GetPath(); got != "assets/" {
		t.Errorf("GetPath() = %q, want %q", got, "assets/")
	}

	for _, tc := range []struct {
		name    string
		path    string
		want    string
		wantErr error
	}{
		{name: "local takes precedence", path: "local.txt", want: "local contents"},
		{name: "parent fallback", path: "nested/parent.txt", want: "parent-only contents"},
		{name: "missing in both locations", path: "missing.txt", wantErr: iofs.ErrNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := dir.Open(tc.path)
			if err == nil {
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						t.Errorf("Close() error = %v", err)
					}
				})
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Open(%q) error = %v, want %v", tc.path, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				return
			}
			contents, err := io.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			if string(contents) != tc.want {
				t.Errorf("Open(%q) contents = %q, want %q", tc.path, contents, tc.want)
			}
		})
	}
}

func TestReadDir(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"work/assets/sprites/z.txt",
		"work/assets/sprites/a.txt",
		"work/assets/sprites/sub/inside.txt",
		"assets/sprites/parent.txt",
		"assets/sounds/z.wav",
		"assets/sounds/a.wav",
	} {
		writeAssetFile(t, root, name, name)
	}
	if err := os.MkdirAll(filepath.Join(root, "work", "assets", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "work"))

	dir, err := asset.Open("assets")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dir.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	reader, ok := dir.(spxfs.ReadDirer)
	if !ok {
		t.Fatalf("%T does not implement fs.ReadDirer", dir)
	}

	for _, tc := range []struct {
		name    string
		path    string
		want    []spxfs.DirEntry
		wantErr error
	}{
		{
			name: "local sorted direct children",
			path: "sprites",
			want: []spxfs.DirEntry{{Name: "a.txt"}, {Name: "sub", IsDir: true}, {Name: "z.txt"}},
		},
		{
			name: "parent fallback",
			path: "sounds",
			want: []spxfs.DirEntry{{Name: "a.wav"}, {Name: "z.wav"}},
		},
		{name: "empty directory", path: "empty"},
		{name: "missing in both locations", path: "missing", wantErr: iofs.ErrNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := reader.ReadDir(tc.path)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ReadDir(%q) error = %v, want %v", tc.path, err, tc.wantErr)
			}
			if !slices.Equal(entries, tc.want) {
				t.Errorf("ReadDir(%q) = %#v, want %#v", tc.path, entries, tc.want)
			}
		})
	}
}

func writeAssetFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
