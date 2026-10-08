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
	"reflect"
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

func TestManifestAndZipEntryConflicts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		entries []testZipEntry
		problem string
	}{
		{"valid", []testZipEntry{{name: "a/", mode: fs.ModeDir}, {name: "a/b", data: "x"}}, ""},
		{"valid-directory-after-child", []testZipEntry{{name: "a/b", data: "x"}, {name: "a/", mode: fs.ModeDir}}, ""},
		{"duplicate", []testZipEntry{{name: "a", data: "1"}, {name: "a", data: "2"}}, `duplicate entry "a"`},
		{"duplicate-directory", []testZipEntry{{name: "a/", mode: fs.ModeDir}, {name: "a/", mode: fs.ModeDir}}, `duplicate entry "a/"`},
		{"case-fold", []testZipEntry{{name: "A", data: "1"}, {name: "a", data: "2"}}, `case-fold/normalization collision between "A" and "a"`},
		{"unicode-normalization", []testZipEntry{{name: "café", data: "1"}, {name: "cafe\u0301", data: "2"}}, "case-fold/normalization collision between \"café\" and \"cafe\u0301\""},
		{"file-directory-alias", []testZipEntry{{name: "a", data: "1"}, {name: "a/", mode: fs.ModeDir}}, `case-fold/normalization collision between "a" and "a/"`},
		{"directory-file-alias", []testZipEntry{{name: "a/", mode: fs.ModeDir}, {name: "a", data: "1"}}, `case-fold/normalization collision between "a/" and "a"`},
		{"file-parent", []testZipEntry{{name: "a", data: "1"}, {name: "a/b", data: "2"}}, `file "a" is also a parent of "a/b"`},
		{"file-after-child", []testZipEntry{{name: "a/b", data: "2"}, {name: "a", data: "1"}}, `file "a" is also a parent of "a/b"`},
		{"folded-parent", []testZipEntry{{name: "A", data: "1"}, {name: "a/b/c", data: "2"}}, `file "A" is also a parent of "a/b/c"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bundle := Bundle{}
			for _, item := range tt.entries {
				bundle.Entries = append(bundle.Entries, Entry{
					Name: item.name, Mode: uint32(item.mode), Size: int64(len(item.data)), SHA256: testDigest(item.data),
				})
			}
			manifest, err := json.Marshal(bundle)
			if err != nil {
				t.Fatal(err)
			}
			_, manifestErr := ParseManifest(manifest)
			_, zipErr := VerifyZip(writeTestZip(t, tt.entries...))
			for input, err := range map[string]error{"manifest": manifestErr, "zip": zipErr} {
				if tt.problem == "" {
					if err != nil {
						t.Fatalf("%s: %v", input, err)
					}
				} else if !errors.Is(err, ErrUnsafeArchive) || !strings.HasSuffix(err.Error(), ErrUnsafeArchive.Error()+": "+tt.problem) {
					t.Fatalf("%s: error = %v, want unsafe archive: %s", input, err, tt.problem)
				}
			}
		})
	}
}

func TestVerifyZipRejectsDuplicateBeforeModeAndContent(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mode    fs.FileMode
		corrupt bool
	}{
		{name: "mode", mode: fs.ModeSymlink | 0o777},
		{name: "content", corrupt: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestZip(t, testZipEntry{name: "a", data: "first"}, testZipEntry{name: "a", mode: tt.mode, data: "second"})
			if tt.corrupt {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				offset := bytes.Index(data, []byte("second"))
				if offset < 0 {
					t.Fatal("missing stored payload")
				}
				data[offset] ^= 1
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := VerifyZip(path)
			if !errors.Is(err, ErrUnsafeArchive) || !strings.HasSuffix(err.Error(), `duplicate entry "a"`) {
				t.Fatalf("error = %v, want duplicate before mode/content validation", err)
			}
		})
	}
}

func TestVerifyZipRejectsReservedNamesAndSpecialFiles(t *testing.T) {
	tests := []struct {
		name    string
		entries []testZipEntry
	}{
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

func TestManifestMethodsEnforceSerializedLimit(t *testing.T) {
	bundle, err := (Bundle{Entries: []Entry{{
		Name:   "runtime",
		SHA256: testDigest("runtime"),
	}}}).WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	limits := Limits{MaxManifestBytes: 1}
	if _, err := ParseManifestWithLimits(data, limits); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("ParseManifestWithLimits error = %v, want ErrArchiveLimit", err)
	}
	if _, err := bundle.CanonicalBytesWithLimits(limits); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("CanonicalBytesWithLimits error = %v, want ErrArchiveLimit", err)
	}
}

func TestManifestCanonicalGolden(t *testing.T) {
	const want = `{"schema":"runtimebundle/v1","namespace":"engine","entries":[{"name":"a/","mode":2147484141,"size":0,"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},{"name":"cafe` + "\u0301" + `","mode":384,"size":0,"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},{"name":"z.bin","mode":420,"size":3,"sha256":"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"}]}`
	const wantDigest = "a35f70020c5b539bc58dfb6a6be7e9a28f0228de3a907ff685491599a8d7d4d9"
	bundle := Bundle{Namespace: NamespaceEngine, Entries: []Entry{
		{Name: "z.bin", Mode: 0o644, Size: 3, SHA256: testDigest("abc")},
		{Name: "cafe\u0301", Mode: 0o600, SHA256: testDigest("")},
		{Name: "a/", Mode: uint32(fs.ModeDir | 0o755), SHA256: testDigest("")},
	}}
	originalEntries := append([]Entry(nil), bundle.Entries...)
	for _, suppliedDigest := range []string{"", "not-a-digest", strings.Repeat("0", 64), wantDigest} {
		bundle.Digest = suppliedDigest
		data, err := bundle.CanonicalBytes()
		if err != nil || string(data) != want {
			t.Fatalf("CanonicalBytes with digest %q = %s, %v; want %s", suppliedDigest, data, err, want)
		}
		digest, err := bundle.IdentityDigest()
		if err != nil || digest != wantDigest {
			t.Fatalf("IdentityDigest = %q, %v; want %q", digest, err, wantDigest)
		}
		withDigest, err := bundle.WithDigest()
		if err != nil || withDigest.Schema != SchemaV1 || withDigest.Digest != wantDigest {
			t.Fatalf("WithDigest = %#v, %v", withDigest, err)
		}
		if !reflect.DeepEqual(withDigest.Entries, originalEntries) || !reflect.DeepEqual(bundle.Entries, originalEntries) {
			t.Fatal("digest methods changed caller entry order or names")
		}
		if err := withDigest.Validate(); err != nil {
			t.Fatalf("Validate with identity digest: %v", err)
		}
	}
}

func TestManifestEmptyEntries(t *testing.T) {
	for _, entries := range [][]Entry{nil, {}} {
		bundle := Bundle{Entries: entries}
		data, err := bundle.CanonicalBytes()
		if err != nil || string(data) != `{"schema":"runtimebundle/v1","entries":[]}` {
			t.Fatalf("CanonicalBytes = %s, %v", data, err)
		}
		withDigest, err := bundle.WithDigest()
		if err != nil || !reflect.DeepEqual(withDigest.Entries, entries) {
			t.Fatalf("WithDigest changed nil/empty entries: %#v, %v", withDigest, err)
		}
		encoded, err := json.Marshal(withDigest)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseManifest(encoded)
		if err != nil || !reflect.DeepEqual(parsed, withDigest) {
			t.Fatalf("ParseManifest = %#v, %v; want %#v", parsed, err, withDigest)
		}
	}
}

func TestManifestValidationErrorOrder(t *testing.T) {
	entry := Entry{Name: "a", Size: 1, SHA256: testDigest("a")}
	wrongDigest := Bundle{Entries: []Entry{entry}, Digest: strings.Repeat("0", 64)}
	digest, err := wrongDigest.IdentityDigest()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		bundle  Bundle
		limits  Limits
		want    error
		message string
	}{
		{"limits-before-schema", Bundle{Schema: "bad"}, Limits{MaxEntries: -1}, ErrInvalidManifest, "runtimebundle: negative archive limit"},
		{"schema-before-namespace", Bundle{Schema: "bad", Namespace: "bad"}, Limits{}, ErrInvalidManifest, `unsupported schema "bad"`},
		{"namespace-before-count", Bundle{Namespace: "bad", Entries: []Entry{entry, entry}}, Limits{MaxEntries: 1}, ErrInvalidManifest, `unsupported namespace "bad"`},
		{"count-before-entry", Bundle{Entries: []Entry{{}, {}}}, Limits{MaxEntries: 1}, ErrArchiveLimit, "2 entries exceeds limit 1"},
		{"entry-before-digest", Bundle{Entries: []Entry{{Name: "a", Size: -1}}, Digest: "bad"}, Limits{}, ErrInvalidManifest, `"a" has negative size`},
		{"entry-before-collision", Bundle{Entries: []Entry{entry, {Name: "a", Size: -1}}}, Limits{}, ErrInvalidManifest, `"a" has negative size`},
		{"size-before-collision", Bundle{Entries: []Entry{entry, {Name: "a", Size: 2, SHA256: entry.SHA256}}}, Limits{MaxEntrySize: 1}, ErrArchiveLimit, `entry "a" size 2 exceeds limit 1`},
		{"total-before-collision", Bundle{Entries: []Entry{entry, entry}}, Limits{MaxEntrySize: 1, MaxTotalSize: 1}, ErrArchiveLimit, "total size exceeds limit 1"},
		{"collision-before-parent", Bundle{Entries: []Entry{entry, {Name: "a/b", SHA256: entry.SHA256}, entry}}, Limits{}, ErrUnsafeArchive, `duplicate entry "a"`},
		{"parent-before-digest", Bundle{Entries: []Entry{entry, {Name: "a/b", SHA256: entry.SHA256}}, Digest: "bad"}, Limits{}, ErrUnsafeArchive, `file "a" is also a parent of "a/b"`},
		{"digest-before-serialized-limit", Bundle{Entries: []Entry{entry}, Digest: "bad"}, Limits{MaxManifestBytes: 1}, ErrInvalidManifest, "bundle digest: sha256 must be a full 64-hex-character digest"},
		{"unsigned-manifest-skips-serialized-limit", Bundle{Entries: []Entry{entry}}, Limits{MaxManifestBytes: 1}, nil, ""},
		{"serialized-limit-before-digest-mismatch", wrongDigest, Limits{MaxManifestBytes: 1}, ErrArchiveLimit, ""},
		{"digest-mismatch", wrongDigest, Limits{}, ErrDigestMismatch, "manifest digest " + wrongDigest.Digest + " does not match identity " + digest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bundle.ValidateWithLimits(tt.limits)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateWithLimits error = %v; want %v", err, tt.want)
			}
			if tt.message != "" && err.Error() != tt.want.Error()+": "+tt.message {
				t.Fatalf("ValidateWithLimits error = %v; want %v: %s", err, tt.want, tt.message)
			}
		})
	}
}

func BenchmarkManifest(b *testing.B) {
	for _, count := range []int{1, 1000, 10000} {
		bundle := Bundle{Namespace: NamespaceEngine, Entries: make([]Entry, count)}
		for i := range bundle.Entries {
			bundle.Entries[i] = Entry{Name: "runtime/entry-" + strconv.Itoa(count-i), SHA256: testDigest("")}
		}
		withDigest, err := bundle.WithDigest()
		if err != nil {
			b.Fatal(err)
		}
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			b.Run("canonical", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := bundle.CanonicalBytes(); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("validate", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if err := bundle.Validate(); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("validate-digest", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if err := withDigest.Validate(); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func TestLimitsWithDefaults(t *testing.T) {
	defaults := Limits{
		MaxEntries: MaxEntries, MaxEntrySize: MaxEntrySize, MaxTotalSize: MaxTotalSize,
		MaxArchiveBytes: MaxArchiveBytes, MaxCentralDirectoryBytes: MaxCentralDirectoryBytes,
		MaxManifestBytes: MaxManifestBytes, MaxCompressionRatio: MaxCompressionRatio,
	}
	if MaxEntries <= 0 || MaxEntrySize <= 0 || MaxTotalSize <= 0 || MaxArchiveBytes <= 0 ||
		MaxCentralDirectoryBytes <= 0 || MaxManifestBytes <= 0 || MaxCompressionRatio == 0 {
		t.Fatal("default archive limits must be positive")
	}
	custom := Limits{
		MaxEntries: 1, MaxEntrySize: 2, MaxTotalSize: 3,
		MaxArchiveBytes: 4, MaxCentralDirectoryBytes: 5,
		MaxManifestBytes: 6, MaxCompressionRatio: 7,
	}
	mixed := defaults
	mixed.MaxEntries, mixed.MaxCompressionRatio = 1, 1
	equalSizes := defaults
	equalSizes.MaxEntrySize = MaxTotalSize
	const negative = "runtimebundle: negative archive limit"
	for _, tt := range []struct {
		name    string
		input   Limits
		want    Limits
		wantErr string
	}{
		{"all defaults", Limits{}, defaults, ""},
		{"all positive", custom, custom, ""},
		{"mixed defaults", Limits{MaxEntries: 1, MaxCompressionRatio: 1}, mixed, ""},
		{"equal entry and total size", Limits{MaxEntrySize: MaxTotalSize}, equalSizes, ""},
		{"negative entries", Limits{MaxEntries: -1}, Limits{}, negative},
		{"negative entry size", Limits{MaxEntrySize: -1}, Limits{}, negative},
		{"negative total size", Limits{MaxTotalSize: -1}, Limits{}, negative},
		{"negative archive bytes", Limits{MaxArchiveBytes: -1}, Limits{}, negative},
		{"negative directory bytes", Limits{MaxCentralDirectoryBytes: -1}, Limits{}, negative},
		{"negative manifest bytes", Limits{MaxManifestBytes: -1}, Limits{}, negative},
		{"entry exceeds total", Limits{MaxEntrySize: 2, MaxTotalSize: 1}, Limits{}, "runtimebundle: max entry size 2 exceeds max total size 1"},
		{"negative error takes precedence", Limits{MaxEntries: -1, MaxEntrySize: 2, MaxTotalSize: 1}, Limits{}, negative},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.withDefaults()
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr || got != tt.want {
					t.Fatalf("withDefaults() = (%+v, %v), want (%+v, %q)", got, err, tt.want, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("withDefaults() = (%+v, %v), want (%+v, nil)", got, err, tt.want)
			}
			again, err := got.withDefaults()
			if err != nil || again != got {
				t.Fatalf("withDefaults() not idempotent: (%+v, %v)", again, err)
			}
		})
	}
}
