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
	"io"
	"os"
	"strings"
	"testing"

	spxfs "github.com/goplus/spx/v3/fs"
)

type panicPathDir struct{}

func (panicPathDir) Open(string) (io.ReadCloser, error) {
	panic("base Open called for packed config")
}

func (panicPathDir) Close() error {
	return nil
}

func (panicPathDir) GetPath() string {
	panic("GetPath bypassed packed config")
}

func TestLoadJSONReadsPackedConfigWithoutInspectingResourcePath(t *testing.T) {
	fs := &packedConfigDir{
		Dir:   panicPathDir{},
		index: packedConfigIndex{projectRaw: []byte(`{"name":"packed"}`)},
	}
	var config struct {
		Name string `json:"name"`
	}
	if err := LoadJSON(&config, fs, "index.json"); err != nil {
		t.Fatal(err)
	}
	if config.Name != "packed" {
		t.Fatalf("config name = %q, want packed", config.Name)
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

type rawCapabilityDir struct{}

func (*rawCapabilityDir) Open(string) (io.ReadCloser, error) {
	return nil, os.ErrNotExist
}

func (*rawCapabilityDir) Close() error {
	return nil
}

func (*rawCapabilityDir) GetPath() string {
	return "res://assets"
}

type rawReadDirCapabilityDir struct {
	*rawCapabilityDir
	entries []spxfs.DirEntry
	readDir string
}

func (d *rawReadDirCapabilityDir) ReadDir(name string) ([]spxfs.DirEntry, error) {
	d.readDir = name
	return d.entries, nil
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

func TestRawConfigDirPreservesOptionalReadDirCapability(t *testing.T) {
	withoutReadDir := adaptConfigDir(&rawCapabilityDir{})
	if _, ok := withoutReadDir.(spxfs.ReadDirer); ok {
		t.Fatalf("raw wrapper unexpectedly implements spxfs.ReadDirer: %T", withoutReadDir)
	}
	if got := adaptConfigDir(withoutReadDir); got != withoutReadDir {
		t.Fatalf("raw wrapper was wrapped again: %T", got)
	}

	base := &rawReadDirCapabilityDir{
		rawCapabilityDir: &rawCapabilityDir{},
		entries:          []spxfs.DirEntry{{Name: "Shared", IsDir: true}},
	}
	withReadDir := adaptConfigDir(base)
	reader, ok := withReadDir.(spxfs.ReadDirer)
	if !ok {
		t.Fatalf("raw wrapper does not implement spxfs.ReadDirer: %T", withReadDir)
	}
	if got := adaptConfigDir(withReadDir); got != withReadDir {
		t.Fatalf("raw ReadDir wrapper was wrapped again: %T", got)
	}
	entries, err := reader.ReadDir("fonts")
	if err != nil {
		t.Fatal(err)
	}
	if base.readDir != "fonts" {
		t.Fatalf("delegated ReadDir path = %q, want fonts", base.readDir)
	}
	if len(entries) != 1 || entries[0] != base.entries[0] {
		t.Fatalf("delegated ReadDir entries = %#v, want %#v", entries, base.entries)
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
