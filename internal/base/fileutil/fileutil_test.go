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
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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

type zipEntry struct {
	name    string
	content string
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
