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

package shared

import (
	"archive/zip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

type roundTripBodyTransport struct {
	status        int
	contentLength int64
	body          string
}

func (transport roundTripBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode:    transport.status,
		Status:        http.StatusText(transport.status),
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(transport.body)),
		ContentLength: transport.contentLength,
		Request:       req,
	}, nil
}

type extractZipFixture struct {
	name   string
	data   string
	method uint16
}

func writeExtractZipFixture(t *testing.T, entries ...extractZipFixture) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.zip")
	output, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(output)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name, Method: item.method}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = output.Close()
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(item.data)); err != nil {
			_ = output.Close()
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultRuntimeVersionUsesRuntimeLock(t *testing.T) {
	got, err := defaultRuntimeVersion()
	if err != nil {
		t.Fatal(err)
	}
	if want := release.DefaultRuntimeLock().RuntimeVersion; got != want {
		t.Fatalf("default runtime version = %q, want locked version %q", got, want)
	}
}

func TestExtractZipWithOptionsRejectsResourceExhaustion(t *testing.T) {
	tests := []struct {
		name    string
		entries []extractZipFixture
		limits  ZipLimits
	}{
		{
			name: "entry count",
			entries: []extractZipFixture{
				{name: "one", data: "1", method: zip.Store},
				{name: "two", data: "2", method: zip.Store},
			},
			limits: ZipLimits{MaxEntries: 1},
		},
		{
			name:    "entry size",
			entries: []extractZipFixture{{name: "large", data: "12345", method: zip.Store}},
			limits:  ZipLimits{MaxEntrySize: 4},
		},
		{
			name: "total size",
			entries: []extractZipFixture{
				{name: "one", data: "123", method: zip.Store},
				{name: "two", data: "456", method: zip.Store},
			},
			limits: ZipLimits{MaxEntrySize: 5, MaxTotalSize: 5},
		},
		{
			name:    "compression ratio",
			entries: []extractZipFixture{{name: "repetitive", data: strings.Repeat("a", 4096), method: zip.Deflate}},
			limits:  ZipLimits{MaxCompressionRatio: 2},
		},
		{
			name:    "central directory",
			entries: []extractZipFixture{{name: "metadata", data: "1", method: zip.Store}},
			limits:  ZipLimits{MaxCentralDirectoryBytes: 1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := writeExtractZipFixture(t, test.entries...)
			err := ExtractZipWithOptions(archive, filepath.Join(t.TempDir(), "out"), ZipExtractOptions{Limits: test.limits})
			if !errors.Is(err, runtimebundle.ErrArchiveLimit) {
				t.Fatalf("ExtractZipWithOptions error = %v, want ErrArchiveLimit", err)
			}
		})
	}
}

func TestExtractZipRejectsUnsafePathAndDestinationSymlink(t *testing.T) {
	t.Run("path traversal", func(t *testing.T) {
		archive := writeExtractZipFixture(t, extractZipFixture{name: "../escape", data: "bad", method: zip.Store})
		err := ExtractZip(archive, filepath.Join(t.TempDir(), "out"))
		if !errors.Is(err, runtimebundle.ErrInvalidEntryName) {
			t.Fatalf("ExtractZip error = %v, want ErrInvalidEntryName", err)
		}
	})

	t.Run("destination symlink", func(t *testing.T) {
		dst := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(dst, "linked")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		archive := writeExtractZipFixture(t, extractZipFixture{name: "linked/escape", data: "bad", method: zip.Store})
		err := ExtractZip(archive, dst)
		if !errors.Is(err, runtimebundle.ErrUnsafeArchive) {
			t.Fatalf("ExtractZip error = %v, want ErrUnsafeArchive", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "escape")); !os.IsNotExist(err) {
			t.Fatalf("outside destination changed: %v", err)
		}
	})
}

func TestFetchURLToFileLeavesDestinationUntouchedOnInterruptedDownload(t *testing.T) {
	tempDir := t.TempDir()
	dst := filepath.Join(tempDir, "asset.zip")
	if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", dst, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}
		conn, buf, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("Hijack returned error: %v", err)
			return
		}
		defer conn.Close()
		if _, err := buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 10\r\n\r\nabc"); err != nil {
			t.Errorf("WriteString returned error: %v", err)
			return
		}
		if err := buf.Flush(); err != nil {
			t.Errorf("Flush returned error: %v", err)
		}
	}))
	defer server.Close()

	if err := fetchURLToFile(server.URL, dst); err == nil {
		t.Fatal("expected interrupted download to fail")
	}

	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", dst, err)
	}
	if string(content) != "existing" {
		t.Fatalf("destination content = %q, want original content preserved", string(content))
	}

	if matches, err := filepath.Glob(filepath.Join(tempDir, "asset.zip.tmp-*")); err != nil {
		t.Fatalf("Glob returned error: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("unexpected temporary download files left behind: %v", matches)
	}
}

func TestFetchURLToFileReplacesExistingDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("replacement"))
	}))
	defer server.Close()

	dst := filepath.Join(t.TempDir(), "asset.zip")
	if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fetchURLToFile(server.URL, dst); err != nil {
		t.Fatalf("fetchURLToFile returned error: %v", err)
	}
	if data, err := os.ReadFile(dst); err != nil || string(data) != "replacement" {
		t.Fatalf("destination content = %q, err = %v", data, err)
	}
}

func TestFetchURLToFileWithLimitRejectsDeclaredSizeBeforeCreatingTempFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "5")
		_, _ = w.Write([]byte("12345"))
	}))
	defer server.Close()

	parent := filepath.Join(t.TempDir(), "missing")
	err := fetchURLToFileWithLimit(server.URL, filepath.Join(parent, "ndk.zip"), 4)
	if !errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("fetchURLToFileWithLimit error = %v, want ErrArchiveLimit", err)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Fatalf("download directory was created before Content-Length rejection: %v", err)
	}
}

func TestFetchURLToFileWithLimitRejectsChunkedBodyAboveLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("12345"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dst := filepath.Join(tempDir, "ndk.zip")
	if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := fetchURLToFileWithLimit(server.URL, dst, 4)
	if !errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("fetchURLToFileWithLimit error = %v, want ErrArchiveLimit", err)
	}
	if content, readErr := os.ReadFile(dst); readErr != nil || string(content) != "existing" {
		t.Fatalf("destination content = %q, err = %v; want original content", content, readErr)
	}
	if matches, globErr := filepath.Glob(filepath.Join(tempDir, "ndk.zip.tmp-*")); globErr != nil || len(matches) != 0 {
		t.Fatalf("temporary download files = %v, err = %v; want none", matches, globErr)
	}
}

func TestFetchURLToFileWithLimitRejectsShortDeclaredBody(t *testing.T) {
	oldClient := fileDownloadHTTPClient
	fileDownloadHTTPClient = &http.Client{Transport: roundTripBodyTransport{
		status:        http.StatusOK,
		contentLength: 5,
		body:          "123",
	}}
	t.Cleanup(func() { fileDownloadHTTPClient = oldClient })

	dir := t.TempDir()
	dst := filepath.Join(dir, "short.zip")
	if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := fetchURLToFileWithLimit("https://example.invalid/short.zip", dst, 10)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("fetchURLToFileWithLimit error = %v, want io.ErrUnexpectedEOF", err)
	}
	if data, readErr := os.ReadFile(dst); readErr != nil || string(data) != "existing" {
		t.Fatalf("destination content = %q, err = %v; want original content", data, readErr)
	}
	if matches, globErr := filepath.Glob(dst + ".tmp-*"); globErr != nil || len(matches) != 0 {
		t.Fatalf("temporary download files = %v, err = %v", matches, globErr)
	}
}

func TestFetchURLToFileRejectsHTTPSDowngrade(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("replacement"))
	}))
	defer target.Close()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, target.URL, http.StatusFound)
	}))
	defer server.Close()

	oldClient := fileDownloadHTTPClient
	fileDownloadHTTPClient = server.Client()
	callbackCalled := false
	fileDownloadHTTPClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		callbackCalled = true
		return nil
	}
	t.Cleanup(func() { fileDownloadHTTPClient = oldClient })

	dst := filepath.Join(t.TempDir(), "asset.zip")
	if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := fetchURLToFile(server.URL, dst)
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("fetchURLToFile error = %v, want HTTPS downgrade rejection", err)
	}
	if callbackCalled {
		t.Fatal("custom CheckRedirect ran before HTTPS downgrade validation")
	}
	if data, readErr := os.ReadFile(dst); readErr != nil || string(data) != "existing" {
		t.Fatalf("destination content = %q, err = %v; want original content", data, readErr)
	}
}

func TestFetchURLToFileHonorsHTTPClientTimeout(t *testing.T) {
	oldClient := fileDownloadHTTPClient
	fileDownloadHTTPClient = &http.Client{Timeout: 20 * time.Millisecond}
	t.Cleanup(func() {
		fileDownloadHTTPClient = oldClient
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("late"))
	}))
	defer server.Close()

	err := fetchURLToFile(server.URL, filepath.Join(t.TempDir(), "asset.zip"))
	if err == nil {
		t.Fatal("expected timeout error")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "timeout") && !strings.Contains(msg, "deadline") {
		t.Fatalf("fetchURLToFile error = %v, want timeout/deadline error", err)
	}
}
