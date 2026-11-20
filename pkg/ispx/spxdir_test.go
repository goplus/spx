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
	"io"
	"io/fs"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/pkg/ispx/internal/memfs"
)

type literalResTestFS struct{ fs.FS }

func (literalResTestFS) AllowsLiteralResPrefix() bool { return true }

func TestSpxDirAllowsLiteralResPrefix(t *testing.T) {
	base := fstest.MapFS{}
	if got, want := spxfs.AllowsLiteralResPrefix(newSpxDir(base, "assets")), runtime.GOOS == "js"; got != want {
		t.Errorf("literal res: filename with MapFS = %v, want %v", got, want)
	}
	if !spxfs.AllowsLiteralResPrefix(newSpxDir(literalResTestFS{base}, "assets")) {
		t.Fatal("native filename support was not forwarded")
	}
}

func TestWebProjectResourcePaths(t *testing.T) {
	if runtime.GOOS != "js" {
		t.Skip("requires the Web interpreter filesystem")
	}
	// The Web runner keeps images in Godot and sends only configs and code to ispx.
	for format, files := range map[string]map[string][]byte{
		"source": {
			"assets/index.json":                  []byte(`{"backdrops":[{"path":"res:image.png"}]}`),
			"assets/sprites/Cat/index.json":      []byte(`{"costumes":[{"path":"res:image.png"}]}`),
			"assets/sprites/URI/index.json":      []byte(`{"costumes":[{"path":"res://image.png"}]}`),
			"assets/sprites/Absolute/index.json": []byte(`{"costumes":[{"path":"/image.png"}]}`),
		},
		"packed": {
			"assets/index_pack.json": []byte(`{
				"backdrops":[{"path":"res:image.png"}],
				"sprites":{
					"Cat":{"costumes":[{"path":"res:image.png"}]},
					"URI":{"costumes":[{"path":"res://image.png"}]},
					"Absolute":{"costumes":[{"path":"/image.png"}]}
				}
			}`),
		},
	} {
		t.Run(format, func(t *testing.T) {
			opened, err := project.OpenBuilderResources(newSpxDir(memfs.New(files), "assets"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer opened.FS.Close()
			if got := opened.Project.Backdrops[0].Path; got != "./res:image.png" {
				t.Errorf("backdrop path = %q, want ./res:image.png", got)
			}
			sprite, err := project.LoadSpriteConfig(opened.FS, "Cat")
			if err != nil {
				t.Fatal(err)
			}
			if got := sprite.Costumes[0].Path; got != "sprites/Cat/res:image.png" {
				t.Errorf("costume path = %q, want sprites/Cat/res:image.png", got)
			}
			for _, name := range []string{"URI", "Absolute"} {
				if _, err := project.LoadSpriteConfig(opened.FS, name); err == nil || !strings.Contains(err.Error(), "must be relative") {
					t.Errorf("LoadSpriteConfig(%q) error = %v, want relative path error", name, err)
				}
			}
		})
	}
}

func TestSpxDirOpen(t *testing.T) {
	fsys := fstest.MapFS{
		"file.txt":        {Data: []byte("hello")},
		"subdir/file.txt": {Data: []byte("world")},
	}
	for _, tt := range []struct{ name, prefix, content string }{
		{"WithoutPrefix", "", "hello"},
		{"WithPrefix", "subdir", "world"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := newSpxDir(fsys, tt.prefix)
			if dir == nil {
				t.Fatal("expected non-nil dir")
			}
			if path := dir.(spxfs.GdDir).GetPath(); path != tt.prefix {
				t.Fatalf("GetPath = %q, want %q", path, tt.prefix)
			}
			reader, err := dir.Open("file.txt")
			if err != nil {
				t.Fatal(err)
			}
			data, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			if readErr != nil || closeErr != nil || string(data) != tt.content {
				t.Fatalf("file = %q, read=%v close=%v; want %q", data, readErr, closeErr, tt.content)
			}
			for _, name := range []string{"/file.txt", "C:/file.txt", `C:\file.txt`} {
				if _, err := dir.Open(name); !errors.Is(err, fs.ErrInvalid) {
					t.Errorf("Open(%q) = %v, want invalid path", name, err)
				}
			}
			if _, err := dir.Open("nonexistent.txt"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("missing file error = %v", err)
			}
			if err := dir.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func TestSpxDirResourceCapabilities(t *testing.T) {
	fsys := fstest.MapFS{
		"assets/fonts/Pixel/index.json": &fstest.MapFile{Data: []byte(`{}`)},
		"assets/fonts/Serif/index.json": &fstest.MapFile{Data: []byte(`{}`)},
		"assets/index.json":             &fstest.MapFile{Data: []byte(`{}`)},
	}
	dir := newSpxDir(fsys, "assets")

	gdDir, ok := dir.(spxfs.GdDir)
	if !ok {
		t.Fatalf("%T does not implement GdDir", dir)
	}
	if got := gdDir.GetPath(); got != "assets" {
		t.Fatalf("GetPath() = %q, want assets", got)
	}
	reader, ok := dir.(spxfs.ReadDirer)
	if !ok {
		t.Fatalf("%T does not implement ReadDirer", dir)
	}
	entries, err := reader.ReadDir("fonts")
	if err != nil {
		t.Fatal(err)
	}
	want := []spxfs.DirEntry{{Name: "Pixel", IsDir: true}, {Name: "Serif", IsDir: true}}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
	entries, err = reader.ReadDir(".")
	want = []spxfs.DirEntry{{Name: "fonts", IsDir: true}, {Name: "index.json"}}
	if err != nil || !reflect.DeepEqual(entries, want) {
		t.Fatalf("resource root entries = %#v, %v; want %#v", entries, err, want)
	}
	if _, err := reader.ReadDir("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing resource directory error = %v", err)
	}
}

func TestXGoParserFileSystem(t *testing.T) {
	parser := newXGoParserFS(fstest.MapFS{
		"sources/main.spx": {Data: []byte("onStart => {}")},
	})
	if parser == nil {
		t.Fatal("expected non-nil parser filesystem")
	}
	data, err := parser.ReadFile("sources/main.spx")
	if err != nil || string(data) != "onStart => {}" {
		t.Fatalf("parser source = %q, %v", data, err)
	}
	entries, err := parser.ReadDir("sources")
	if err != nil || len(entries) != 1 || entries[0].Name() != "main.spx" || entries[0].IsDir() {
		t.Fatalf("parser directory = %v, %v", entries, err)
	}
	if _, err := parser.ReadFile("missing.spx"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing source error = %v", err)
	}
	if _, err := parser.ReadDir("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing source directory error = %v", err)
	}
	if joined := parser.Join("sources", "nested", "..", "main.spx"); joined != "sources/main.spx" {
		t.Fatalf("joined parser path = %q", joined)
	}
	for _, tt := range []struct{ name, base string }{
		{"sources/main.spx", "main.spx"}, {"sources/", "sources"}, {"", "."},
	} {
		if base := parser.Base(tt.name); base != tt.base {
			t.Errorf("Base(%q) = %q, want %q", tt.name, base, tt.base)
		}
	}
	if absolute, err := parser.Abs("sources/main.spx"); absolute != "" || !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("absolute parser path = %q, %v; want unsupported", absolute, err)
	}
}

// This filesystem supports case-insensitive metadata lookup without enumeration.
type caseInsensitiveMetadataFS struct{ fstest.MapFS }

func (f caseInsensitiveMetadataFS) Open(name string) (fs.File, error) {
	return f.MapFS.Open(strings.ToLower(name))
}

func (f caseInsensitiveMetadataFS) Lstat(name string) (fs.FileInfo, error) {
	return f.MapFS.Lstat(strings.ToLower(name))
}

func (caseInsensitiveMetadataFS) ReadDir(string) ([]fs.DirEntry, error) {
	return nil, errors.ErrUnsupported
}

func TestSpxDirValidatesConfigResourceSymlinks(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("browser sources have no physical symlinks")
	}
	files := fstest.MapFS{
		"assets/image.png": {Data: []byte("image")},
		"assets/link":      {Mode: fs.ModeSymlink, Data: []byte("../outside")},
	}
	for _, source := range []fs.FS{caseInsensitiveMetadataFS{files}, struct{ fs.FS }{files}} {
		dir := newSpxDir(source, "assets")
		for _, name := range []string{"image.png", "missing-binary.png"} {
			if err := spxfs.ValidateResourcePath(dir, name); err != nil {
				t.Fatal(err)
			}
		}
		if err := spxfs.ValidateResourcePath(dir, "link/image.png"); err == nil {
			t.Fatal("accepted symlink in resource path")
		}
	}
	dir := newSpxDir(caseInsensitiveMetadataFS{files}, "assets")
	if err := spxfs.ValidateResourcePath(dir, "LINK/image.png"); err == nil {
		t.Fatal("accepted case alias of symlink")
	}
}

func TestSpxDirDirectoryValidationWithoutLstat(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("browser sources have no physical directory entries to verify")
	}
	files := caseInsensitiveMetadataFS{fstest.MapFS{"assets/image.png": {}}}
	// Open-only wrappers fall back to enumerating the parent directory.
	dir := newSpxDir(struct{ fs.FS }{files}, "").(spxfs.ReadDirer)
	if entries, err := dir.ReadDir("."); err != nil || len(entries) != 1 || entries[0].Name != "assets" {
		t.Fatalf("ReadDir(root) = %v, %v", entries, err)
	}
	if _, err := dir.ReadDir("ASSETS"); err == nil || !strings.Contains(err.Error(), "cannot identify directory entry") {
		t.Errorf("ReadDir(case alias) = %v, want unverifiable entry", err)
	}
	dir = newSpxDir(struct{ fs.ReadDirFS }{files}, "assets").(spxfs.ReadDirer)
	if _, err := dir.ReadDir("."); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("ReadDir with unavailable parent listing = %v, want unsupported", err)
	}
}

func BenchmarkSpxDirMemoryResourceValidation(b *testing.B) {
	for _, count := range []int{1, 2000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			files := make(map[string][]byte, count)
			for i := range count {
				files["assets/sprites/Hero/"+strconv.Itoa(i)+".png"] = nil
			}
			dir := newSpxDir(memfs.New(files), "assets")
			b.ReportAllocs()
			for b.Loop() {
				if err := spxfs.ValidateResourcePath(dir, "sprites/Hero/0.png"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
