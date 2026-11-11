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

package interpsession

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
)

type scaffoldTestReader func([]byte) (int, error)

func TestReplaceRootFileFailurePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.WriteFile(filepath.Join(dir, "asset"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("source interrupted")
	if err := replaceRootFile(root, "asset", io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(failure)), 0600); !errors.Is(err, failure) {
		t.Fatalf("write failure=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "asset")); err != nil || string(data) != "old" {
		t.Fatalf("destination=%q,%v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v,%v", entries, err)
	}
	for _, name := range []string{"", ".", "../escape", "nested/file"} {
		if err := replaceRootFile(root, name, strings.NewReader("bad"), 0600); err == nil {
			t.Errorf("accepted unsafe name%q", name)
		}
	}
	if err := root.Mkdir("directory", 0700); err != nil {
		t.Fatal(err)
	}
	if err := replaceRootFile(root, "directory", strings.NewReader("bad"), 0600); err == nil {
		t.Fatal("replaced directory")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := replaceRootFile(root, "asset", strings.NewReader("bad"), 0600); err == nil {
		t.Fatal("closed root accepted")
	}
}

func TestPinnedDirectoryRejectsInvalidAndChangedRoots(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, filepath.Join(dir, "missing")} {
		if root, err := openPinnedRoot("root", path); err == nil {
			root.Close()
			t.Errorf("accepted non-directory %s", path)
		}
	}
	if err := ensureDirectoryNoSymlink("root", file, 0700); err == nil {
		t.Fatal("accepted regular file as directory")
	}
	if err := ensureDirectoryNoSymlink("root", filepath.Join(file, "child"), 0700); err == nil {
		t.Fatal("created directory below file")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, name := range []string{"file", "file/child", "missing/child", "../escape"} {
		if sub, err := ensurePinnedSubdirectory(root, name, 0700); err == nil {
			sub.Close()
			t.Errorf("accepted invalid subdirectory%q", name)
		}
	}
	sub, err := ensurePinnedSubdirectory(root, "valid", 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if err := verifyPinnedSubdirectory(root, "file", sub); err == nil {
		t.Fatal("accepted replaced subdirectory identity")
	}
	if err := verifyPinnedRootPath("root", file, root); err == nil {
		t.Fatal("accepted replaced root identity")
	}
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyPinnedSubdirectory(root, "valid", sub); err == nil {
		t.Fatal("accepted closed subdirectory")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyPinnedRootPath("root", dir, root); err == nil {
		t.Fatal("accepted closed root")
	}
	if sub, err := ensurePinnedSubdirectory(root, "valid", 0700); err == nil {
		sub.Close()
		t.Fatal("accepted closed parent")
	}
}

func TestEnsureDirectoryCreationFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires POSIX permissions and an unprivileged user")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
	path := filepath.Join(parent, "session")
	err := ensureDirectoryNoSymlink("SessionDir", path, 0o700)
	if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "create SessionDir") {
		t.Fatalf("ensureDirectoryNoSymlink = %v; want create failure wrapping permission error", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("failed creation left a session directory: %v", err)
	}
}

func TestReplaceRootFileTemporaryNameCollisions(t *testing.T) {
	// Keep this serial with scaffold tests: collision names use a shared sequence.
	for _, collisions := range []int{1, 100} {
		t.Run(fmt.Sprintf("collisions=%d", collisions), func(t *testing.T) {
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			const name = "asset"
			if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			// Reserve the next names without changing the sequence: an existing
			// temporary file must never be overwritten or removed by a retry.
			sequence := scaffoldTempSequence.Load()
			reserved := make([]string, collisions)
			for i := range reserved {
				reserved[i] = fmt.Sprintf(".%s.tmp-%d-%d", name, os.Getpid(), sequence+uint64(i)+1)
				if err := os.WriteFile(filepath.Join(dir, reserved[i]), []byte("reserved"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err = replaceRootFile(root, name, strings.NewReader("new"), 0o600)
			want := "new"
			if collisions == 100 {
				want = "old"
				if err == nil || !strings.Contains(err.Error(), "could not allocate temporary scaffold") {
					t.Fatalf("exhausted retry error = %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(data) != want {
				t.Fatalf("destination = %q, %v; want %q", data, err, want)
			}
			for _, reservedName := range reserved {
				if data, err := os.ReadFile(filepath.Join(dir, reservedName)); err != nil || string(data) != "reserved" {
					t.Errorf("reserved file %q = %q, %v", reservedName, data, err)
				}
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != collisions+1 {
				t.Fatalf("unexpected temporary files: %v, %v", entries, err)
			}
		})
	}
}

func TestReplaceRootFilePreservesCompetingDestination(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	target := filepath.Join(directory, "asset")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader("replacement")
	replaced := false
	reader := scaffoldTestReader(func(p []byte) (int, error) {
		if !replaced {
			replaced = true
			// Create a conflicting directory while the replacement is being
			// streamed, after the original destination has been inspected.
			if err := os.Rename(target, target+".old"); err != nil {
				return 0, err
			}
			if err := os.Mkdir(target, 0o700); err != nil {
				return 0, err
			}
			if err := os.WriteFile(filepath.Join(target, "keep"), []byte("concurrent file"), 0o600); err != nil {
				return 0, err
			}
		}
		return input.Read(p)
	})
	if err := replaceRootFile(root, "asset", reader, 0o600); err == nil {
		t.Fatal("replacement overwrote a competing directory")
	}
	for path, want := range map[string]string{target + ".old": "original", filepath.Join(target, "keep"): "concurrent file"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatalf("preserved file %q = %q, %v; want %q", path, data, err, want)
		}
	}
	if entries, err := os.ReadDir(directory); err != nil || len(entries) != 2 {
		t.Fatalf("failed publication leaked temporary files: %v, %v", entries, err)
	}
}

func (f scaffoldTestReader) Read(p []byte) (int, error) {
	return f(p)
}
