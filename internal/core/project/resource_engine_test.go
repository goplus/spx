//go:build !js && !pure_engine

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

package project

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type configTestResMgr struct {
	pkgengine.IResMgr
	files map[string]string
	reads map[string]int
}

func (m *configTestResMgr) HasFile(path string) bool {
	_, ok := m.files[path]
	return ok
}

func (m *configTestResMgr) ReadAllText(path string) string {
	m.reads[path]++
	return m.files[path]
}

func (*configTestResMgr) ListDirectories(string) string {
	return "[]"
}

type configTestGdDir struct {
	path   string
	closed int
}

func (*configTestGdDir) Open(string) (io.ReadCloser, error) {
	return nil, os.ErrNotExist
}

func (d *configTestGdDir) Close() error {
	d.closed++
	return nil
}

func (d *configTestGdDir) GetPath() string {
	return d.path
}

func installConfigTestResMgr(t *testing.T, files map[string]string) *configTestResMgr {
	t.Helper()
	enginewrap.Init(func(call func()) { call() })
	manager := &configTestResMgr{files: files, reads: make(map[string]int)}
	previous := pkgengine.ResMgr
	pkgengine.ResMgr = manager
	t.Cleanup(func() { pkgengine.ResMgr = previous })
	return manager
}

func TestOpenBuilderResourcesKeepsEngineReadsBelowPackedOverlay(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{
		"res://assets/index.json": `{
			"run":{"title":"source"},
			"fontPreferences":["Shared"]
		}`,
		"res://assets/index_pack.json": `{
			"run":{"title":"packed"},
			"fontPreferences":["Shared"],
			"sprites":{"Hero":{"costumes":[{"name":"packed","path":"packed.png"}]}},
			"sounds":{"Jump":{"path":"packed.wav","rate":2}},
			"fonts":{"Shared":{"faces":[{"path":"packed.ttf"}]}}
		}`,
		"res://assets/sprites/Hero/index.json": `{"costumes":[{"name":"source","path":"source.png"}]}`,
		"res://assets/sounds/Jump/index.json":  `{"path":"source.wav","rate":1}`,
		"res://assets/fonts/Shared/index.json": `{"faces":[{"path":"source.ttf"}]}`,
		"res://assets/fonts/Shared/packed.ttf": "packed-font",
		"res://assets/fonts/Shared/source.ttf": "source-font",
	})
	base := &configTestGdDir{path: "res://assets"}

	opened, err := OpenBuilderResources(base, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Config.Title != "packed" {
		t.Fatalf("project title = %q, want packed", opened.Config.Title)
	}
	if len(opened.Fonts.Families) != 1 || len(opened.Fonts.Families[0].Faces) != 1 {
		t.Fatalf("font families = %#v, want one packed face", opened.Fonts.Families)
	}
	if got := opened.Fonts.Families[0].Faces[0].Path; got != "fonts/Shared/packed.ttf" {
		t.Fatalf("font face = %q, want packed face", got)
	}

	sprite, err := LoadSpriteConfig(opened.FS, "Hero")
	if err != nil {
		t.Fatal(err)
	}
	if got := sprite.Costumes[0].Path; got != "sprites/Hero/packed.png" {
		t.Fatalf("sprite costume = %q, want packed costume", got)
	}
	sound, err := LoadSoundConfig(opened.FS, "Jump")
	if err != nil {
		t.Fatal(err)
	}
	if got := sound.Path; got != "sounds/Jump/packed.wav" {
		t.Fatalf("sound path = %q, want packed sound", got)
	}

	if err := opened.FS.Close(); err != nil {
		t.Fatal(err)
	}
	if base.closed != 1 {
		t.Fatalf("base directory close count = %d, want 1", base.closed)
	}
	if got := manager.reads["res://assets/index.json"]; got != 1 {
		t.Fatalf("source root engine reads = %d, want only the overlay merge read", got)
	}
	for _, path := range []string{
		"res://assets/sprites/Hero/index.json",
		"res://assets/sounds/Jump/index.json",
		"res://assets/fonts/Shared/index.json",
	} {
		if manager.reads[path] != 0 {
			t.Fatalf("packed load bypassed overlay and read engine source %q", path)
		}
	}
}

func TestOpenBuilderResourcesReadsUnpackedEngineConfigStrictly(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "valid", content: `{"run":{"title":"engine"}}`},
		{name: "trailing value", content: `{"run":{"title":"engine"}} {}`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installConfigTestResMgr(t, map[string]string{"res://assets/index.json": tt.content})
			base := &configTestGdDir{path: "res://assets"}

			opened, err := OpenBuilderResources(base, nil)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "unexpected content after JSON value") {
					t.Fatalf("OpenBuilderResources() error = %v, want trailing JSON error", err)
				}
				if base.closed != 1 {
					t.Fatalf("failed load close count = %d, want 1", base.closed)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if opened.Config.Title != "engine" {
				t.Fatalf("project title = %q, want engine", opened.Config.Title)
			}
			if err := opened.FS.Close(); err != nil {
				t.Fatal(err)
			}
			if base.closed != 1 {
				t.Fatalf("successful load close count = %d, want 1", base.closed)
			}
		})
	}
}

func TestLoadJSONReadsBareEngineConfigStrictly(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "valid", content: `{"name":"engine"}`},
		{name: "trailing value", content: `{"name":"engine"} {}`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			manager := installConfigTestResMgr(t, map[string]string{"res://assets/direct.json": tt.content})
			base := &configTestGdDir{path: "res://assets"}
			var config struct {
				Name string `json:"name"`
			}

			err := LoadJSON(&config, base, "direct.json")
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "unexpected content after JSON value") {
					t.Fatalf("LoadJSON() error = %v, want trailing JSON error", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if config.Name != "engine" {
					t.Fatalf("config name = %q, want engine", config.Name)
				}
			}
			if got := manager.reads["res://assets/direct.json"]; got != 1 {
				t.Fatalf("engine reads = %d, want 1", got)
			}
		})
	}
}
