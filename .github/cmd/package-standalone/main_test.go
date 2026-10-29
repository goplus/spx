package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPackageStandaloneNormalizesModificationTime(t *testing.T) {
	for _, name := range []string{"spx", "spx.exe"} {
		t.Run(name, func(t *testing.T) {
			originalArgs := os.Args
			t.Cleanup(func() { os.Args = originalArgs })
			root := t.TempDir()
			binary := filepath.Join(root, name)
			content := []byte("standalone binary with embedded runtime")
			if err := os.WriteFile(binary, content, 0o755); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(root, "release.zip")
			var previous []byte
			for _, year := range []int{2001, 2037} {
				stamp := time.Date(year, time.January, 2, 3, 4, 5, 0, time.UTC)
				if err := os.Chtimes(binary, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				os.Args = []string{"package-standalone", "--binary", binary, "--output", output}
				main()
				data, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if previous != nil && !bytes.Equal(previous, data) {
					t.Fatal("archive bytes changed after binary mtime changed")
				}
				previous = data
			}
			archive, err := zip.OpenReader(output)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			if len(archive.File) != 1 || archive.File[0].Name != name {
				t.Fatalf("archive must contain only %q: %+v", name, archive.File)
			}
			reader, err := archive.File[0].Open()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			got, err := io.ReadAll(reader)
			if err != nil || !bytes.Equal(got, content) {
				t.Fatalf("archived binary = %q, %v", got, err)
			}
		})
	}
}

func TestPackageStandaloneRejectsInvalidInputsWithoutRemovingOutput(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "release.zip")
	if err := os.WriteFile(output, []byte("previous archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []string{"missing", "directory", "empty", "unsupported name"} {
		t.Run(fixture, func(t *testing.T) {
			binary := filepath.Join(root, "spx")
			switch fixture {
			case "unsupported name":
				binary = filepath.Join(root, "another-tool")
				if err := os.WriteFile(binary, []byte("binary"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(binary, 0o755); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Remove(binary) })
			case "empty":
				if err := os.WriteFile(binary, nil, 0o755); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Remove(binary) })
			}
			if err := packageStandalone(binary, output); err == nil {
				t.Fatal("invalid binary was packaged")
			}
			got, err := os.ReadFile(output)
			if err != nil || string(got) != "previous archive" {
				t.Fatalf("existing output changed: %q, %v", got, err)
			}
		})
	}
}

func TestPackageStandaloneOutputErrorsPreserveInputs(t *testing.T) {
	for _, fixture := range []string{"parent is a file", "parent is a dangling symlink"} {
		t.Run(fixture, func(t *testing.T) {
			root := t.TempDir()
			binary := filepath.Join(root, "spx")
			if err := os.WriteFile(binary, []byte("original binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			parent := filepath.Join(root, "destination")
			missing := filepath.Join(root, "missing")
			output := filepath.Join(parent, "release.zip")
			wantErrorPath := output
			if fixture == "parent is a file" {
				if err := os.WriteFile(parent, []byte("previous archive"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Symlink(missing, parent); err != nil {
					t.Skipf("symbolic links unavailable: %v", err)
				}
				wantErrorPath = parent
			}
			// A regular parent file prevents reading destination metadata; a
			// dangling parent passes Lstat and prevents creating the archive.
			err := run([]string{"--binary", binary, "--output", output})
			var pathError *os.PathError
			if !errors.As(err, &pathError) || pathError.Path != wantErrorPath {
				t.Fatalf("output error = %v; want filesystem error at %s", err, wantErrorPath)
			}
			if data, err := os.ReadFile(binary); err != nil || string(data) != "original binary" {
				t.Fatalf("changed input binary: %q, %v", data, err)
			}
			if fixture == "parent is a file" {
				if data, err := os.ReadFile(parent); err != nil || string(data) != "previous archive" {
					t.Fatalf("changed existing file: %q, %v", data, err)
				}
			} else {
				if target, err := os.Readlink(parent); err != nil || target != missing {
					t.Fatalf("changed existing symlink: %q, %v", target, err)
				}
				if _, err := os.Stat(missing); !os.IsNotExist(err) {
					t.Fatalf("created symlink target: %v", err)
				}
			}
		})
	}
}

func TestPackageStandaloneRejectsDestructiveDestinations(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "spx")
	if err := os.WriteFile(binary, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{binary, root} {
		if err := packageStandalone(binary, output); err == nil {
			t.Fatalf("accepted unsafe archive destination %s", output)
		}
		if data, err := os.ReadFile(binary); err != nil || string(data) != "binary" {
			t.Fatalf("input was removed: %q, %v", data, err)
		}
	}
}

func TestPackageStandaloneCLIRequiresExplicitPaths(t *testing.T) {
	for _, args := range [][]string{nil, {"--binary", "spx"}, {"--output", "spx.zip"}, {"--unknown"}, {"extra"}} {
		if err := run(args); err == nil {
			t.Fatalf("run(%q) succeeded", args)
		}
	}
}

func TestPackageStandaloneMissingWorkingDirectoryPreservesFiles(t *testing.T) {
	root := t.TempDir()
	binary, output := filepath.Join(root, "spx"), filepath.Join(root, "release.zip")
	contents := map[string]string{binary: "original binary", output: "previous archive"}
	for path, content := range contents {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.Remove(cwd); err != nil {
		t.Skipf("cannot remove current working directory: %v", err)
	}
	if _, err := os.Getwd(); err == nil {
		t.Skip("Getwd succeeds after removing current working directory")
	}
	for _, test := range []struct{ name, binary, output string }{
		{"relative binary", "spx", output},
		{"relative output", binary, "release.zip"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := run([]string{"--binary", test.binary, "--output", test.output})
			if !errors.Is(err, os.ErrNotExist) || !strings.HasPrefix(err.Error(), "getwd: ") {
				t.Fatalf("relative path error = %v; want Getwd failure", err)
			}
			for path, want := range contents {
				if data, err := os.ReadFile(path); err != nil || string(data) != want {
					t.Fatalf("changed %s: %q, %v", path, data, err)
				}
			}
		})
	}
}

func TestPackageStandaloneRejectsInputAliasesWithoutChangingFiles(t *testing.T) {
	for _, alias := range []string{"hard link", "parent symlink"} {
		t.Run(alias, func(t *testing.T) {
			root := t.TempDir()
			binary := filepath.Join(root, "spx")
			if err := os.WriteFile(binary, []byte("original binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(root, "archive.zip")
			if alias == "hard link" {
				if err := os.Link(binary, output); err != nil {
					t.Skipf("hard links unavailable: %v", err)
				}
			} else {
				link := filepath.Join(root, "alias")
				if err := os.Symlink(root, link); err != nil {
					t.Skipf("symbolic links unavailable: %v", err)
				}
				output = filepath.Join(link, "spx")
			}
			if err := packageStandalone(binary, output); err == nil || !strings.Contains(err.Error(), "replace its input binary") {
				t.Fatalf("input alias error = %v", err)
			}
			for _, path := range []string{binary, output} {
				if data, err := os.ReadFile(path); err != nil || string(data) != "original binary" {
					t.Fatalf("changed %s: %q, %v", path, data, err)
				}
			}
		})
	}
}
