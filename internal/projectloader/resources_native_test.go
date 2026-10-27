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

package projectloader

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type configTestResMgr struct {
	pkgengine.IResMgr
	files map[string]string
	reads map[string]int
	lists map[string]string
}

type sourceConfigDir struct {
	resourceTestDir
	files   map[string]string
	openErr error
}

type closeErrorReader struct {
	io.Reader
	err error
}

type closeErrorDir struct {
	resourceTestDir
	err error
}

func TestOpenResourcesKeepsEngineReadsBelowPackedOverlay(t *testing.T) {
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
	base := &resourceTestDir{path: "res://assets"}

	opened, err := OpenResources(base, nil)
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

	sprite, err := project.LoadSpriteConfig(opened.FS, "Hero")
	if err != nil {
		t.Fatal(err)
	}
	if got := sprite.Costumes[0].Path; got != "sprites/Hero/packed.png" {
		t.Fatalf("sprite costume = %q, want packed costume", got)
	}
	sound, err := project.LoadSoundConfig(opened.FS, "Jump")
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

func TestOpenResourcesReadsUnpackedEngineConfigStrictly(t *testing.T) {
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
			base := &resourceTestDir{path: "res://assets"}

			opened, err := OpenResources(base, nil)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "unexpected content after JSON value") {
					t.Fatalf("OpenResources() error = %v, want trailing JSON error", err)
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
			base := &resourceTestDir{path: "res://assets"}
			var config struct {
				Name string `json:"name"`
			}

			err := project.LoadJSON(&config, withEngineAssets(base), "direct.json")
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

func TestOpenResourcesPreservesNativeAndWebFontSources(t *testing.T) {
	for _, test := range []struct {
		name, root, engineFile string
	}{
		{"native", "assets", ""},
		{"web binary fallback", "assets", "res://assets/fonts/Local/local.ttf"},
		{"Windows root binary fallback", `\`, "fonts/Local/local.ttf"},
	} {
		t.Run(test.name, func(t *testing.T) {
			engineFiles := map[string]string{}
			base := &sourceConfigDir{
				resourceTestDir: resourceTestDir{path: test.root},
				files: map[string]string{
					"index.json":             `{}`,
					"fonts/Local/index.json": `{"faces":[{"path":"local.ttf"}]}`,
				},
			}
			if test.engineFile == "" {
				base.files["fonts/Local/local.ttf"] = "native font"
			} else {
				engineFiles[test.engineFile] = "mounted font"
			}
			manager := installConfigTestResMgr(t, engineFiles)
			opened, err := OpenResources(base, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer opened.FS.Close()
			if len(opened.Fonts.Families) != 1 || opened.Fonts.Families[0].Name != "Local" {
				t.Fatalf("font families = %+v, want Local", opened.Fonts.Families)
			}
			if len(manager.reads) != 0 {
				t.Fatalf("local JSON or binary font read through text bridge: %v", manager.reads)
			}
		})
	}
}

func TestPackedOverlayDelegatesEngineFontDiscovery(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{
		"res://assets/index_pack.json":        `{}`,
		"res://assets/fonts/Local/index.json": `{"faces":[{"path":"local.ttf"}]}`,
		"res://assets/fonts/Local/local.ttf":  "font",
	})
	manager.lists = map[string]string{"res://assets/fonts": `["Local"]`}
	opened, err := OpenResources(&resourceTestDir{path: "res://assets"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.FS.Close()
	if len(opened.Fonts.Families) != 1 || opened.Fonts.Families[0].Name != "Local" {
		t.Fatalf("font families = %+v, want engine-discovered Local", opened.Fonts.Families)
	}
}

func TestEngineFontVerificationDoesNotFallBackToSource(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{
		"res://assets/index.json":             `{}`,
		"res://assets/fonts/Local/index.json": `{"faces":[{"path":"local.ttf"}]}`,
	})
	manager.lists = map[string]string{"res://assets/fonts": `["Local"]`}
	base := &sourceConfigDir{
		resourceTestDir: resourceTestDir{path: "res://assets"},
		files:           map[string]string{"fonts/Local/local.ttf": "source font"},
	}
	if _, err := OpenResources(base, nil); err == nil || !strings.Contains(err.Error(), "open font face") {
		t.Fatalf("missing engine font error = %v", err)
	}
	if base.closed != 1 {
		t.Fatalf("source closes = %d, want 1 after failed load", base.closed)
	}
}

func TestOpenResourcesPreservesStringAssetDirWithoutPathCapability(t *testing.T) {
	base := &sourceConfigDir{files: map[string]string{"index.json": `{}`}}
	spxfs.RegisterSchema("projectloader-test", func(string) (spxfs.Dir, error) {
		return struct{ spxfs.Dir }{base}, nil
	})
	const resource = "projectloader-test://assets"
	opened, err := OpenResources(resource, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opened.AssetDir != resource {
		t.Fatalf("asset directory = %q, want original resource %q", opened.AssetDir, resource)
	}
	if base.closed != 0 {
		t.Fatal("loader closed successfully transferred resources")
	}
	if err := opened.FS.Close(); err != nil {
		t.Fatal(err)
	}
	if base.closed != 1 {
		t.Fatalf("source closes = %d, want 1", base.closed)
	}
}

func TestOpenResourcesReportsMalformedEngineDirectoryListing(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{"res://assets/index.json": `{}`})
	manager.lists = map[string]string{"res://assets/fonts": `{}`}
	base := &resourceTestDir{path: "res://assets"}
	if _, err := OpenResources(base, nil); err == nil || !strings.Contains(err.Error(), "scan fonts") {
		t.Fatalf("directory listing error = %v", err)
	}
	if base.closed != 1 {
		t.Fatalf("source closes = %d, want 1 after failed listing", base.closed)
	}
}

func TestVerifyFileDoesNotMaskSourceCloseErrorWithEngineFallback(t *testing.T) {
	installConfigTestResMgr(t, map[string]string{"res://assets/font.ttf": "mounted font"})
	wantErr := errors.New("source close failed")
	dir := withEngineAssets(&closeErrorDir{resourceTestDir: resourceTestDir{path: "assets"}, err: wantErr})
	if err := spxfs.VerifyFile(dir, "font.ttf"); err != wantErr {
		t.Fatalf("font verification error = %v, want original close error %v", err, wantErr)
	}
}

func TestVerifyFilePreservesSourceOpenErrorWhenEngineFileMissing(t *testing.T) {
	manager := installConfigTestResMgr(t, nil)
	for _, wantErr := range []error{os.ErrNotExist, os.ErrPermission} {
		base := &sourceConfigDir{resourceTestDir: resourceTestDir{path: "assets"}, openErr: wantErr}
		if err := spxfs.VerifyFile(withEngineAssets(base), "font.ttf"); err != wantErr {
			t.Fatalf("font verification error = %v, want original source error %v", err, wantErr)
		}
	}
	if len(manager.reads) != 0 {
		t.Fatalf("binary verification read through text bridge: %v", manager.reads)
	}
}

func TestOpenResourcesDoesNotReadPreviousProjectAssets(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{
		"res://next-assets/index.json": `{}`,
	})
	for _, root := range []string{"../assets", "res://assets"} {
		manager.files[root+"/sprites/Hero/index.json"] = `{"costumes":[{"path":"old.png"}]}`
		manager.files[root+"/fonts/Local/local.ttf"] = "old-font"
	}
	opened, err := OpenResources(&resourceTestDir{path: "res://next-assets"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.FS.Close()
	if _, err := project.LoadSpriteConfig(opened.FS, "Hero"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing sprite error = %v, want ErrNotExist without previous project fallback", err)
	}
	if err := spxfs.VerifyFile(opened.FS, "fonts/Local/local.ttf"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing font error = %v, want ErrNotExist without previous project fallback", err)
	}
	for file := range manager.reads {
		if file != "res://next-assets/index.json" {
			t.Fatalf("read %q from the previous project", file)
		}
	}
}

func (m *configTestResMgr) HasFile(path string) bool {
	_, ok := m.files[path]
	return ok
}

func (m *configTestResMgr) ReadAllText(path string) string {
	m.reads[path]++
	return m.files[path]
}

func (m *configTestResMgr) ListDirectories(path string) string {
	if listing, ok := m.lists[path]; ok {
		return listing
	}
	return "[]"
}

func (d *sourceConfigDir) Open(name string) (io.ReadCloser, error) {
	if value, ok := d.files[name]; ok {
		return io.NopCloser(strings.NewReader(value)), nil
	}
	if d.openErr != nil {
		return nil, d.openErr
	}
	return nil, os.ErrNotExist
}

func (d *sourceConfigDir) ReadDir(name string) ([]spxfs.DirEntry, error) {
	if name == "fonts" {
		return []spxfs.DirEntry{{Name: "Local", IsDir: true}, {Name: "README.txt"}}, nil
	}
	return nil, os.ErrNotExist
}

func (r closeErrorReader) Close() error { return r.err }

func (d *closeErrorDir) Open(string) (io.ReadCloser, error) {
	return closeErrorReader{err: d.err}, nil
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
