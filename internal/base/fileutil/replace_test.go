package fileutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReplaceRegularFile(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint("existing=", existing), func(t *testing.T) {
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
			writeReplacementFile(t, src, "new", 0o600)
			sourceInfo, err := os.Stat(src)
			if err != nil {
				t.Fatal(err)
			}
			if existing {
				writeReplacementFile(t, dst, "old", 0o600)
			}
			// A read-only requested mode makes the Windows chmod exemption visible:
			// Windows must retain the writable source file's attributes.
			if err := ReplaceRegularFile(src, dst, 0o511); err != nil {
				t.Fatal(err)
			}
			assertReplacementContent(t, dst, "new")
			if _, err := os.Lstat(src); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("source after rename: %v", err)
			}
			info, err := os.Stat(dst)
			if err != nil {
				t.Fatal(err)
			}
			wantMode := os.FileMode(0o511)
			if runtime.GOOS == "windows" {
				wantMode = sourceInfo.Mode().Perm()
			}
			if info.Mode().Perm() != wantMode {
				t.Fatalf("destination mode = %o, want %o", info.Mode().Perm(), wantMode)
			}
		})
	}
}

func TestReplaceRegularFileRejectsDestinationBeforeRenaming(t *testing.T) {
	for _, kind := range []string{"directory", "symlink", "dangling symlink"} {
		for _, missingSource := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/missing-source=%v", kind, missingSource), func(t *testing.T) {
				dir := t.TempDir()
				src, dst, target := filepath.Join(dir, "source"), filepath.Join(dir, "destination"), filepath.Join(dir, "target")
				if !missingSource {
					writeReplacementFile(t, src, "new", 0o600)
				}
				if kind == "directory" {
					if err := os.Mkdir(dst, 0o700); err != nil {
						t.Fatal(err)
					}
				} else {
					if kind == "symlink" {
						writeReplacementFile(t, target, "old", 0o600)
					}
					if err := os.Symlink(target, dst); err != nil {
						t.Skipf("symlinks unavailable: %v", err)
					}
				}
				before, err := os.Lstat(dst)
				if err != nil {
					t.Fatal(err)
				}
				err = ReplaceRegularFile(src, dst, 0o751)
				want := fmt.Sprintf("destination %q is not a regular non-symlink file", dst)
				if err == nil || err.Error() != want {
					t.Fatalf("error = %v, want %q", err, want)
				}
				after, err := os.Lstat(dst)
				if err != nil || !os.SameFile(before, after) {
					t.Fatalf("destination changed: %v", err)
				}
				if !missingSource {
					assertReplacementContent(t, src, "new")
				}
				if kind == "symlink" {
					assertReplacementContent(t, target, "old")
				}
			})
		}
	}
}

func TestReplaceRegularFilePreservesFilesystemErrors(t *testing.T) {
	t.Run("lstat before rename", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "file")
		writeReplacementFile(t, parent, "old", 0o600)
		dst := filepath.Join(parent, "child")
		_, want := os.Lstat(dst)
		if os.IsNotExist(want) {
			t.Skip("host reports non-directory parent as not-exist")
		}
		got := ReplaceRegularFile(filepath.Join(dir, "missing"), dst, 0o751)
		var pathErr *os.PathError
		if !errors.As(got, &pathErr) || got.Error() != want.Error() {
			t.Fatalf("error = %v, want original Lstat error %v", got, want)
		}
		assertReplacementContent(t, parent, "old")
	})
	t.Run("rename before chmod", func(t *testing.T) {
		dir := t.TempDir()
		src, dst := filepath.Join(dir, "missing"), filepath.Join(dir, "destination")
		writeReplacementFile(t, dst, "old", 0o600)
		err := ReplaceRegularFile(src, dst, 0o751)
		var linkErr *os.LinkError
		if !errors.Is(err, os.ErrNotExist) || !errors.As(err, &linkErr) || linkErr.Op != "rename" || linkErr.Old != src || linkErr.New != dst {
			t.Fatalf("error = %v, want rename failure", err)
		}
		assertReplacementContent(t, dst, "old")
		info, err := os.Stat(dst)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatal("chmod ran after failed rename")
		}
	})
	t.Run("chmod after rename", func(t *testing.T) {
		dir := t.TempDir()
		src, dst := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
		// Source validation remains the caller's responsibility. A dangling source
		// symlink exposes the post-rename chmod failure without production hooks.
		if err := os.Symlink(filepath.Join(dir, "missing"), src); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		err := ReplaceRegularFile(src, dst, 0o751)
		if runtime.GOOS == "windows" {
			if err != nil {
				t.Fatalf("Windows must skip chmod: %v", err)
			}
		} else {
			var pathErr *os.PathError
			if !errors.Is(err, os.ErrNotExist) || !errors.As(err, &pathErr) || pathErr.Op != "chmod" || pathErr.Path != dst {
				t.Fatalf("error = %v, want chmod failure", err)
			}
		}
		if _, err := os.Lstat(src); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("rename did not consume source: %v", err)
		}
		if info, err := os.Lstat(dst); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("rename did not precede chmod: %v", err)
		}
	})
}

func writeReplacementFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func assertReplacementContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s = %q, %v; want %q", path, got, err, want)
	}
}
