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

package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v3/internal/strictjson"
)

const LocalRuntimeManifestSchema = "spx-local-engine/v1"

// LocalRuntimeFile is one explicitly declared local Engine/PCK output. Name
// must be a basename; the file is resolved relative to the manifest directory.
type LocalRuntimeFile struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// LocalRuntimeManifest describes a runtime produced by a local build workflow.
// It is deliberately separate from RuntimeManifest: a local manifest proves
// consistency with an explicitly selected local output, not publication.
type LocalRuntimeManifest struct {
	Schema         string           `json:"schema"`
	Mode           string           `json:"mode"`
	RuntimeVersion string           `json:"runtime_version"`
	RuntimeABI     int              `json:"runtime_abi"`
	LockSHA256     string           `json:"lock_sha256"`
	GOOS           string           `json:"goos"`
	GOARCH         string           `json:"goarch"`
	Engine         LocalRuntimeFile `json:"engine"`
	Pack           LocalRuntimeFile `json:"pack"`
}

// ParseLocalRuntimeManifest decodes and structurally validates a local
// manifest. ValidateForLock performs the host/runtime-specific checks.
func ParseLocalRuntimeManifest(data []byte) (LocalRuntimeManifest, error) {
	var manifest LocalRuntimeManifest
	if err := strictjson.Decode(data, &manifest); err != nil {
		return LocalRuntimeManifest{}, fmt.Errorf("decode local runtime manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return LocalRuntimeManifest{}, err
	}
	return manifest, nil
}

// LoadLocalRuntimeManifest reads and parses a local manifest file.
func LoadLocalRuntimeManifest(path string) (LocalRuntimeManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LocalRuntimeManifest{}, fmt.Errorf("read local runtime manifest: %w", err)
	}
	return ParseLocalRuntimeManifest(data)
}

func (m LocalRuntimeManifest) Validate() error {
	if m.Schema != LocalRuntimeManifestSchema {
		return fmt.Errorf("release: local runtime manifest schema = %q, want %q", m.Schema, LocalRuntimeManifestSchema)
	}
	if m.Mode != "local" {
		return fmt.Errorf("release: local runtime manifest mode = %q, want local", m.Mode)
	}
	if !runtimeVersionPattern.MatchString(m.RuntimeVersion) {
		return fmt.Errorf("release: invalid local runtime version %q", m.RuntimeVersion)
	}
	if m.RuntimeABI <= 0 {
		return fmt.Errorf("release: local runtime ABI must be positive")
	}
	if !isLowerHexDigest(m.LockSHA256, sha256.Size*2) {
		return fmt.Errorf("release: invalid local runtime lock SHA-256 %q", m.LockSHA256)
	}
	if m.GOOS == "" || strings.ContainsAny(m.GOOS, `/\\`) || m.GOARCH == "" || strings.ContainsAny(m.GOARCH, `/\\`) {
		return fmt.Errorf("release: invalid local runtime target %q/%q", m.GOOS, m.GOARCH)
	}
	if err := validateLocalRuntimeFile("engine", m.Engine); err != nil {
		return err
	}
	if err := validateLocalRuntimeFile("pack", m.Pack); err != nil {
		return err
	}
	return nil
}

func validateLocalRuntimeFile(label string, file LocalRuntimeFile) error {
	if file.Name == "" || file.Name == "." || file.Name == ".." || filepath.Base(file.Name) != file.Name || strings.ContainsAny(file.Name, `/\\`) {
		return fmt.Errorf("release: local runtime %s has invalid name %q", label, file.Name)
	}
	if file.Size <= 0 {
		return fmt.Errorf("release: local runtime %s size must be positive", label)
	}
	if !isLowerHexDigest(file.SHA256, sha256.Size*2) {
		return fmt.Errorf("release: local runtime %s has invalid SHA-256 %q", label, file.SHA256)
	}
	return nil
}

// ValidateForLock verifies the local manifest's runtime and target identity.
func (m LocalRuntimeManifest) ValidateForLock(lock RuntimeLock, goos, goarch string) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}
	lockSHA, err := lock.SHA256()
	if err != nil {
		return err
	}
	if m.RuntimeVersion != lock.RuntimeVersion || m.RuntimeABI != lock.RuntimeABI {
		return fmt.Errorf("release: local runtime identity does not match lock")
	}
	if m.LockSHA256 != lockSHA {
		return fmt.Errorf("release: local runtime lock SHA-256 %q does not match %q", m.LockSHA256, lockSHA)
	}
	if m.GOOS != goos || m.GOARCH != goarch {
		return fmt.Errorf("release: local runtime target %s/%s does not match host %s/%s", m.GOOS, m.GOARCH, goos, goarch)
	}
	spec, err := HostRuntimeSpecFor(lock, goos, goarch)
	if err != nil {
		return err
	}
	if m.Engine.Name != localRuntimeObjectName(spec.RuntimeName, m.Engine.SHA256) || m.Pack.Name != localRuntimeObjectName(spec.PackName, m.Pack.SHA256) {
		return fmt.Errorf("release: local runtime file names do not match locked host runtime")
	}
	return nil
}

// VerifyFiles verifies the declared local files beneath manifestDir using
// no-follow/open/re-stat checks and exact size/digest comparisons.
func (m LocalRuntimeManifest) VerifyFiles(manifestDir string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	for label, file := range map[string]LocalRuntimeFile{"engine": m.Engine, "pack": m.Pack} {
		path := filepath.Join(manifestDir, file.Name)
		if err := verifyLocalRuntimeFile(path, file); err != nil {
			return fmt.Errorf("release: verify local runtime %s: %w", label, err)
		}
	}
	return nil
}

func verifyLocalRuntimeFile(path string, want LocalRuntimeFile) error {
	got, err := readVerifiedLocalRuntimeFile(path, nil)
	if err != nil {
		return err
	}
	if got.Size != want.Size {
		return fmt.Errorf("%s size = %d, want %d", path, got.Size, want.Size)
	}
	if got.SHA256 != want.SHA256 {
		return fmt.Errorf("%s SHA-256 = %s, want %s", path, got.SHA256, want.SHA256)
	}
	return nil
}

// JSON returns canonical local manifest bytes.
func (m LocalRuntimeManifest) JSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode local runtime manifest: %w", err)
	}
	return append(data, '\n'), nil
}

// LocalRuntimeManifestPath returns the deterministic source-mode manifest
// location for one locked host runtime.
func LocalRuntimeManifestPath(repoRoot string, lock RuntimeLock, goos, goarch string) (string, error) {
	if repoRoot == "" || !filepath.IsAbs(repoRoot) || filepath.Clean(repoRoot) != repoRoot {
		return "", fmt.Errorf("release: local runtime repository root must be absolute and clean: %q", repoRoot)
	}
	spec, err := HostRuntimeSpecFor(lock, goos, goarch)
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, ".spx", "runtime", lock.RuntimeVersion, spec.GOOS+"-"+spec.GOARCH, "engine-manifest.json"), nil
}

// WriteLocalRuntimeManifest writes a validated local runtime manifest using a
// temporary file and rename so readers never observe a partially written JSON
// document.
func WriteLocalRuntimeManifest(path string, manifest LocalRuntimeManifest) error {
	data, err := manifest.JSON()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create local runtime manifest directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".engine-manifest-*")
	if err != nil {
		return fmt.Errorf("create local runtime manifest: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod local runtime manifest: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write local runtime manifest: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync local runtime manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close local runtime manifest: %w", err)
	}
	if err := replaceLocalRuntimeFile(tmpPath, path, 0o644); err != nil {
		return fmt.Errorf("install local runtime manifest: %w", err)
	}
	return nil
}

// PublishLocalRuntimeManifest copies the verified local Engine and PCK beside
// path, then writes the manifest last. The manifest's basename references are
// therefore self-contained and never point back into GOPATH/bin.
func PublishLocalRuntimeManifest(path string, manifest LocalRuntimeManifest, enginePath, packPath string) error {
	lock, err := RuntimeLockForVersion(manifest.RuntimeVersion)
	if err != nil {
		return err
	}
	if err := manifest.ValidateForLock(lock, manifest.GOOS, manifest.GOARCH); err != nil {
		return err
	}
	// Verify both sources before publishing either content-addressed object.
	// The copy below repeats this check while holding each source open.
	if err := verifyLocalRuntimeFile(enginePath, manifest.Engine); err != nil {
		return fmt.Errorf("verify local runtime Engine before publish: %w", err)
	}
	if err := verifyLocalRuntimeFile(packPath, manifest.Pack); err != nil {
		return fmt.Errorf("verify local runtime PCK before publish: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create local runtime manifest directory: %w", err)
	}
	if err := copyVerifiedLocalRuntimeFile(enginePath, filepath.Join(dir, manifest.Engine.Name), manifest.Engine, 0o755); err != nil {
		return fmt.Errorf("publish local runtime Engine: %w", err)
	}
	if err := copyVerifiedLocalRuntimeFile(packPath, filepath.Join(dir, manifest.Pack.Name), manifest.Pack, 0o644); err != nil {
		return fmt.Errorf("publish local runtime PCK: %w", err)
	}
	if err := WriteLocalRuntimeManifest(path, manifest); err != nil {
		return err
	}
	if err := manifest.VerifyFiles(dir); err != nil {
		return fmt.Errorf("verify published local runtime: %w", err)
	}
	return nil
}

// NewLocalRuntimeManifest constructs a manifest for two already-built files.
func NewLocalRuntimeManifest(lock RuntimeLock, goos, goarch, enginePath, packPath string) (LocalRuntimeManifest, error) {
	lockSHA, err := lock.SHA256()
	if err != nil {
		return LocalRuntimeManifest{}, err
	}
	spec, err := HostRuntimeSpecFor(lock, goos, goarch)
	if err != nil {
		return LocalRuntimeManifest{}, err
	}
	if filepath.Base(enginePath) != spec.RuntimeName || filepath.Base(packPath) != spec.PackName {
		return LocalRuntimeManifest{}, fmt.Errorf("release: local runtime source file names do not match locked host runtime")
	}
	engine, err := hashLocalRuntimeFile(enginePath)
	if err != nil {
		return LocalRuntimeManifest{}, fmt.Errorf("hash local Engine: %w", err)
	}
	pack, err := hashLocalRuntimeFile(packPath)
	if err != nil {
		return LocalRuntimeManifest{}, fmt.Errorf("hash local runtime PCK: %w", err)
	}
	engine.Name = localRuntimeObjectName(spec.RuntimeName, engine.SHA256)
	pack.Name = localRuntimeObjectName(spec.PackName, pack.SHA256)
	manifest := LocalRuntimeManifest{
		Schema: LocalRuntimeManifestSchema, Mode: "local", RuntimeVersion: lock.RuntimeVersion,
		RuntimeABI: lock.RuntimeABI, LockSHA256: lockSHA, GOOS: goos, GOARCH: goarch,
		Engine: engine, Pack: pack,
	}
	if err := manifest.ValidateForLock(lock, goos, goarch); err != nil {
		return LocalRuntimeManifest{}, err
	}
	return manifest, nil
}

func localRuntimeObjectName(name, digest string) string {
	extension := filepath.Ext(name)
	base := strings.TrimSuffix(name, extension)
	return base + "." + digest + extension
}

func hashLocalRuntimeFile(path string) (LocalRuntimeFile, error) {
	return readVerifiedLocalRuntimeFile(path, nil)
}

// readVerifiedLocalRuntimeFile reads a regular non-symlink file after checking
// that the path still names the file that was opened. The optional sink is
// created after the source passes its opening checks and receives the bytes
// before they are hashed.
func readVerifiedLocalRuntimeFile(path string, newSink func() (io.Writer, error)) (LocalRuntimeFile, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return LocalRuntimeFile{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return LocalRuntimeFile{}, fmt.Errorf("%s is not a regular non-symlink file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return LocalRuntimeFile{}, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return LocalRuntimeFile{}, err
	}
	if !os.SameFile(info, opened) {
		_ = file.Close()
		return LocalRuntimeFile{}, fmt.Errorf("%s changed while opening", path)
	}
	hasher := sha256.New()
	var writer io.Writer = hasher
	if newSink != nil {
		sink, err := newSink()
		if err != nil {
			_ = file.Close()
			return LocalRuntimeFile{}, err
		}
		writer = io.MultiWriter(sink, writer)
	}
	size, err := io.Copy(writer, file)
	closeErr := file.Close()
	if err != nil {
		return LocalRuntimeFile{}, err
	}
	if closeErr != nil {
		return LocalRuntimeFile{}, closeErr
	}
	after, err := os.Lstat(path)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) || after.Size() != size {
		return LocalRuntimeFile{}, fmt.Errorf("%s changed while reading", path)
	}
	return LocalRuntimeFile{Name: filepath.Base(path), Size: size, SHA256: hex.EncodeToString(hasher.Sum(nil))}, nil
}

func copyVerifiedLocalRuntimeFile(src, dst string, want LocalRuntimeFile, mode os.FileMode) (err error) {
	var tmp *os.File
	var tmpPath string
	defer func() {
		if tmp != nil {
			_ = tmp.Close()
		}
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()
	got, err := readVerifiedLocalRuntimeFile(src, func() (io.Writer, error) {
		var err error
		tmp, err = os.CreateTemp(filepath.Dir(dst), ".local-runtime-file-*")
		if err != nil {
			return nil, err
		}
		tmpPath = tmp.Name()
		return tmp, nil
	})
	if err != nil {
		return err
	}
	if got.Size != want.Size {
		_ = tmp.Close()
		return fmt.Errorf("%s size = %d, want %d", src, got.Size, want.Size)
	}
	if got.SHA256 != want.SHA256 {
		_ = tmp.Close()
		return fmt.Errorf("%s SHA-256 = %s, want %s", src, got.SHA256, want.SHA256)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := replaceLocalRuntimeFile(tmpPath, dst, mode); err != nil {
		return err
	}
	return verifyLocalRuntimeFile(dst, want)
}

func replaceLocalRuntimeFile(src, dst string, mode os.FileMode) error {
	if info, err := os.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("destination %q is not a regular non-symlink file", dst)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return os.Chmod(dst, mode)
	}
	return nil
}
