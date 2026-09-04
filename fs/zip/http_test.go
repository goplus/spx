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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestOpenHTTPRejectsPathTraversalBeforeRequest(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	victim := filepath.Join(root, "victim.zip")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	oldBase := spxBaseDir
	spxBaseDir = cache
	t.Cleanup(func() { spxBaseDir = oldBase })

	_, err := OpenHttp(server.Listener.Addr().String() + "/../victim.zip")
	if !errors.Is(err, ErrInvalidRemoteURL) {
		t.Fatalf("OpenHttp error = %v, want ErrInvalidRemoteURL", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("server requests = %d, want 0", got)
	}
	if got, readErr := os.ReadFile(victim); readErr != nil || string(got) != "keep" {
		t.Fatalf("victim changed: %q, err=%v", got, readErr)
	}
}

func TestOpenHTTPRejectsEmbeddedAbsoluteURL(t *testing.T) {
	for _, raw := range []string{
		"https://example.test/archive.zip",
		"//example.test/archive.zip",
		"example.test/../../archive.zip",
		"example.test/%2e%2e/archive.zip",
		"example.test\\archive.zip",
	} {
		if _, _, err := parseRemoteURL(raw, "http://"); !errors.Is(err, ErrInvalidRemoteURL) {
			t.Errorf("parseRemoteURL(%q) error = %v, want ErrInvalidRemoteURL", raw, err)
		}
	}
}

func TestOpenHTTPStatusDoesNotPopulateCache(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	oldBase := spxBaseDir
	spxBaseDir = filepath.Join(root, "cache")
	t.Cleanup(func() { spxBaseDir = oldBase })

	_, err := OpenHttp(server.Listener.Addr().String() + "/missing.zip")
	if !errors.Is(err, ErrHTTPStatus) {
		t.Fatalf("OpenHttp error = %v, want ErrHTTPStatus", err)
	}
	if _, statErr := os.Stat(spxBaseDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("cache root stat error = %v, want not exist", statErr)
	}
}

func TestOpenHTTPInvalidZipDoesNotPublish(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "this is not a zip")
	}))
	defer server.Close()
	oldBase := spxBaseDir
	spxBaseDir = filepath.Join(root, "cache")
	t.Cleanup(func() { spxBaseDir = oldBase })

	_, err := OpenHttp(server.Listener.Addr().String() + "/invalid.zip")
	if err == nil || !strings.Contains(err.Error(), "valid ZIP") {
		t.Fatalf("OpenHttp error = %v, want invalid ZIP error", err)
	}
	entries, readErr := os.ReadDir(filepath.Join(spxBaseDir, "http"))
	if readErr != nil {
		t.Fatalf("ReadDir cache = %v", readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".zip-cache-") || strings.HasSuffix(entry.Name(), ".zip") {
			t.Fatalf("invalid response published cache entry %q", entry.Name())
		}
	}
}

func TestOpenHTTPDownloadsAndCachesByCanonicalURL(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	archive := testRemoteZip(t, "hello.txt", "hello")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/assets/archive.zip" {
			t.Errorf("request path = %q, want /assets/archive.zip", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	oldBase := spxBaseDir
	spxBaseDir = cache
	t.Cleanup(func() { spxBaseDir = oldBase })

	raw := server.Listener.Addr().String() + "/assets/archive.zip"
	dir, err := OpenHttp(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := readRemoteEntry(t, dir, "hello.txt"); got != "hello" {
		t.Fatalf("cached entry = %q, want hello", got)
	}
	if err := dir.Close(); err != nil {
		t.Fatal(err)
	}

	dir, err = OpenHttp(raw)
	if err != nil {
		t.Fatal(err)
	}
	_ = dir.Close()
	if got := requests.Load(); got != 1 {
		t.Fatalf("server requests = %d, want 1 after cache hit", got)
	}

	_, key, err := parseRemoteURL(raw, "http://")
	if err != nil {
		t.Fatal(err)
	}
	local, err := remoteCachePath(cache, "http://", key)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(local)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		t.Fatalf("cache target mode = %v, want regular non-symlink", info.Mode())
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("cache target permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestOpenHTTPDoesNotFollowCacheSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows")
	}
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	victim := filepath.Join(root, "victim.zip")
	if err := os.WriteFile(victim, []byte("victim"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := testRemoteZip(t, "safe.txt", "safe")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	oldBase := spxBaseDir
	spxBaseDir = cache
	t.Cleanup(func() { spxBaseDir = oldBase })

	raw := server.Listener.Addr().String() + "/safe.zip"
	_, key, err := parseRemoteURL(raw, "http://")
	if err != nil {
		t.Fatal(err)
	}
	local, err := remoteCachePath(cache, "http://", key)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureCacheDirectories(filepath.Dir(local)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, local); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	dir, err := OpenHttp(raw)
	if err != nil {
		t.Fatal(err)
	}
	_ = dir.Close()
	info, err := os.Lstat(local)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("cache symlink was followed or retained")
	}
	if got, readErr := os.ReadFile(victim); readErr != nil || string(got) != "victim" {
		t.Fatalf("victim changed: %q, err=%v", got, readErr)
	}
}

func TestOpenHTTPEnforcesResponseLimitWithoutContentLength(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		for i := 0; i < 64; i++ {
			_, _ = w.Write([]byte("0123456789"))
		}
	}))
	defer server.Close()
	oldBase, oldLimit := spxBaseDir, maxRemoteZipBytes
	spxBaseDir = filepath.Join(root, "cache")
	maxRemoteZipBytes = 32
	t.Cleanup(func() {
		spxBaseDir = oldBase
		maxRemoteZipBytes = oldLimit
	})

	_, err := OpenHttp(server.Listener.Addr().String() + "/large.zip")
	if !errors.Is(err, ErrRemoteSizeLimit) {
		t.Fatalf("OpenHttp error = %v, want ErrRemoteSizeLimit", err)
	}
}

func TestHTTPSClientRejectsDowngradeRedirect(t *testing.T) {
	downgrade := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("downgrade target was contacted")
	}))
	defer downgrade.Close()
	client := remoteClientForScheme(&http.Client{}, "https://")
	requestURL, err := url.Parse("https://example.test/archive.zip")
	if err != nil {
		t.Fatal(err)
	}
	redirectURL, err := url.Parse(downgrade.URL + "/archive.zip")
	if err != nil {
		t.Fatal(err)
	}
	request := &http.Request{URL: redirectURL}
	if err := client.CheckRedirect(request, []*http.Request{{URL: requestURL}}); !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("CheckRedirect error = %v, want ErrInsecureRedirect", err)
	}
}

func TestOpenHTTPConcurrentDownloadsPublishCompleteArchive(t *testing.T) {
	root := t.TempDir()
	archive := testRemoteZip(t, "concurrent.txt", strings.Repeat("x", 1024))
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	oldBase := spxBaseDir
	spxBaseDir = filepath.Join(root, "cache")
	t.Cleanup(func() { spxBaseDir = oldBase })

	raw := server.Listener.Addr().String() + "/concurrent.zip"
	const callers = 8
	var wait sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			dir, err := OpenHttp(raw)
			if err != nil {
				errs <- err
				return
			}
			defer dir.Close()
			file, err := dir.Open("concurrent.txt")
			if err != nil {
				errs <- err
				return
			}
			_, err = io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				errs <- err
			}
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent OpenHttp error: %v", err)
	}
	if requests.Load() == 0 {
		t.Fatal("server was not contacted")
	}
}

func TestParseRemoteURLCanonicalizesHostAndSeparatesProtocols(t *testing.T) {
	_, httpKey, err := parseRemoteURL("Example.TEST/assets/a.zip?token=1", "http://")
	if err != nil {
		t.Fatal(err)
	}
	remote, httpsKey, err := parseRemoteURL("example.test/assets/a.zip?token=1", "https://")
	if err != nil {
		t.Fatal(err)
	}
	if remote != "https://example.test/assets/a.zip?token=1" {
		t.Fatalf("canonical remote = %q", remote)
	}
	if httpKey == httpsKey {
		t.Fatal("HTTP and HTTPS cache keys collided")
	}
}

func testRemoteZip(t *testing.T, name, contents string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(entry, contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func readRemoteEntry(t *testing.T, dir interface {
	Open(string) (io.ReadCloser, error)
}, name string) string {
	t.Helper()
	file, err := dir.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
