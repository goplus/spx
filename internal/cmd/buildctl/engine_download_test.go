package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExtractZipRejectsPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "archive.zip")
	if err := writeZipFixture(zipPath, map[string]string{
		"../evil.txt": "bad",
	}); err != nil {
		t.Fatalf("writeZipFixture returned error: %v", err)
	}

	dstDir := filepath.Join(tempDir, "extract")
	if err := extractZip(zipPath, dstDir); err == nil {
		t.Fatal("expected extractZip to reject zip-slip path traversal")
	} else if !strings.Contains(err.Error(), "illegal path in archive entry") {
		t.Fatalf("extractZip error = %v, want illegal path error", err)
	}

	if fileExists(filepath.Join(tempDir, "evil.txt")) {
		t.Fatal("unexpected file extracted outside destination directory")
	}
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

func TestFetchURLToFileHonorsHTTPClientTimeout(t *testing.T) {
	oldClient := engineDownloadHTTPClient
	engineDownloadHTTPClient = &http.Client{Timeout: 20 * time.Millisecond}
	defer func() {
		engineDownloadHTTPClient = oldClient
	}()

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
	if !strings.Contains(strings.ToLower(err.Error()), "timeout") && !strings.Contains(strings.ToLower(err.Error()), "deadline") {
		t.Fatalf("fetchURLToFile error = %v, want timeout/deadline error", err)
	}
}

func TestLinkOrCopyFilePrefersHardLinkWhenAvailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hard link behavior varies on Windows")
	}

	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src.bin")
	dst := filepath.Join(tempDir, "dst.bin")
	if err := os.WriteFile(src, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", src, err)
	}

	if err := linkOrCopyFile(src, dst); err != nil {
		t.Fatalf("linkOrCopyFile returned error: %v", err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatalf("Stat(%s) returned error: %v", src, err)
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat(%s) returned error: %v", dst, err)
	}
	if !os.SameFile(srcInfo, dstInfo) {
		t.Fatal("expected destination to be created as a hard link")
	}
}

func TestLinkOrCopyFileReplacesExistingHardLinkWithoutTruncatingSource(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src.bin")
	dst := filepath.Join(tempDir, "dst.bin")
	if err := os.WriteFile(src, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", src, err)
	}
	if err := os.Link(src, dst); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	if err := linkOrCopyFile(src, dst); err != nil {
		t.Fatalf("linkOrCopyFile returned error: %v", err)
	}

	content, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", src, err)
	}
	if string(content) != "content" {
		t.Fatalf("source content = %q, want original content preserved", string(content))
	}

	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", dst, err)
	}
	if string(dstContent) != "content" {
		t.Fatalf("destination content = %q, want copied content preserved", string(dstContent))
	}
}
