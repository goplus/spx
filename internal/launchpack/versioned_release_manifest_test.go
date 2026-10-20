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

package launchpack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

type testVersionedManifest struct {
	Version string `json:"version"`
	Value   string `json:"value"`
}

func TestVersionedReleaseManifestPrefersLocalMirror(t *testing.T) {
	cacheRoot, mirrorDir := t.TempDir(), t.TempDir()
	spec := testVersionedReleaseManifestSpec(cacheRoot)
	spec.MirrorDir = mirrorDir
	writeVersionedReleaseManifestTestFile(t, testVersionedReleaseManifestPath(spec), []byte(`{"version":"1.2.3","value":"cache"}`))
	mirrorData := []byte(`{"version":"1.2.3","value":"mirror"}`)
	writeVersionedReleaseManifestTestFile(t, filepath.Join(mirrorDir, spec.Name), mirrorData)
	fetchCalls := 0
	spec.Fetch = func(context.Context, string, io.Writer) error {
		fetchCalls++
		return nil
	}

	manifest, data, err := acquireTestVersionedReleaseManifest(spec)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Value != "mirror" || !bytes.Equal(data, mirrorData) {
		t.Fatalf("mirrored manifest = %#v, %q", manifest, data)
	}
	if fetchCalls != 0 {
		t.Fatalf("fetch calls = %d, want 0", fetchCalls)
	}
}

func TestVersionedReleaseManifestRevalidatesAndRefreshesCache(t *testing.T) {
	cacheRoot := t.TempDir()
	spec := testVersionedReleaseManifestSpec(cacheRoot)
	cachePath := testVersionedReleaseManifestPath(spec)
	writeVersionedReleaseManifestTestFile(t, cachePath, []byte(`{"version":"9.9.9","value":"stale"}`))
	fresh := []byte(`{"version":"1.2.3","value":"fresh"}`)
	fetchCalls := 0
	spec.Fetch = func(_ context.Context, _ string, dst io.Writer) error {
		fetchCalls++
		_, err := dst.Write(fresh)
		return err
	}

	manifest, data, err := acquireTestVersionedReleaseManifest(spec)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Value != "fresh" || !bytes.Equal(data, fresh) || fetchCalls != 1 {
		t.Fatalf("refreshed manifest = %#v, %q; fetch calls = %d", manifest, data, fetchCalls)
	}
	if cached, err := os.ReadFile(cachePath); err != nil || !bytes.Equal(cached, fresh) {
		t.Fatalf("cached manifest = %q, err = %v", cached, err)
	}

	spec.Offline = true
	spec.Fetch = func(context.Context, string, io.Writer) error {
		t.Fatal("offline cache hit fetched")
		return nil
	}
	if manifest, data, err := acquireTestVersionedReleaseManifest(spec); err != nil || manifest.Value != "fresh" || !bytes.Equal(data, fresh) {
		t.Fatalf("offline cache hit = %#v, %q, %v", manifest, data, err)
	}

	if err := os.WriteFile(cachePath, []byte(`{"version":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := acquireTestVersionedReleaseManifest(spec); err == nil {
		t.Fatal("offline corrupt cache was accepted")
	}
}

func TestVersionedReleaseManifestOfflineCacheMiss(t *testing.T) {
	spec := testVersionedReleaseManifestSpec(t.TempDir())
	spec.Offline = true
	spec.Fetch = nil
	if _, _, err := acquireTestVersionedReleaseManifest(spec); !errors.Is(err, runtimebundle.ErrOfflineCacheMiss) {
		t.Fatalf("offline cache miss = %v", err)
	}
}

func TestVersionedReleaseManifestRejectsOversizedDownloadWithoutPublishing(t *testing.T) {
	spec := testVersionedReleaseManifestSpec(t.TempDir())
	spec.MaxSize = 8
	spec.Fetch = func(_ context.Context, _ string, dst io.Writer) error {
		_, err := dst.Write([]byte(`{"version":"1.2.3"}`))
		return err
	}
	if _, _, err := acquireTestVersionedReleaseManifest(spec); !errors.Is(err, errVersionedReleaseManifestTooLarge) {
		t.Fatalf("oversized manifest error = %v", err)
	}
	if _, err := os.Lstat(testVersionedReleaseManifestPath(spec)); !os.IsNotExist(err) {
		t.Fatalf("oversized manifest was published: %v", err)
	}
}

func TestVersionedReleaseManifestFailedRefreshPreservesCache(t *testing.T) {
	spec := testVersionedReleaseManifestSpec(t.TempDir())
	cachePath := testVersionedReleaseManifestPath(spec)
	stale := []byte(`{"version":"9.9.9","value":"stale"}`)
	writeVersionedReleaseManifestTestFile(t, cachePath, stale)
	spec.Fetch = func(_ context.Context, _ string, dst io.Writer) error {
		_, err := dst.Write([]byte(`{"version":"8.8.8","value":"wrong"}`))
		return err
	}
	if _, _, err := acquireTestVersionedReleaseManifest(spec); err == nil {
		t.Fatal("downloaded wrong-version manifest was accepted")
	}
	if cached, err := os.ReadFile(cachePath); err != nil || !bytes.Equal(cached, stale) {
		t.Fatalf("failed refresh changed cache to %q, err = %v", cached, err)
	}
}

func testVersionedReleaseManifestSpec(cacheRoot string) versionedReleaseManifestSpec {
	return versionedReleaseManifestSpec{
		CacheRoot: cacheRoot,
		Namespace: "runtime",
		Version:   "1.2.3",
		Name:      "manifest.json",
		URL:       "https://example.invalid/manifest.json",
		MaxSize:   1024,
	}
}

func testVersionedReleaseManifestPath(spec versionedReleaseManifestSpec) string {
	return filepath.Join(spec.CacheRoot, versionedReleaseManifestCacheDirectory, spec.Namespace, spec.Version, spec.Name)
}

func acquireTestVersionedReleaseManifest(spec versionedReleaseManifestSpec) (testVersionedManifest, []byte, error) {
	return acquireVersionedReleaseManifest(context.Background(), spec, func(data []byte) (testVersionedManifest, error) {
		var manifest testVersionedManifest
		err := json.Unmarshal(data, &manifest)
		return manifest, err
	}, func(manifest testVersionedManifest) string {
		return manifest.Version
	})
}

func writeVersionedReleaseManifestTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVersionedManifestRejectsInvalidInputs(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		change     func(*versionedReleaseManifestSpec)
	}{
		{"relative cache", "cache root", func(s *versionedReleaseManifestSpec) { s.CacheRoot = "relative" }},
		{"namespace traversal", "namespace", func(s *versionedReleaseManifestSpec) { s.Namespace = ".." }},
		{"version separator", "version", func(s *versionedReleaseManifestSpec) { s.Version = "1/2" }},
		{"name whitespace", "name", func(s *versionedReleaseManifestSpec) { s.Name = " manifest.json" }},
		{"name backslash", "name", func(s *versionedReleaseManifestSpec) { s.Name = "folder\\manifest.json" }},
		{"relative mirror", "mirror", func(s *versionedReleaseManifestSpec) { s.MirrorDir = "relative" }},
		{"zero budget", "size limit", func(s *versionedReleaseManifestSpec) { s.MaxSize = 0 }},
		{"empty URL", "URL is required", func(s *versionedReleaseManifestSpec) { s.URL = " \t " }},
		{"missing fetcher", "fetcher is required", func(s *versionedReleaseManifestSpec) {}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spec := testVersionedReleaseManifestSpec(t.TempDir())
			tt.change(&spec)
			if _, _, err := acquireTestVersionedReleaseManifest(spec); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("acquire error = %v, want %q", err, tt.want)
			}
		})
	}
	spec := testVersionedReleaseManifestSpec(t.TempDir())
	parse := func(data []byte) (string, error) { return string(data), nil }
	version := func(s string) string { return s }
	if _, _, err := acquireVersionedReleaseManifest(nil, spec, parse, version); err == nil || !strings.Contains(err.Error(), "nil context") {
		t.Fatalf("nil context: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := acquireVersionedReleaseManifest(ctx, spec, parse, version); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: %v", err)
	}
	if _, _, err := acquireVersionedReleaseManifest[string](context.Background(), spec, nil, version); err == nil || !strings.Contains(err.Error(), "parser") {
		t.Fatalf("nil parser: %v", err)
	}
	if _, _, err := acquireVersionedReleaseManifest(context.Background(), spec, parse, nil); err == nil || !strings.Contains(err.Error(), "accessor") {
		t.Fatalf("nil accessor: %v", err)
	}
}

func TestVersionedManifestRejectsUnusableMirrorsAndCache(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		mirror     bool
		setup      func(*testing.T, string)
	}{
		{"missing mirror", "read mirrored release manifest", true, func(*testing.T, string) {}},
		{"oversized mirror", "exceeds size limit", true, func(t *testing.T, path string) {
			writeVersionedReleaseManifestTestFile(t, path, bytes.Repeat([]byte("x"), 1025))
		}},
		{"malformed mirror", "read mirrored release manifest", true, func(t *testing.T, path string) { writeVersionedReleaseManifestTestFile(t, path, []byte("{")) }},
		{"wrong mirror version", "does not match", true, func(t *testing.T, path string) {
			writeVersionedReleaseManifestTestFile(t, path, []byte(`{"version":"9.9.9"}`))
		}},
		{"blocked cache ancestor", "inspect cached release manifest", false, func(t *testing.T, path string) {
			writeVersionedReleaseManifestTestFile(t, filepath.Dir(path), []byte("blocked"))
		}},
		{"directory cache", "not a regular non-symlink file", false, func(t *testing.T, path string) {
			if err := os.MkdirAll(path, 0700); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink cache", "not a regular non-symlink file", false, func(t *testing.T, path string) {
			writeVersionedReleaseManifestTestFile(t, path+".target", []byte(`{"version":"1.2.3"}`))
			if err := os.Symlink(path+".target", path); err != nil {
				t.Skip(err)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spec := testVersionedReleaseManifestSpec(t.TempDir())
			spec.Offline = true
			path := testVersionedReleaseManifestPath(spec)
			if tt.mirror {
				spec.MirrorDir = t.TempDir()
				path = filepath.Join(spec.MirrorDir, spec.Name)
			}
			tt.setup(t, path)
			if _, _, err := acquireTestVersionedReleaseManifest(spec); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("acquire error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestVersionedManifestDownloadFailuresDoNotPublish(t *testing.T) {
	for _, name := range []string{"ignored size error", "invalid json", "cancelled after fetch"} {
		t.Run(name, func(t *testing.T) {
			spec := testVersionedReleaseManifestSpec(t.TempDir())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			spec.Fetch = func(_ context.Context, _ string, w io.Writer) error {
				switch name {
				case "ignored size error":
					_, _ = w.Write(bytes.Repeat([]byte("x"), 1025))
				case "invalid json":
					_, _ = io.WriteString(w, "{")
				case "cancelled after fetch":
					_, _ = io.WriteString(w, `{"version":"1.2.3"}`)
					cancel()
				}
				return nil
			}
			_, _, err := acquireVersionedReleaseManifest(ctx, spec, func(data []byte) (testVersionedManifest, error) {
				var m testVersionedManifest
				err := json.Unmarshal(data, &m)
				return m, err
			}, func(m testVersionedManifest) string { return m.Version })
			if err == nil {
				t.Fatal("failed download accepted")
			}
			if name == "ignored size error" && !errors.Is(err, errVersionedReleaseManifestTooLarge) {
				t.Fatalf("size failure = %v", err)
			}
			if name == "cancelled after fetch" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation = %v", err)
			}
			if _, err := os.Stat(testVersionedReleaseManifestPath(spec)); !os.IsNotExist(err) {
				t.Fatalf("failed download published: %v", err)
			}
		})
	}
}

func TestCachedVersionedManifestRevalidatesPersistedBytes(t *testing.T) {
	for _, name := range []string{"parser error", "version mismatch", "size limit"} {
		t.Run(name, func(t *testing.T) {
			spec := testVersionedReleaseManifestSpec(t.TempDir())
			data := []byte("1.2.3")
			parse := func(data []byte) (string, error) {
				if name == "parser error" {
					return "", errors.New("invalid cached data")
				}
				return string(data), nil
			}
			version := func(s string) string {
				if name == "version mismatch" {
					return "9.9.9"
				}
				return s
			}
			if name == "size limit" {
				spec.MaxSize = 1
			}
			_, _, err := acquireCachedVersionedReleaseManifest(context.Background(), t.TempDir(), spec, data, parse, version, false)
			if err == nil {
				t.Fatal("invalid persisted manifest accepted")
			}
			switch name {
			case "parser error":
				if !strings.Contains(err.Error(), "parse cached release manifest") {
					t.Fatal(err)
				}
			case "version mismatch":
				if !strings.Contains(err.Error(), "cached release manifest version") {
					t.Fatal(err)
				}
			case "size limit":
				if !errors.Is(err, errVersionedReleaseManifestTooLarge) {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestVersionedManifestOfflineDetectsReplacementAfterParsing(t *testing.T) {
	spec := testVersionedReleaseManifestSpec(t.TempDir())
	spec.Offline = true
	spec.Fetch = func(context.Context, string, io.Writer) error {
		t.Fatal("offline cache replacement triggered a fetch")
		return nil
	}
	path := testVersionedReleaseManifestPath(spec)
	writeVersionedReleaseManifestTestFile(t, path, []byte(`{"version":"1.2.3","value":"original"}`))
	parse := func(data []byte) (testVersionedManifest, error) {
		var m testVersionedManifest
		err := json.Unmarshal(data, &m)
		// Simulate another writer replacing the entry between the optimistic
		// parse and acquisition of the locked, digest-checked cache handle.
		writeVersionedReleaseManifestTestFile(t, path, []byte(`{"version":"1.2.3","value":"replaced"}`))
		return m, err
	}
	_, data, err := acquireVersionedReleaseManifest(context.Background(), spec, parse, func(m testVersionedManifest) string { return m.Version })
	if err == nil || !strings.Contains(err.Error(), "offline cached release manifest is invalid") || data != nil {
		t.Fatalf("replaced cache = %q, %v", data, err)
	}
}
