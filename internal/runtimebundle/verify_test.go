/*
 * Copyright (c) 2021-2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
)

type testZipEntry struct {
	name   string
	mode   fs.FileMode
	data   string
	method uint16
	flags  uint16
}

// zipFailureReader exercises read and close errors through archive/zip's public
// decompressor hook, rather than changing the verifier's implementation.
type zipFailureReader struct {
	io.Reader
	closeErr error
	closes   int
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
		"", "invalid\xff", "nul\x00name", "double//slash",
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
	for _, tt := range []struct {
		name  string
		entry testZipEntry
		want  error
	}{
		{"reserved-complete", testZipEntry{name: completeMarkerName, data: "x"}, ErrUnsafeArchive},
		{"reserved-manifest", testZipEntry{name: cacheManifestName, data: "x"}, ErrUnsafeArchive},
		{"symlink", testZipEntry{name: "link", mode: fs.ModeSymlink | 0o777, data: "target"}, ErrUnsupportedArchiveEntry},
		{"device", testZipEntry{name: "device", mode: fs.ModeDevice | 0o600}, ErrUnsupportedArchiveEntry},
		{"encrypted", testZipEntry{name: "encrypted", flags: 1, data: "trusted"}, ErrUnsafeArchive},
		{"directory mode without directory name", testZipEntry{name: "directory", mode: fs.ModeDir | 0o755}, ErrUnsupportedArchiveEntry},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := VerifyZip(writeTestZip(t, tt.entry)); !errors.Is(err, tt.want) {
				t.Fatalf("VerifyZip = %v, want %v", err, tt.want)
			}
		})
	}
}

// Preserve each archive format's existing Unicode folding policy.
func TestPortableComponentKelvinSignPolicy(t *testing.T) {
	err := validatePortableComponent("CLOCK$")
	if err == nil {
		t.Fatalf("Unicode device-name policy changed: %v", err)
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
	wrongDigest := bundle
	wrongDigest.Digest = strings.Repeat("0", sha256.Size*2)
	invalid := Bundle{Entries: []Entry{{Name: "x", Size: -1}}, Digest: wrongDigest.Digest}
	for _, tt := range []struct {
		name     string
		expected Bundle
		limits   Limits
		want     error
		message  string
	}{
		{name: "matching", expected: bundle},
		{name: "digest mismatch", expected: wrongDigest, want: ErrDigestMismatch},
		{name: "invalid entry before identity comparison", expected: invalid, want: ErrInvalidManifest, message: `"x" has negative size`},
		{name: "negative limits before expected validation", expected: invalid, limits: Limits{MaxEntries: -1}, message: "negative archive limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyZip(path, VerifyOptions{Limits: tt.limits, Expected: &tt.expected})
			if (tt.want != nil || tt.message == "") && !errors.Is(err, tt.want) {
				t.Fatalf("VerifyZip error = %v, want %v", err, tt.want)
			}
			if tt.message != "" && (err == nil || !strings.Contains(err.Error(), tt.message)) {
				t.Fatalf("VerifyZip error = %v, want %q", err, tt.message)
			}
		})
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

func TestPinnedRootRejectsChangedOrClosedPath(t *testing.T) {
	for _, replacement := range []string{"directory", "file", "missing", "closed descriptor"} {
		t.Run(replacement, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "dst")
			if err := os.Mkdir(dst, 0o700); err != nil {
				t.Fatal(err)
			}
			root, err := openPinnedRoot(dst)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if replacement == "closed descriptor" {
				err = root.Close()
			} else {
				if err := os.Rename(dst, dst+".old"); err != nil {
					t.Fatal(err)
				}
				switch replacement {
				case "directory":
					err = os.Mkdir(dst, 0o700)
				case "file":
					err = os.WriteFile(dst, []byte("replacement"), 0o600)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := checkPinnedRootPath(dst, root); !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("changed root check = %v, want ErrUnsafeArchive", err)
			}
		})
	}
}

func TestVerifyZipEnforcesContentLimits(t *testing.T) {
	for _, tt := range []struct {
		name    string
		entries []testZipEntry
		limits  Limits
		message string
	}{
		{"compression ratio", []testZipEntry{{name: "bomb", data: strings.Repeat("A", 16*1024), method: zip.Deflate}}, Limits{MaxCompressionRatio: 2}, "compression ratio"},
		{"entry size", []testZipEntry{{name: "large", data: "123"}}, Limits{MaxEntrySize: 2}, "entry"},
		{"total size", []testZipEntry{{name: "one", data: "123"}, {name: "two", data: "456"}}, Limits{MaxEntrySize: 3, MaxTotalSize: 5}, "total size"},
		{"manifest bytes", []testZipEntry{{name: "entry", data: "x"}}, Limits{MaxManifestBytes: 1}, "manifest size"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyZip(writeTestZip(t, tt.entries...), VerifyOptions{Limits: tt.limits})
			if !errors.Is(err, ErrArchiveLimit) || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("VerifyZip = %v, want ErrArchiveLimit with %q", err, tt.message)
			}
		})
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
	if got := bundle.Identity(); got != wantDigest {
		t.Fatalf("Identity = %q, want %q", got, wantDigest)
	}
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
		{"nonempty directory", Bundle{Entries: []Entry{{Name: "dir/", Mode: uint32(fs.ModeDir), Size: 1, SHA256: entry.SHA256}}}, Limits{}, ErrInvalidManifest, ""},
		{"unsupported mode", Bundle{Entries: []Entry{{Name: "x", Mode: uint32(fs.ModeSymlink), SHA256: entry.SHA256}}}, Limits{}, ErrInvalidManifest, ""},
		{"invalid entry digest", Bundle{Entries: []Entry{{Name: "x", SHA256: "invalid"}}}, Limits{}, ErrInvalidManifest, ""},
		{"invalid entry name", Bundle{Entries: []Entry{{Name: "../x", SHA256: entry.SHA256}}}, Limits{}, ErrInvalidEntryName, ""},
		{"file with directory name", Bundle{Entries: []Entry{{Name: "dir/", SHA256: entry.SHA256}}}, Limits{}, ErrInvalidManifest, ""},
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

func TestZipEntryPointsValidateOptionsAndAgree(t *testing.T) {
	path := writeTestZip(t, testZipEntry{name: "nested/bin/run", mode: 0o755, data: "trusted"})
	want, err := (Bundle{Entries: []Entry{{Name: "nested/bin/run", Mode: 0o755, Size: 7, SHA256: testDigest("trusted")}}}).WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"VerifyZip", "VerifyZipReader", "ExtractZip", "ExtractZipReader"} {
		for _, test := range []string{"valid", "multiple options", "negative limits"} {
			t.Run(operation+"/"+test, func(t *testing.T) {
				dst := filepath.Join(t.TempDir(), "out")
				var options []VerifyOptions
				switch test {
				case "multiple options":
					options = []VerifyOptions{{}, {}}
				case "negative limits":
					options = []VerifyOptions{{Limits: Limits{MaxEntries: -1}}}
				}
				var bundle Bundle
				var err error
				switch operation {
				case "VerifyZip":
					bundle, err = VerifyZip(path, options...)
				case "VerifyZipReader":
					bundle, err = VerifyZipReader(bytes.NewReader(data), int64(len(data)), options...)
				case "ExtractZip":
					bundle, err = ExtractZip(path, dst, options...)
				case "ExtractZipReader":
					bundle, err = ExtractZipReader(bytes.NewReader(data), int64(len(data)), dst, options...)
				}
				if test != "valid" {
					message := "negative archive limit"
					if test == "multiple options" {
						message = operation + " accepts at most one options value"
					}
					if err == nil || !strings.Contains(err.Error(), message) {
						t.Fatalf("error = %v, want %q", err, message)
					}
					if _, err := os.Lstat(dst); !os.IsNotExist(err) {
						t.Fatalf("invalid request created extraction destination: %v", err)
					}
					return
				}
				if err != nil || !reflect.DeepEqual(bundle, want) {
					t.Fatalf("bundle = %+v, %v; want %+v", bundle, err, want)
				}
				if strings.HasPrefix(operation, "Extract") {
					if payload, err := os.ReadFile(filepath.Join(dst, "nested/bin/run")); err != nil || string(payload) != "trusted" {
						t.Fatalf("extracted payload = %q, %v", payload, err)
					}
				}
			})
		}
	}
}

func TestExtractZipRejectsInvalidSourceAndDestination(t *testing.T) {
	path := writeTestZip(t, testZipEntry{name: "bin/run", data: "trusted"})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, fromReader := range []bool{false, true} {
		for _, destination := range []string{"empty", "file", "file parent", "existing output"} {
			t.Run(strconv.FormatBool(fromReader)+"/"+destination, func(t *testing.T) {
				dst := t.TempDir()
				preserved := filepath.Join(dst, "bin")
				switch destination {
				case "empty":
					dst = ""
				case "file":
					dst = filepath.Join(dst, "output")
					preserved = dst
				case "existing output":
					if err := os.Mkdir(preserved, 0o700); err != nil {
						t.Fatal(err)
					}
					preserved = filepath.Join(preserved, "run")
				}
				if err := os.WriteFile(preserved, []byte("untouched"), 0o600); err != nil {
					t.Fatal(err)
				}
				var err error
				if fromReader {
					_, err = ExtractZipReader(bytes.NewReader(data), int64(len(data)), dst)
				} else {
					_, err = ExtractZip(path, dst)
				}
				switch destination {
				case "empty":
					if err == nil || !strings.Contains(err.Error(), "destination is empty") {
						t.Fatalf("empty destination = %v", err)
					}
				case "existing output":
					if !errors.Is(err, os.ErrExist) {
						t.Fatalf("existing output = %v, want os.ErrExist", err)
					}
				default:
					if !errors.Is(err, ErrUnsafeArchive) {
						t.Fatalf("unsafe destination = %v, want ErrUnsafeArchive", err)
					}
				}
				if data, err := os.ReadFile(preserved); err != nil || string(data) != "untouched" {
					t.Fatalf("existing destination changed: %q, %v", data, err)
				}
			})
		}
	}
	if _, err := ExtractZip(filepath.Join(t.TempDir(), "missing"), t.TempDir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing ZIP = %v", err)
	}
}

func TestHashReaderEnforcesSizeAndPreservesReadErrors(t *testing.T) {
	readErr := errors.New("source failed")
	for _, tt := range []struct {
		name     string
		reader   io.Reader
		limit    int64
		wantSize int64
		wantErr  error
	}{
		{"empty", strings.NewReader(""), 0, 0, nil},
		{"exact", strings.NewReader("abc"), 3, 3, nil},
		{"too large", strings.NewReader("abc"), 2, 3, ErrDigestMismatch},
		{"maximum limit", strings.NewReader("abc"), math.MaxInt64, 3, nil},
		{"read failure", io.MultiReader(strings.NewReader("abc"), iotest.ErrReader(readErr)), 4, 3, readErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			digest, size, err := hashReader(tt.reader, tt.limit)
			if size != tt.wantSize || !errors.Is(err, tt.wantErr) {
				t.Fatalf("hashReader = %q, %d, %v; want size %d, error %v", digest, size, err, tt.wantSize, tt.wantErr)
			}
			if tt.wantErr != nil {
				if digest != "" {
					t.Fatalf("failed read returned digest %q", digest)
				}
			} else if digest != testDigest("abc"[:tt.wantSize]) {
				t.Fatalf("digest = %q, want digest of verified bytes", digest)
			}
		})
	}
}

func TestZipReadersPreserveFailuresAndExtractionCleansUp(t *testing.T) {
	const payload = "target"
	zipPath := writeTestZip(t, testZipEntry{name: "asset", data: payload})
	data, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected ZIP stream failure")
	for _, operation := range []string{"hash", "symlink hash", "extract"} {
		for _, fault := range []string{"open", "read", "close"} {
			t.Run(operation+"/"+fault, func(t *testing.T) {
				reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
				if err != nil {
					t.Fatal(err)
				}
				file := reader.File[0]
				file.Method = 99
				stream := &zipFailureReader{Reader: strings.NewReader(payload)}
				want := failure
				switch fault {
				case "open":
					want = zip.ErrAlgorithm
				case "read":
					stream.Reader = iotest.ErrReader(failure)
				case "close":
					stream.closeErr = failure
				}
				if fault != "open" {
					reader.RegisterDecompressor(99, func(io.Reader) io.ReadCloser { return stream })
				}
				switch operation {
				case "hash":
					_, _, err = hashZipEntry(file, 100)
				case "symlink hash":
					_, _, err = hashMaterializedSymlinkEntry(file, "asset")
				case "extract":
					destination := t.TempDir()
					root, openErr := os.OpenRoot(destination)
					if openErr != nil {
						t.Fatal(openErr)
					}
					defer root.Close()
					entry := Entry{Name: "asset", Size: int64(len(payload)), SHA256: testDigest(payload)}
					err = extractVerifiedRoot(verifiedArchive{entries: []verifiedEntry{{entry: entry, file: file}}}, root)
					if fault == "read" {
						want = ErrDigestMismatch
					}
					if _, statErr := root.Lstat("asset"); !errors.Is(statErr, os.ErrNotExist) {
						t.Fatalf("failed extraction left an asset: %v", statErr)
					}
				}
				if !errors.Is(err, want) {
					t.Fatalf("%s = %v, want %v", operation, err, want)
				}
				if fault != "open" && stream.closes != 1 {
					t.Fatalf("ZIP stream closed %d times, want once", stream.closes)
				}
			})
		}
	}
}

func TestVerifyRejectsInvalidZIPDataLocations(t *testing.T) {
	for _, fault := range []string{"missing local header", "nonempty directory", "overlapping entries"} {
		t.Run(fault, func(t *testing.T) {
			entries := []testZipEntry{{name: "a", data: "same"}, {name: "b", data: "same"}}
			if fault == "nonempty directory" {
				entries = []testZipEntry{{name: "directory/"}}
			}
			data, err := os.ReadFile(writeTestZip(t, entries...))
			if err != nil {
				t.Fatal(err)
			}
			central := bytes.Index(data, []byte("PK\x01\x02"))
			if central < 0 {
				t.Fatal("fixture has no central directory")
			}
			switch fault {
			case "missing local header":
				data[0] = 'X'
			case "nonempty directory":
				binary.LittleEndian.PutUint32(data[central+24:], 1)
			case "overlapping entries":
				second := bytes.Index(data[central+4:], []byte("PK\x01\x02")) + central + 4
				if second <= central+4 {
					t.Fatal("fixture has no second central entry")
				}
				copy(data[second+42:second+46], data[central+42:central+46])
			}
			if _, err := VerifyZipReader(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("invalid ZIP location = %v, want ErrUnsafeArchive", err)
			}
		})
	}
	data, err := os.ReadFile(writeTestZip(t, testZipEntry{name: "asset", data: "payload"}))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	file := reader.File[0]
	offset, err := file.DataOffset()
	if err != nil {
		t.Fatal(err)
	}
	for _, archiveSize := range []int64{offset - 1, offset + int64(file.CompressedSize64) - 1} {
		if err := validateZipOffset(file, archiveSize); !errors.Is(err, ErrUnsafeArchive) {
			t.Fatalf("archive boundary %d = %v, want ErrUnsafeArchive", archiveSize, err)
		}
	}
}

func TestEnsureNoSymlinkParentsRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		root   string
		target string
		want   error
	}{
		{"existing nested path", root, filepath.Join(root, "directory"), nil},
		{"missing nested path", root, filepath.Join(root, "missing", "child"), nil},
		{"outside path", root, filepath.Dir(root), ErrUnsafeArchive},
		{"missing root", filepath.Join(root, "absent"), filepath.Join(root, "absent", "child"), os.ErrNotExist},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := ensureNoSymlinkParents(tt.root, tt.target); !errors.Is(err, tt.want) {
				t.Fatalf("ensureNoSymlinkParents = %v, want %v", err, tt.want)
			}
		})
	}
	if err := ensureNoSymlinkParents(root, filepath.Join(root, "file", "child")); err == nil || errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("filesystem traversal error was lost: %v", err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := ensureNoSymlinkParents(root, filepath.Join(root, "link", "child")); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("symlink parent = %v", err)
	}
	if opened, err := openPinnedRoot(filepath.Join(root, "file")); opened != nil || !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("pin regular file = %v, %v", opened, err)
	}
}

func TestManifestCanonicalizationRejectsInvalidInputs(t *testing.T) {
	valid := Bundle{Entries: []Entry{{Name: "asset", Size: 1, SHA256: testDigest("x")}}}
	negative := Limits{MaxEntries: -1}
	for name, operation := range map[string]func() error{
		"canonical":      func() error { _, err := valid.CanonicalBytesWithLimits(negative); return err },
		"parse":          func() error { _, err := ParseManifestWithLimits([]byte(`{}`), negative); return err },
		"compare limits": func() error { return manifestEntriesEqualWithLimits(valid, valid, negative) },
		"invalid left":   func() error { return manifestEntriesEqualWithLimits(Bundle{Schema: "invalid"}, valid, Limits{}) },
		"invalid right":  func() error { return manifestEntriesEqualWithLimits(valid, Bundle{Schema: "invalid"}, Limits{}) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("invalid manifest = %v", err)
			}
		})
	}
	for _, other := range []Bundle{{}, {Namespace: NamespaceEngine, Entries: valid.Entries}} {
		if err := manifestEntriesEqualWithLimits(valid, other, Limits{}); !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("different manifest identity = %v", err)
		}
	}
}

func TestZipPreflightRejectsMalformedAndOverLimitArchives(t *testing.T) {
	data, err := os.ReadFile(writeTestZip(t,
		testZipEntry{name: "one", data: "1"},
		testZipEntry{name: "two", data: "2"},
	))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name                      string
		data                      []byte
		limits                    Limits
		wantPreflight, wantVerify error
	}{
		{"entry limit", data, Limits{MaxEntries: 1}, ErrArchiveLimit, ErrArchiveLimit},
		{"trailing data", append(append([]byte(nil), data...), 0x7f), Limits{}, zip.ErrFormat, ErrUnsafeArchive},
	} {
		t.Run(tt.name, func(t *testing.T) {
			limits, err := tt.limits.withDefaults()
			if err != nil {
				t.Fatal(err)
			}
			if err := preflightZipArchive(bytes.NewReader(tt.data), int64(len(tt.data)), limits); !errors.Is(err, tt.wantPreflight) {
				t.Fatalf("preflightZipArchive = %v, want %v", err, tt.wantPreflight)
			}
			if _, err := VerifyZipReader(bytes.NewReader(tt.data), int64(len(tt.data)), VerifyOptions{Limits: tt.limits}); !errors.Is(err, tt.wantVerify) {
				t.Fatalf("VerifyZipReader = %v, want %v", err, tt.wantVerify)
			}
		})
	}
}

func (r *zipFailureReader) Close() error {
	r.closes++
	return r.closeErr
}

func writeTestZip(t *testing.T, entries ...testZipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.zip")
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name, Method: item.method, Flags: item.flags}
		if item.mode != 0 {
			header.SetMode(item.mode)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(item.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

func testDigest(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}
