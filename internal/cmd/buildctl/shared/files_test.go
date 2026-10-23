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
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/runtimebundle"
)

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

	if err := FetchURLToFile(server.URL, dst); err == nil {
		t.Fatal("expected interrupted download to fail")
	} else if strings.Contains(err.Error(), "ended after") {
		t.Fatalf("download hid the interrupted response error: %v", err)
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
	if err := FetchURLToFile(server.URL, dst); err != nil {
		t.Fatalf("FetchURLToFile returned error: %v", err)
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
	err := FetchURLToFileWithLimit(server.URL, filepath.Join(parent, "ndk.zip"), 4)
	if !errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("FetchURLToFileWithLimit error = %v, want ErrArchiveLimit", err)
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
	err := FetchURLToFileWithLimit(server.URL, dst, 4)
	if !errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("FetchURLToFileWithLimit error = %v, want ErrArchiveLimit", err)
	}
	if content, readErr := os.ReadFile(dst); readErr != nil || string(content) != "existing" {
		t.Fatalf("destination content = %q, err = %v; want original content", content, readErr)
	}
	if matches, globErr := filepath.Glob(filepath.Join(tempDir, "ndk.zip.tmp-*")); globErr != nil || len(matches) != 0 {
		t.Fatalf("temporary download files = %v, err = %v; want none", matches, globErr)
	}
}

func TestDownloadURLReportsProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "8")
		_, _ = w.Write([]byte("contents"))
	}))
	defer server.Close()

	dst := filepath.Join(t.TempDir(), "asset.zip")
	var done, total int64
	err := DownloadURL(server.URL, dst, 8, func(n, size int64) {
		done, total = n, size
	})
	if err != nil {
		t.Fatal(err)
	}
	if done != 8 || total != 8 {
		t.Fatalf("last progress = (%d, %d), want (8, 8)", done, total)
	}
	if data, err := os.ReadFile(dst); err != nil || string(data) != "contents" {
		t.Fatalf("downloaded content = %q, err = %v", data, err)
	}
}

func TestDownloadURLReturnsHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	var statusErr *HTTPStatusError
	err := DownloadURL(server.URL, filepath.Join(t.TempDir(), "asset.zip"), 8, nil)
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("DownloadURL error = %v, want typed HTTP 404", err)
	}
	if !strings.Contains(err.Error(), server.URL) || !strings.Contains(err.Error(), "404 Not Found") {
		t.Fatalf("status error lacks request context: %v", err)
	}
}

func TestDownloadURLRejectsInvalidInputBeforeRequest(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "asset.zip")
	if err := DownloadURL("https://example.test/asset.zip", dst, 0, nil); err == nil || !strings.Contains(err.Error(), "invalid download size limit") {
		t.Fatalf("zero limit error = %v, want invalid limit", err)
	}
	if err := DownloadURL("https://example.test/asset.zip", dst, -1, nil); err == nil || !strings.Contains(err.Error(), "invalid download size limit") {
		t.Fatalf("negative limit error = %v, want invalid limit", err)
	}
	if err := DownloadURL("://invalid", dst, 1, nil); err == nil {
		t.Fatal("invalid URL was accepted")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("destination changed after invalid input: %v", err)
	}
}

func TestFileHelpersRoundTrip(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(src, "nested", "asset.txt")
	if err := os.WriteFile(original, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(root, "copy")
	if err := CopyDir(src, copied); err != nil {
		t.Fatal(err)
	}
	if !DirExists(copied) || DirExists(original) || DirExists(filepath.Join(root, "missing")) {
		t.Fatal("incorrect directory classification")
	}
	single := filepath.Join(root, "single", "asset.txt")
	if err := CopyFile(original, single); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		write func(string) error
		entry string
	}{
		{"directory", func(dst string) error { return ZipDirectory(copied, dst) }, "nested/asset.txt"},
		{"named", func(dst string) error { return WriteNamedZip(dst, map[string]string{"renamed.txt": single}) }, "renamed.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			archive := filepath.Join(root, tc.name+".zip")
			if err := tc.write(archive); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(root, tc.name+"-extracted")
			if err := ExtractZip(archive, out); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(tc.entry))); err != nil || string(data) != "payload" {
				t.Fatalf("round trip = %q, %v", data, err)
			}
		})
	}
	for name, call := range map[string]func() error{
		"copy file":      func() error { return CopyFile(filepath.Join(root, "absent"), single) },
		"copy directory": func() error { return CopyDir(filepath.Join(root, "absent"), copied) },
		"zip directory":  func() error { return ZipDirectory(single, filepath.Join(root, "bad.zip")) },
		"named zip": func() error {
			return WriteNamedZip(filepath.Join(root, "bad.zip"), map[string]string{"missing": filepath.Join(root, "absent")})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("missing source or wrong kind accepted")
			}
		})
	}
}

func TestDownloadLimitAvoidsOverflow(t *testing.T) {
	for _, n := range []int64{1, 4096, math.MaxInt64 - 1, math.MaxInt64} {
		want := n
		if n < math.MaxInt64 {
			want++
		}
		if got := downloadLimitWithOverflow(n); got != want {
			t.Errorf("limit(%d)=%d, want%d", n, got, want)
		}
	}
}

type downloadRoundTripFunc func(*http.Request) (*http.Response, error)

func (f downloadRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type downloadResponseBody struct {
	io.Reader
	closeErr error
	closed   bool
}

func (b *downloadResponseBody) Close() error {
	b.closed = true
	return b.closeErr
}

func TestDownloadURLPreservesErrorsAndDestination(t *testing.T) {
	failure := errors.New("response failed")
	for _, test := range []struct {
		name          string
		declared      int64
		transportErr  error
		closeErr      error
		wantErr       error
		wantCommitted bool
	}{
		{name: "transport failure", transportErr: failure, wantErr: failure},
		{name: "short body", declared: 4, wantErr: io.ErrUnexpectedEOF},
		{name: "long body", declared: 2, wantErr: io.ErrUnexpectedEOF},
		{name: "close failure after commit", declared: 3, closeErr: failure, wantErr: failure, wantCommitted: true},
		{name: "size error takes precedence over close", declared: 4, closeErr: failure, wantErr: io.ErrUnexpectedEOF},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			dst := filepath.Join(dir, "asset.zip")
			if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
				t.Fatal(err)
			}
			body := &downloadResponseBody{Reader: strings.NewReader("new"), closeErr: test.closeErr}
			previous := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = previous })
			calls := 0
			http.DefaultTransport = downloadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if test.transportErr != nil {
					return nil, test.transportErr
				}
				return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: test.declared, Request: req}, nil
			})
			if err := DownloadURL("https://example.invalid/asset.zip", dst, 8, nil); !errors.Is(err, test.wantErr) {
				t.Fatalf("download error = %v, want %v", err, test.wantErr)
			}
			if calls != 1 || body.closed != (test.transportErr == nil) {
				t.Fatalf("requests = %d, body closed = %v", calls, body.closed)
			}
			want := "existing"
			if test.wantCommitted {
				want = "new"
			}
			if data, err := os.ReadFile(dst); err != nil || string(data) != want {
				t.Fatalf("destination = %q, %v; want %q", data, err, want)
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
				t.Fatalf("download left temporary files: %v, %v", entries, err)
			}
		})
	}
}

func TestDownloadURLRejectsUnusableDestination(t *testing.T) {
	for _, kind := range []string{"parent file", "directory destination", "temporary name too long"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			dst, preserved := filepath.Join(dir, "asset.zip"), ""
			switch kind {
			case "parent file":
				preserved = filepath.Join(dir, "parent")
				dst = filepath.Join(preserved, "asset.zip")
			case "directory destination":
				if err := os.Mkdir(dst, 0o755); err != nil {
					t.Fatal(err)
				}
				preserved = filepath.Join(dst, "keep")
			case "temporary name too long":
				// Check the filesystem's name support before exercising the temporary suffix.
				dst = filepath.Join(dir, strings.Repeat("a", 255))
				preserved = dst
			}
			if err := os.WriteFile(preserved, []byte("preserved"), 0o644); err != nil {
				if kind == "temporary name too long" {
					t.Skipf("filesystem cannot create the long destination name: %v", err)
				}
				t.Fatal(err)
			}
			if kind == "temporary name too long" {
				if probe, err := os.CreateTemp(dir, filepath.Base(dst)+".tmp-*"); err == nil {
					if err := probe.Close(); err != nil {
						t.Fatal(err)
					}
					t.Skip("filesystem also supports the longer temporary name")
				}
			}
			body := &downloadResponseBody{Reader: strings.NewReader("new")}
			previous := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = previous })
			http.DefaultTransport = downloadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: 3, Request: req}, nil
			})
			if err := DownloadURL("https://example.invalid/asset.zip", dst, 8, nil); err == nil {
				t.Fatal("unusable destination accepted")
			}
			if !body.closed {
				t.Fatal("response body was not closed after filesystem failure")
			}
			if data, err := os.ReadFile(preserved); err != nil || string(data) != "preserved" {
				t.Fatalf("existing file changed: %q, %v", data, err)
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
				t.Fatalf("download left temporary files: %v, %v", entries, err)
			}
		})
	}
}
