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

package interpruntime

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testRoots(t *testing.T) Roots {
	t.Helper()
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	sessionDir := filepath.Join(t.TempDir(), "session")
	for _, dir := range []string{assetDir, sessionDir} {
		if err := mkdirAll(dir); err != nil {
			t.Fatal(err)
		}
	}
	return Roots{ProjectDir: projectDir, AssetDir: assetDir, SessionDir: sessionDir}
}

func TestRootsFromEnv(t *testing.T) {
	roots := testRoots(t)
	env := []string{
		"PATH=/bin",
		ProjectDirEnv + "=" + roots.ProjectDir,
		AssetDirEnv + "=" + roots.AssetDir,
		SessionDirEnv + "=" + roots.SessionDir,
	}
	got, err := RootsFromEnv(env)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, roots) {
		t.Fatalf("RootsFromEnv() = %#v, want %#v", got, roots)
	}
}

func TestRootsFromEnvRejectsInvalidContract(t *testing.T) {
	roots := testRoots(t)
	valid := []string{
		ProjectDirEnv + "=" + roots.ProjectDir,
		AssetDirEnv + "=" + roots.AssetDir,
		SessionDirEnv + "=" + roots.SessionDir,
	}
	tests := []struct {
		name string
		env  []string
		want string
	}{
		{name: "missing", env: valid[:2], want: SessionDirEnv},
		{name: "duplicate", env: append(append([]string{}, valid...), ProjectDirEnv+"="+roots.ProjectDir), want: "duplicate"},
		{name: "relative", env: []string{ProjectDirEnv + "=.", valid[1], valid[2]}, want: "not absolute"},
		{name: "unclean", env: []string{ProjectDirEnv + "=" + roots.ProjectDir + string(filepath.Separator) + ".", valid[1], valid[2]}, want: "not clean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RootsFromEnv(tt.env)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("RootsFromEnv() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestEnvironmentReplacesAmbientRoots(t *testing.T) {
	roots := testRoots(t)
	base := []string{
		"PATH=/bin",
		ProjectDirEnv + "=/attacker/project",
		AssetDirEnv + "=/attacker/assets",
		SessionDirEnv + "=/attacker/session",
		PortableConfigDirEnv + "=/attacker/config",
		PortableConfigIdentityEnv + "=sha256:attacker",
		"UNCHANGED=value",
	}
	got, err := roots.Environment(base)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"PATH=/bin",
		"UNCHANGED=value",
		ProjectDirEnv + "=" + roots.ProjectDir,
		AssetDirEnv + "=" + roots.AssetDir,
		SessionDirEnv + "=" + roots.SessionDir,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Environment() = %#v, want %#v", got, want)
	}
}

func TestPortableConfigDirFromEnv(t *testing.T) {
	if value, found, err := PortableConfigDirFromEnv([]string{"KEEP=value"}); err != nil || found || value != "" {
		t.Fatalf("PortableConfigDirFromEnv() = %q, %v, %v, want absent", value, found, err)
	}
	if value, found, err := PortableConfigDirFromEnv([]string{PortableConfigDirEnv + "=/trusted"}); err != nil || !found || value != "/trusted" {
		t.Fatalf("PortableConfigDirFromEnv() = %q, %v, %v", value, found, err)
	}
	_, _, err := PortableConfigDirFromEnv([]string{
		PortableConfigDirEnv + "=/first",
		PortableConfigDirEnv + "=/second",
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("PortableConfigDirFromEnv() duplicate error = %v", err)
	}
}

func TestPortableConfigIdentityFromEnv(t *testing.T) {
	value := "sha256:0123456789abcdef"
	got, found, err := PortableConfigIdentityFromEnv([]string{PortableConfigIdentityEnv + "=" + value})
	if err != nil || !found || got != value {
		t.Fatalf("PortableConfigIdentityFromEnv() = %q, %v, %v", got, found, err)
	}
	_, _, err = PortableConfigIdentityFromEnv([]string{
		PortableConfigIdentityEnv + "=first",
		PortableConfigIdentityEnv + "=second",
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("PortableConfigIdentityFromEnv() duplicate error = %v", err)
	}
}

func TestPortableConfigEnvironmentKeysAreCaseInsensitiveOnWindows(t *testing.T) {
	for _, key := range []string{PortableConfigDirEnv, PortableConfigIdentityEnv} {
		lower := strings.ToLower(key)
		if canonical, ok := rootEnvKeyForGOOS(lower, "windows"); !ok || canonical != key {
			t.Fatalf("rootEnvKeyForGOOS(%q, windows) = %q, %v", lower, canonical, ok)
		}
		if _, ok := rootEnvKeyForGOOS(lower, "linux"); ok {
			t.Fatalf("rootEnvKeyForGOOS(%q, linux) accepted case variant", lower)
		}
	}
}

func TestRootsRejectSymlinkAssetAndSessionRoots(t *testing.T) {
	projectDir := t.TempDir()
	realAssetDir := t.TempDir()
	realSessionDir := t.TempDir()

	for _, tt := range []struct {
		name       string
		assetDir   string
		sessionDir string
		linkTarget string
		linkPath   string
	}{
		{
			name:       "asset",
			assetDir:   filepath.Join(projectDir, "assets-link"),
			sessionDir: realSessionDir,
			linkTarget: realAssetDir,
			linkPath:   filepath.Join(projectDir, "assets-link"),
		},
		{
			name:       "session",
			assetDir:   realAssetDir,
			sessionDir: filepath.Join(projectDir, "session-link"),
			linkTarget: realSessionDir,
			linkPath:   filepath.Join(projectDir, "session-link"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Symlink(tt.linkTarget, tt.linkPath); err != nil {
				t.Skipf("symlink not supported: %v", err)
			}
			t.Cleanup(func() { _ = os.Remove(tt.linkPath) })
			err := (Roots{ProjectDir: projectDir, AssetDir: tt.assetDir, SessionDir: tt.sessionDir}).Validate()
			if err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("Roots.Validate() error = %v, want symlink rejection", err)
			}
		})
	}
}

func TestRootsRejectMissingAssetDirectory(t *testing.T) {
	projectDir := t.TempDir()
	missingAssetDir := filepath.Join(projectDir, "assets")
	sessionDir := t.TempDir()
	roots := Roots{ProjectDir: projectDir, AssetDir: missingAssetDir, SessionDir: sessionDir}
	if err := roots.Validate(); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Roots.Validate() error = %v, want missing asset directory", err)
	}
}

func TestRootsRejectAssetDirectoryOutsideProject(t *testing.T) {
	projectDir := t.TempDir()
	assetDir := t.TempDir()
	sessionDir := t.TempDir()
	roots := Roots{ProjectDir: projectDir, AssetDir: assetDir, SessionDir: sessionDir}
	if err := roots.Validate(); err == nil || !strings.Contains(err.Error(), "within ProjectDir") {
		t.Fatalf("Roots.Validate() error = %v, want ProjectDir containment rejection", err)
	}
}

func TestRootsRejectAssetDirectoryThroughSymlinkOutsideProject(t *testing.T) {
	projectDir := t.TempDir()
	externalParent := t.TempDir()
	assetDir := filepath.Join(externalParent, "assets")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(projectDir, "linked")
	if err := os.Symlink(externalParent, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	roots := Roots{
		ProjectDir: projectDir,
		AssetDir:   filepath.Join(link, "assets"),
		SessionDir: t.TempDir(),
	}
	if err := roots.Validate(); err == nil || !strings.Contains(err.Error(), "within ProjectDir") {
		t.Fatalf("Roots.Validate() error = %v, want canonical ProjectDir containment rejection", err)
	}
}
