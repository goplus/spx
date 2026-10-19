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

package xgolauncher

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/runtimepayload"
)

func TestProcessStatusSuccess(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status ProcessStatus
		want   bool
	}{
		{name: "zero", want: true},
		{name: "exit code", status: ProcessStatus{Code: 1}},
		{name: "negative code", status: ProcessStatus{Code: -1}},
		{name: "signal", status: ProcessStatus{Signal: 15}},
		{name: "code and signal", status: ProcessStatus{Code: 1, Signal: 15}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.Success(); got != tt.want {
				t.Fatalf("%+v.Success() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestRunRejectsInvalidPayload(t *testing.T) {
	payload, payloadDigest, manifestDigest, _ := launcherTestPayload(t, "unused")
	for _, tt := range []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "empty", want: "invalid expected payload SHA-256"},
		{
			name: "payload digest",
			cfg:  Config{Payload: payload, PayloadSHA256: strings.Repeat("0", 64), ManifestSHA256: manifestDigest},
			want: "payload SHA-256 mismatch",
		},
		{
			name: "manifest digest",
			cfg:  Config{Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: strings.Repeat("0", 64)},
			want: "manifest SHA-256 mismatch",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			status, err := Run(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) || status != (ProcessStatus{}) {
				t.Fatalf("Run() = (%+v, %v), want zero status and %q", status, err, tt.want)
			}
		})
	}
}

func TestRunContextRejectsInvalidContext(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "nil"},
		{name: "canceled", ctx: canceled, want: context.Canceled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			status, err := RunContext(tt.ctx, Config{})
			if err == nil || status != (ProcessStatus{}) {
				t.Fatalf("RunContext() = (%+v, %v), want zero status and an error", status, err)
			}
			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("error = %v, want %v before payload verification", err, tt.want)
				}
			} else if err.Error() != "xgolauncher: nil context" {
				t.Fatalf("error = %v, want nil context error", err)
			}
		})
	}
}

func TestRunRejectsInvalidCacheRoot(t *testing.T) {
	payload, payloadDigest, manifestDigest, _ := launcherTestPayload(t, "unused")
	status, err := run(t.Context(), Config{
		Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest,
	}, "relative-cache")
	if err == nil || !strings.Contains(err.Error(), "xgolauncher: validate cache root:") || status != (ProcessStatus{}) {
		t.Fatalf("run() = (%+v, %v), want zero status and cache root error", status, err)
	}
}

func TestRunWorkDirectoryError(t *testing.T) {
	payload, payloadDigest, manifestDigest, _ := launcherTestPayload(t, "unused")
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	launcherTestTempDir(t, filepath.Join(t.TempDir(), "missing"))
	status, err := run(t.Context(), Config{
		Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest,
	}, cacheRoot)
	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "create launcher work directory:") || status != (ProcessStatus{}) {
		t.Fatalf("run() = (%+v, %v), want work directory not found", status, err)
	}
	if _, err := os.Stat(cacheRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cache root stat error = %v, want no materialization", err)
	}
}

func TestRunMaterializationErrors(t *testing.T) {
	payload, payloadDigest, manifestDigest, manifest := launcherTestPayload(t, "unused")
	components := []struct {
		namespace runtimebundle.Namespace
		digest    string
		want      string
	}{
		{runtimebundle.NamespaceEngine, manifest.Engine.BundleDigest, "materialize Engine:"},
		{runtimebundle.NamespaceBridge, manifest.Bridge.BundleDigest, "materialize bridge:"},
		{runtimebundle.NamespaceProject, manifest.Project.BundleDigest, "materialize project:"},
	}
	for index, component := range components {
		t.Run(string(component.namespace), func(t *testing.T) {
			root := t.TempDir()
			cacheRoot := filepath.Join(root, "cache")
			if err := runtimebundle.NewCache(cacheRoot).Permissions.EnsureDir(cacheRoot); err != nil {
				t.Fatal(err)
			}
			blocked := filepath.Join(cacheRoot, string(component.namespace))
			if err := os.WriteFile(blocked, []byte("blocked"), 0o600); err != nil {
				t.Fatal(err)
			}
			tempRoot := t.TempDir()
			launcherTestTempDir(t, tempRoot)
			status, err := run(t.Context(), Config{
				Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest,
			}, cacheRoot)
			if err == nil || !strings.Contains(err.Error(), component.want) || status != (ProcessStatus{}) {
				t.Fatalf("run() = (%+v, %v), want zero status and %q", status, err, component.want)
			}
			if entries, err := os.ReadDir(tempRoot); err != nil || len(entries) != 0 {
				t.Fatalf("temporary files = %v, error = %v, want cleanup", entries, err)
			}
			if data, err := os.ReadFile(blocked); err != nil || string(data) != "blocked" {
				t.Fatalf("blocked namespace = %q, error = %v, want unchanged file", data, err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			for _, previous := range components[:index] {
				path := filepath.Join(cacheRoot, string(previous.namespace), previous.digest)
				lease, err := (runtimebundle.CrossProcessLockProvider{}).AcquireExclusive(ctx, path)
				if err != nil {
					t.Fatalf("acquire %s after failure: %v", previous.namespace, err)
				}
				if err := lease.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestRunSessionPreparationError(t *testing.T) {
	payload, payloadDigest, manifestDigest, _ := launcherTestPayload(t, "unused", func(cfg *runtimepayload.BuildConfig) {
		cfg.Project.PackDirectory = "missing-assets"
	})
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	tempRoot := t.TempDir()
	launcherTestTempDir(t, tempRoot)
	status, err := run(t.Context(), Config{
		Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest,
	}, cacheRoot)
	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "prepare Engine session:") || status != (ProcessStatus{}) {
		t.Fatalf("run() = (%+v, %v), want missing assets error", status, err)
	}
	if entries, err := os.ReadDir(tempRoot); err != nil || len(entries) != 0 {
		t.Fatalf("temporary files = %v, error = %v, want work and session cleanup", entries, err)
	}
}

func launcherTestTempDir(t *testing.T, dir string) {
	t.Helper()
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, dir)
	}
}

func launcherTestPayload(t *testing.T, engineScript string, configure ...func(*runtimepayload.BuildConfig)) ([]byte, string, string, runtimepayload.Manifest) {
	t.Helper()
	engineFiles := []launcherTestFile{
		{Name: "runtime-manifest.json", Mode: 0o644, Data: []byte(`{"schema":"test-engine/v1"}`)},
		{Name: "engine", Mode: 0o755, Data: []byte(engineScript)},
		{Name: "engine.pck", Mode: 0o644, Data: []byte("pack")},
	}
	bridgeFiles := []launcherTestFile{
		{Name: "bridge-manifest.json", Mode: 0o644, Data: []byte(`{"schema":"test-bridge/v1"}`)},
		{Name: "bridge.so", Mode: 0o755, Data: []byte("bridge")},
	}
	projectFiles := []launcherTestFile{
		{Name: "main.spx", Mode: 0o644, Data: []byte("onStart => {}")},
		{Name: "assets/index.json", Mode: 0o644, Data: []byte(`{"zorder":[]}`)},
	}
	engineZIP := launcherTestZIP(t, engineFiles)
	bridgeZIP := launcherTestZIP(t, bridgeFiles)
	projectZIP := launcherTestZIP(t, projectFiles)
	engine := launcherTestBundle(t, engineZIP, runtimebundle.NamespaceEngine)
	bridge := launcherTestBundle(t, bridgeZIP, runtimebundle.NamespaceBridge)
	project := launcherTestBundle(t, projectZIP, runtimebundle.NamespaceProject)
	projectSum := sha256.Sum256(projectZIP)
	interfaceSum := sha256.Sum256([]byte("interface"))
	config := runtimepayload.BuildConfig{
		SPX:    runtimepayload.SourceIdentity{SelectedPath: "github.com/goplus/spx/v3", EffectivePath: "github.com/goplus/spx/v3", SourceMode: true},
		Target: runtimepayload.Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		Engine: runtimepayload.Engine{
			RuntimeVersion: "test", RuntimeABI: 1, EngineInterfaceDigest: hex.EncodeToString(interfaceSum[:]),
			Executable: "engine", Pack: "engine.pck", BundleDigest: engine.Digest,
		},
		Bridge: runtimepayload.Bridge{File: "bridge.so", BundleDigest: bridge.Digest},
		Project: runtimepayload.Project{
			PackDirectory: "assets", BundleDigest: project.Digest, ArchiveSHA256: hex.EncodeToString(projectSum[:]),
		},
	}
	for _, configure := range configure {
		configure(&config)
	}
	sources := launcherTestSources(
		launcherTestFile{Name: "engine/runtime-manifest.json", Mode: 0o644, Data: engineFiles[0].Data},
		launcherTestFile{Name: "engine/engine", Mode: 0o755, Data: engineFiles[1].Data},
		launcherTestFile{Name: "engine/engine.pck", Mode: 0o644, Data: engineFiles[2].Data},
		launcherTestFile{Name: "bridge/bridge-manifest.json", Mode: 0o644, Data: bridgeFiles[0].Data},
		launcherTestFile{Name: "bridge/bridge.so", Mode: 0o755, Data: bridgeFiles[1].Data},
		launcherTestFile{Name: runtimepayload.ProjectZipPath, Mode: 0o644, Data: projectZIP},
	)
	var payload bytes.Buffer
	payloadDigest, manifestDigest, err := runtimepayload.BuildTo(&payload, config, sources)
	if err != nil {
		t.Fatal(err)
	}
	payloadBytes := append([]byte(nil), payload.Bytes()...)
	verified, err := runtimepayload.Verify(payloadBytes, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	return payloadBytes, payloadDigest, manifestDigest, verified.Manifest
}

type launcherTestFile struct {
	Name string
	Mode os.FileMode
	Data []byte
}

func launcherTestSources(files ...launcherTestFile) []runtimepayload.FileSource {
	sources := make([]runtimepayload.FileSource, len(files))
	for i, file := range files {
		sources[i] = runtimepayload.FileSource{
			Name: file.Name, Mode: file.Mode, ReaderAt: bytes.NewReader(file.Data), Size: int64(len(file.Data)),
		}
	}
	return sources
}

func launcherTestZIP(t *testing.T, files []launcherTestFile) []byte {
	t.Helper()
	files = append([]launcherTestFile(nil), files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	canonicalTime := time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.Name, Method: zip.Store}
		header.SetMode(file.Mode)
		header.SetModTime(canonicalTime)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(file.Data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func launcherTestBundle(t *testing.T, data []byte, namespace runtimebundle.Namespace) runtimebundle.Bundle {
	t.Helper()
	bundle, err := runtimepayload.ComponentBundleReaderAt(bytes.NewReader(data), int64(len(data)), namespace)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}
