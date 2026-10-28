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

package projectpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/base/stablefile"
)

func TestValidatePortableConfig(t *testing.T) {
	projectDir := t.TempDir()
	if err := ValidatePortableConfig(projectDir); err != nil {
		t.Fatalf("ValidatePortableConfig() without .config: %v", err)
	}
	configPath := filepath.Join(projectDir, configName)
	writePolicyTestFile(t, configPath, []byte(`{"name":"demo"}`))
	if err := ValidatePortableConfig(projectDir); err != nil {
		t.Fatalf("ValidatePortableConfig() with supported fields: %v", err)
	}
	for _, key := range []string{"extasset", "ExtAsset", "EXTASSET"} {
		writePolicyTestFile(t, configPath, []byte(`{"`+key+`":"../shared-assets"}`))
		if err := ValidatePortableConfig(projectDir); err == nil || !strings.Contains(err.Error(), "unsupported extasset") {
			t.Fatalf("ValidatePortableConfig() key %q error = %v, want extasset rejection", key, err)
		}
	}
	writePolicyTestFile(t, configPath, []byte(`{"name":`))
	if err := ValidatePortableConfig(projectDir); err == nil || !strings.Contains(err.Error(), "parse project config") {
		t.Fatalf("malformed .config validation = %v", err)
	}
}

func TestValidatePortableConfigRejectsSymlink(t *testing.T) {
	projectDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "config")
	writePolicyTestFile(t, target, []byte(`{}`))
	if err := os.Symlink(target, filepath.Join(projectDir, configName)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := ValidatePortableConfig(projectDir); err == nil {
		t.Fatal("ValidatePortableConfig accepted symlink .config")
	}
}

func TestPortableConfigSnapshotRejectsIdentityAndContentDrift(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, configName)
	writePolicyTestFile(t, configPath, []byte(`{"name":"first"}`))
	snapshot, err := SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snapshot.Bytes()); got != `{"name":"first"}` {
		t.Fatalf("snapshot bytes = %q", got)
	}
	copy := snapshot.Bytes()
	copy[0] = '!'
	if got := string(snapshot.Bytes()); got != `{"name":"first"}` {
		t.Fatalf("caller changed retained snapshot contents: %q", got)
	}
	digest := sha256.Sum256([]byte(`{"name":"first"}`))
	wantIdentity := portableConfigSHA256Prefix + hex.EncodeToString(digest[:])
	if identity, err := snapshot.Identity(); err != nil || identity != wantIdentity {
		t.Fatalf("snapshot identity = %q, %v, want %q", identity, err, wantIdentity)
	}
	if err := snapshot.Verify(projectDir); err != nil {
		t.Fatalf("unchanged snapshot verification: %v", err)
	}

	// Keep the original inode alive so replacement cannot accidentally reuse it.
	if err := os.Rename(configPath, filepath.Join(projectDir, "original.config")); err != nil {
		t.Fatal(err)
	}
	writePolicyTestFile(t, configPath, []byte(`{"name":"first"}`))
	if err := snapshot.Verify(projectDir); err == nil || !strings.Contains(err.Error(), "changed after validation") {
		t.Fatalf("replacement verification error = %v", err)
	}

	snapshot, err = SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	writePolicyTestFile(t, configPath, []byte(`{"name":"other"}`))
	// Preserve identity, length, and modification time so verification must
	// reject changed bytes even when the filesystem metadata still agrees.
	if err := os.Chtimes(configPath, snapshot.fileInfo.ModTime(), snapshot.fileInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(snapshot.fileInfo, info) || !stablefile.SameMetadata(snapshot.fileInfo, info) {
		t.Fatal("content-drift fixture also changed identity or metadata")
	}
	if err := snapshot.Verify(projectDir); err == nil || !strings.Contains(err.Error(), "changed after validation") {
		t.Fatalf("content drift verification error = %v", err)
	}
	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(configPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Verify(projectDir); err == nil || !strings.Contains(err.Error(), "revalidate project config") {
		t.Fatalf("non-regular replacement verification = %v", err)
	}
}

func TestSnapshotPortableConfigRootUsesPinnedDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not permit renaming directories held by os.Root")
	}
	container := t.TempDir()
	projectDir := filepath.Join(container, "project")
	if err := os.Mkdir(projectDir, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(projectDir, configName)
	data := []byte(`{"name":"pinned"}`)
	writePolicyTestFile(t, configPath, data)
	root, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(projectDir, filepath.Join(container, "original-project")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(projectDir, 0700); err != nil {
		t.Fatal(err)
	}
	writePolicyTestFile(t, configPath, []byte(`{"name":"replacement"}`))
	snapshot, err := SnapshotPortableConfigRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snapshot.Bytes()); got != string(data) {
		t.Fatalf("pinned snapshot bytes = %q, want %q", got, data)
	}
}

func TestAbsentPortableConfigSnapshotRejectsCreation(t *testing.T) {
	projectDir := t.TempDir()
	snapshot, err := SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Present() || snapshot.Bytes() != nil {
		t.Fatal("absent config snapshot reported content")
	}
	if identity, err := snapshot.Identity(); err != nil || identity != portableConfigAbsentIdentity {
		t.Fatalf("absent snapshot identity = %q, %v", identity, err)
	}
	if err := snapshot.Verify(projectDir); err != nil {
		t.Fatalf("unchanged absent config verification: %v", err)
	}
	writePolicyTestFile(t, filepath.Join(projectDir, configName), []byte(`{}`))
	if err := snapshot.Verify(projectDir); err == nil || !strings.Contains(err.Error(), "changed after validation") {
		t.Fatalf("created config verification error = %v", err)
	}
}

func TestPortableConfigSnapshotRejectsOversizeFile(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, configName)
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxPortableConfigBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := SnapshotPortableConfig(projectDir); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("SnapshotPortableConfig() oversize error = %v", err)
	}
}

func TestPortableConfigSnapshotRejectsZeroValue(t *testing.T) {
	var snapshot PortableConfigSnapshot
	if snapshot.Present() || snapshot.Bytes() != nil {
		t.Fatal("zero snapshot reported config contents")
	}
	if identity, err := snapshot.Identity(); err == nil || identity != "" {
		t.Fatalf("zero snapshot identity = %q, %v, want initialization error", identity, err)
	}
	if err := snapshot.Verify(t.TempDir()); err == nil || !strings.Contains(err.Error(), "uninitialized") {
		t.Fatalf("zero snapshot verification = %v", err)
	}
}

func TestPortableConfigSnapshotRejectsUnavailableRoots(t *testing.T) {
	if _, err := SnapshotPortableConfig(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing project root = %v", err)
	}
	if _, err := SnapshotPortableConfigRoot(nil); err == nil || !strings.Contains(err.Error(), "root is nil") {
		t.Fatalf("nil root = %v", err)
	}
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := SnapshotPortableConfigRoot(root); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed root = %v", err)
	}
}

func TestReadStableFileRejectsUnusableAccess(t *testing.T) {
	for _, test := range []struct {
		name, message string
		want          error
	}{
		{"initial stat failure", "", os.ErrPermission},
		{"open failure", "", os.ErrPermission},
		{"removed before open", "", os.ErrNotExist},
		{"closed handle", "", os.ErrClosed},
		{"write-only handle", "", nil},
		{"opened directory", "changed while opening", nil},
		{"opened replacement inode", "changed while opening", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			configPath := filepath.Join(projectDir, configName)
			writePolicyTestFile(t, configPath, []byte(`{}`))
			access := stablefile.Reader{Lstat: os.Lstat, Open: os.Open}
			switch test.name {
			case "initial stat failure":
				access.Lstat = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
			case "open failure":
				access.Open = func(string) (*os.File, error) { return nil, os.ErrPermission }
			case "removed before open":
				access.Open = func(name string) (*os.File, error) {
					if err := os.Remove(name); err != nil {
						t.Fatal(err)
					}
					return os.Open(name)
				}
			case "closed handle":
				access.Open = func(name string) (*os.File, error) {
					file, err := os.Open(name)
					if err == nil {
						err = file.Close()
					}
					return file, err
				}
			case "write-only handle":
				access.Open = func(name string) (*os.File, error) { return os.OpenFile(name, os.O_WRONLY, 0) }
			case "opened directory":
				access.Open = func(string) (*os.File, error) { return os.Open(projectDir) }
			case "opened replacement inode":
				access.Open = func(name string) (*os.File, error) {
					if err := os.Rename(name, name+".original"); err != nil {
						t.Fatal(err)
					}
					writePolicyTestFile(t, name, []byte(`{}`))
					return os.Open(name)
				}
			}
			var opened *os.File
			open := access.Open
			access.Open = func(name string) (*os.File, error) {
				file, err := open(name)
				opened = file
				if file != nil {
					t.Cleanup(func() { _ = file.Close() })
				}
				return file, err
			}
			data, info, err := readStableFile(configPath, access)
			if err == nil || (test.want != nil && !errors.Is(err, test.want)) || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("readStableFile = %v, want %v / %q", err, test.want, test.message)
			}
			if test.name == "write-only handle" {
				var pathErr *os.PathError
				if !errors.As(err, &pathErr) || pathErr.Op != "read" {
					t.Fatalf("write-only failure = %v, want read error", err)
				}
			}
			if data != nil || info != nil {
				t.Fatalf("failed read returned trusted snapshot: %q, %v", data, info)
			}
			if opened != nil {
				if _, err := opened.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("failed read did not close handle: %v", err)
				}
			}
		})
	}
}

func TestReadStableFileRejectsChangesDuringRead(t *testing.T) {
	for _, kind := range []string{"oversized on open", "removed after read", "replaced after read", "symlink after read", "metadata after read"} {
		t.Run(kind, func(t *testing.T) {
			projectDir := t.TempDir()
			configPath := filepath.Join(projectDir, configName)
			writePolicyTestFile(t, configPath, []byte(`{}`))
			access := stablefile.Reader{Lstat: os.Lstat, Open: os.Open}
			want := "changed while reading"
			if kind == "oversized on open" {
				want = "exceeds"
				access.Open = func(name string) (*os.File, error) {
					if err := os.Truncate(name, maxPortableConfigBytes+1); err != nil {
						t.Fatal(err)
					}
					return os.Open(name)
				}
			} else {
				calls := 0
				access.Lstat = func(name string) (os.FileInfo, error) {
					calls++
					if calls == 2 {
						switch kind {
						case "removed after read":
							if err := os.Remove(name); err != nil {
								t.Fatal(err)
							}
						case "replaced after read", "symlink after read":
							if err := os.Rename(name, name+".original"); err != nil {
								t.Fatal(err)
							}
							if kind == "symlink after read" {
								if err := os.Symlink(name+".original", name); err != nil {
									t.Skipf("symlinks unavailable: %v", err)
								}
							} else if err := os.WriteFile(name, []byte(`{}`), 0600); err != nil {
								t.Fatal(err)
							}
						case "metadata after read":
							if err := os.Chtimes(name, time.Unix(42, 0), time.Unix(42, 0)); err != nil {
								t.Fatal(err)
							}
						}
					}
					return os.Lstat(name)
				}
			}
			data, info, err := readStableFile(configPath, access)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("readStableFile = %v, want %q", err, want)
			}
			if data != nil || info != nil {
				t.Fatalf("changed file returned trusted snapshot: %q, %v", data, info)
			}
		})
	}
}

func writePolicyTestFile(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
}
