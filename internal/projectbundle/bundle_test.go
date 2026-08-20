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
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestArchiveCanonicalAndDeterministic(t *testing.T) {
	projectDir := t.TempDir()
	outputDir := t.TempDir()

	writeBundleTestFile(t, filepath.Join(projectDir, "z.go"), "z", 0o700)
	writeBundleTestFile(t, filepath.Join(projectDir, "main.go"), "main", 0o600)
	writeBundleTestFile(t, filepath.Join(projectDir, ".config"), "config", 0o600)
	writeBundleTestFile(t, filepath.Join(projectDir, "assets", "z.txt"), "asset-z", 0o600)
	writeBundleTestFile(t, filepath.Join(projectDir, "assets", "nested", "a.txt"), "asset-a", 0o700)

	output := filepath.Join(outputDir, "output.zip")
	finalOutput := filepath.Join(outputDir, "final.zip")
	writeBundleTestFile(t, output, "keep-output", 0o600)
	writeBundleTestFile(t, finalOutput, "keep-final", 0o600)

	config := Config{
		ProjectDir:    projectDir,
		ProjectFiles:  []string{"z.go", "main.go"},
		IncludeConfig: true,
		PackDir:       "assets",
		Output:        output,
		FinalOutput:   finalOutput,
	}
	bundle, err := Collect(config)
	if err != nil {
		t.Fatal(err)
	}
	wantEntries := []Entry{
		{Name: ".config", Size: 6},
		{Name: "assets/nested/a.txt", Size: 7},
		{Name: "assets/z.txt", Size: 7},
		{Name: "main.go", Size: 4},
		{Name: "z.go", Size: 1},
	}
	if got := bundle.Entries(); !reflect.DeepEqual(got, wantEntries) {
		t.Fatalf("Entries() = %#v, want %#v", got, wantEntries)
	}
	if got, want := bundle.TotalBytes(), int64(25); got != want {
		t.Fatalf("TotalBytes() = %d, want %d", got, want)
	}

	data, digest, err := bundle.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if want := Digest(sha256.Sum256(data)); digest != want {
		t.Fatalf("digest = %s, want %s", digest, want)
	}
	if len(digest.String()) != sha256.Size*2 {
		t.Fatalf("digest string length = %d, want %d", len(digest.String()), sha256.Size*2)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	contents := make(map[string]string, len(reader.File))
	for i, file := range reader.File {
		if file.Name != wantEntries[i].Name {
			t.Fatalf("ZIP entry %d = %q, want %q", i, file.Name, wantEntries[i].Name)
		}
		if strings.ContainsRune(file.Name, '\\') {
			t.Fatalf("ZIP entry %q contains a host separator", file.Name)
		}
		if file.Method != zip.Deflate {
			t.Fatalf("ZIP entry %q method = %d, want Deflate", file.Name, file.Method)
		}
		if !file.Modified.Equal(canonicalZipTime) {
			t.Fatalf("ZIP entry %q time = %s, want %s", file.Name, file.Modified, canonicalZipTime)
		}
		if got := file.Mode(); got != 0o644 {
			t.Fatalf("ZIP entry %q mode = %s, want 0644", file.Name, got)
		}
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(input)
		closeErr := input.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		contents[file.Name] = string(content)
	}
	if contents["assets/nested/a.txt"] != "asset-a" {
		t.Fatalf("unexpected ZIP contents: %#v", contents)
	}

	reordered := config
	reordered.ProjectFiles = []string{"main.go", "z.go"}
	secondData, secondDigest, err := Archive(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, secondData) || digest != secondDigest {
		t.Fatal("archive changed when allowlist order changed")
	}

	changedTime := time.Date(2037, time.August, 9, 10, 11, 12, 0, time.UTC)
	if err := os.Chmod(filepath.Join(projectDir, "main.go"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(projectDir, "main.go"), changedTime, changedTime); err != nil {
		t.Fatal(err)
	}
	thirdData, thirdDigest, err := Archive(config)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, thirdData) || digest != thirdDigest {
		t.Fatal("archive changed when source metadata changed")
	}

	var streamed bytes.Buffer
	streamDigest, err := WriteArchive(&streamed, config)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, streamed.Bytes()) || digest != streamDigest {
		t.Fatal("WriteArchive output differs from Archive")
	}
	assertBundleTestFile(t, output, "keep-output")
	assertBundleTestFile(t, finalOutput, "keep-final")
}

func TestCollectUsesImmutableConfigBytes(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{name: "content", data: []byte(`{"name":"validated"}`)},
		{name: "empty", data: []byte{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			writeBundleTestFile(t, filepath.Join(projectDir, ".config"), `{"extasset":"live"}`, 0o600)

			bundle, err := Collect(Config{
				ProjectDir: projectDir, IncludeConfig: true, ConfigBytes: tt.data,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := bundle.entries; len(got) != 1 || got[0].name != ".config" || !bytes.Equal(got[0].data, tt.data) {
				t.Fatalf("collected entries = %#v, want immutable .config %q", got, tt.data)
			}
			if len(tt.data) != 0 {
				tt.data[0] ^= 0xff
				if bytes.Equal(bundle.entries[0].data, tt.data) {
					t.Fatal("bundle retained caller-owned config storage")
				}
			}
		})
	}
}

func TestCollectRejectsInvalidRelativePaths(t *testing.T) {
	projectDir := t.TempDir()
	writeBundleTestFile(t, filepath.Join(projectDir, "file.txt"), "file", 0o600)

	tests := []struct {
		name   string
		config Config
	}{
		{name: "project absolute", config: Config{ProjectDir: projectDir, ProjectFiles: []string{filepath.Join(projectDir, "file.txt")}}},
		{name: "project parent", config: Config{ProjectDir: projectDir, ProjectFiles: []string{"../file.txt"}}},
		{name: "project unclean", config: Config{ProjectDir: projectDir, ProjectFiles: []string{"dir/../file.txt"}}},
		{name: "project backslash", config: Config{ProjectDir: projectDir, ProjectFiles: []string{`dir\file.txt`}}},
		{name: "project windows absolute", config: Config{ProjectDir: projectDir, ProjectFiles: []string{"C:/file.txt"}}},
		{name: "project windows ADS", config: Config{ProjectDir: projectDir, ProjectFiles: []string{"file.txt:secret"}}},
		{name: "project DOS device", config: Config{ProjectDir: projectDir, ProjectFiles: []string{"CON.txt"}}},
		{name: "project trailing dot", config: Config{ProjectDir: projectDir, ProjectFiles: []string{"file.txt."}}},
		{name: "project trailing space", config: Config{ProjectDir: projectDir, ProjectFiles: []string{"file.txt "}}},
		{name: "project reserved wildcard", config: Config{ProjectDir: projectDir, ProjectFiles: []string{"file?.txt"}}},
		{name: "pack parent", config: Config{ProjectDir: projectDir, PackDir: "../pack"}},
		{name: "pack dot", config: Config{ProjectDir: projectDir, PackDir: "."}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Collect(test.config)
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("Collect() error = %v, want ErrInvalidPath", err)
			}
		})
	}
}

func TestValidateRelativePathRejectsPortableAliases(t *testing.T) {
	rejected := []string{
		"file:stream",
		"dir/file.",
		"dir/file ",
		"dir/bad<name",
		`dir/bad"name`,
		"dir/bad|name",
		"dir/bad?name",
		"dir/bad*name",
		"dir/bad\x1fname",
		"CON",
		"con.txt",
		"PRN.tar.gz",
		"dir/AUX",
		"dir/nul.json",
		"CLOCK$",
		"CONIN$.txt",
		"conout$",
		"COM1",
		"com9.log",
		"LPT1",
		"lpt9.log",
		"COM\u00b9.txt",
		"LPT\u00b2",
	}
	for _, name := range rejected {
		t.Run(name, func(t *testing.T) {
			if _, err := validateRelativePath(name, "test"); !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("validateRelativePath(%q) error = %v, want ErrInvalidPath", name, err)
			}
		})
	}

	allowed := []string{".config", "console.txt", "com0.txt", "com10.txt", "lpt10.txt", "file name.txt", "caf\u00e9.txt"}
	for _, name := range allowed {
		if got, err := validateRelativePath(name, "test"); err != nil || got != name {
			t.Errorf("validateRelativePath(%q) = %q, %v", name, got, err)
		}
	}
}

func TestCollectRejectsEntryNameCollisions(t *testing.T) {
	projectDir := t.TempDir()
	writeBundleTestFile(t, filepath.Join(projectDir, "same.txt"), "same", 0o600)
	writeBundleTestFile(t, filepath.Join(projectDir, "pack", "same.txt"), "pack", 0o600)

	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "duplicate declarations",
			config: Config{ProjectDir: projectDir, ProjectFiles: []string{"same.txt", "same.txt"}},
		},
		{
			name:   "project and pack",
			config: Config{ProjectDir: projectDir, ProjectFiles: []string{"pack/same.txt"}, PackDir: "pack"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Collect(test.config)
			if !errors.Is(err, ErrCollision) {
				t.Fatalf("Collect() error = %v, want ErrCollision", err)
			}
		})
	}
}

func TestCollectorRejectsCanonicalEntryNameCollisions(t *testing.T) {
	limits, err := resolveLimits(Limits{})
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		first  string
		second string
	}{
		{name: "case folded", first: "Shared/File.txt", second: "shared/file.TXT"},
		{name: "unicode canonical", first: "café.txt", second: "cafe\u0301.txt"},
	} {
		t.Run(test.name, func(t *testing.T) {
			collector := newCollector(limits)
			if err := collector.reserveName(test.first); err != nil {
				t.Fatal(err)
			}
			if err := collector.reserveName(test.second); !errors.Is(err, ErrCollision) {
				t.Fatalf("reserveName(%q) error = %v, want ErrCollision", test.second, err)
			}
		})
	}
}

func TestCollectRejectsSymlinksAndNonRegularFiles(t *testing.T) {
	t.Run("project file symlink", func(t *testing.T) {
		projectDir := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.txt")
		writeBundleTestFile(t, outside, "outside", 0o600)
		if err := os.Symlink(outside, filepath.Join(projectDir, "link.txt")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := Collect(Config{ProjectDir: projectDir, ProjectFiles: []string{"link.txt"}})
		if !errors.Is(err, ErrUnsafeFile) {
			t.Fatalf("Collect() error = %v, want ErrUnsafeFile", err)
		}
	})

	t.Run("project parent symlink", func(t *testing.T) {
		projectDir := t.TempDir()
		outside := t.TempDir()
		writeBundleTestFile(t, filepath.Join(outside, "outside.txt"), "outside", 0o600)
		if err := os.Symlink(outside, filepath.Join(projectDir, "linked")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := Collect(Config{ProjectDir: projectDir, ProjectFiles: []string{"linked/outside.txt"}})
		if !errors.Is(err, ErrUnsafeFile) {
			t.Fatalf("Collect() error = %v, want ErrUnsafeFile", err)
		}
	})

	t.Run("pack symlink", func(t *testing.T) {
		projectDir := t.TempDir()
		writeBundleTestFile(t, filepath.Join(projectDir, "pack", "file.txt"), "file", 0o600)
		if err := os.Symlink("file.txt", filepath.Join(projectDir, "pack", "link.txt")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := Collect(Config{ProjectDir: projectDir, PackDir: "pack"})
		if !errors.Is(err, ErrUnsafeFile) {
			t.Fatalf("Collect() error = %v, want ErrUnsafeFile", err)
		}
	})

	t.Run("PackDir symlink", func(t *testing.T) {
		projectDir := t.TempDir()
		realPack := filepath.Join(projectDir, "real-pack")
		writeBundleTestFile(t, filepath.Join(realPack, "file.txt"), "file", 0o600)
		if err := os.Symlink(realPack, filepath.Join(projectDir, "linked-pack")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := Collect(Config{ProjectDir: projectDir, PackDir: "linked-pack"})
		if !errors.Is(err, ErrUnsafeFile) {
			t.Fatalf("Collect() error = %v, want ErrUnsafeFile", err)
		}
	})

	t.Run("PackDir parent symlink", func(t *testing.T) {
		projectDir := t.TempDir()
		realParent := filepath.Join(projectDir, "real-parent")
		if err := os.MkdirAll(filepath.Join(realParent, "pack"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(realParent, filepath.Join(projectDir, "linked-parent")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := Collect(Config{ProjectDir: projectDir, PackDir: "linked-parent/pack"})
		if !errors.Is(err, ErrUnsafeFile) {
			t.Fatalf("Collect() error = %v, want ErrUnsafeFile", err)
		}
	})

	t.Run("listed directory", func(t *testing.T) {
		projectDir := t.TempDir()
		if err := os.Mkdir(filepath.Join(projectDir, "directory"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := Collect(Config{ProjectDir: projectDir, ProjectFiles: []string{"directory"}})
		if !errors.Is(err, ErrUnsafeFile) {
			t.Fatalf("Collect() error = %v, want ErrUnsafeFile", err)
		}
	})
}

func TestCollectCanonicalizesConfiguredRootPaths(t *testing.T) {
	container := t.TempDir()
	realRoot := filepath.Join(container, "real")
	projectDir := filepath.Join(realRoot, "project")
	writeBundleTestFile(t, filepath.Join(projectDir, "main.go"), "main", 0o600)
	if err := os.Symlink(realRoot, filepath.Join(container, "alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	bundle, err := Collect(Config{
		ProjectDir:   filepath.Join(container, "alias", "project"),
		ProjectFiles: []string{"main.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := bundle.Entries(); len(got) != 1 {
		t.Fatalf("Entries() = %#v, want one entry", got)
	}
}

func TestOpenSafeRootRejectsReplacementAfterObservation(t *testing.T) {
	container := t.TempDir()
	rootPath := filepath.Join(container, "root")
	if err := os.Mkdir(rootPath, 0o755); err != nil {
		t.Fatal(err)
	}
	observation, err := observeRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(rootPath, filepath.Join(container, "original-root")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(rootPath, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := openSafeRoot(observation.path, observation.info)
	if root != nil {
		_ = root.Close()
	}
	if !errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("openSafeRoot() error = %v, want ErrUnsafeFile", err)
	}
}

func TestCollectRejectsOutputsWithinRecursiveRoots(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, "pack"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(projectDir, "pack"), filepath.Join(projectDir, "pack-alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	base := Config{
		ProjectDir: projectDir,
		PackDir:    "pack",
	}
	tests := []struct {
		name        string
		output      string
		finalOutput string
	}{
		{name: "pack root", output: filepath.Join(projectDir, "pack")},
		{name: "pack descendant missing", output: filepath.Join(projectDir, "pack", "new", "out.zip")},
		{name: "pack symlink alias", output: filepath.Join(projectDir, "pack-alias", "out.zip")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := base
			config.Output = test.output
			config.FinalOutput = test.finalOutput
			_, err := Collect(config)
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("Collect() error = %v, want ErrInvalidPath", err)
			}
		})
	}

	allowed := base
	allowed.Output = filepath.Join(t.TempDir(), "out.zip")
	if _, err := Collect(allowed); err != nil {
		t.Fatalf("Collect() with outside output: %v", err)
	}
}

func TestCollectEnforcesLimits(t *testing.T) {
	projectDir := t.TempDir()
	writeBundleTestFile(t, filepath.Join(projectDir, "one"), "123", 0o600)
	writeBundleTestFile(t, filepath.Join(projectDir, "two"), "456", 0o600)

	tests := []struct {
		name   string
		limits Limits
		files  []string
	}{
		{name: "entries", limits: Limits{MaxEntries: 1}, files: []string{"one", "two"}},
		{name: "one file", limits: Limits{MaxFileBytes: 2}, files: []string{"one"}},
		{name: "total", limits: Limits{MaxTotalBytes: 5}, files: []string{"one", "two"}},
		{name: "negative", limits: Limits{MaxEntries: -1}, files: []string{"one"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Collect(Config{ProjectDir: projectDir, ProjectFiles: test.files, Limits: test.limits})
			if !errors.Is(err, ErrLimit) {
				t.Fatalf("Collect() error = %v, want ErrLimit", err)
			}
		})
	}

	reference, _, err := Archive(Config{ProjectDir: projectDir, ProjectFiles: []string{"one"}})
	if err != nil {
		t.Fatal(err)
	}
	exactBundle, err := Collect(Config{
		ProjectDir:   projectDir,
		ProjectFiles: []string{"one"},
		Limits:       Limits{MaxArchiveBytes: int64(len(reference))},
	})
	if err != nil {
		t.Fatal(err)
	}
	exact, _, err := exactBundle.Bytes()
	if err != nil {
		t.Fatalf("Bytes() at exact archive limit: %v", err)
	}
	if !bytes.Equal(exact, reference) {
		t.Fatal("archive changed when exact archive limit was applied")
	}
	overBundle, err := Collect(Config{
		ProjectDir:   projectDir,
		ProjectFiles: []string{"one"},
		Limits:       Limits{MaxArchiveBytes: int64(len(reference) - 1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := overBundle.Bytes(); !errors.Is(err, ErrLimit) {
		t.Fatalf("Bytes() error = %v, want ErrLimit", err)
	}
}

func writeBundleTestFile(t *testing.T, name, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(name, mode); err != nil {
		t.Fatal(err)
	}
}

func assertBundleTestFile(t *testing.T, name, want string) {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != want {
		t.Fatalf("file %q = %q, want %q", name, got, want)
	}
}
