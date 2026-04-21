package runtimeasset

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
)

func TestPrepareExtractsEmbeddedAssets(t *testing.T) {
	oldFS := assetsFS
	oldCacheBaseDirFn := cacheBaseDirFn
	t.Cleanup(func() {
		assetsFS = oldFS
		cacheBaseDirFn = oldCacheBaseDirFn
	})

	assetsFS = fstest.MapFS{
		"assets/gdspxrt9.9.9":              &fstest.MapFile{Data: []byte("runtime")},
		"assets/gdspxrt9.9.9.pck":          &fstest.MapFile{Data: []byte("runtime-pck")},
		"assets/gdspx-linux-amd64.so":      &fstest.MapFile{Data: []byte("shared-lib")},
		"assets/runtime.gdextension":       &fstest.MapFile{Data: []byte("unused")},
		"assets/placeholder.txt":           &fstest.MapFile{Data: []byte("placeholder")},
		"assets/gdspx-darwin-amd64.dylib":  &fstest.MapFile{Data: []byte("unused-darwin")},
		"assets/gdspx-windows-amd64.dll":   &fstest.MapFile{Data: []byte("unused-windows")},
		"assets/gdspx-linux-arm64.so":      &fstest.MapFile{Data: []byte("unused-linux-arm64")},
		"assets/gdspxrt9.9.9.exe":          &fstest.MapFile{Data: []byte("unused-runtime-exe")},
		"assets/gdspxrt9.9.9.exe.pck":      &fstest.MapFile{Data: []byte("unused-runtime-exe-pck")},
		"assets/gdspx-windows-arm64.dll":   &fstest.MapFile{Data: []byte("unused-windows-arm64")},
		"assets/gdspx-darwin-arm64.dylib":  &fstest.MapFile{Data: []byte("unused-darwin-arm64")},
		"assets/gdspx-linux-x86_64.so":     &fstest.MapFile{Data: []byte("unused-linux-x86_64")},
		"assets/gdspx-linux-x86_32.so":     &fstest.MapFile{Data: []byte("unused-linux-x86_32")},
		"assets/gdspx-windows-x86_64.dll":  &fstest.MapFile{Data: []byte("unused-windows-x86_64")},
		"assets/gdspx-windows-x86_32.dll":  &fstest.MapFile{Data: []byte("unused-windows-x86_32")},
		"assets/gdspx-darwin-x86_64.dylib": &fstest.MapFile{Data: []byte("unused-darwin-x86_64")},
	}
	cacheRoot := t.TempDir()
	cacheBaseDirFn = func() string { return cacheRoot }

	dir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9", "gdspxrt9.9.9.pck", "gdspx-linux-amd64.so")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if !ok {
		t.Fatal("Prepare returned ok=false, want true")
	}

	assertFileContent(t, filepath.Join(dir, "gdspxrt9.9.9"), "runtime")
	assertFileContent(t, filepath.Join(dir, "gdspxrt9.9.9.pck"), "runtime-pck")
	assertFileContent(t, filepath.Join(dir, "gdspx-linux-amd64.so"), "shared-lib")

	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "gdspxrt9.9.9"))
		if err != nil {
			t.Fatalf("stat runtime executable: %v", err)
		}
		if info.Mode()&0o111 == 0 {
			t.Fatalf("runtime executable mode = %v, want executable bit", info.Mode())
		}
	}
}

func TestPrepareReturnsFalseWhenAssetMissing(t *testing.T) {
	oldFS := assetsFS
	oldCacheBaseDirFn := cacheBaseDirFn
	t.Cleanup(func() {
		assetsFS = oldFS
		cacheBaseDirFn = oldCacheBaseDirFn
	})

	assetsFS = fstest.MapFS{
		"assets/placeholder.txt": &fstest.MapFile{Data: []byte("placeholder")},
	}
	cacheBaseDirFn = func() string { return t.TempDir() }

	dir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if ok {
		t.Fatalf("Prepare returned ok=true with dir=%s, want false", dir)
	}
}

func TestPrepareUsesManifestCacheKeyWhenAvailable(t *testing.T) {
	oldFS := assetsFS
	oldCacheBaseDirFn := cacheBaseDirFn
	t.Cleanup(func() {
		assetsFS = oldFS
		cacheBaseDirFn = oldCacheBaseDirFn
	})

	cacheRoot := t.TempDir()
	cacheBaseDirFn = func() string { return cacheRoot }
	assetsFS = fstest.MapFS{
		"assets/manifest.json":        &fstest.MapFile{Data: []byte(`{"cache_key":"manifest-key","names":["gdspxrt9.9.9","gdspxrt9.9.9.pck","gdspx-linux-amd64.so"]}`)},
		"assets/gdspxrt9.9.9":         &fstest.MapFile{Data: []byte("runtime")},
		"assets/gdspxrt9.9.9.pck":     &fstest.MapFile{Data: []byte("runtime-pck")},
		"assets/gdspx-linux-amd64.so": &fstest.MapFile{Data: []byte("shared-lib")},
		"assets/placeholder.txt":      &fstest.MapFile{Data: []byte("placeholder")},
	}

	dir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9", "gdspxrt9.9.9.pck", "gdspx-linux-amd64.so")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if !ok {
		t.Fatal("Prepare returned ok=false, want true")
	}
	if filepath.Base(dir) != "manifest-key" {
		t.Fatalf("Prepare cache dir = %s, want suffix manifest-key", dir)
	}
	assertFileContent(t, filepath.Join(dir, "gdspx-linux-amd64.so"), "shared-lib")
}

func TestPrepareSeparatesCacheDirsByEmbeddedContent(t *testing.T) {
	oldFS := assetsFS
	oldCacheBaseDirFn := cacheBaseDirFn
	t.Cleanup(func() {
		assetsFS = oldFS
		cacheBaseDirFn = oldCacheBaseDirFn
	})

	cacheRoot := t.TempDir()
	cacheBaseDirFn = func() string { return cacheRoot }

	assetsFS = fstest.MapFS{
		"assets/gdspxrt9.9.9":         &fstest.MapFile{Data: []byte("runtime-v1")},
		"assets/gdspxrt9.9.9.pck":     &fstest.MapFile{Data: []byte("pack-v1")},
		"assets/gdspx-linux-amd64.so": &fstest.MapFile{Data: []byte("shared-one")},
	}
	dirV1, ok, err := Prepare("9.9.9", "gdspxrt9.9.9", "gdspxrt9.9.9.pck", "gdspx-linux-amd64.so")
	if err != nil {
		t.Fatalf("Prepare(v1) returned error: %v", err)
	}
	if !ok {
		t.Fatal("Prepare(v1) returned ok=false, want true")
	}
	assertFileContent(t, filepath.Join(dirV1, "gdspx-linux-amd64.so"), "shared-one")

	assetsFS = fstest.MapFS{
		"assets/gdspxrt9.9.9":         &fstest.MapFile{Data: []byte("runtime-v2")},
		"assets/gdspxrt9.9.9.pck":     &fstest.MapFile{Data: []byte("pack-v2")},
		"assets/gdspx-linux-amd64.so": &fstest.MapFile{Data: []byte("shared-two")},
	}
	dirV2, ok, err := Prepare("9.9.9", "gdspxrt9.9.9", "gdspxrt9.9.9.pck", "gdspx-linux-amd64.so")
	if err != nil {
		t.Fatalf("Prepare(v2) returned error: %v", err)
	}
	if !ok {
		t.Fatal("Prepare(v2) returned ok=false, want true")
	}
	if dirV1 == dirV2 {
		t.Fatalf("Prepare reused cache dir %s for different embedded content", dirV1)
	}
	assertFileContent(t, filepath.Join(dirV1, "gdspx-linux-amd64.so"), "shared-one")
	assertFileContent(t, filepath.Join(dirV2, "gdspx-linux-amd64.so"), "shared-two")
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("file %s = %q, want %q", path, string(data), want)
	}
}
