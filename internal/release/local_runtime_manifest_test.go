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

package release

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type localManifestErrorWriter struct{ err error }

func TestLocalRuntimeManifestWriteAndVerify(t *testing.T) {
	root := t.TempDir()
	lock, enginePath, packPath := writeHostRuntimeTestFiles(t, root, "engine-v1", "pack-v1")

	manifest, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := LocalRuntimeManifestPath(root, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadLocalRuntimeManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.ValidateForLock(lock, runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatal(err)
	}
	manifestDir := filepath.Dir(manifestPath)
	if err := loaded.VerifyFiles(manifestDir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{loaded.Engine.Name, loaded.Pack.Name} {
		if _, err := os.Stat(filepath.Join(manifestDir, name)); err != nil {
			t.Fatalf("published local runtime file %s: %v", name, err)
		}
	}

	if err := os.WriteFile(enginePath, []byte("engine-v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	updated, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, updated, enginePath, packPath); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadLocalRuntimeManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Engine.SHA256 == loaded.Engine.SHA256 {
		t.Fatal("rewriting local runtime manifest did not replace the old digest")
	}
	if reloaded.Engine.Name == loaded.Engine.Name {
		t.Fatal("rewriting local runtime manifest reused the old content address")
	}
	if err := reloaded.VerifyFiles(manifestDir); err != nil {
		t.Fatal(err)
	}
	if err := loaded.VerifyFiles(manifestDir); err != nil {
		t.Fatalf("previous manifest stopped resolving after atomic update: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".engine-manifest-") {
			t.Fatalf("temporary manifest file was left behind: %s", entry.Name())
		}
	}
}

func TestLocalRuntimeManifestSourceValidationUsesVersion(t *testing.T) {
	root := t.TempDir()
	lock, enginePath, packPath := writeHostRuntimeTestFiles(t, root, "engine", "pack")
	manifest, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest.RuntimeABI++
	manifest.LockSHA256 = strings.Repeat("0", 64)
	if err := manifest.ValidateForVersion(lock, runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatalf("source-mode version validation: %v", err)
	}
	if err := manifest.ValidateForLock(lock, runtime.GOOS, runtime.GOARCH); err == nil {
		t.Fatal("strict validation accepted stale local metadata")
	}
	manifest.RuntimeVersion = "9.9.9"
	if err := manifest.ValidateForVersion(lock, runtime.GOOS, runtime.GOARCH); err == nil {
		t.Fatal("source-mode validation accepted a different runtime version")
	}
}

func TestNewLocalRuntimeManifestRejectsSymlink(t *testing.T) {
	lock := DefaultRuntimeLock()
	spec, err := HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	engineTarget := filepath.Join(root, "engine-target")
	enginePath := filepath.Join(root, spec.RuntimeName)
	packPath := filepath.Join(root, spec.PackName)
	if err := os.WriteFile(engineTarget, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(engineTarget, enginePath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(packPath, []byte("pack"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("NewLocalRuntimeManifest error = %v, want symlink rejection", err)
	}
}

func TestNewLocalRuntimeManifestRejectsUnlockedNames(t *testing.T) {
	lock := DefaultRuntimeLock()
	root := t.TempDir()
	enginePath := filepath.Join(root, "engine")
	packPath := filepath.Join(root, "runtime.pck")
	if err := os.WriteFile(enginePath, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("pack"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath); err == nil || !strings.Contains(err.Error(), "file names") {
		t.Fatalf("NewLocalRuntimeManifest error = %v, want locked-name rejection", err)
	}
}

func TestLocalRuntimeManifestPathRequiresAbsoluteCleanRoot(t *testing.T) {
	lock := DefaultRuntimeLock()
	for _, root := range []string{"", ".", t.TempDir() + string(filepath.Separator) + ".."} {
		if _, err := LocalRuntimeManifestPath(root, lock, runtime.GOOS, runtime.GOARCH); err == nil {
			t.Fatalf("LocalRuntimeManifestPath accepted root %q", root)
		}
	}
}

func TestPublishLocalRuntimeManifestRejectsChangedSource(t *testing.T) {
	root := t.TempDir()
	lock, enginePath, packPath := writeHostRuntimeTestFiles(t, root, "engine-v1", "pack-v1")
	manifest, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(enginePath, []byte("engine-v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath, err := LocalRuntimeManifestPath(root, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("PublishLocalRuntimeManifest error = %v, want digest mismatch", err)
	}
	if _, err := os.Lstat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("manifest after failed publish: err=%v", err)
	}
}

func TestPublishLocalRuntimeManifestKeepsPreviousGenerationOnPackFailure(t *testing.T) {
	sourceDir := t.TempDir()
	repoRoot := t.TempDir()
	lock, enginePath, packPath := writeHostRuntimeTestFiles(t, sourceDir, "engine-v1", "pack-v1")
	previous, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := LocalRuntimeManifestPath(repoRoot, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, previous, enginePath, packPath); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(enginePath, []byte("engine-v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("pack-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("changed-after-manifest"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, next, enginePath, packPath); err == nil {
		t.Fatal("publish with changed PCK unexpectedly succeeded")
	}

	loaded, err := loadLocalRuntimeManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Engine.SHA256 != previous.Engine.SHA256 || loaded.Pack.SHA256 != previous.Pack.SHA256 {
		t.Fatalf("failed publish replaced previous manifest: %#v", loaded)
	}
	if err := loaded.VerifyFiles(filepath.Dir(manifestPath)); err != nil {
		t.Fatalf("failed publish damaged previous generation: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(manifestPath), next.Engine.Name)); !os.IsNotExist(err) {
		t.Fatalf("failed preflight left next Engine object: %v", err)
	}
}

func TestPublishLocalRuntimeManifestRejectsDestinationSymlink(t *testing.T) {
	root := t.TempDir()
	lock, enginePath, packPath := writeHostRuntimeTestFiles(t, root, "engine", "pack")
	manifest, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := LocalRuntimeManifestPath(root, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	manifestDir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside-engine")
	if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(manifestDir, manifest.Engine.Name)
	if err := os.Symlink(target, destination); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("PublishLocalRuntimeManifest error = %v, want destination symlink rejection", err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "outside" {
		t.Fatalf("symlink target changed: data=%q err=%v", got, err)
	}
	if _, err := os.Lstat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("manifest after failed publish: err=%v", err)
	}
}

func TestLocalManifestRejectsInvalidStructureAtPublicBoundaries(t *testing.T) {
	base, _, _ := localManifestFixture(t)
	for _, tt := range []struct {
		name, want string
		change     func(*LocalRuntimeManifest)
	}{
		{"schema", "schema", func(m *LocalRuntimeManifest) { m.Schema = "unknown" }},
		{"mode", "mode", func(m *LocalRuntimeManifest) { m.Mode = "published" }},
		{"version", "version", func(m *LocalRuntimeManifest) { m.RuntimeVersion = "bad version" }},
		{"ABI", "ABI", func(m *LocalRuntimeManifest) { m.RuntimeABI = 0 }},
		{"lock digest", "lock SHA-256", func(m *LocalRuntimeManifest) { m.LockSHA256 = "bad" }},
		{"target", "target", func(m *LocalRuntimeManifest) { m.GOOS = "../linux" }},
		{"engine name", "engine has invalid name", func(m *LocalRuntimeManifest) { m.Engine.Name = "../engine" }},
		{"pack size", "pack size", func(m *LocalRuntimeManifest) { m.Pack.Size = 0 }},
		{"pack digest", "pack has invalid SHA-256", func(m *LocalRuntimeManifest) { m.Pack.SHA256 = "bad" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := base
			tt.change(&m)
			for _, err := range []error{m.Validate(), m.VerifyFiles(t.TempDir()), m.ValidateForLock(DefaultRuntimeLock(), "linux", "amd64"), m.ValidateForVersion(DefaultRuntimeLock(), "linux", "amd64")} {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("invalid manifest = %v, want %q", err, tt.want)
				}
			}
			if _, err := m.JSON(); err == nil {
				t.Fatal("invalid manifest serialized")
			}
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseLocalRuntimeManifest(data); err == nil {
				t.Fatal("invalid manifest parsed")
			}
		})
	}
	if _, err := ParseLocalRuntimeManifest([]byte("{")); err == nil {
		t.Fatal("malformed JSON accepted")
	}
}

func TestLocalManifestRejectsMismatchedLockAndTarget(t *testing.T) {
	base, engine, pack := localManifestFixture(t)
	for _, tt := range []struct {
		name, want string
		change     func(*LocalRuntimeManifest, *RuntimeLock)
	}{
		{"invalid lock", "schema", func(m *LocalRuntimeManifest, l *RuntimeLock) { l.Schema = 0 }},
		{"lock digest", "does not match", func(m *LocalRuntimeManifest, l *RuntimeLock) { m.LockSHA256 = strings.Repeat("0", 64) }},
		{"target", "does not match host", func(m *LocalRuntimeManifest, l *RuntimeLock) { m.GOOS = "darwin" }},
		{"name", "file names", func(m *LocalRuntimeManifest, l *RuntimeLock) { m.Engine.Name = "engine" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, lock := base, DefaultRuntimeLock()
			tt.change(&m, &lock)
			if err := m.ValidateForLock(lock, "linux", "amd64"); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("lock validation = %v", err)
			}
		})
	}
	invalid := RuntimeLock{}
	if err := base.ValidateForVersion(invalid, "linux", "amd64"); err == nil {
		t.Fatal("invalid version lock accepted")
	}
	if _, err := NewLocalRuntimeManifest(invalid, "linux", "amd64", engine, pack); err == nil {
		t.Fatal("invalid construction lock accepted")
	}
	if _, err := LocalRuntimeManifestPath(t.TempDir(), DefaultRuntimeLock(), "plan9", "amd64"); err == nil {
		t.Fatal("unsupported manifest target accepted")
	}
	base.GOOS = "plan9"
	if err := base.ValidateForVersion(DefaultRuntimeLock(), "plan9", "amd64"); err == nil {
		t.Fatal("unsupported matching target accepted")
	}
	if _, err := NewLocalRuntimeManifest(DefaultRuntimeLock(), "plan9", "amd64", engine, pack); err == nil {
		t.Fatal("unsupported construction target accepted")
	}
}

func TestLocalManifestConstructionAndPublicationFailures(t *testing.T) {
	for _, scenario := range []string{"missing pack", "empty engine", "unknown version", "invalid manifest", "blocked directory", "blocked pack", "blocked manifest"} {
		t.Run(scenario, func(t *testing.T) {
			manifest, engine, pack := localManifestFixture(t)
			root := t.TempDir()
			output := filepath.Join(root, "manifest.json")
			want := ""
			switch scenario {
			case "missing pack":
				if err := os.Remove(pack); err != nil {
					t.Fatal(err)
				}
				_, err := NewLocalRuntimeManifest(DefaultRuntimeLock(), "linux", "amd64", engine, pack)
				if err == nil || !strings.Contains(err.Error(), "hash local runtime PCK") {
					t.Fatalf("missing pack = %v", err)
				}
				return
			case "empty engine":
				if err := os.Truncate(engine, 0); err != nil {
					t.Fatal(err)
				}
				if _, err := NewLocalRuntimeManifest(DefaultRuntimeLock(), "linux", "amd64", engine, pack); err == nil || !strings.Contains(err.Error(), "size must be positive") {
					t.Fatalf("empty engine = %v", err)
				}
				return
			case "unknown version":
				manifest.RuntimeVersion, want = "9.9.9", "no runtime lock snapshot"
			case "invalid manifest":
				manifest.Mode, want = "wrong", "mode"
			case "blocked directory":
				if err := os.WriteFile(output, []byte("blocking"), 0600); err != nil {
					t.Fatal(err)
				}
				output, want = filepath.Join(output, "manifest.json"), "create local runtime manifest directory"
			case "blocked pack":
				if err := os.Mkdir(filepath.Join(root, manifest.Pack.Name), 0700); err != nil {
					t.Fatal(err)
				}
				want = "publish local runtime PCK"
			case "blocked manifest":
				if err := os.Mkdir(output, 0700); err != nil {
					t.Fatal(err)
				}
				want = "install local runtime manifest"
			}
			if err := PublishLocalRuntimeManifest(output, manifest, engine, pack); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("publication = %v, want %q", err, want)
			}
			for _, pattern := range []string{".engine-manifest-*", ".local-runtime-file-*"} {
				matches, err := filepath.Glob(filepath.Join(root, pattern))
				if err != nil || len(matches) != 0 {
					t.Fatalf("temporary publication files = %v, %v", matches, err)
				}
			}
		})
	}
}

func TestVerifiedLocalCopyRejectsChangedDataAndFailedSink(t *testing.T) {
	for _, scenario := range []string{"size mismatch", "digest mismatch", "missing destination"} {
		t.Run(scenario, func(t *testing.T) {
			manifest, engine, _ := localManifestFixture(t)
			dir := t.TempDir()
			dst := filepath.Join(dir, "copied")
			want := manifest.Engine
			switch scenario {
			case "size mismatch":
				want.Size++
			case "digest mismatch":
				want.SHA256 = strings.Repeat("0", 64)
			case "missing destination":
				dst = filepath.Join(dir, "missing", "copied")
			}
			if err := copyVerifiedLocalRuntimeFile(engine, dst, want, 0600); err == nil {
				t.Fatal("invalid local copy accepted")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed copy leaked files: %v, %v", entries, err)
			}
		})
	}
	for _, scenario := range []string{"sink creation", "sink write", "source truncated", "source grows"} {
		t.Run(scenario, func(t *testing.T) {
			_, engine, _ := localManifestFixture(t)
			sentinel := errors.New("sink failed")
			_, err := readVerifiedLocalRuntimeFile(engine, func() (io.Writer, error) {
				switch scenario {
				case "sink creation":
					return nil, sentinel
				case "sink write":
					return localManifestErrorWriter{sentinel}, nil
				case "source truncated":
					if err := os.Truncate(engine, 0); err != nil {
						t.Fatal(err)
					}
				case "source grows":
					f, err := os.OpenFile(engine, os.O_APPEND|os.O_WRONLY, 0)
					if err != nil {
						t.Fatal(err)
					}
					_, writeErr := f.WriteString("extra")
					closeErr := f.Close()
					if writeErr != nil || closeErr != nil {
						t.Fatalf("append = %v, %v", writeErr, closeErr)
					}
				}
				return io.Discard, nil
			})
			if err == nil {
				t.Fatal("invalid local read accepted")
			}
			if strings.HasPrefix(scenario, "sink") && !errors.Is(err, sentinel) {
				t.Fatalf("lost sink error: %v", err)
			}
		})
	}
	manifest, engine, _ := localManifestFixture(t)
	if err := verifyLocalRuntimeFile(engine, LocalRuntimeFile{Size: manifest.Engine.Size + 1}); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("size verification = %v", err)
	}
	if err := writeLocalRuntimeManifest(filepath.Join(t.TempDir(), "manifest"), LocalRuntimeManifest{}); err == nil {
		t.Fatal("invalid manifest written")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeLocalRuntimeManifest(filepath.Join(blocked, "manifest"), manifest); err == nil {
		t.Fatal("blocked manifest directory accepted")
	}
	if err := replaceLocalRuntimeFile(engine+".missing", filepath.Join(t.TempDir(), "file"), 0600); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing replacement = %v", err)
	}
}

func TestLocalManifestVerificationRejectsMissingPublishedObjects(t *testing.T) {
	manifest, _, _ := localManifestFixture(t)
	if err := manifest.VerifyFiles(t.TempDir()); err == nil || !strings.Contains(err.Error(), "verify local runtime engine") || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing published object = %v", err)
	}
}

func (w localManifestErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func writeHostRuntimeTestFiles(t *testing.T, root, engine, pack string) (RuntimeLock, string, string) {
	t.Helper()
	lock := DefaultRuntimeLock()
	spec, err := HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	enginePath := filepath.Join(root, spec.RuntimeName)
	packPath := filepath.Join(root, spec.PackName)
	if err := os.WriteFile(enginePath, []byte(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte(pack), 0o644); err != nil {
		t.Fatal(err)
	}
	return lock, enginePath, packPath
}

func loadLocalRuntimeManifest(path string) (LocalRuntimeManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LocalRuntimeManifest{}, err
	}
	return ParseLocalRuntimeManifest(data)
}

func localManifestFixture(t *testing.T) (LocalRuntimeManifest, string, string) {
	t.Helper()
	lock := DefaultRuntimeLock()
	spec, err := HostRuntimeSpecFor(lock, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	engine, pack := filepath.Join(root, spec.RuntimeName), filepath.Join(root, spec.PackName)
	for name, content := range map[string]string{engine: "engine", pack: "pack"} {
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := NewLocalRuntimeManifest(lock, "linux", "amd64", engine, pack)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, engine, pack
}
