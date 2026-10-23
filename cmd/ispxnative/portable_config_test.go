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
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goplus/spx/v4/internal/interpruntime"
	"github.com/goplus/spx/v4/internal/projectpolicy"
)

func TestPortableConfigFSUsesCapturedBytes(t *testing.T) {
	projectDir := t.TempDir()
	writeNativeTestFile(t, filepath.Join(projectDir, ".config"), `{"extasset":"live"}`)
	writeNativeTestFile(t, filepath.Join(projectDir, "main.spx"), "onStart => {}\n")
	want := []byte(`{"name":"captured"}`)
	overlayFS := newPortableConfigFS(os.DirFS(projectDir), &portableConfigOverlay{present: true, data: want})

	got, err := fs.ReadFile(overlayFS, ".config")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf(".config = %q, want %q", got, want)
	}
	assertSinglePortableConfigEntry(t, mustReadDir(t, overlayFS))

	root, err := overlayFS.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	directory, ok := root.(fs.ReadDirFile)
	if !ok {
		t.Fatal("Open root does not implement fs.ReadDirFile")
	}
	entries, err := directory.ReadDir(-1)
	if err != nil {
		t.Fatal(err)
	}
	assertSinglePortableConfigEntry(t, entries)
}

func TestPortableConfigFSHidesAbsentConfig(t *testing.T) {
	projectDir := t.TempDir()
	writeNativeTestFile(t, filepath.Join(projectDir, ".config"), `{"name":"late"}`)
	overlayFS := newPortableConfigFS(os.DirFS(projectDir), &portableConfigOverlay{})
	if _, err := fs.ReadFile(overlayFS, ".config"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile(.config) error = %v, want not exist", err)
	}
	if _, err := overlayFS.Open(".config/child"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open(.config/child) error = %v, want not exist", err)
	}
	for _, entry := range mustReadDir(t, overlayFS) {
		if isPortableConfigName(entry.Name()) {
			t.Fatalf("root listing exposed %q", entry.Name())
		}
	}
}

func TestLoadPortableConfigOverlayValidatesSessionContainment(t *testing.T) {
	sessionDir := t.TempDir()
	configDir := makePortableConfigTestDir(t, sessionDir)
	want := []byte(`{"name":"captured"}`)
	writeNativeTestFile(t, filepath.Join(configDir, ".config"), string(want))
	roots := interpruntime.Roots{SessionDir: sessionDir}
	env := portableConfigTestEnv(t, configDir)
	overlay, err := loadPortableConfigOverlay(env, roots)
	if err != nil {
		t.Fatal(err)
	}
	if overlay == nil || !overlay.present || string(overlay.data) != string(want) {
		t.Fatalf("loaded overlay = %#v, want captured content", overlay)
	}
	legacy, err := loadPortableConfigOverlay(nil, roots)
	if err != nil || legacy != nil {
		t.Fatalf("legacy overlay = %#v, %v, want nil", legacy, err)
	}

	outside := t.TempDir()
	_, err = loadPortableConfigOverlay([]string{
		interpruntime.PortableConfigDirEnv + "=" + outside,
		env[1],
	}, roots)
	if err == nil || !strings.Contains(err.Error(), "below session") {
		t.Fatalf("outside overlay error = %v, want containment rejection", err)
	}
}

func TestOpenPortableConfigRootPinsSessionTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement semantics differ on Windows")
	}
	sessionDir := t.TempDir()
	configDir := makePortableConfigTestDir(t, sessionDir)
	writeNativeTestFile(t, filepath.Join(configDir, ".config"), `{"name":"inside"}`)
	root, err := openPortableConfigRoot(sessionDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	outside := t.TempDir()
	writeNativeTestFile(t, filepath.Join(outside, ".config"), `{"name":"outside"}`)
	if err := os.Rename(configDir, filepath.Join(sessionDir, "portable-config-old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, configDir); err != nil {
		t.Fatal(err)
	}
	snapshot, err := projectpolicy.SnapshotPortableConfigRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snapshot.Bytes()); got != `{"name":"inside"}` {
		t.Fatalf("pinned config = %q, want inside snapshot", got)
	}
}

func TestLoadPortableConfigOverlayRejectsSnapshotDrift(t *testing.T) {
	for _, test := range []struct{ name, before, after string }{
		{"replaced file", `{"name":"validated"}`, `{"name":"different-but-valid"}`},
		{"previously absent", "", `{"name":"late"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := t.TempDir()
			config := makePortableConfigTestDir(t, session)
			path := filepath.Join(config, ".config")
			if test.before != "" {
				writeNativeTestFile(t, path, test.before)
			}
			env := portableConfigTestEnv(t, config)
			if test.before != "" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			writeNativeTestFile(t, path, test.after)
			_, err := loadPortableConfigOverlay(env, interpruntime.Roots{SessionDir: session})
			if err == nil || !strings.Contains(err.Error(), "identity mismatch") {
				t.Fatalf("snapshot drift error = %v, want identity mismatch", err)
			}
		})
	}
}

func TestLoadPortableConfigOverlayRejectsIncompleteEnvironment(t *testing.T) {
	sessionDir := t.TempDir()
	configDir := makePortableConfigTestDir(t, sessionDir)
	for _, env := range [][]string{
		{interpruntime.PortableConfigDirEnv + "=" + configDir},
		{interpruntime.PortableConfigIdentityEnv + "=absent"},
	} {
		if _, err := loadPortableConfigOverlay(env, interpruntime.Roots{SessionDir: sessionDir}); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("loadPortableConfigOverlay(%v) error = %v", env, err)
		}
	}
}

func TestPortableConfigFSDelegationAndMetadata(t *testing.T) {
	base := fstest.MapFS{
		".config":        {Data: []byte("live config")},
		"main.spx":       {Data: []byte("game source")},
		"nested/.config": {Data: []byte("nested config")},
	}
	want := `{"name":"snapshot"}`
	overlay := newPortableConfigFS(base, &portableConfigOverlay{present: true, data: []byte(want)})
	for name, content := range map[string]string{
		".config": want, "main.spx": "game source", "nested/.config": "nested config",
	} {
		got, err := fs.ReadFile(overlay, name)
		if err != nil || string(got) != content {
			t.Errorf("ReadFile(%q) = (%q, %v), want %q", name, got, err, content)
		}
	}
	entries, err := overlay.ReadDir("nested")
	if err != nil || len(entries) != 1 || entries[0].Name() != ".config" {
		t.Fatalf("nested directory = (%v, %v), want original .config", entries, err)
	}
	info, err := fs.Stat(overlay, ".config")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != ".config" || info.Size() != int64(len(want)) || info.Mode() != 0o444 ||
		!info.ModTime().IsZero() || info.IsDir() || info.Sys() != nil {
		t.Errorf("synthetic config metadata = %#v", info)
	}
	for _, name := range []string{"missing", ".config", ".config/child"} {
		if _, err := overlay.ReadDir(name); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("ReadDir(%q) error = %v, want not exist", name, err)
		}
	}
	if _, err := overlay.Open("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open(missing) error = %v, want not exist", err)
	}
}

func TestPortableConfigRootReadDirPagination(t *testing.T) {
	type page struct {
		n     int
		names []string
		err   error
	}
	for _, test := range []struct {
		name  string
		pages []page
	}{
		{"one at a time", []page{
			{1, []string{".config"}, nil}, {1, []string{"a.spx"}, nil},
			{1, []string{"z.spx"}, nil}, {1, nil, io.EOF}, {0, nil, nil},
		}},
		{"short final page", []page{
			{2, []string{".config", "a.spx"}, nil}, {2, []string{"z.spx"}, io.EOF}, {2, nil, io.EOF},
		}},
		{"read remainder", []page{
			{1, []string{".config"}, nil}, {-1, []string{"a.spx", "z.spx"}, nil}, {-1, nil, nil},
		}},
		{"zero reads all", []page{{0, []string{".config", "a.spx", "z.spx"}, nil}, {1, nil, io.EOF}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := fstest.MapFS{"a.spx": {}, "z.spx": {}}
			file, err := base.Open(".")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			directory := &portableConfigTestDirectory{File: file, entries: mustReadDir(t, base)}
			overlay := newPortableConfigFS(portableConfigTestFS{file: directory}, &portableConfigOverlay{present: true})
			root, err := overlay.Open(".")
			if err != nil {
				t.Fatal(err)
			}
			for _, page := range test.pages {
				entries, err := root.(fs.ReadDirFile).ReadDir(page.n)
				var names []string
				for _, entry := range entries {
					names = append(names, entry.Name())
				}
				if !errors.Is(err, page.err) || !reflect.DeepEqual(names, page.names) {
					t.Fatalf("ReadDir(%d) = (%v, %v), want (%v, %v)", page.n, names, err, page.names, page.err)
				}
				// Later pages must use the captured listing, even if the base changes.
				directory.entries = nil
				directory.err = fs.ErrPermission
			}
			if directory.readCalls != 1 {
				t.Errorf("base directory reads = %d, want 1", directory.readCalls)
			}
		})
	}
}

func TestPortableConfigRootReadDirErrorsAreCached(t *testing.T) {
	for _, readable := range []bool{false, true} {
		name, wantErr := "not a ReadDirFile", fs.ErrInvalid
		if readable {
			name, wantErr = "underlying read fails", fs.ErrPermission
		}
		t.Run(name, func(t *testing.T) {
			file, err := (fstest.MapFS{"file": {}}).Open("file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			directory := &portableConfigTestDirectory{File: file, err: wantErr}
			var baseFile fs.File = struct{ fs.File }{file}
			if readable {
				baseFile = directory
			}
			overlay := newPortableConfigFS(portableConfigTestFS{file: baseFile}, &portableConfigOverlay{})
			root, err := overlay.Open(".")
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range []int{1, -1} {
				entries, err := root.(fs.ReadDirFile).ReadDir(n)
				if len(entries) != 0 || !errors.Is(err, wantErr) {
					t.Errorf("ReadDir(%d) = (%v, %v), want %v", n, entries, err, wantErr)
				}
				directory.err = nil
			}
			if readable && directory.readCalls != 1 {
				t.Errorf("base directory reads = %d, want 1", directory.readCalls)
			}
		})
	}
}

type portableConfigTestFS struct{ file fs.File }

func (f portableConfigTestFS) Open(string) (fs.File, error) { return f.file, nil }

type portableConfigTestDirectory struct {
	fs.File
	entries   []fs.DirEntry
	err       error
	readCalls int
}

func (d *portableConfigTestDirectory) ReadDir(int) ([]fs.DirEntry, error) {
	d.readCalls++
	return d.entries, d.err
}

func TestPortableConfigRootValidation(t *testing.T) {
	base := t.TempDir()
	session := filepath.Join(base, "session")
	config := filepath.Join(session, "config")
	if err := os.MkdirAll(config, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(session, "file")
	writeNativeTestFile(t, file, "")
	for _, test := range []struct {
		name, session, config, validateError, openError string
	}{
		{"valid child", session, config, "", ""},
		{"empty", session, "", "is empty", "outside session"},
		{"relative", session, "config", "not absolute", "outside session"},
		{"unclean", session, config + string(filepath.Separator) + ".", "not clean", ""},
		{"missing config", session, filepath.Join(session, "missing"), "inspect portable config", "inspect portable config"},
		{"config is file", session, file, "must be a real directory", "must be a real directory"},
		{"same directory", session, session, "must be below session", "outside session"},
		{"parent directory", session, base, "must be below session", "outside session"},
		{"sibling directory", config, file, "must be a real directory", "outside session"},
		{"missing session", filepath.Join(base, "missing"), filepath.Join(base, "missing", "config"), "inspect portable config", "inspect session"},
		{"missing session with existing config", filepath.Join(base, "missing"), config, "canonicalize session", "outside session"},
		{"session is file", file, filepath.Join(file, "config"), "inspect portable config", "must be a real directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertPortableConfigError(t, validatePortableConfigDir(test.session, test.config), test.validateError)
			root, err := openPortableConfigRoot(test.session, test.config)
			if root != nil {
				defer root.Close()
			}
			assertPortableConfigError(t, err, test.openError)
			if test.openError == "" {
				if root == nil {
					t.Fatal("successful open returned no root")
				}
				info, err := root.Stat(".")
				if err != nil || !info.IsDir() {
					t.Fatalf("opened root stat = (%v, %v), want directory", info, err)
				}
			}
		})
	}
}

func TestPortableConfigRootRejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	base := t.TempDir()
	session := filepath.Join(base, "session")
	config := filepath.Join(session, "config")
	if err := os.MkdirAll(config, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, link := range []struct{ target, name string }{
		{config, filepath.Join(session, "config-link")},
		{session, filepath.Join(base, "session-link")},
		{base, filepath.Join(session, "escape")},
	} {
		if err := os.Symlink(link.target, link.name); err != nil {
			t.Fatal(err)
		}
	}
	assertPortableConfigError(t, validatePortableConfigDir(session, filepath.Join(session, "config-link")), "must be a real directory")
	root, err := openPortableConfigRoot(session, filepath.Join(session, "config-link"))
	if root != nil {
		root.Close()
	}
	assertPortableConfigError(t, err, "must be a real directory")
	assertPortableConfigError(t, validatePortableConfigDir(session, filepath.Join(session, "escape", "session")), "must be below session")

	// Canonical containment alone accepts this alias, but opening must reject
	// the session symlink before consuming a portable config snapshot.
	aliasedSession := filepath.Join(base, "session-link")
	aliasedConfig := filepath.Join(aliasedSession, "config")
	assertPortableConfigError(t, validatePortableConfigDir(aliasedSession, aliasedConfig), "")
	_, err = loadPortableConfigOverlay(portableConfigTestEnv(t, aliasedConfig), interpruntime.Roots{SessionDir: aliasedSession})
	assertPortableConfigError(t, err, "must be a real directory")
}

func TestLoadPortableConfigOverlayAbsentAndInvalidSnapshots(t *testing.T) {
	for _, test := range []struct {
		name, content, wantError string
		directory, duplicateEnv  bool
	}{
		{name: "absent"},
		{name: "invalid json", content: "{", wantError: "load portable config snapshot: parse project config"},
		{name: "external assets", content: `{"extasset":"outside"}`, wantError: "unsupported extasset"},
		{name: "config is directory", directory: true, wantError: "load portable config snapshot"},
		{name: "duplicate environment", duplicateEnv: true, wantError: "duplicate environment variable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := t.TempDir()
			config := makePortableConfigTestDir(t, session)
			path := filepath.Join(config, ".config")
			if test.directory {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if test.content != "" {
				writeNativeTestFile(t, path, test.content)
			}
			env := []string{interpruntime.PortableConfigDirEnv + "=" + config, interpruntime.PortableConfigIdentityEnv + "=absent"}
			if test.duplicateEnv {
				env = append(env, env[0])
			}
			overlay, err := loadPortableConfigOverlay(env, interpruntime.Roots{SessionDir: session})
			assertPortableConfigError(t, err, test.wantError)
			if test.wantError == "" && (overlay == nil || overlay.present || overlay.data != nil) {
				t.Errorf("absent overlay = %#v, want captured absence", overlay)
			}
			if test.wantError != "" && overlay != nil {
				t.Errorf("invalid snapshot returned overlay %#v", overlay)
			}
		})
	}
}

func assertPortableConfigError(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	} else if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want to contain %q", err, want)
	}
}

func makePortableConfigTestDir(t *testing.T, sessionDir string) string {
	t.Helper()
	configDir := filepath.Join(sessionDir, "portable-config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return configDir
}

func portableConfigTestEnv(t *testing.T, configDir string) []string {
	t.Helper()
	snapshot, err := projectpolicy.SnapshotPortableConfig(configDir)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := snapshot.Identity()
	if err != nil {
		t.Fatal(err)
	}
	return []string{
		interpruntime.PortableConfigDirEnv + "=" + configDir,
		interpruntime.PortableConfigIdentityEnv + "=" + identity,
	}
}

func mustReadDir(t *testing.T, fsys fs.FS) []fs.DirEntry {
	t.Helper()
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func assertSinglePortableConfigEntry(t *testing.T, entries []fs.DirEntry) {
	t.Helper()
	count := 0
	for _, entry := range entries {
		if isPortableConfigName(entry.Name()) {
			count++
			info, err := entry.Info()
			if err != nil {
				t.Fatal(err)
			}
			if info.IsDir() {
				t.Fatal("synthetic .config is a directory")
			}
		}
	}
	if count != 1 {
		t.Fatalf("portable config entry count = %d, want 1", count)
	}
}
