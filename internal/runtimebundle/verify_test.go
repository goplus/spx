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

package runtimebundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type testZipEntry struct {
	name   string
	mode   fs.FileMode
	data   string
	method uint16
}

func writeTestZip(t *testing.T, entries ...testZipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name, Method: item.method}
		if header.Method == 0 {
			header.Method = zip.Store
		}
		if item.mode != 0 {
			header.SetMode(item.mode)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(item.data)); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func testDigest(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func TestVerifyZipBuildsFullManifestAndRejectsUnsafeNames(t *testing.T) {
	good := writeTestZip(t,
		testZipEntry{name: "bin/run", mode: 0o755, data: "run"},
		testZipEntry{name: "assets/", mode: fs.ModeDir | 0o755},
		testZipEntry{name: "assets/a.txt", mode: 0o644, data: "hello"},
	)
	bundle, err := VerifyZip(good)
	if err != nil {
		t.Fatalf("VerifyZip(good): %v", err)
	}
	if len(bundle.Digest) != sha256.Size*2 || len(bundle.Entries) != 3 {
		t.Fatalf("manifest identity or entries invalid: %#v", bundle)
	}
	for _, entry := range bundle.Entries {
		if len(entry.SHA256) != sha256.Size*2 {
			t.Fatalf("entry %q digest = %q", entry.Name, entry.SHA256)
		}
	}

	unsafe := []string{
		"../escape", "/absolute", `a\b`, "a/./b", "a/../b", "foo:bar",
		"CON.txt", "CLOCK$.txt", "CONIN$.txt", "COM¹.txt", "bad<name",
		"bad\x1fname", "file. ",
	}
	for _, name := range unsafe {
		t.Run(name, func(t *testing.T) {
			_, err := VerifyZip(writeTestZip(t, testZipEntry{name: name, data: "x"}))
			if !errors.Is(err, ErrInvalidEntryName) {
				t.Fatalf("VerifyZip(%q) error = %v", name, err)
			}
		})
	}
}

func TestVerifyZipRejectsDuplicatesCollisionsAndSpecialFiles(t *testing.T) {
	tests := []struct {
		name    string
		entries []testZipEntry
	}{
		{"duplicate", []testZipEntry{{name: "a", data: "1"}, {name: "a", data: "2"}}},
		{"case-fold", []testZipEntry{{name: "A", data: "1"}, {name: "a", data: "2"}}},
		{"unicode-normalization", []testZipEntry{{name: "caf\u00e9", data: "1"}, {name: "cafe\u0301", data: "2"}}},
		{"file-directory-alias", []testZipEntry{{name: "a", data: "1"}, {name: "a/", mode: fs.ModeDir}}},
		{"file-parent", []testZipEntry{{name: "a", data: "1"}, {name: "a/b", data: "2"}}},
		{"reserved-complete", []testZipEntry{{name: completeMarkerName, data: "x"}}},
		{"reserved-manifest", []testZipEntry{{name: cacheManifestName, data: "x"}}},
		{"symlink", []testZipEntry{{name: "link", mode: fs.ModeSymlink | 0o777, data: "target"}}},
		{"device", []testZipEntry{{name: "device", mode: fs.ModeDevice | 0o600}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := VerifyZip(writeTestZip(t, test.entries...)); err == nil {
				t.Fatal("VerifyZip accepted unsafe archive")
			}
		})
	}
}

func TestVerifyZipMaterializedSymlinkRejectsSpecialPermissions(t *testing.T) {
	archive := writeTestZip(t, testZipEntry{
		name: "link",
		mode: fs.ModeSymlink | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky | 0o777,
		data: "target",
	})
	_, err := VerifyZip(archive, VerifyOptions{MaterializeSymlinksAsFiles: true})
	if !errors.Is(err, ErrUnsupportedArchiveEntry) {
		t.Fatalf("VerifyZip error = %v, want ErrUnsupportedArchiveEntry", err)
	}
}

func TestExtractZipMaterializesVettedSymlinkAsFile(t *testing.T) {
	archive := writeTestZip(t, testZipEntry{
		name: "lib64/libc++.so",
		mode: fs.ModeSymlink | 0o777,
		data: "../lib/libc++.so",
	})
	if _, err := VerifyZip(archive); !errors.Is(err, ErrUnsupportedArchiveEntry) {
		t.Fatalf("default VerifyZip error = %v, want ErrUnsupportedArchiveEntry", err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	bundle, err := ExtractZip(archive, dst, VerifyOptions{MaterializeSymlinksAsFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Entries) != 1 || bundle.Entries[0].Mode&uint32(fs.ModeType) != 0 {
		t.Fatalf("materialized manifest entry = %#v, want one regular file", bundle.Entries)
	}
	path := filepath.Join(dst, "lib64", "libc++.so")
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
		t.Fatalf("materialized mode = %v, want regular non-symlink", info.Mode())
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "../lib/libc++.so" {
		t.Fatalf("materialized target = %q, err = %v", data, err)
	}
}

func TestVerifyZipMaterializedSymlinkValidatesTarget(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   error
	}{
		{name: "too large", target: strings.Repeat("a", int(maxMaterializedSymlinkBytes)+1), want: ErrArchiveLimit},
		{name: "empty", target: "", want: ErrUnsafeArchive},
		{name: "non UTF-8", target: string([]byte{0xff}), want: ErrUnsafeArchive},
		{name: "NUL", target: "target\x00suffix", want: ErrUnsafeArchive},
		{name: "absolute", target: "/outside", want: ErrUnsafeArchive},
		{name: "network absolute", target: "//server/share", want: ErrUnsafeArchive},
		{name: "drive absolute", target: "C:/outside", want: ErrUnsafeArchive},
		{name: "backslash", target: `..\outside`, want: ErrUnsafeArchive},
		{name: "root escape", target: "../../outside", want: ErrUnsafeArchive},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := writeTestZip(t, testZipEntry{
				name: "dir/link",
				mode: fs.ModeSymlink | 0o777,
				data: test.target,
			})
			_, err := VerifyZip(archive, VerifyOptions{MaterializeSymlinksAsFiles: true})
			if !errors.Is(err, test.want) {
				t.Fatalf("VerifyZip error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestParseManifestIsStrict(t *testing.T) {
	digest := testDigest("x")
	valid := `{"schema":"runtimebundle/v1","entries":[{"name":"x","mode":420,"size":1,"sha256":"` + digest + `"}]}`
	if _, err := ParseManifest([]byte(valid)); err != nil {
		t.Fatalf("ParseManifest(valid): %v", err)
	}
	for _, data := range []string{valid + " {}", `{"entries":[],"entries":[]}`, `{"entries":[],"extra":1}`, `null`} {
		if _, err := ParseManifest([]byte(data)); err == nil {
			t.Fatalf("ParseManifest accepted invalid JSON %q", data)
		}
	}
	emptyDigest := testDigest("")
	dirMode := strconv.FormatUint(uint64(fs.ModeDir), 10)
	bad := `{"entries":[{"name":"dir/","mode":` + dirMode + `,"size":0,"sha256":"` + digest + `"}]}`
	if _, err := ParseManifest([]byte(bad)); err == nil {
		t.Fatal("ParseManifest accepted a directory with non-empty digest")
	}
	good := `{"entries":[{"name":"dir/","mode":` + dirMode + `,"size":0,"sha256":"` + emptyDigest + `"}]}`
	if _, err := ParseManifest([]byte(good)); err != nil {
		t.Fatalf("ParseManifest rejected valid directory: %v", err)
	}
}

func TestVerifyZipRejectsNilReaderAndArchiveByteLimit(t *testing.T) {
	var reader *bytes.Reader
	if _, err := VerifyZipReader(reader, 0); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("nil ReaderAt error = %v", err)
	}
	path := writeTestZip(t, testZipEntry{name: "x", data: "payload"})
	if _, err := VerifyZip(path, VerifyOptions{Limits: Limits{MaxArchiveBytes: 1}}); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("archive byte limit error = %v", err)
	}
}

func TestExtractZipReaderUsesOpenedArchiveAfterPathReplacement(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", mode: 0o755, data: "trusted"})
	file, err := os.Open(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(zipPath, zipPath+".opened"); err != nil {
		t.Skipf("cannot replace an open ZIP: %v", err)
	}
	if err := os.WriteFile(zipPath, []byte("replacement is not a ZIP"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out")
	if _, err := ExtractZipReader(file, info.Size(), dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "runtime")); err != nil || string(got) != "trusted" {
		t.Fatalf("extracted runtime = %q, err=%v", got, err)
	}
}

func TestVerifyZipExpectedIdentityIsStrict(t *testing.T) {
	path := writeTestZip(t, testZipEntry{name: "x", data: "payload"})
	bundle, err := VerifyZip(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyZip(path, VerifyOptions{Expected: &bundle}); err != nil {
		t.Fatal(err)
	}
	bundle.Digest = strings.Repeat("0", sha256.Size*2)
	if _, err := VerifyZip(path, VerifyOptions{Expected: &bundle}); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("digest error = %v", err)
	}
}

func TestExtractVerifiedDetectsInPlaceSourceMutation(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", mode: 0o755, data: "trusted"})
	file, err := openSourceZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	archive, err := verifyReaderAt(file, info.Size(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	index := bytes.Index(data, []byte("trusted"))
	if index < 0 {
		t.Fatal("test ZIP does not contain stored payload")
	}
	writer, err := os.OpenFile(zipPath, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteAt([]byte("changed"), int64(index)); err != nil {
		_ = writer.Close()
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := extractVerified(archive, dst); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("mutation error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "runtime")); !os.IsNotExist(err) {
		t.Fatalf("mutated output exists: %v", err)
	}
}

func TestVerifyZipRejectsSourceSymlink(t *testing.T) {
	realZip := writeTestZip(t, testZipEntry{name: "x", data: "x"})
	link := filepath.Join(t.TempDir(), "bundle.zip")
	if err := os.Symlink(realZip, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := VerifyZip(link); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("source symlink error = %v", err)
	}
}

func TestExtractZipRejectsDestinationSymlink(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "bin/run", mode: 0o755, data: "run"})
	dst := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dst, "bin")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ExtractZip(zipPath, dst); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("destination symlink error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "run")); !os.IsNotExist(err) {
		t.Fatalf("symlink target modified: %v", err)
	}
}

func TestPinnedRootRejectsPathReplacement(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := openPinnedRoot(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(dst, dst+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkPinnedRootPath(dst, root); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("replaced root check = %v", err)
	}
}

func TestVerifyZipEnforcesCompressionRatio(t *testing.T) {
	path := writeTestZip(t, testZipEntry{name: "bomb", data: strings.Repeat("A", 16*1024), method: zip.Deflate})
	_, err := VerifyZip(path, VerifyOptions{Limits: Limits{MaxCompressionRatio: 2}})
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("compression ratio error = %v", err)
	}
}

func TestVerifyZipPreservesCallerMaxEntriesForDigest(t *testing.T) {
	entries := make([]testZipEntry, MaxEntries+1)
	for i := range entries {
		entries[i] = testZipEntry{name: "entry-" + strconv.Itoa(i)}
	}
	limits := Limits{MaxEntries: len(entries)}
	archive := writeTestZip(t, entries...)
	bundle, err := VerifyZip(archive, VerifyOptions{Limits: limits})
	if err != nil {
		t.Fatalf("VerifyZip with caller MaxEntries returned error: %v", err)
	}
	if len(bundle.Entries) != len(entries) {
		t.Fatalf("manifest entry count = %d, want %d", len(bundle.Entries), len(entries))
	}
	if err := bundle.ValidateWithLimits(limits); err != nil {
		t.Fatalf("caller-limited manifest digest failed validation: %v", err)
	}
	if _, err := VerifyZip(archive, VerifyOptions{Limits: limits, Expected: &bundle}); err != nil {
		t.Fatalf("VerifyZip with caller-limited expected manifest returned error: %v", err)
	}
}

func TestBundleDigestMethodsPreserveCallerLimits(t *testing.T) {
	emptyDigest := testDigest("")
	entryLimited := Bundle{Entries: make([]Entry, MaxEntries+1)}
	for i := range entryLimited.Entries {
		entryLimited.Entries[i] = Entry{
			Name:   "entry-" + strconv.Itoa(i),
			SHA256: emptyDigest,
		}
	}
	largeSize := MaxTotalSize + 1
	totalLimited := Bundle{Entries: []Entry{{
		Name:   "large",
		Size:   largeSize,
		SHA256: emptyDigest,
	}}}

	tests := []struct {
		name   string
		bundle Bundle
		limits Limits
	}{
		{name: "entries", bundle: entryLimited, limits: Limits{MaxEntries: MaxEntries + 1}},
		{name: "total bytes", bundle: totalLimited, limits: Limits{MaxEntrySize: largeSize, MaxTotalSize: largeSize}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.bundle.WithDigest(); !errors.Is(err, ErrArchiveLimit) {
				t.Fatalf("default WithDigest error = %v, want ErrArchiveLimit", err)
			}
			withDigest, err := test.bundle.WithDigestWithLimits(test.limits)
			if err != nil {
				t.Fatalf("WithDigestWithLimits returned error: %v", err)
			}
			if err := withDigest.ValidateWithLimits(test.limits); err != nil {
				t.Fatalf("ValidateWithLimits rejected caller-limited digest: %v", err)
			}
			data, err := json.Marshal(withDigest)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseManifest(data); !errors.Is(err, ErrArchiveLimit) {
				t.Fatalf("default ParseManifest error = %v, want ErrArchiveLimit", err)
			}
			if _, err := ParseManifestWithLimits(data, test.limits); err != nil {
				t.Fatalf("ParseManifestWithLimits returned error: %v", err)
			}
		})
	}
}
