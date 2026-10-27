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

package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	spxfs "github.com/goplus/spx/v4/fs"
)

type panicPathDir struct{}

type rawCapabilityDir struct{}

type failingConfigDir struct {
	spxfs.Dir
	failPath string
	err      error
	closed   int
}

type streamedConfigDir struct{ reader io.ReadCloser }

func TestLoadJSONReadsPackedConfigWithoutInspectingResourcePath(t *testing.T) {
	fs := &packedConfigDir{
		Dir: panicPathDir{},
		index: packedConfigIndex{
			raw:        []byte(`{"name":"original"}`),
			projectRaw: []byte(`{"name":"packed"}`),
			sprites:    map[string]json.RawMessage{"Hero": json.RawMessage(`{"name":"hero"}`)},
		},
	}
	for _, tt := range []struct{ path, name string }{
		{packedIndexJSON, "original"},
		{"index.json", "packed"},
		{"/index.json", "packed"},
		{"sprites/Hero/index.json", "hero"},
		{`sprites\Hero\index.json`, "hero"},
		{"/sprites/Other/../Hero/index.json", "hero"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			if err := spxfs.VerifyFile(fs, tt.path); err != nil {
				t.Fatalf("verify packed config: %v", err)
			}
			var config struct {
				Name string `json:"name"`
			}
			if err := LoadJSON(&config, fs, tt.path); err != nil {
				t.Fatal(err)
			}
			if config.Name != tt.name {
				t.Fatalf("config name = %q, want %q", config.Name, tt.name)
			}
		})
	}
}

func TestResourceDirKeepsPackedConfigOutermost(t *testing.T) {
	packed := &packedConfigDir{Dir: panicPathDir{}}
	got, err := ResourceDir(packed)
	if err != nil {
		t.Fatal(err)
	}
	if got != packed {
		t.Fatalf("ResourceDir() = %T, want original packed config directory", got)
	}
}

func TestResourceDirPreservesRawDirectoryIdentity(t *testing.T) {
	base := &rawCapabilityDir{}
	got, err := ResourceDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if got != base {
		t.Fatalf("ResourceDir() = %T, want original directory", got)
	}
}

func TestPackedResourcesPreserveSourceOpenErrors(t *testing.T) {
	for _, file := range []string{packedIndexJSON, "index.json", "custom/settings.json"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			writeProjectFile(t, dir, packedIndexJSON, `{}`)
			writeProjectFile(t, dir, "index.json", `{}`)
			base := &failingConfigDir{Dir: localDir{base: dir}, failPath: file, err: os.ErrPermission}
			opened, err := OpenBuilderResources(base, nil)
			if file == "custom/settings.json" {
				if err != nil {
					t.Fatal(err)
				}
				var config any
				err = LoadJSON(&config, opened.FS, file)
				if base.closed != 0 {
					t.Fatal("child read failure closed the resource directory")
				}
				if closeErr := opened.FS.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
			} else if opened.FS != nil {
				t.Fatal("failed preparation returned a resource directory")
			}
			if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "open "+file) {
				t.Fatalf("source open error = %v, want permission error with path %q", err, file)
			}
			if base.closed != 1 {
				t.Fatalf("source directory closes = %d, want 1", base.closed)
			}
		})
	}
}

func TestPackedRootPreservesOriginalJSONWithoutSourceFields(t *testing.T) {
	packed := []byte("{ \"bgm\": \"first.wav\", \"bgm\": \"last.wav\" }\n")
	for _, source := range []string{"", "{}", "null"} {
		t.Run(source, func(t *testing.T) {
			index, err := parsePackedConfigIndex(packed, []byte(source))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(index.projectRaw, packed) || !bytes.Equal(index.raw, packed) {
				t.Fatalf("unmerged packed JSON was rewritten: project=%s, raw=%s", index.projectRaw, index.raw)
			}
		})
	}
}

func TestPackedSectionsPreservePresenceAndErrorOrder(t *testing.T) {
	for _, tt := range []struct {
		name, packed string
		hasFonts     bool
		emptyFonts   bool
		errorPrefix  string
	}{
		{name: "absent", packed: `{}`},
		{name: "null", packed: `{"fonts": null}`, hasFonts: true},
		{name: "empty", packed: `{"fonts": {}}`, hasFonts: true, emptyFonts: true},
		{name: "sprites first", packed: `{"sprites": [], "sounds": false, "fonts": 1}`, errorPrefix: "sprites must be an object:"},
		{name: "sounds second", packed: `{"sprites": null, "sounds": false, "fonts": 1}`, errorPrefix: "sounds must be an object:"},
		{name: "fonts third", packed: `{"sprites": {}, "sounds": null, "fonts": []}`, errorPrefix: "fonts must be an object:"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			index, err := parsePackedConfigIndex([]byte(tt.packed), nil)
			if tt.errorPrefix != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tt.errorPrefix) {
					t.Fatalf("error = %v, want prefix %q", err, tt.errorPrefix)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if index.hasFonts != tt.hasFonts || (index.fonts != nil) != tt.emptyFonts || len(index.fonts) != 0 {
				t.Fatalf("font catalog = (%v, %#v)", index.hasFonts, index.fonts)
			}
		})
	}
}

func TestPackedResourcesPreserveMissingFileErrors(t *testing.T) {
	dir := &packedConfigDir{Dir: &rawCapabilityDir{}}
	if _, err := dir.Open("missing.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing packed fallback error = %v, want ErrNotExist", err)
	}
	if err := spxfs.VerifyFile(dir, "missing.ttf"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing binary error = %v, want ErrNotExist", err)
	}
}

func TestPackedResourcesKeepSourceReaderOwnership(t *testing.T) {
	// A non-overlay resource remains a stream owned by the caller. In
	// particular, Open must not eagerly read or close a binary resource.
	reader := io.NopCloser(strings.NewReader("resource"))
	dir := &packedConfigDir{Dir: streamedConfigDir{reader: reader}}
	file, err := dir.Open("resource.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if file != reader {
		t.Fatal("packed fallback replaced the source reader")
	}
	data, err := io.ReadAll(file)
	if err != nil || string(data) != "resource" {
		t.Fatalf("resource = %q, %v", data, err)
	}
}

func (panicPathDir) Open(string) (io.ReadCloser, error) {
	panic("base Open called for packed config")
}

func (panicPathDir) Close() error {
	return nil
}

func (panicPathDir) GetPath() string {
	panic("GetPath bypassed packed config")
}

func (*rawCapabilityDir) Open(string) (io.ReadCloser, error) {
	return nil, os.ErrNotExist
}

func (*rawCapabilityDir) Close() error {
	return nil
}

func (*rawCapabilityDir) GetPath() string {
	return "res://assets"
}

func (d *failingConfigDir) Open(name string) (io.ReadCloser, error) {
	if name == d.failPath {
		return nil, d.err
	}
	return d.Dir.Open(name)
}

func (d *failingConfigDir) Close() error {
	d.closed++
	return d.Dir.Close()
}

func (d streamedConfigDir) Open(string) (io.ReadCloser, error) { return d.reader, nil }

func (streamedConfigDir) Close() error { return nil }
