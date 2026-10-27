//go:build !js

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

package zip

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

type limitZipEntry struct {
	name   string
	data   string
	method uint16
}

func TestOpenRejectsTooManyZipEntries(t *testing.T) {
	entries := make([]limitZipEntry, MaxZipEntries+1)
	for i := range entries {
		entries[i].name = fmt.Sprintf("entry-%d", i)
	}
	path := makeLimitZip(t, entries...)

	_, err := Open(path)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("Open error = %v, want ErrArchiveLimit", err)
	}
}

func TestOpenRejectsExcessiveZipCompressionRatio(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{
		name:   "repetitive",
		data:   strings.Repeat("a", 65536),
		method: zip.Deflate,
	})

	_, err := Open(path)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("Open error = %v, want ErrArchiveLimit", err)
	}
	if !strings.Contains(err.Error(), "compression ratio") {
		t.Fatalf("Open error = %v, want compression-ratio detail", err)
	}
}

func TestOpenRejectsNonEmptyDirectoryDeclaration(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "dir/", data: "", method: zip.Store})
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	reader.File[0].UncompressedSize64 = 1
	// The changed declaration models a malformed central-directory entry.
	if err := validateZipReader(&reader.Reader); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("validateZipReader error = %v, want ErrArchiveLimit", err)
	}
}

func TestOpenRejectsAmbiguousZipPaths(t *testing.T) {
	tests := []struct {
		name    string
		entries []limitZipEntry
	}{
		{"duplicate", []limitZipEntry{{name: "a"}, {name: "a"}}},
		{"duplicate directory", []limitZipEntry{{name: "a/"}, {name: "a/"}}},
		{"file then directory", []limitZipEntry{{name: "a"}, {name: "a/"}}},
		{"directory then file", []limitZipEntry{{name: "a/"}, {name: "a"}}},
		{"file before child", []limitZipEntry{{name: "a"}, {name: "a-b"}, {name: "a/b"}}},
		{"file after child", []limitZipEntry{{name: "a/b/c"}, {name: "a/b"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Open(makeLimitZip(t, test.entries...))
			if !errors.Is(err, zip.ErrFormat) {
				t.Fatalf("Open error = %v, want archive/zip format error", err)
			}
		})
	}
}

func TestOpenRejectsInvalidZipPaths(t *testing.T) {
	for _, name := range []string{"", "/a", `a\b`, "a//b", "a/./b", "a/../b", "../a", "C:/a", ".", "a//"} {
		t.Run(name, func(t *testing.T) {
			_, err := Open(makeLimitZip(t, limitZipEntry{name: name}))
			if !errors.Is(err, zip.ErrFormat) && !errors.Is(err, zip.ErrInsecurePath) {
				t.Fatalf("Open error = %v, want invalid path error", err)
			}
		})
	}
}

func TestOpenAcceptsUnambiguousZipPaths(t *testing.T) {
	tests := []struct {
		name    string
		entries []limitZipEntry
	}{
		{"directory before child", []limitZipEntry{{name: "a/"}, {name: "a/b", data: "b"}}},
		{"directory after child", []limitZipEntry{{name: "a/b", data: "b"}, {name: "a/"}}},
		{"adjacent prefix", []limitZipEntry{{name: "a"}, {name: "ab/c"}}},
		{"deep path", []limitZipEntry{{name: strings.Repeat("a/", 32_000) + "z"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir, err := Open(makeLimitZip(t, test.entries...))
			if err != nil {
				t.Fatal(err)
			}
			if err := dir.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOpenAndReadZip(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "exact", data: "1234", method: zip.Store})

	dir, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	file, err := dir.Open("exact")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("read exact entry: data=%q err=%v close=%v", data, err, closeErr)
	}
	if string(data) != "1234" {
		t.Fatalf("entry = %q, want 1234", data)
	}
}

func TestOpenReadRejectsEntryThatExceedsDeclaration(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "grow", data: "123456", method: zip.Store})
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	// Mutating the exported header simulates an archive whose declaration was
	// tampered with after parsing. The wrapper must still fail closed.
	reader.File[0].UncompressedSize64 = 4
	zipFS := &FS{Reader: &reader.Reader}
	entry, err := zipFS.Open("grow")
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(entry)
	_ = entry.Close()
	if readErr == nil {
		t.Fatal("reading tampered entry succeeded")
	}
	if len(data) > 4 {
		t.Fatalf("read %d bytes, want at most declared size", len(data))
	}
}

func TestFSNilCloseIsSafe(t *testing.T) {
	var nilFS *FS
	if err := nilFS.Close(); err != nil {
		t.Fatalf("nil FS Close error = %v", err)
	}
	if err := (&FS{}).Close(); err != nil {
		t.Fatalf("empty FS Close error = %v", err)
	}
}

func TestOpenMissingArchiveAndEntry(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.zip")
	if dir, err := Open(missing); dir != nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Open missing archive = (%v, %v), want nil and ErrNotExist", dir, err)
	}
	dir, err := Open(makeLimitZip(t, limitZipEntry{name: "present", data: "contents"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dir.Close() })
	if entry, err := dir.Open("absent"); entry != nil || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("Open missing entry = (%v, %v), want nil and named ErrNotExist", entry, err)
	}
	if err := dir.Close(); err != nil {
		t.Fatal(err)
	}
	if err := dir.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("second Close error = %v, want ErrClosed", err)
	}
}

func TestOpenRejectsMalformedZIP64Entry(t *testing.T) {
	archive := testRemoteZip(t, "entry", "contents")
	central := bytes.Index(archive, []byte{'P', 'K', 1, 2})
	if central < 0 {
		t.Fatal("test archive has no central directory")
	}
	// A ZIP64 compressed-size marker requires a ZIP64 extra field. The directory
	// still fits the preflight bounds, but archive/zip must reject this entry.
	binary.LittleEndian.PutUint32(archive[central+20:central+24], 0xffffffff)
	if err := preflightZipArchive(bytes.NewReader(archive), int64(len(archive))); err != nil {
		t.Fatalf("fixture failed preflight instead of ZIP parsing: %v", err)
	}
	path := filepath.Join(t.TempDir(), "malformed.zip")
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if dir, err := Open(path); dir != nil || !errors.Is(err, zip.ErrFormat) {
		t.Fatalf("Open malformed ZIP64 entry = (%v, %v), want nil and ErrFormat", dir, err)
	}
}

func TestOpenHTTPSDownloadsAndCaches(t *testing.T) {
	archive := testRemoteZip(t, "secure.txt", "secure contents")
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.TLS == nil || r.URL.RequestURI() != "/archive.zip?token=1" {
			t.Errorf("request = %s, TLS = %v", r.URL.RequestURI(), r.TLS != nil)
		}
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	setRemoteHTTPClient(t, server.Client())
	setRemoteCacheBase(t, filepath.Join(t.TempDir(), "cache"))
	raw := strings.TrimPrefix(server.URL, "https://") + "/archive.zip?token=1"
	for range 2 {
		dir, err := OpenHttps(raw)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = dir.Close() })
		if got := readRemoteEntry(t, dir, "secure.txt"); got != "secure contents" {
			t.Errorf("HTTPS archive entry = %q", got)
		}
		if err := dir.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("HTTPS server requests = %d, want 1 after cache hit", got)
	}
	_, key, err := parseRemoteURL(raw, "https://")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(spxBaseDir, "https", key+".zip")); err != nil {
		t.Fatalf("HTTPS archive not cached under its protocol: %v", err)
	}
}

func TestOpenHTTPRejectsInvalidCacheDirectoryBeforeRequest(t *testing.T) {
	setRemoteCacheBase(t, "relative-cache")
	requests := 0
	setRemoteHTTPClient(t, &http.Client{Transport: remoteRoundTripper(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("unexpected request")
	})})
	dir, err := OpenHttp("example.test/archive.zip")
	if dir != nil || err == nil || !strings.Contains(err.Error(), "absolute clean path") || requests != 0 {
		t.Fatalf("OpenHttp = (%v, %v), requests = %d; want invalid cache directory before a request", dir, err, requests)
	}
}

func TestOpenHTTPReplacesCorruptCache(t *testing.T) {
	archive := testRemoteZip(t, "recovered.txt", "recovered")
	setRemoteCacheBase(t, filepath.Join(t.TempDir(), "cache"))
	requests := 0
	setRemoteHTTPClient(t, &http.Client{Transport: remoteRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)), Request: req}, nil
	})})
	raw := "example.test/archive.zip"
	_, key, err := parseRemoteURL(raw, "http://")
	if err != nil {
		t.Fatal(err)
	}
	local, err := remoteCachePath(spxBaseDir, "http://", key)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureCacheDirectories(filepath.Dir(local)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("incomplete download"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, err := OpenHttp(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if got := readRemoteEntry(t, dir, "recovered.txt"); got != "recovered" || requests != 1 {
		t.Fatalf("recovered entry = %q, requests = %d; want recovered and 1", got, requests)
	}
	if got, err := os.ReadFile(local); err != nil || !bytes.Equal(got, archive) {
		t.Fatalf("cache was not replaced by the complete archive: %v", err)
	}
}

func TestRemoteCachePath(t *testing.T) {
	base := t.TempDir()
	key := strings.Repeat("a", 64)
	for _, scheme := range []string{"http://", "https://"} {
		got, err := remoteCachePath(base+string(filepath.Separator)+".", scheme, key)
		want := filepath.Join(base, strings.TrimSuffix(scheme, "://"), key+".zip")
		if err != nil || got != want {
			t.Errorf("remoteCachePath(%q) = (%q, %v), want %q", scheme, got, err, want)
		}
	}
	for _, test := range []struct{ name, base, scheme, key string }{
		{"empty base", "", "http://", key},
		{"relative base", "relative", "http://", key},
		{"short key", base, "http://", key[:63]},
		{"long key", base, "http://", key + "a"},
		{"unsupported protocol", base, "ftp://", key},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := remoteCachePath(test.base, test.scheme, test.key); err == nil || got != "" {
				t.Fatalf("remoteCachePath = (%q, %v), want empty path and error", got, err)
			}
		})
	}
}

func TestEnsureCacheDirectories(t *testing.T) {
	t.Run("private permissions", func(t *testing.T) {
		protocol := filepath.Join(t.TempDir(), "cache", "http")
		if err := os.MkdirAll(protocol, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := ensureCacheDirectories(protocol); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{filepath.Dir(protocol), protocol} {
			info, err := os.Lstat(path)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				t.Fatalf("cache path %q is not a real directory: %v", path, err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
				t.Errorf("cache directory permissions = %o, want 700", info.Mode().Perm())
			}
		}
	})
	t.Run("file in parent path", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(parent, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := ensureCacheDirectories(filepath.Join(parent, "http")); err == nil || !strings.Contains(err.Error(), "create cache directory") {
			t.Fatalf("ensureCacheDirectories = %v, want directory creation error", err)
		}
	})
	for _, symlinkBase := range []bool{false, true} {
		name := "protocol symlink"
		if symlinkBase {
			name = "base symlink"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "target")
			if err := os.Mkdir(target, 0o755); err != nil {
				t.Fatal(err)
			}
			base := filepath.Join(root, "cache")
			link := base
			if !symlinkBase {
				if err := os.Mkdir(base, 0o700); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(base, "http")
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if err := ensureCacheDirectories(filepath.Join(base, "http")); err == nil || !strings.Contains(err.Error(), "not a real directory") {
				t.Fatalf("ensureCacheDirectories = %v, want symlink rejection", err)
			}
		})
	}
}

func TestSaveToRejectsInvalidArguments(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cache", "http", "archive.zip")
	for _, test := range []struct {
		name, path string
		resp       *http.Response
		wantErr    error
		message    string
	}{
		{name: "empty path", message: "must be absolute"},
		{name: "relative path", path: "relative.zip", message: "must be absolute"},
		{name: "nil response", path: cache, message: "nil HTTP response"},
		{name: "bad status", path: cache, resp: &http.Response{StatusCode: http.StatusNotFound}, wantErr: ErrHTTPStatus},
		{name: "missing body", path: cache, resp: &http.Response{StatusCode: http.StatusOK}, message: "no body"},
		{name: "oversized response", path: cache, resp: &http.Response{StatusCode: http.StatusOK, ContentLength: MaxRemoteZipBytes + 1}, wantErr: ErrRemoteSizeLimit},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := saveTo(test.path, test.resp)
			if err == nil || test.wantErr != nil && !errors.Is(err, test.wantErr) || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("saveTo = %v, want %v containing %q", err, test.wantErr, test.message)
			}
			if _, err := os.Stat(filepath.Dir(cache)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid response created cache directory: %v", err)
			}
		})
	}
}

func TestSaveToReadErrorPreservesExistingCacheAndCleansTemporaryFile(t *testing.T) {
	protocol := filepath.Join(t.TempDir(), "cache", "http")
	if err := ensureCacheDirectories(protocol); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(protocol, "archive.zip")
	if err := os.WriteFile(local, []byte("existing cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("download interrupted")
	response := &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Body: io.NopCloser(io.MultiReader(strings.NewReader("partial data"), remoteErrorReader{wantErr}))}
	defer response.Body.Close()
	if err := saveTo(local, response); !errors.Is(err, wantErr) {
		t.Fatalf("saveTo = %v, want wrapped read error", err)
	}
	if got, err := os.ReadFile(local); err != nil || string(got) != "existing cache" {
		t.Fatalf("cache after read error = %q, %v; want existing cache", got, err)
	}
	assertOnlyCacheEntry(t, protocol, "archive.zip")
}

func TestSaveToDirectoryAndPublishErrors(t *testing.T) {
	t.Run("cache directory blocked", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "cache")
		if err := os.WriteFile(base, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		response := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(testRemoteZip(t, "entry", "data")))}
		defer response.Body.Close()
		if err := saveTo(filepath.Join(base, "http", "archive.zip"), response); err == nil || !strings.Contains(err.Error(), "create cache directory") {
			t.Fatalf("saveTo = %v, want cache directory error", err)
		}
	})
	t.Run("nonempty directory target", func(t *testing.T) {
		protocol := filepath.Join(t.TempDir(), "cache", "http")
		local := filepath.Join(protocol, "archive.zip")
		if err := os.MkdirAll(local, 0o700); err != nil {
			t.Fatal(err)
		}
		keep := filepath.Join(local, "keep")
		if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		response := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(testRemoteZip(t, "entry", "data")))}
		defer response.Body.Close()
		if err := saveTo(local, response); err == nil {
			t.Fatal("saveTo replaced a nonempty directory")
		}
		if got, err := os.ReadFile(keep); err != nil || string(got) != "keep" {
			t.Fatalf("directory contents changed: %q, %v", got, err)
		}
		assertOnlyCacheEntry(t, protocol, "archive.zip")
	})
}

func TestPublishCacheFilePreservesTargetOnFailure(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "archive.zip")
	if err := os.WriteFile(local, []byte("existing cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := publishCacheFile(filepath.Join(root, "missing"), local); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("publishCacheFile = %v, want ErrNotExist", err)
	}
	if got, err := os.ReadFile(local); err != nil || string(got) != "existing cache" {
		t.Fatalf("target changed after failed publish: %q, %v", got, err)
	}
}

func TestPublishCacheFileRejectsDirectoryConflict(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	for _, dir := range []string{source, target} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keep := filepath.Join(target, "keep")
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := publishCacheFile(source, target); err == nil {
		t.Fatal("publishCacheFile replaced a nonempty directory")
	}
	if got, err := os.ReadFile(keep); err != nil || string(got) != "keep" {
		t.Fatalf("directory target changed: %q, %v", got, err)
	}
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		t.Fatalf("source directory changed after failure: %v", err)
	}
}

func TestWindowsRenameConflictClassification(t *testing.T) {
	for _, test := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{os.ErrExist, true},
		{&os.LinkError{Op: "rename", Old: "source", New: "target", Err: os.ErrExist}, true},
		{os.ErrNotExist, false},
		{os.ErrPermission, false},
	} {
		if got := isWindowsRenameConflict(test.err); got != test.want {
			t.Errorf("isWindowsRenameConflict(%v) = %v, want %v", test.err, got, test.want)
		}
	}
}

func TestDefaultSPXBaseDir(t *testing.T) {
	t.Run("absolute home", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		if got, want := defaultSPXBaseDir(), filepath.Join(homeDir, ".spx"); got != want {
			t.Fatalf("defaultSPXBaseDir = %q, want %q", got, want)
		}
	})
	t.Run("user cache fallback", func(t *testing.T) {
		t.Setenv("HOME", "")
		cacheDir := t.TempDir()
		t.Setenv("XDG_CACHE_HOME", cacheDir)
		t.Setenv("LocalAppData", cacheDir)
		cache, err := os.UserCacheDir()
		if err != nil || !filepath.IsAbs(cache) {
			t.Skip("platform requires a home directory for its user cache")
		}
		if got, want := defaultSPXBaseDir(), filepath.Join(cache, "spx"); got != want {
			t.Fatalf("defaultSPXBaseDir = %q, want %q", got, want)
		}
	})
	t.Run("temporary fallback", func(t *testing.T) {
		t.Setenv("HOME", "")
		t.Setenv("XDG_CACHE_HOME", "")
		t.Setenv("LocalAppData", "")
		if cache, err := os.UserCacheDir(); err == nil && filepath.IsAbs(cache) {
			t.Skip("platform provides a user cache without environment variables")
		}
		if got, want := defaultSPXBaseDir(), filepath.Join(os.TempDir(), "spx"); got != want {
			t.Fatalf("defaultSPXBaseDir = %q, want %q", got, want)
		}
	})
}

func setRemoteCacheBase(t *testing.T, base string) {
	t.Helper()
	previous := spxBaseDir
	spxBaseDir = base
	t.Cleanup(func() { spxBaseDir = previous })
}

func assertOnlyCacheEntry(t *testing.T, path, name string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		t.Fatalf("cache entries = %v, want only %q and no temporary files", entries, name)
	}
}

func makeLimitZip(t *testing.T, entries ...limitZipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name, Method: item.method}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, item.data); err != nil {
			file.Close()
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
