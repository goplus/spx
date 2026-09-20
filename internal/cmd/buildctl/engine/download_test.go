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

package engine

import (
	"archive/zip"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
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
	} else if !errors.Is(err, runtimebundle.ErrInvalidEntryName) {
		t.Fatalf("extractZip error = %v, want ErrInvalidEntryName", err)
	}

	if fileExists(filepath.Join(tempDir, "evil.txt")) {
		t.Fatal("unexpected file extracted outside destination directory")
	}
}

func TestExtractZipAllowsEngineSizedEntryMetadata(t *testing.T) {
	// The Windows editor with debug symbols is the largest v3.3.0 engine asset.
	const windowsEditorEntrySize uint64 = 1_890_193_473
	declaredSize := windowsEditorEntrySize
	if declaredSize <= uint64(runtimebundle.MaxEntrySize) {
		t.Fatalf("test entry size %d does not exceed default limit %d", declaredSize, runtimebundle.MaxEntrySize)
	}
	if declaredSize > uint64(maxEngineArchiveEntrySize) {
		t.Fatalf("test entry size %d exceeds engine limit %d", declaredSize, maxEngineArchiveEntrySize)
	}

	zipPath := writeRawZipFixture(t, declaredSize)
	if err := shared.ExtractZip(zipPath, filepath.Join(t.TempDir(), "default")); !errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("default ZIP extraction error = %v, want ErrArchiveLimit", err)
	}

	dstDir := filepath.Join(t.TempDir(), "engine")
	err := extractZip(zipPath, dstDir)
	if err == nil {
		t.Fatal("expected inconsistent ZIP metadata to be rejected")
	}
	if errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("extractZip rejected entry metadata within the engine limit: %v", err)
	}
	if !errors.Is(err, runtimebundle.ErrUnsafeArchive) {
		t.Fatalf("extractZip error = %v, want ErrUnsafeArchive", err)
	}
	if _, err := os.Stat(dstDir); !os.IsNotExist(err) {
		t.Fatalf("extraction destination was created before verification completed: %v", err)
	}
}

func TestExtractZipRejectsEntryAboveEngineSizeLimit(t *testing.T) {
	declaredSize := uint64(maxEngineArchiveEntrySize + 1)
	zipPath := writeRawZipFixture(t, declaredSize)
	err := extractZip(zipPath, filepath.Join(t.TempDir(), "extract"))
	if !errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("extractZip error = %v, want ErrArchiveLimit", err)
	}
	if want := fmt.Sprintf("limit %d", maxEngineArchiveEntrySize); !strings.Contains(err.Error(), want) {
		t.Fatalf("extractZip error = %v, want %q", err, want)
	}
}

func writeRawZipFixture(t *testing.T, declaredSize uint64) string {
	t.Helper()
	dataSize := declaredSize/runtimebundle.MaxCompressionRatio + 1
	data := make([]byte, int(dataSize))
	zipPath := filepath.Join(t.TempDir(), "archive.zip")
	output, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(output)
	entry, err := writer.CreateRaw(&zip.FileHeader{
		Name:               "editor",
		Method:             zip.Store,
		CRC32:              crc32.ChecksumIEEE(data),
		CompressedSize64:   uint64(len(data)),
		UncompressedSize64: declaredSize,
	})
	if err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if _, err := entry.Write(data); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	return zipPath
}

func TestDownloadLinuxAssetsRequiresLinuxPlatform(t *testing.T) {
	for _, env := range []engineDownloadEnv{{platform: "darwin", arch: linuxRuntimePackArch}} {
		if err := downloadLinuxAssets(env, false); err == nil {
			t.Fatalf("downloadLinuxAssets accepted %s", env.platform)
		} else if !strings.Contains(err.Error(), "Linux runtime assets require platform linux") {
			t.Fatalf("downloadLinuxAssets error = %v", err)
		}
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

func TestFetchURLToFileRejectsDeclaredSizeBeforeCreatingTempFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "5")
		_, _ = w.Write([]byte("12345"))
	}))
	defer server.Close()

	parent := filepath.Join(t.TempDir(), "missing")
	err := fetchURLToFileWithLimit(server.URL, filepath.Join(parent, "asset.zip"), 4)
	if !errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("fetchURLToFileWithLimit error = %v, want ErrArchiveLimit", err)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Fatalf("download directory was created before Content-Length rejection: %v", err)
	}
}

func TestFetchURLToFileRejectsChunkedBodyAboveLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("12345"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dst := filepath.Join(tempDir, "asset.zip")
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
	if matches, globErr := filepath.Glob(filepath.Join(tempDir, "asset.zip.tmp-*")); globErr != nil || len(matches) != 0 {
		t.Fatalf("temporary download files = %v, err = %v; want none", matches, globErr)
	}
}

func TestLoadEngineAssetManifestExplainsUnavailableRuntime(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	env := engineDownloadEnv{
		version:   release.DefaultRuntimeLock().RuntimeVersion,
		cacheDir:  t.TempDir(),
		urlPrefix: server.URL + "/",
	}
	err := loadEngineAssetManifest(&env)
	if err == nil {
		t.Fatal("expected missing runtime manifest to fail")
	}
	want := "locked runtime " + release.DefaultRuntimeLock().RuntimeReleaseTag() + " is unavailable: runtime-manifest.json returned 404 Not Found\n" +
		"Published-asset setup requires a complete runtime release.\n" +
		"Publish the locked runtime, or build from source with \"make dev MODE=normal\"."
	if err.Error() != want {
		t.Fatalf("loadEngineAssetManifest error = %q, want %q", err, want)
	}
}

func TestLoadEngineAssetManifestKeepsNonNotFoundFailuresClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	env := engineDownloadEnv{
		version:   release.DefaultRuntimeLock().RuntimeVersion,
		cacheDir:  t.TempDir(),
		urlPrefix: server.URL + "/",
	}
	err := loadEngineAssetManifest(&env)
	if err == nil {
		t.Fatal("expected runtime manifest server failure")
	}
	if !strings.Contains(err.Error(), "download runtime manifest") || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("loadEngineAssetManifest error = %q, want original server failure", err)
	}
	if strings.Contains(err.Error(), "make dev") {
		t.Fatalf("server failure must not be classified as an unavailable release: %q", err)
	}
}

func TestLoadEngineAssetManifestRejectsOversizedDownload(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
	}{
		{
			name: "declared",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", fmt.Sprint(maxRuntimeManifestBytes+1))
				w.WriteHeader(http.StatusOK)
			}),
		},
		{
			name: "chunked",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				_, _ = io.Copy(w, strings.NewReader(strings.Repeat("x", int(maxRuntimeManifestBytes+1))))
			}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()
			cacheDir := t.TempDir()
			manifestPath := filepath.Join(cacheDir, release.DefaultRuntimeLock().Manifest)
			if err := os.WriteFile(manifestPath, []byte("existing"), 0o644); err != nil {
				t.Fatal(err)
			}
			env := engineDownloadEnv{
				version: release.DefaultRuntimeLock().RuntimeVersion, cacheDir: cacheDir, urlPrefix: server.URL + "/",
			}
			if err := loadEngineAssetManifest(&env); !errors.Is(err, runtimebundle.ErrArchiveLimit) {
				t.Fatalf("loadEngineAssetManifest error = %v, want ErrArchiveLimit", err)
			}
			if data, err := os.ReadFile(manifestPath); err != nil || string(data) != "existing" {
				t.Fatalf("manifest content = %q, err = %v; want original content", data, err)
			}
			if matches, err := filepath.Glob(manifestPath + ".tmp-*"); err != nil || len(matches) != 0 {
				t.Fatalf("manifest temporary files = %v, err = %v", matches, err)
			}
		})
	}
}

func TestLoadEngineAssetManifestRejectsOversizedLocalFile(t *testing.T) {
	assetDir := t.TempDir()
	lock := release.DefaultRuntimeLock()
	manifestPath := filepath.Join(assetDir, lock.Manifest)
	file, err := os.Create(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxRuntimeManifestBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	env := engineDownloadEnv{version: lock.RuntimeVersion, cacheDir: t.TempDir(), assetDir: assetDir}
	if err := loadEngineAssetManifest(&env); !errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("loadEngineAssetManifest error = %v, want ErrArchiveLimit", err)
	}
}

func TestLoadEngineAssetManifestAcceptsSameVersionBuildMetadata(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	assets := make([]release.RuntimeAsset, 0, len(lock.RequiredAssets))
	for _, name := range lock.RequiredAssets {
		assets = append(assets, release.RuntimeAsset{Name: name, Size: 1, SHA256: strings.Repeat("0", 64)})
	}
	manifest := release.RuntimeManifest{
		Schema:            release.RuntimeManifestSchema,
		RuntimeVersion:    lock.RuntimeVersion,
		RuntimeABI:        lock.RuntimeABI + 1,
		ReleaseRepository: "example/runtime",
		LockSHA256:        strings.Repeat("1", 64),
		Provenance: release.RuntimeProvenance{
			SPXCommit: strings.Repeat("2", 40), GodotCommit: strings.Repeat("3", 40), ModuleTree: strings.Repeat("4", 40),
			RuntimePackSourceSHA256: strings.Repeat("5", 64), BuildRecipeSHA256: strings.Repeat("6", 64), Toolchain: lock.Toolchain,
		},
		Assets: assets,
	}
	data, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()

	env := engineDownloadEnv{
		version: lock.RuntimeVersion, cacheDir: t.TempDir(), urlPrefix: server.URL + "/",
	}
	if err := loadEngineAssetManifest(&env); err != nil {
		t.Fatalf("same-version manifest rejected stale build metadata: %v", err)
	}
	if env.manifest == nil || env.manifest.ReleaseRepository != "example/runtime" {
		t.Fatalf("loaded manifest = %#v", env.manifest)
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

func TestShouldRefreshPreparedAssetsDefaultsToGitHubActions(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	previous, ok := os.LookupEnv("SPX_PREPARE_FORCE_REFRESH")
	t.Cleanup(func() {
		if !ok {
			_ = os.Unsetenv("SPX_PREPARE_FORCE_REFRESH")
			return
		}
		_ = os.Setenv("SPX_PREPARE_FORCE_REFRESH", previous)
	})
	if err := os.Unsetenv("SPX_PREPARE_FORCE_REFRESH"); err != nil {
		t.Fatalf("Unsetenv returned error: %v", err)
	}

	if !shouldRefreshPreparedAssets() {
		t.Fatal("expected GitHub Actions runs to refresh prepared assets by default")
	}
}

func TestShouldRefreshPreparedAssetsAllowsExplicitDisableInCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("SPX_PREPARE_FORCE_REFRESH", "0")

	if shouldRefreshPreparedAssets() {
		t.Fatal("expected SPX_PREPARE_FORCE_REFRESH=0 to disable forced refresh in CI")
	}
}

func TestShouldRefreshPreparedAssetsAllowsExplicitEnableLocally(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("SPX_PREPARE_FORCE_REFRESH", "1")

	if !shouldRefreshPreparedAssets() {
		t.Fatal("expected SPX_PREPARE_FORCE_REFRESH=1 to force refresh locally")
	}
}
