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

package runtimepayload

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/iotest"

	"github.com/goplus/spx/v4/internal/runtimebundle"
)

type testFile struct {
	Name string
	Mode fs.FileMode
	Data []byte
}

type testPayload struct {
	BuildConfig
	Files []testFile
}

func buildPayload(config testPayload) ([]byte, string, string, error) {
	cfg, sources := fileSources(config)
	var output bytes.Buffer
	payloadDigest, manifestDigest, err := BuildTo(&output, cfg, sources)
	if err != nil {
		return nil, "", "", err
	}
	return output.Bytes(), payloadDigest, manifestDigest, nil
}

func newTestPayload(t *testing.T) ([]byte, string, string) {
	t.Helper()
	payload, payloadDigest, manifestDigest, err := buildPayload(testBuildConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	return payload, payloadDigest, manifestDigest
}

func componentBundleBytes(data []byte, namespace runtimebundle.Namespace) (runtimebundle.Bundle, error) {
	return ComponentBundleReaderAt(bytes.NewReader(data), int64(len(data)), namespace)
}

func verifiedComponentZIP(verified *Verified, prefix string) ([]byte, error) {
	var output bytes.Buffer
	err := verified.WriteComponentZIP(prefix, &output)
	return output.Bytes(), err
}

func verifiedProjectZIP(t *testing.T, verified *Verified) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := verified.WriteProjectZIP(&output); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func canonicalComponentZIP(files []testFile) ([]byte, error) {
	files = append([]testFile(nil), files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.Name, Method: zip.Store}
		header.SetMode(canonicalFileMode(file.Mode))
		header.Modified = canonicalTime.UTC()
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err := entry.Write(file.Data); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func TestBuildToMatchesBuildBytes(t *testing.T) {
	config := testBuildConfig(t)
	want, wantPayloadDigest, wantManifestDigest, err := buildPayload(config)
	if err != nil {
		t.Fatal(err)
	}
	streamConfig, sources := fileSources(config)
	var output bytes.Buffer
	gotPayloadDigest, gotManifestDigest, err := BuildTo(&output, streamConfig, sources)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatal("BuildTo bytes differ from Build")
	}
	if gotPayloadDigest != wantPayloadDigest || gotManifestDigest != wantManifestDigest {
		t.Fatalf("BuildTo digests = %q/%q, want %q/%q", gotPayloadDigest, gotManifestDigest, wantPayloadDigest, wantManifestDigest)
	}
}

func TestBuildToRejectsShortReadWriteFailureAndSourceChange(t *testing.T) {
	t.Run("short read", func(t *testing.T) {
		config, sources := fileSources(testBuildConfig(t))
		sources[0].Size++
		if _, _, err := BuildTo(io.Discard, config, sources); err == nil || !strings.Contains(err.Error(), "short read") {
			t.Fatalf("BuildTo error = %v, want short read", err)
		}
	})

	t.Run("write failure", func(t *testing.T) {
		config, sources := fileSources(testBuildConfig(t))
		writer := &failingWriter{remaining: 64}
		if _, _, err := BuildTo(writer, config, sources); !errors.Is(err, errWriteFailure) {
			t.Fatalf("BuildTo error = %v, want %v", err, errWriteFailure)
		}
	})

	t.Run("source change", func(t *testing.T) {
		base := testBuildConfig(t)
		config, sources := fileSources(base)
		for i := range sources {
			if sources[i].Name == "engine/engine" {
				first := append([]byte(nil), configFileData(t, base, sources[i].Name)...)
				second := bytes.Repeat([]byte{'x'}, len(first))
				sources[i].ReaderAt = &changingReaderAt{first: first, second: second}
				break
			}
		}
		if _, _, err := BuildTo(io.Discard, config, sources); err == nil || !strings.Contains(err.Error(), "changed between hash and write") {
			t.Fatalf("BuildTo error = %v, want source-change rejection", err)
		}
	})
}

func TestBuildToRejectsInvalidMetadataBeforeReadingSources(t *testing.T) {
	t.Run("entry count", func(t *testing.T) {
		config := testBuildConfig(t).BuildConfig
		guard := &countingReaderAt{reader: bytes.NewReader([]byte("must not read"))}
		sources := make([]FileSource, runtimebundle.MaxEntries)
		sources[0] = FileSource{Name: "entry", ReaderAt: guard, Size: 0}
		if _, _, err := BuildTo(io.Discard, config, sources); !errors.Is(err, runtimebundle.ErrArchiveLimit) {
			t.Fatalf("BuildTo error = %v, want runtimebundle.ErrArchiveLimit", err)
		}
		if reads, _ := guard.counts(); reads != 0 {
			t.Fatalf("oversized entry-count source bytes read = %d, want 0", reads)
		}
	})

	t.Run("total size", func(t *testing.T) {
		config, sources := fileSources(testBuildConfig(t))
		for i := 0; i < 3; i++ {
			sources = append(sources, FileSource{Name: "extra-" + string(rune('a'+i))})
		}
		guards := make([]*countingReaderAt, len(sources))
		for i := range sources {
			guards[i] = &countingReaderAt{reader: bytes.NewReader(nil)}
			sources[i].ReaderAt = guards[i]
			sources[i].Size = runtimebundle.MaxEntrySize
		}
		if _, _, err := BuildTo(io.Discard, config, sources); !errors.Is(err, runtimebundle.ErrArchiveLimit) {
			t.Fatalf("BuildTo error = %v, want runtimebundle.ErrArchiveLimit", err)
		}
		for i, guard := range guards {
			if reads, _ := guard.counts(); reads != 0 {
				t.Fatalf("total-limit source %d bytes read = %d, want 0", i, reads)
			}
		}
	})

	for _, tt := range []struct {
		name, want string
		limit      bool
		mutate     func(*BuildConfig, *[]FileSource)
	}{
		{"late invalid path", "invalid entry path", false, func(_ *BuildConfig, sources *[]FileSource) {
			(*sources)[len(*sources)-1].Name = "zz/../bad"
		}},
		{"late nil reader", "nil ReaderAt", false, func(_ *BuildConfig, sources *[]FileSource) {
			(*sources)[len(*sources)-1].ReaderAt = nil
		}},
		{"late negative size", "negative size", false, func(_ *BuildConfig, sources *[]FileSource) {
			(*sources)[len(*sources)-1].Size = -1
		}},
		{"long name", "name is too long", false, func(_ *BuildConfig, sources *[]FileSource) {
			(*sources)[len(*sources)-1].Name = strings.Repeat("z", zipMaxNameBytes+1)
		}},
		{"late oversized entry", "exceeds limit", true, func(_ *BuildConfig, sources *[]FileSource) {
			(*sources)[len(*sources)-1].Size = runtimebundle.MaxEntrySize + 1
		}},
		{"manifest in total", "total size", true, func(_ *BuildConfig, sources *[]FileSource) {
			for _, name := range []string{"extra-a", "extra-b"} {
				*sources = append(*sources, FileSource{Name: name, ReaderAt: (*sources)[0].ReaderAt})
			}
			for i := range *sources {
				(*sources)[i].Size = runtimebundle.MaxEntrySize
			}
		}},
		{"duplicate", "duplicate entry", false, func(_ *BuildConfig, sources *[]FileSource) {
			*sources = append(*sources, (*sources)[len(*sources)-1])
		}},
		{"reserved manifest", "reserved", false, func(_ *BuildConfig, sources *[]FileSource) {
			(*sources)[len(*sources)-1].Name = ManifestPath
		}},
		{"oversized manifest", "manifest size", true, func(cfg *BuildConfig, _ *[]FileSource) {
			cfg.SPX.EffectivePath = strings.Repeat("a", maxPayloadManifestBytes)
		}},
		{"source bounds before manifest", "entry ", true, func(cfg *BuildConfig, sources *[]FileSource) {
			cfg.SPX.EffectivePath = strings.Repeat("a", maxPayloadManifestBytes)
			(*sources)[len(*sources)-1].Size = runtimebundle.MaxEntrySize + 1
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config, sources := fileSources(testBuildConfig(t))
			reads := 0
			for i := range sources {
				sources[i].ReaderAt = payloadReaderAtFunc(func([]byte, int64) (int, error) {
					reads++
					return 0, errors.New("unexpected source read")
				})
			}
			tt.mutate(&config, &sources)
			var output bytes.Buffer
			payloadDigest, manifestDigest, err := BuildTo(&output, config, sources)
			if err == nil || !strings.Contains(err.Error(), tt.want) || errors.Is(err, runtimebundle.ErrArchiveLimit) != tt.limit {
				t.Fatalf("BuildTo error = %v, want %q", err, tt.want)
			}
			if reads != 0 || output.Len() != 0 || payloadDigest != "" || manifestDigest != "" {
				t.Fatalf("invalid metadata read sources %d times and wrote %d bytes; digests = %q/%q", reads, output.Len(), payloadDigest, manifestDigest)
			}
		})
	}
}

func TestVerifyReaderAtRejectsOversizedArchiveBeforeReading(t *testing.T) {
	reader := &countingReaderAt{reader: bytes.NewReader([]byte("must not read"))}
	_, err := VerifyReaderAt(reader, runtimebundle.MaxArchiveBytes+1, strings.Repeat("0", sha256.Size*2), strings.Repeat("0", sha256.Size*2), runtime.GOOS, runtime.GOARCH)
	if !errors.Is(err, runtimebundle.ErrArchiveLimit) {
		t.Fatalf("VerifyReaderAt error = %v, want runtimebundle.ErrArchiveLimit", err)
	}
	if reads, _ := reader.counts(); reads != 0 {
		t.Fatalf("oversized payload bytes read = %d, want 0", reads)
	}
}

func TestEstimatePayloadArchiveSizeMatchesBuild(t *testing.T) {
	payload, _, _ := newTestPayload(t)
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]runtimebundle.Entry, 0, len(reader.File))
	for _, file := range reader.File {
		entries = append(entries, runtimebundle.Entry{Name: file.Name, Size: int64(file.UncompressedSize64)})
	}
	estimated, err := estimatePayloadArchiveSize(entries)
	if err != nil {
		t.Fatal(err)
	}
	if estimated != int64(len(payload)) {
		t.Fatalf("estimated payload size = %d, want %d", estimated, len(payload))
	}
}

func TestBuildToAndVerifyReaderAtStreamLargeEntry(t *testing.T) {
	config, sources := fileSources(testBuildConfig(t))
	const largeSize = int64(16 << 20)
	large := &patternReaderAt{size: largeSize}
	for i := range sources {
		if sources[i].Name == "engine/engine" {
			sources[i].ReaderAt = large
			sources[i].Size = largeSize
			break
		}
	}
	engineBundle, err := ComponentBundleSources(componentSources(sources, "engine/"), runtimebundle.NamespaceEngine)
	if err != nil {
		t.Fatal(err)
	}
	config.Engine.BundleDigest = engineBundle.Digest
	large.resetCounts()

	payloadFile, err := os.CreateTemp(t.TempDir(), "payload-*.spxpkg")
	if err != nil {
		t.Fatal(err)
	}
	defer payloadFile.Close()
	payloadDigest, manifestDigest, err := BuildTo(payloadFile, config, sources)
	if err != nil {
		t.Fatal(err)
	}
	reads, maxRead := large.counts()
	if reads != 2*largeSize {
		t.Fatalf("large source bytes read = %d, want %d (two passes)", reads, 2*largeSize)
	}
	if maxRead > 1<<20 {
		t.Fatalf("largest source read = %d, want bounded streaming reads", maxRead)
	}
	if err := payloadFile.Sync(); err != nil {
		t.Fatal(err)
	}
	info, err := payloadFile.Stat()
	if err != nil {
		t.Fatal(err)
	}
	payloadReader := &countingReaderAt{reader: payloadFile}
	verified, err := VerifyReaderAt(payloadReader, info.Size(), payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	_, payloadMaxRead := payloadReader.counts()
	if payloadMaxRead > 1<<20 {
		t.Fatalf("largest payload ReaderAt request = %d, want bounded streaming reads", payloadMaxRead)
	}
	if err := verified.WriteComponentZIP("engine/", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := verified.WriteProjectZIP(io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestBuildVerifyCanonicalPayload(t *testing.T) {
	config := testBuildConfig(t)
	first, firstPayloadDigest, firstManifestDigest, err := buildPayload(config)
	if err != nil {
		t.Fatal(err)
	}
	second, secondPayloadDigest, secondManifestDigest, err := buildPayload(config)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || firstPayloadDigest != secondPayloadDigest || firstManifestDigest != secondManifestDigest {
		t.Fatal("Build is not deterministic")
	}

	verified, err := Verify(first, firstPayloadDigest, firstManifestDigest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Manifest.Protocol != ProtocolV1 || verified.Manifest.Schema != SchemaV1 {
		t.Fatalf("manifest identity = %q/%q", verified.Manifest.Schema, verified.Manifest.Protocol)
	}
	if len(verified.Manifest.Entries) != 6 {
		t.Fatalf("manifest entries = %d, want 6", len(verified.Manifest.Entries))
	}
	for _, entry := range verified.Manifest.Entries {
		if entry.Name == ManifestPath {
			t.Fatal("manifest entry table contains itself")
		}
	}
	project := verifiedProjectZIP(t, verified)
	project[0] ^= 0xff
	if bytes.Equal(project, verifiedProjectZIP(t, verified)) {
		t.Fatal("ProjectZIP returned mutable internal storage")
	}
	engineZIP, err := verifiedComponentZIP(verified, "engine/")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := componentBundleBytes(engineZIP, runtimebundle.NamespaceEngine)
	if err != nil {
		t.Fatal(err)
	}
	if engine.Digest != verified.Manifest.Engine.BundleDigest {
		t.Fatalf("engine digest = %s, want %s", engine.Digest, verified.Manifest.Engine.BundleDigest)
	}
}

func TestVerifyRejectsDigestTargetAndManifestTampering(t *testing.T) {
	payload, payloadDigest, manifestDigest := newTestPayload(t)

	t.Run("payload digest", func(t *testing.T) {
		changed := append([]byte(nil), payload...)
		changed[len(changed)/2] ^= 1
		if _, err := Verify(changed, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH); err == nil || !strings.Contains(err.Error(), "payload SHA-256 mismatch") {
			t.Fatalf("Verify error = %v", err)
		}
	})
	t.Run("manifest digest", func(t *testing.T) {
		if _, err := Verify(payload, payloadDigest, strings.Repeat("0", sha256.Size*2), runtime.GOOS, runtime.GOARCH); err == nil || !strings.Contains(err.Error(), "manifest SHA-256 mismatch") {
			t.Fatalf("Verify error = %v", err)
		}
	})
	t.Run("target", func(t *testing.T) {
		if _, err := Verify(payload, payloadDigest, manifestDigest, "not-"+runtime.GOOS, runtime.GOARCH); err == nil || !strings.Contains(err.Error(), "does not match host") {
			t.Fatalf("Verify error = %v", err)
		}
	})
	t.Run("entry table", func(t *testing.T) {
		changed, changedPayloadDigest, changedManifestDigest := rewritePayload(t, payload, func(manifest *Manifest, _ []testFile) {
			manifest.Entries[0].SHA256 = strings.Repeat("0", sha256.Size*2)
		})
		if _, err := Verify(changed, changedPayloadDigest, changedManifestDigest, runtime.GOOS, runtime.GOARCH); err == nil || !strings.Contains(err.Error(), "entry table") {
			t.Fatalf("Verify error = %v", err)
		}
	})
	t.Run("component identity", func(t *testing.T) {
		changed, changedPayloadDigest, changedManifestDigest := rewritePayload(t, payload, func(manifest *Manifest, _ []testFile) {
			manifest.Engine.BundleDigest = strings.Repeat("0", sha256.Size*2)
		})
		if _, err := Verify(changed, changedPayloadDigest, changedManifestDigest, runtime.GOOS, runtime.GOARCH); err == nil || !strings.Contains(err.Error(), "engine bundle digest mismatch") {
			t.Fatalf("Verify error = %v", err)
		}
	})
}

func TestBuildRejectsInvalidLayoutInputs(t *testing.T) {
	base := testBuildConfig(t)
	for _, test := range []struct {
		name   string
		mutate func(*testPayload)
	}{
		{name: "manifest reserved", mutate: func(config *testPayload) {
			config.Files[0].Name = ManifestPath
		}},
		{name: "duplicate", mutate: func(config *testPayload) {
			config.Files[1].Name = config.Files[0].Name
		}},
		{name: "traversal", mutate: func(config *testPayload) {
			config.Files[0].Name = "../engine"
		}},
		{name: "bad digest", mutate: func(config *testPayload) {
			config.Engine.BundleDigest = "not-a-digest"
		}},
		{name: "bad pack", mutate: func(config *testPayload) {
			config.Project.PackDirectory = "../assets"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := base
			config.Files = append([]testFile(nil), base.Files...)
			test.mutate(&config)
			if _, _, _, err := buildPayload(config); err == nil {
				t.Fatal("Build accepted invalid input")
			}
		})
	}
}

var errWriteFailure = errors.New("test writer failure")

type failingWriter struct {
	remaining int
}

func (w *failingWriter) Write(data []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errWriteFailure
	}
	if len(data) > w.remaining {
		written := w.remaining
		w.remaining = 0
		return written, errWriteFailure
	}
	w.remaining -= len(data)
	return len(data), nil
}

type changingReaderAt struct {
	mu        sync.Mutex
	first     []byte
	second    []byte
	completed bool
	changed   bool
}

func (r *changingReaderAt) ReadAt(data []byte, offset int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if offset == 0 && r.completed {
		r.changed = true
	}
	source := r.first
	if r.changed {
		source = r.second
	}
	if offset >= int64(len(source)) {
		return 0, io.EOF
	}
	count := copy(data, source[offset:])
	if offset+int64(count) == int64(len(source)) {
		r.completed = true
	}
	if count != len(data) {
		return count, io.EOF
	}
	return count, nil
}

type patternReaderAt struct {
	mu      sync.Mutex
	size    int64
	read    int64
	maxRead int
}

func (r *patternReaderAt) ReadAt(data []byte, offset int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(data) > r.maxRead {
		r.maxRead = len(data)
	}
	if offset >= r.size {
		return 0, io.EOF
	}
	count := len(data)
	if remaining := r.size - offset; int64(count) > remaining {
		count = int(remaining)
	}
	for i := 0; i < count; i++ {
		data[i] = byte((offset + int64(i)) % 251)
	}
	r.read += int64(count)
	if count != len(data) {
		return count, io.EOF
	}
	return count, nil
}

func (r *patternReaderAt) resetCounts() {
	r.mu.Lock()
	r.read = 0
	r.maxRead = 0
	r.mu.Unlock()
}

func (r *patternReaderAt) counts() (int64, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.read, r.maxRead
}

type countingReaderAt struct {
	mu      sync.Mutex
	reader  io.ReaderAt
	read    int64
	maxRead int
}

func (r *countingReaderAt) ReadAt(data []byte, offset int64) (int, error) {
	count, err := r.reader.ReadAt(data, offset)
	r.mu.Lock()
	r.read += int64(count)
	if len(data) > r.maxRead {
		r.maxRead = len(data)
	}
	r.mu.Unlock()
	return count, err
}

func (r *countingReaderAt) counts() (int64, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.read, r.maxRead
}

func fileSources(config testPayload) (BuildConfig, []FileSource) {
	sources := make([]FileSource, len(config.Files))
	for i, file := range config.Files {
		sources[i] = FileSource{
			Name: file.Name, Mode: file.Mode,
			ReaderAt: bytes.NewReader(file.Data), Size: int64(len(file.Data)),
		}
	}
	return config.BuildConfig, sources
}

func componentSources(sources []FileSource, prefix string) []FileSource {
	var component []FileSource
	for _, source := range sources {
		if strings.HasPrefix(source.Name, prefix) {
			source.Name = strings.TrimPrefix(source.Name, prefix)
			component = append(component, source)
		}
	}
	return component
}

func configFileData(t *testing.T, config testPayload, name string) []byte {
	t.Helper()
	for _, file := range config.Files {
		if file.Name == name {
			return file.Data
		}
	}
	t.Fatalf("missing test config file %q", name)
	return nil
}

func testBuildConfig(t *testing.T) testPayload {
	t.Helper()
	engineFiles := []testFile{
		{Name: "runtime-manifest.json", Mode: 0o644, Data: []byte(`{"schema":"test-engine/v1"}`)},
		{Name: "engine", Mode: 0o755, Data: []byte("engine")},
		{Name: "engine.pck", Mode: 0o644, Data: []byte("pack")},
	}
	bridgeFiles := []testFile{
		{Name: "bridge-manifest.json", Mode: 0o644, Data: []byte(`{"schema":"test-bridge/v1"}`)},
		{Name: "bridge.so", Mode: 0o755, Data: []byte("bridge")},
	}
	projectZIP, err := canonicalComponentZIP([]testFile{
		{Name: "main.spx", Mode: 0o644, Data: []byte("onStart => {}")},
		{Name: "assets/index.json", Mode: 0o644, Data: []byte(`{"zorder":[]}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	engineZIP, err := canonicalComponentZIP(engineFiles)
	if err != nil {
		t.Fatal(err)
	}
	bridgeZIP, err := canonicalComponentZIP(bridgeFiles)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := componentBundleBytes(engineZIP, runtimebundle.NamespaceEngine)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := componentBundleBytes(bridgeZIP, runtimebundle.NamespaceBridge)
	if err != nil {
		t.Fatal(err)
	}
	project, err := componentBundleBytes(projectZIP, runtimebundle.NamespaceProject)
	if err != nil {
		t.Fatal(err)
	}
	projectSum := sha256.Sum256(projectZIP)
	interfaceSum := sha256.Sum256([]byte("interface"))
	return testPayload{
		BuildConfig: BuildConfig{
			SPX:    SourceIdentity{SelectedPath: "github.com/goplus/spx/v4", EffectivePath: "github.com/goplus/spx/v4", SourceMode: true},
			Target: Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
			Engine: Engine{
				RuntimeVersion: "test", RuntimeABI: 1, EngineInterfaceDigest: hex.EncodeToString(interfaceSum[:]),
				Executable: "engine", Pack: "engine.pck", BundleDigest: engine.Digest,
			},
			Bridge: Bridge{File: "bridge.so", BundleDigest: bridge.Digest},
			Project: Project{
				PackDirectory: "assets", BundleDigest: project.Digest, ArchiveSHA256: hex.EncodeToString(projectSum[:]),
			},
		},
		Files: []testFile{
			{Name: "engine/runtime-manifest.json", Mode: 0o644, Data: engineFiles[0].Data},
			{Name: "engine/engine", Mode: 0o755, Data: engineFiles[1].Data},
			{Name: "engine/engine.pck", Mode: 0o644, Data: engineFiles[2].Data},
			{Name: "bridge/bridge-manifest.json", Mode: 0o644, Data: bridgeFiles[0].Data},
			{Name: "bridge/bridge.so", Mode: 0o755, Data: bridgeFiles[1].Data},
			{Name: ProjectZipPath, Mode: 0o644, Data: projectZIP},
		},
	}
}

func rewritePayload(t *testing.T, payload []byte, mutate func(*Manifest, []testFile)) ([]byte, string, string) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	var files []testFile
	var manifest Manifest
	for _, file := range reader.File {
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(input)
		closeErr := input.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if file.Name == ManifestPath {
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			continue
		}
		files = append(files, testFile{Name: file.Name, Mode: file.Mode(), Data: data})
	}
	mutate(&manifest, files)
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, testFile{Name: ManifestPath, Mode: 0o644, Data: manifestData})
	archive, err := canonicalComponentZIP(files)
	if err != nil {
		t.Fatal(err)
	}
	payloadSum := sha256.Sum256(archive)
	manifestSum := sha256.Sum256(manifestData)
	return archive, hex.EncodeToString(payloadSum[:]), hex.EncodeToString(manifestSum[:])
}

func TestBuildRejectsInvalidIdentityAndSourcesBeforeWriting(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*BuildConfig, *[]FileSource)
		want   string
	}{
		{"missing identity", func(c *BuildConfig, _ *[]FileSource) { c.SPX.SelectedPath = "" }, "selected SPX path is empty"},
		{"invalid ABI", func(c *BuildConfig, _ *[]FileSource) { c.Engine.RuntimeABI = 0 }, "ABI must be positive"},
		{"nonhex digest", func(c *BuildConfig, _ *[]FileSource) { c.Engine.BundleDigest = strings.Repeat("g", 64) }, "invalid engine bundle digest"},
		{"unsafe basename", func(c *BuildConfig, _ *[]FileSource) { c.Engine.Executable = "../engine" }, "unsafe component basename"},
		{"invalid path", func(_ *BuildConfig, s *[]FileSource) { (*s)[0].Name = "/absolute" }, "invalid entry path"},
		{"engine claim", func(c *BuildConfig, _ *[]FileSource) { c.Engine.BundleDigest = strings.Repeat("0", 64) }, "engine bundle digest does not match"},
		{"bridge claim", func(c *BuildConfig, _ *[]FileSource) { c.Bridge.BundleDigest = strings.Repeat("0", 64) }, "bridge bundle digest does not match"},
		{"project archive claim", func(c *BuildConfig, _ *[]FileSource) { c.Project.ArchiveSHA256 = strings.Repeat("0", 64) }, "project archive digest does not match"},
		{"project bundle claim", func(c *BuildConfig, _ *[]FileSource) { c.Project.BundleDigest = strings.Repeat("0", 64) }, "project bundle digest does not match"},
		{"missing engine", func(_ *BuildConfig, s *[]FileSource) { *s = (*s)[3:] }, "empty engine component"},
		{"missing bridge", func(_ *BuildConfig, s *[]FileSource) { *s = append((*s)[:3], (*s)[5:]...) }, "empty bridge component"},
		{"missing project", func(_ *BuildConfig, s *[]FileSource) { *s = (*s)[:5] }, "missing required entry"},
		{"invalid project ZIP", func(c *BuildConfig, s *[]FileSource) {
			data := []byte("not a ZIP")
			(*s)[5].ReaderAt, (*s)[5].Size = bytes.NewReader(data), int64(len(data))
			sum := sha256.Sum256(data)
			c.Project.ArchiveSHA256 = hex.EncodeToString(sum[:])
		}, "verify project archive"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config, sources := fileSources(testBuildConfig(t))
			tt.mutate(&config, &sources)
			var output bytes.Buffer
			payloadDigest, manifestDigest, err := BuildTo(&output, config, sources)
			if err == nil || !strings.Contains(err.Error(), tt.want) || output.Len() != 0 || payloadDigest != "" || manifestDigest != "" {
				t.Fatalf("BuildTo = %q/%q, %v; output=%d, want %q before writing", payloadDigest, manifestDigest, err, output.Len(), tt.want)
			}
		})
	}
	config, sources := fileSources(testBuildConfig(t))
	if _, _, err := BuildTo(nil, config, sources); err == nil || !strings.Contains(err.Error(), "nil payload writer") {
		t.Fatalf("nil writer = %v", err)
	}
}

func TestPrepareSourcesPreservesCallerMetadata(t *testing.T) {
	reader := &countingReaderAt{reader: bytes.NewReader([]byte("must not read"))}
	sources := []FileSource{
		{Name: "z", Mode: 0o700, ReaderAt: reader, Size: 2},
		{Name: "a", Mode: 0o600, ReaderAt: reader, Size: 1},
	}
	prepared, err := prepareSources(sources, ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if sources[0].Name != "z" || sources[0].Mode != 0o700 || sources[1].Name != "a" || sources[1].Mode != 0o600 {
		t.Fatalf("preparation mutated caller metadata: %+v", sources)
	}
	for i, want := range []runtimebundle.Entry{
		{Name: "a", Mode: 0o644, Size: 1},
		{Name: "z", Mode: 0o755, Size: 2},
	} {
		if prepared[i].entry != want {
			t.Fatalf("entry %d = %+v, want %+v", i, prepared[i].entry, want)
		}
		source := prepared[i].source
		if source.Name != want.Name || uint32(source.Mode) != want.Mode || source.Size != want.Size || source.ReaderAt != reader {
			t.Fatalf("prepared source %d does not match its metadata: %+v", i, source)
		}
	}
	// Duplicates need not be adjacent in the caller's input.
	sources = append(sources, sources[0])
	if _, err := prepareSources(sources, ManifestPath); err == nil || err.Error() != `runtimepayload: duplicate entry "z"` {
		t.Fatalf("nonadjacent duplicate = %v", err)
	}
	if reads, _ := reader.counts(); reads != 0 {
		t.Fatalf("metadata preparation read sources %d times", reads)
	}
}

func TestComponentBundleSourcesRejectsInvalidInputs(t *testing.T) {
	good := FileSource{Name: "asset", ReaderAt: bytes.NewReader([]byte("x")), Size: 1}
	for _, tt := range []struct {
		name    string
		sources []FileSource
		want    string
	}{
		{"empty", nil, "empty"},
		{"invalid path", []FileSource{{Name: "../asset"}}, "invalid entry path"},
		{"duplicate", []FileSource{good, good}, "duplicate entry"},
		{"nil reader", []FileSource{{Name: "asset"}}, "nil ReaderAt"},
		{"negative size", []FileSource{{Name: "asset", ReaderAt: good.ReaderAt, Size: -1}}, "negative size"},
		{"short read", []FileSource{{Name: "asset", ReaderAt: good.ReaderAt, Size: 2}}, "short read"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if bundle, err := ComponentBundleSources(tt.sources, runtimebundle.NamespaceEngine); bundle.Digest != "" || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ComponentBundleSources = %+v, %v; want %q", bundle, err, tt.want)
			}
		})
	}
	good.Name = ManifestPath
	if _, err := ComponentBundleSources([]FileSource{good}, runtimebundle.NamespaceProject); err != nil {
		t.Fatalf("component sources must not reserve the outer manifest path: %v", err)
	}
}

func TestVerifyRejectsMalformedMetadataAndClaims(t *testing.T) {
	payload, _, _ := newTestPayload(t)
	for _, tt := range []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{"schema", func(m *Manifest) { m.Schema = "unknown" }, "unsupported manifest schema/protocol"},
		{"protocol", func(m *Manifest) { m.Protocol = "unknown" }, "unsupported manifest schema/protocol"},
		{"missing identity", func(m *Manifest) { m.SPX.EffectivePath = "" }, "effective SPX path is empty"},
		{"missing entry table", func(m *Manifest) { m.Entries = nil }, "entry table does not match"},
		{"required executable", func(m *Manifest) { m.Engine.Executable = "missing" }, "missing required entry"},
		{"archive claim", func(m *Manifest) { m.Project.ArchiveSHA256 = strings.Repeat("0", 64) }, "project archive SHA-256 mismatch"},
		{"project claim", func(m *Manifest) { m.Project.BundleDigest = strings.Repeat("0", 64) }, "project bundle digest mismatch"},
		{"bridge claim", func(m *Manifest) { m.Bridge.BundleDigest = strings.Repeat("0", 64) }, "bridge bundle digest mismatch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			changed, payloadDigest, manifestDigest := rewritePayload(t, payload, func(m *Manifest, _ []testFile) { tt.mutate(m) })
			if result, err := Verify(changed, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH); result != nil || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Verify = %v, %v; want %q", result, err, tt.want)
			}
		})
	}
}

func TestVerifyReaderAtRejectsInvalidInputs(t *testing.T) {
	payload, payloadDigest, manifestDigest := newTestPayload(t)
	for _, tt := range []struct {
		name                          string
		reader                        io.ReaderAt
		size                          int64
		payloadDigest, manifestDigest string
		want                          string
	}{
		{"nil reader", nil, 0, payloadDigest, manifestDigest, "nil payload reader"},
		{"negative size", bytes.NewReader(payload), -1, payloadDigest, manifestDigest, "negative payload size"},
		{"invalid payload digest", bytes.NewReader(payload), int64(len(payload)), "bad", manifestDigest, "invalid expected payload SHA-256"},
		{"invalid manifest digest", bytes.NewReader(payload), int64(len(payload)), payloadDigest, "bad", "invalid expected manifest SHA-256"},
		{"truncated reader", bytes.NewReader(payload[:10]), int64(len(payload)), payloadDigest, manifestDigest, "hash payload"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if result, err := VerifyReaderAt(tt.reader, tt.size, tt.payloadDigest, tt.manifestDigest, runtime.GOOS, runtime.GOARCH); result != nil || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("VerifyReaderAt = %v, %v; want %q", result, err, tt.want)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		files []testFile
		want  string
	}{
		{"missing manifest", testBuildConfig(t).Files, "missing " + ManifestPath},
		{"malformed manifest", []testFile{{Name: ManifestPath, Data: []byte("{")}}, "decode manifest"},
		{"oversized manifest", []testFile{{Name: ManifestPath, Data: bytes.Repeat([]byte(" "), maxPayloadManifestBytes+1)}}, "read manifest"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := canonicalComponentZIP(tt.files)
			if err != nil {
				t.Fatal(err)
			}
			payloadSum := sha256.Sum256(data)
			manifestSum := sha256.Sum256(tt.files[0].Data)
			if result, err := Verify(data, hex.EncodeToString(payloadSum[:]), hex.EncodeToString(manifestSum[:]), runtime.GOOS, runtime.GOARCH); result != nil || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Verify = %v, %v; want %q", result, err, tt.want)
			}
		})
	}
	invalid := []byte("not a ZIP")
	sum := sha256.Sum256(invalid)
	if _, err := Verify(invalid, hex.EncodeToString(sum[:]), manifestDigest, runtime.GOOS, runtime.GOARCH); !errors.Is(err, runtimebundle.ErrUnsafeArchive) {
		t.Fatalf("invalid ZIP = %v", err)
	}
}

func TestVerifiedWritersRejectInvalidRequestsAndWriteFailures(t *testing.T) {
	payload, payloadDigest, manifestDigest := newTestPayload(t)
	verified, err := Verify(payload, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		call func() error
		want string
	}{
		{"nil component", func() error { return (*Verified)(nil).WriteComponentZIP("engine/", io.Discard) }, "invalid component prefix"},
		{"invalid prefix", func() error { return verified.WriteComponentZIP("project/", io.Discard) }, "invalid component prefix"},
		{"nil component writer", func() error { return verified.WriteComponentZIP("engine/", nil) }, "nil component writer"},
		{"empty component", func() error { return (&Verified{}).WriteComponentZIP("engine/", io.Discard) }, "empty component"},
		{"missing component", func() error { return (&Verified{entries: verified.entries}).WriteComponentZIP("engine/", io.Discard) }, "missing component entry"},
		{"nil project", func() error { return (*Verified)(nil).WriteProjectZIP(io.Discard) }, "nil verified payload"},
		{"nil project writer", func() error { return verified.WriteProjectZIP(nil) }, "nil project writer"},
		{"missing project", func() error { return (&Verified{}).WriteProjectZIP(io.Discard) }, "missing required entry"},
		{"component sink failure", func() error { return verified.WriteComponentZIP("engine/", &failingWriter{}) }, "close component ZIP"},
		{"project sink failure", func() error { return verified.WriteProjectZIP(&failingWriter{}) }, "copy entry"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
	for _, kind := range []string{"size", "digest"} {
		t.Run("changed project "+kind, func(t *testing.T) {
			copy := *verified
			copy.entries = append([]runtimebundle.Entry(nil), verified.entries...)
			for i := range copy.entries {
				if copy.entries[i].Name == ProjectZipPath {
					if kind == "size" {
						copy.entries[i].Size++
					} else {
						copy.entries[i].SHA256 = strings.Repeat("0", 64)
					}
				}
			}
			if err := copy.WriteProjectZIP(io.Discard); err == nil || !strings.Contains(err.Error(), kind+" changed") {
				t.Fatalf("changed entry = %v, want %s failure", err, kind)
			}
		})
	}
}

func TestEstimatePayloadArchiveSizeZIP64AndOverflow(t *testing.T) {
	const maxInt64 = int64(^uint64(0) >> 1)
	for _, tt := range []struct {
		name    string
		entries []runtimebundle.Entry
		want    int64
		wantErr string
	}{
		{"empty", nil, 22, ""},
		{"one byte", []runtimebundle.Entry{{Name: "x", Size: 1}}, 135, ""},
		{"ZIP64 entry", []runtimebundle.Entry{{Name: "x", Size: int64(zip32Max)}}, int64(zip32Max) + 246, ""},
		{"largest representable", []runtimebundle.Entry{{Name: "x", Size: maxInt64 - 246}}, maxInt64, ""},
		{"negative entry", []runtimebundle.Entry{{Name: "x", Size: -1}}, 0, "negative size"},
		{"long name", []runtimebundle.Entry{{Name: strings.Repeat("x", zipMaxNameBytes+1)}}, 0, "name is too long"},
		{"local content overflow", []runtimebundle.Entry{{Name: "x", Size: maxInt64}}, 0, "overflow"},
		{"descriptor overflow", []runtimebundle.Entry{{Name: "x", Size: maxInt64 - 40}}, 0, "overflow"},
		{"offset overflow", []runtimebundle.Entry{{Name: "a", Size: maxInt64 / 2}, {Name: "b", Size: maxInt64 / 2}}, 0, "overflow"},
		{"central directory overflow", []runtimebundle.Entry{{Name: "x", Size: maxInt64 - 64}}, 0, "overflow"},
		{"end record overflow", []runtimebundle.Entry{{Name: "x", Size: maxInt64 - 148}}, 0, "overflow"},
		{"ZIP64 end overflow", []runtimebundle.Entry{{Name: "x", Size: maxInt64 - 170}}, 0, "overflow"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := estimatePayloadArchiveSize(tt.entries)
			if tt.wantErr != "" {
				if got != 0 || err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("estimate = %d, %v; want %q", got, err, tt.wantErr)
				}
			} else if err != nil || got != tt.want {
				t.Fatalf("estimate = %d, %v; want %d", got, err, tt.want)
			}
		})
	}
	if _, err := addPayloadSize(-1, 1); err == nil {
		t.Fatal("negative summand accepted")
	}
}

func TestBuildToPropagatesFailuresDuringZIPWriting(t *testing.T) {
	for _, tt := range []struct {
		name string
		size int64
		want string
	}{
		// Fill archive/zip's 4096-byte buffer at the end of the bridge
		// manifest, or overflow it while writing that entry's body.
		{"next header", 4096, "create entry"},
		{"entry body", 8192, "write entry"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config, sources := fileSources(testBuildConfig(t))
			resizeBridgeManifest := func(size int64) {
				for i := range sources {
					if sources[i].Name == "bridge/bridge-manifest.json" {
						sources[i].ReaderAt = bytes.NewReader(bytes.Repeat([]byte("x"), int(size)))
						sources[i].Size = size
					}
				}
				bridge, err := ComponentBundleSources(componentSources(sources, "bridge/"), runtimebundle.NamespaceBridge)
				if err != nil {
					t.Fatal(err)
				}
				config.Bridge.BundleDigest = bridge.Digest
			}
			resizeBridgeManifest(tt.size)
			if tt.name == "next header" {
				var fixture bytes.Buffer
				if _, _, err := BuildTo(&fixture, config, sources); err != nil {
					t.Fatal(err)
				}
				archive, err := zip.NewReader(bytes.NewReader(fixture.Bytes()), int64(fixture.Len()))
				if err != nil {
					t.Fatal(err)
				}
				for _, file := range archive.File {
					if file.Name == "bridge/bridge-manifest.json" {
						offset, err := file.DataOffset()
						if err != nil {
							t.Fatal(err)
						}
						resizeBridgeManifest(4096 - offset)
					}
				}
			}
			payloadDigest, manifestDigest, err := BuildTo(&failingWriter{}, config, sources)
			if !errors.Is(err, errWriteFailure) || !strings.Contains(err.Error(), tt.want) || payloadDigest != "" || manifestDigest != "" {
				t.Fatalf("BuildTo = %q/%q, %v; want %q write failure", payloadDigest, manifestDigest, err, tt.want)
			}
		})
	}
}

type payloadReaderAtFunc func([]byte, int64) (int, error)

func (f payloadReaderAtFunc) ReadAt(data []byte, offset int64) (int, error) { return f(data, offset) }

func TestSourceReadsPropagateErrorsAndRejectTruncation(t *testing.T) {
	readErr := errors.New("source read failure")
	failed := payloadReaderAtFunc(func([]byte, int64) (int, error) { return 0, readErr })
	for _, tt := range []struct {
		name   string
		reader io.ReaderAt
		size   int64
		want   error
	}{
		{"read failure", failed, 1, readErr},
		{"truncated", bytes.NewReader([]byte("x")), 2, io.ErrUnexpectedEOF},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := FileSource{Name: "x", ReaderAt: tt.reader, Size: tt.size}
			for _, operation := range []func(FileSource) (string, error){hashSource, func(s FileSource) (string, error) { return writeSource(io.Discard, s) }} {
				if digest, err := operation(source); digest != "" || !errors.Is(err, tt.want) {
					t.Fatalf("source read = %q, %v; want %v", digest, err, tt.want)
				}
			}
		})
	}
}

func TestVerifyDetectsSourceChangeOnFinalDigestPass(t *testing.T) {
	payload, payloadDigest, manifestDigest := newTestPayload(t)
	fullReads := 0
	reader := payloadReaderAtFunc(func(data []byte, offset int64) (int, error) {
		n, err := bytes.NewReader(payload).ReadAt(data, offset)
		if offset == 0 && len(data) == len(payload) {
			fullReads++
			if fullReads > 1 {
				data[0] ^= 1
			}
		}
		return n, err
	})
	if result, err := VerifyReaderAt(reader, int64(len(payload)), payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH); result != nil || err == nil || !strings.Contains(err.Error(), "payload source changed during verification") || fullReads < 2 {
		t.Fatalf("VerifyReaderAt = %v, %v; full reads=%d", result, err, fullReads)
	}
}

type payloadCloseErrorReader struct {
	io.Reader
	err    error
	closes int
}

func (r *payloadCloseErrorReader) Close() error { r.closes++; return r.err }

func TestZipEntryReadsPreserveOpenReadAndCloseErrors(t *testing.T) {
	data, err := canonicalComponentZIP([]testFile{{Name: "entry", Data: []byte("payload")}})
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("entry stream failure")
	for _, phase := range []string{"open", "read", "close"} {
		for _, operation := range []string{"small entry", "verified copy"} {
			t.Run(operation+"/"+phase, func(t *testing.T) {
				archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
				if err != nil {
					t.Fatal(err)
				}
				file := archive.File[0]
				file.Method = 99
				stream := &payloadCloseErrorReader{Reader: bytes.NewReader([]byte("payload"))}
				want := failure
				if phase == "open" {
					want = zip.ErrAlgorithm
				} else {
					if phase == "close" {
						stream.err = failure
					} else {
						stream.Reader = iotest.ErrReader(failure)
					}
					archive.RegisterDecompressor(99, func(io.Reader) io.ReadCloser { return stream })
				}
				if operation == "small entry" {
					_, err = readSmallZipEntry(file, 100)
				} else {
					sum := sha256.Sum256([]byte("payload"))
					err = copyVerifiedZipEntry(io.Discard, file, runtimebundle.Entry{Name: "entry", Size: 7, SHA256: hex.EncodeToString(sum[:])})
				}
				if !errors.Is(err, want) {
					t.Fatalf("entry error = %v, want %v", err, want)
				}
				if phase != "open" && stream.closes != 1 {
					t.Fatalf("stream closes = %d, want 1", stream.closes)
				}
			})
		}
	}
}

func TestStoredProjectEntryRejectsNoncanonicalZIP(t *testing.T) {
	if _, _, err := (*Verified)(nil).storedEntryReaderAt(nil); err == nil {
		t.Fatal("nil stored entry accepted")
	}
	payload, payloadDigest, manifestDigest := newTestPayload(t)
	for _, phase := range []string{"compression method", "size mismatch", "missing component"} {
		t.Run(phase, func(t *testing.T) {
			verified, err := Verify(payload, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				t.Fatal(err)
			}
			want := "canonical ZIP store mode"
			switch phase {
			case "compression method":
				verified.files[ProjectZipPath].Method = zip.Deflate
			case "size mismatch":
				verified.files[ProjectZipPath].CompressedSize64++
			case "missing component":
				delete(verified.files, ProjectZipPath)
				want = "component entries"
			}
			if err := verified.validateComponents(); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("component validation = %v, want %q", err, want)
			}
		})
	}
}

func TestVerifyRejectsMalformedProjectInsideAuthenticatedPayload(t *testing.T) {
	payload, _, _ := newTestPayload(t)
	changed, payloadDigest, manifestDigest := rewritePayload(t, payload, func(manifest *Manifest, files []testFile) {
		invalidProject := []byte("not a project ZIP")
		sum := sha256.Sum256(invalidProject)
		digest := hex.EncodeToString(sum[:])
		for i := range files {
			if files[i].Name == ProjectZipPath {
				files[i].Data = invalidProject
			}
		}
		for i := range manifest.Entries {
			if manifest.Entries[i].Name == ProjectZipPath {
				manifest.Entries[i].Size = int64(len(invalidProject))
				manifest.Entries[i].SHA256 = digest
			}
		}
		manifest.Project.ArchiveSHA256 = digest
	})
	// The outer ZIP, payload digest, manifest digest, entry table, and project
	// archive digest are all consistent. The embedded ZIP still needs validation.
	if verified, err := Verify(changed, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH); verified != nil || !errors.Is(err, runtimebundle.ErrUnsafeArchive) || !strings.Contains(err.Error(), "verify project archive") {
		t.Fatalf("Verify malformed embedded project = %v, %v", verified, err)
	}
}

func TestVerifiedWritersDetectBackingSourceCorruption(t *testing.T) {
	for _, tt := range []struct {
		name, entry string
		write       func(*Verified, io.Writer) error
	}{
		{"engine", "engine/engine", func(v *Verified, dst io.Writer) error { return v.WriteComponentZIP("engine/", dst) }},
		{"bridge", "bridge/bridge.so", func(v *Verified, dst io.Writer) error { return v.WriteComponentZIP("bridge/", dst) }},
		{"project", ProjectZipPath, (*Verified).WriteProjectZIP},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payload, payloadDigest, manifestDigest := newTestPayload(t)
			verified, err := Verify(payload, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				t.Fatal(err)
			}
			offset, err := verified.files[tt.entry].DataOffset()
			if err != nil {
				t.Fatal(err)
			}
			// VerifyReaderAt intentionally retains its source instead of copying it.
			// A later change must not be silently written out as a trusted component.
			payload[offset] ^= 1
			if err := tt.write(verified, io.Discard); !errors.Is(err, zip.ErrChecksum) || !strings.Contains(err.Error(), tt.entry) {
				t.Fatalf("write corrupted %s = %v, want entry-specific checksum failure", tt.entry, err)
			}
			payload[offset] ^= 1
			if err := tt.write(verified, io.Discard); err != nil {
				t.Fatalf("write restored %s = %v", tt.entry, err)
			}
		})
	}
}
