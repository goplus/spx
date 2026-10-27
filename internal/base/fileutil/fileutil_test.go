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

package fileutil

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type zipEntry struct {
	name    string
	content string
}

func TestCopyFileCreatesParentAndPreservesMode(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", src, err)
	}
	if err := os.Chmod(src, 0o640); err != nil {
		t.Fatalf("Chmod(%s) returned error: %v", src, err)
	}

	dst := filepath.Join(tempDir, "nested", "dst.txt")
	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile(%s, %s) returned error: %v", src, dst, err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", dst, err)
	}
	if string(got) != "payload" {
		t.Fatalf("destination content = %q, want %q", string(got), "payload")
	}
	assertSamePerm(t, src, dst)
}

func TestCopyFileUpdatesExistingDestinationMode(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src.txt")
	dst := filepath.Join(tempDir, "dst.txt")
	if err := os.WriteFile(src, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", src, err)
	}
	if err := os.Chmod(src, 0o644); err != nil {
		t.Fatalf("Chmod(%s) returned error: %v", src, err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", dst, err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile(%s, %s) returned error: %v", src, dst, err)
	}

	assertSamePerm(t, src, dst)
}

func TestCopyDirCopiesNestedFilesAndDirectoryModes(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src")
	nested := filepath.Join(src, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", nested, err)
	}
	if err := os.Chmod(nested, 0o750); err != nil {
		t.Fatalf("Chmod(%s) returned error: %v", nested, err)
	}
	if err := os.WriteFile(filepath.Join(nested, "asset.txt"), []byte("asset"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	dst := filepath.Join(tempDir, "dst")
	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir(%s, %s) returned error: %v", src, dst, err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "nested", "asset.txt"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(got) != "asset" {
		t.Fatalf("copied file content = %q, want %q", string(got), "asset")
	}
	assertSamePerm(t, nested, filepath.Join(dst, "nested"))
}

func TestWriteNamedZipUsesSortedNamesAndContent(t *testing.T) {
	tempDir := t.TempDir()
	first := filepath.Join(tempDir, "first.txt")
	second := filepath.Join(tempDir, "second.txt")
	if err := os.WriteFile(first, []byte("first"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", first, err)
	}
	if err := os.WriteFile(second, []byte("second"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", second, err)
	}

	dst := filepath.Join(tempDir, "out.zip")
	if err := WriteNamedZip(dst, map[string]string{
		"b.txt":   second,
		"a/a.txt": first,
	}); err != nil {
		t.Fatalf("WriteNamedZip(%s) returned error: %v", dst, err)
	}

	entries := readZipEntries(t, dst)
	want := []zipEntry{
		{name: "a/a.txt", content: "first"},
		{name: "b.txt", content: "second"},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("zip entries = %#v, want %#v", entries, want)
	}
}

func TestWriteNamedZipNormalizesMetadata(t *testing.T) {
	tempDir := t.TempDir()
	regular := filepath.Join(tempDir, "regular.txt")
	executable := filepath.Join(tempDir, "executable.sh")
	if err := os.WriteFile(regular, []byte("regular"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", regular, err)
	}
	if err := os.WriteFile(executable, []byte("executable"), 0o700); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", executable, err)
	}
	firstTime := time.Date(2001, time.February, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(regular, firstTime, firstTime); err != nil {
		t.Fatalf("Chtimes(%s) returned error: %v", regular, err)
	}
	if err := os.Chtimes(executable, firstTime, firstTime); err != nil {
		t.Fatalf("Chtimes(%s) returned error: %v", executable, err)
	}

	firstZip := filepath.Join(tempDir, "first.zip")
	files := map[string]string{
		"b.sh":  executable,
		"a.txt": regular,
	}
	if err := WriteNamedZip(firstZip, files); err != nil {
		t.Fatalf("WriteNamedZip(%s) returned error: %v", firstZip, err)
	}

	secondTime := time.Date(2037, time.August, 9, 10, 11, 12, 0, time.UTC)
	if err := os.Chmod(regular, 0o666); err != nil {
		t.Fatalf("Chmod(%s) returned error: %v", regular, err)
	}
	if err := os.Chmod(executable, 0o777); err != nil {
		t.Fatalf("Chmod(%s) returned error: %v", executable, err)
	}
	if err := os.Chtimes(regular, secondTime, secondTime); err != nil {
		t.Fatalf("Chtimes(%s) returned error: %v", regular, err)
	}
	if err := os.Chtimes(executable, secondTime, secondTime); err != nil {
		t.Fatalf("Chtimes(%s) returned error: %v", executable, err)
	}

	secondZip := filepath.Join(tempDir, "second.zip")
	if err := WriteNamedZip(secondZip, files); err != nil {
		t.Fatalf("WriteNamedZip(%s) returned error: %v", secondZip, err)
	}
	firstData, err := os.ReadFile(firstZip)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", firstZip, err)
	}
	secondData, err := os.ReadFile(secondZip)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", secondZip, err)
	}
	if !reflect.DeepEqual(firstData, secondData) {
		t.Fatal("WriteNamedZip output changed after source metadata changed")
	}

	reader, err := zip.OpenReader(secondZip)
	if err != nil {
		t.Fatalf("OpenReader(%s) returned error: %v", secondZip, err)
	}
	defer reader.Close()
	if len(reader.File) != 2 {
		t.Fatalf("zip entry count = %d, want 2", len(reader.File))
	}
	want := []struct {
		name string
		mode os.FileMode
	}{
		{name: "a.txt", mode: 0o644},
		{name: "b.sh", mode: 0o755},
	}
	for index, file := range reader.File {
		if file.Name != want[index].name {
			t.Fatalf("zip entry %d name = %q, want %q", index, file.Name, want[index].name)
		}
		if got := file.Mode().Perm(); got != want[index].mode {
			t.Fatalf("zip entry %q mode = %v, want %v", file.Name, got, want[index].mode)
		}
		if !file.Modified.Equal(normalizedZipModTime) {
			t.Fatalf("zip entry %q timestamp = %s, want %s", file.Name, file.Modified, normalizedZipModTime)
		}
	}
}

func TestZipDirectoryUsesSortedSlashPathsAndContent(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(filepath.Join(src, "dir"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "z.txt"), []byte("z"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "dir", "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	dst := filepath.Join(tempDir, "out.zip")
	if err := ZipDirectory(src, dst); err != nil {
		t.Fatalf("ZipDirectory(%s, %s) returned error: %v", src, dst, err)
	}

	entries := readZipEntries(t, dst)
	want := []zipEntry{
		{name: "dir/a.txt", content: "a"},
		{name: "z.txt", content: "z"},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("zip entries = %#v, want %#v", entries, want)
	}
}

func TestFileOperationsRejectInvalidSourcesAndBlockedDestinations(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	missing := filepath.Join(root, "missing")
	blocked := filepath.Join(source, "child")
	if err := os.WriteFile(source, []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"copy missing":          func() error { return CopyFile(missing, filepath.Join(root, "out")) },
		"copy directory":        func() error { return CopyFile(root, filepath.Join(root, "out")) },
		"copy blocked parent":   func() error { return CopyFile(source, blocked) },
		"copy onto directory":   func() error { return CopyFile(source, root) },
		"tree missing":          func() error { return CopyDir(missing, filepath.Join(root, "copy")) },
		"tree blocked":          func() error { return CopyDir(root, blocked) },
		"archive missing":       func() error { return ZipDirectory(missing, filepath.Join(root, "out.zip")) },
		"archive file":          func() error { return ZipDirectory(source, filepath.Join(root, "out.zip")) },
		"archive bad ancestor":  func() error { return ZipDirectory(blocked, filepath.Join(root, "out.zip")) },
		"archive blocked":       func() error { return ZipDirectory(root, blocked) },
		"named archive blocked": func() error { return WriteNamedZip(blocked, nil) },
		"named archive missing input": func() error {
			return WriteNamedZip(filepath.Join(root, "bad.zip"), map[string]string{"entry": missing})
		},
		"named archive directory input": func() error {
			return WriteNamedZip(filepath.Join(root, "bad-dir.zip"), map[string]string{"entry": root})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("invalid operation succeeded")
			}
			if data, err := os.ReadFile(source); err != nil || string(data) != "preserved" {
				t.Fatalf("source changed=%q,%v", data, err)
			}
		})
	}
}

func TestZipDirectoryPropagatesDanglingSymlinkError(t *testing.T) {
	src := t.TempDir()
	source := filepath.Join(src, "a.txt")
	if err := os.WriteFile(source, []byte("preserved"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(src, "z.txt")
	target := filepath.Join(src, "missing")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := ZipDirectory(src, filepath.Join(t.TempDir(), "out.zip"))
	var pathErr *os.PathError
	if !errors.Is(err, os.ErrNotExist) || !errors.As(err, &pathErr) || pathErr.Path != link {
		t.Fatalf("ZipDirectory error = %v, want missing symlink target for %s", err, link)
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != "preserved" {
		t.Fatalf("source changed: content = %q, error = %v", data, err)
	}
	if got, err := os.Readlink(link); err != nil || got != target {
		t.Fatalf("symlink changed: target = %q, error = %v", got, err)
	}
}

func TestWriteZipFilePropagatesEntryFailure(t *testing.T) {
	dir := t.TempDir()
	failure := errors.New("entry rejected")
	if err := writeZipFile(filepath.Join(dir, "out.zip"), func(*zip.Writer) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("entry error=%v", err)
	}
	if err := writeZipFile(dir, func(*zip.Writer) error { t.Fatal("called entries after open failed"); return nil }); err == nil {
		t.Fatal("directory accepted as output")
	}
	writer := zip.NewWriter(io.Discard)
	src := filepath.Join(dir, "file")
	if err := os.WriteFile(src, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := addFileToZip(writer, src, strings.Repeat("a", 65536)); err == nil {
		t.Fatal("oversized ZIP entry name accepted")
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func readZipEntries(t *testing.T, path string) []zipEntry {
	t.Helper()

	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader(%s) returned error: %v", path, err)
	}
	defer reader.Close()

	entries := make([]zipEntry, 0, len(reader.File))
	for _, file := range reader.File {
		content, err := readZipFile(file)
		if err != nil {
			t.Fatalf("readZipFile(%s) returned error: %v", file.Name, err)
		}
		entries = append(entries, zipEntry{name: file.Name, content: string(content)})
	}
	return entries
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func assertSamePerm(t *testing.T, src, dst string) {
	t.Helper()

	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatalf("Stat(%s) returned error: %v", src, err)
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat(%s) returned error: %v", dst, err)
	}
	if got, want := dstInfo.Mode().Perm(), srcInfo.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %v, want %v from %s", dst, got, want, src)
	}
}
