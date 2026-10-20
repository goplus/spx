//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

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

package projectbundle

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCollectRejectsDeviceLikePackEntry(t *testing.T) {
	projectDir := t.TempDir()
	packDir := filepath.Join(projectDir, "pack")
	if err := os.Mkdir(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(filepath.Join(packDir, "pipe"), 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	_, err := collect(Config{ProjectDir: projectDir, PackDir: "pack"})
	if !errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("collect() error = %v, want ErrUnsafeFile", err)
	}
}

func TestSafeRootRejectsMissingOrUnobservedDirectories(t *testing.T) {
	for _, tt := range []struct {
		name string
		path string
		want error
	}{
		{"relative", "relative", ErrInvalidPath},
		{"missing", filepath.Join(t.TempDir(), "missing"), ErrUnsafeFile},
		{"unobserved", t.TempDir(), ErrUnsafeFile},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, err := openSafeRoot(tt.path, nil)
			if root != nil {
				root.Close()
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("openSafeRoot = %v, want %v", err, tt.want)
			}
		})
	}
	t.Run("filesystem root", func(t *testing.T) {
		observed, err := os.Stat("/")
		if err != nil {
			t.Fatal(err)
		}
		root, err := openSafeRoot("/", observed)
		if err != nil {
			t.Fatalf("open filesystem root: %v", err)
		}
		if err := root.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCollectRejectsUnreadableDirectories(t *testing.T) {
	for _, test := range []struct {
		name, relative, want string
	}{
		{"project root", ".", "open ProjectDir"},
		{"nested pack directory", "pack/private", "open PackDir directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			writeBundleTestFile(t, filepath.Join(projectDir, "pack", "private", "asset"), "asset", 0600)
			name := filepath.Join(projectDir, test.relative)
			if err := os.Chmod(name, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(name, 0700); err != nil {
					t.Error(err)
				}
			})
			probe, err := os.Open(name)
			if err == nil {
				probe.Close()
				t.Skip("filesystem or user permits reading mode-000 directories")
			}
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("permission probe = %v", err)
			}
			cfg := Config{ProjectDir: projectDir}
			if test.relative != "." {
				cfg.PackDir = "pack"
			}
			if _, err := collect(cfg); !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("collect = %v, want permission failure at %s", err, test.want)
			}
		})
	}
}

func TestSafeDirectoryTraversesAndRejectsClosedHandles(t *testing.T) {
	rootPath := t.TempDir()
	writeBundleTestFile(t, filepath.Join(rootPath, "one", "two", "source"), "content", 0600)
	observed, err := observeRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	root, err := openSafeRoot(observed.path, observed.info)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	file, err := root.OpenFile(filepath.Join("one", "two", "source"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := root.OpenFile("one"); !errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("directory as file = %v", err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := root.OpenFile("one"); !errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("closed directory open = %v", err)
	}
	c := newCollector(resolvedLimits{})
	if err := c.addTree(root, "pack"); err == nil {
		t.Fatal("closed directory traversal accepted")
	}
}

func TestBundleRejectsNonportableDiskEntry(t *testing.T) {
	root := t.TempDir()
	writeBundleTestFile(t, filepath.Join(root, "pack", "bad:name"), "asset", 0600)
	if _, err := collect(Config{ProjectDir: root, PackDir: "pack"}); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("nonportable disk entry = %v", err)
	}
}

func TestRelativeFilesystemPathsRejectRemovedWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.Remove(cwd); err != nil {
		t.Fatal(err)
	}
	t.Run("canonical path propagates Getwd failure", func(t *testing.T) {
		// Some hosts (including macOS) can still resolve an unlinked cwd.
		// A missing output path is valid when Getwd succeeds.
		if cwd, err := os.Getwd(); err == nil {
			t.Skipf("Getwd still resolves the removed directory as %q", cwd)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Getwd in removed working directory = %v", err)
		}
		if _, err := canonicalFilesystemPath("output.zip"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("canonical path in removed working directory = %v", err)
		}
	})
	if _, err := observeRoot("."); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("observe removed working directory = %v", err)
	}
}
