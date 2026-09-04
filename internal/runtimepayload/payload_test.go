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

	"github.com/goplus/spx/v3/internal/runtimebundle"
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

func TestEngineComponentManifestName(t *testing.T) {
	if EngineComponentManifestName != "engine-component-manifest.json" {
		t.Fatalf("Engine component manifest name = %q", EngineComponentManifestName)
	}
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
		header.SetModTime(canonicalTime)
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

func TestBuildToRejectsOuterLimitsBeforeReadingSources(t *testing.T) {
	t.Run("entry count", func(t *testing.T) {
		config, sources := fileSources(testBuildConfig(t))
		guard := &countingReaderAt{reader: bytes.NewReader([]byte("must not read"))}
		sources = make([]FileSource, runtimebundle.MaxEntries)
		sources[0] = FileSource{Name: "entry", ReaderAt: guard, Size: 0}
		if _, _, err := BuildTo(io.Discard, config, sources); !errors.Is(err, runtimebundle.ErrArchiveLimit) {
			t.Fatalf("BuildTo error = %v, want runtimebundle.ErrArchiveLimit", err)
		}
		if reads, _ := guard.counts(); reads != 0 {
			t.Fatalf("oversized entry-count source bytes read = %d, want 0", reads)
		}
	})

	t.Run("entry size", func(t *testing.T) {
		config, sources := fileSources(testBuildConfig(t))
		guard := &countingReaderAt{reader: bytes.NewReader([]byte("must not read"))}
		sources[0].ReaderAt = guard
		sources[0].Size = runtimebundle.MaxEntrySize + 1
		if _, _, err := BuildTo(io.Discard, config, sources); !errors.Is(err, runtimebundle.ErrArchiveLimit) {
			t.Fatalf("BuildTo error = %v, want runtimebundle.ErrArchiveLimit", err)
		}
		if reads, _ := guard.counts(); reads != 0 {
			t.Fatalf("oversized entry source bytes read = %d, want 0", reads)
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
	payload, _, _, err := buildPayload(testBuildConfig(t))
	if err != nil {
		t.Fatal(err)
	}
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
	payload, payloadDigest, manifestDigest, err := buildPayload(testBuildConfig(t))
	if err != nil {
		t.Fatal(err)
	}

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
		changed, changedPayloadDigest, changedManifestDigest := rewritePayloadManifest(t, payload, func(manifest *Manifest) {
			manifest.Entries[0].SHA256 = strings.Repeat("0", sha256.Size*2)
		})
		if _, err := Verify(changed, changedPayloadDigest, changedManifestDigest, runtime.GOOS, runtime.GOARCH); err == nil || !strings.Contains(err.Error(), "entry table") {
			t.Fatalf("Verify error = %v", err)
		}
	})
	t.Run("component identity", func(t *testing.T) {
		changed, changedPayloadDigest, changedManifestDigest := rewritePayloadManifest(t, payload, func(manifest *Manifest) {
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
		{Name: EngineComponentManifestName, Mode: 0o644, Data: []byte(`{"schema":"test-engine/v1"}`)},
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
			SPX:    SourceIdentity{SelectedPath: "github.com/goplus/spx/v3", EffectivePath: "github.com/goplus/spx/v3", SourceMode: true},
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
			{Name: "engine/" + EngineComponentManifestName, Mode: 0o644, Data: engineFiles[0].Data},
			{Name: "engine/engine", Mode: 0o755, Data: engineFiles[1].Data},
			{Name: "engine/engine.pck", Mode: 0o644, Data: engineFiles[2].Data},
			{Name: "bridge/bridge-manifest.json", Mode: 0o644, Data: bridgeFiles[0].Data},
			{Name: "bridge/bridge.so", Mode: 0o755, Data: bridgeFiles[1].Data},
			{Name: ProjectZipPath, Mode: 0o644, Data: projectZIP},
		},
	}
}

func rewritePayloadManifest(t *testing.T, payload []byte, mutate func(*Manifest)) ([]byte, string, string) {
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
	mutate(&manifest)
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
