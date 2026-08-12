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

package release

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

const RuntimeLockSchema = 1

var (
	//go:embed runtime.lock.json
	embeddedRuntimeLockJSON []byte
	//go:embed runtime_locks/*.json
	embeddedRuntimeLocks embed.FS

	runtimeVersionPattern    = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	releaseRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	godotRepositoryPattern   = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+\.git$`)
	gitCommitPattern         = regexp.MustCompile(`^[0-9a-f]{40}$`)
	relativePathPartPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	toolchainVersionPattern  = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_-]*$`)
	jdkMajorVersionPattern   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// RuntimeLock is the reproducible source and toolchain contract for one
// runtime release. It describes a release to build; it does not claim that the
// release has already been published.
type RuntimeLock struct {
	Schema            int           `json:"schema"`
	RuntimeVersion    string        `json:"runtime_version"`
	RuntimeABI        int           `json:"runtime_abi"`
	ReleaseRepository string        `json:"release_repository"`
	Manifest          string        `json:"manifest"`
	RequiredAssets    []string      `json:"required_assets"`
	Godot             GodotLock     `json:"godot"`
	Module            ModuleLock    `json:"module"`
	Toolchain         ToolchainLock `json:"toolchain"`
}

// GodotLock pins the exact upstream source used to build the runtime. Ref is
// retained so a shallow fetch can make Commit reachable before checkout.
type GodotLock struct {
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	Commit     string `json:"commit"`
	Version    string `json:"version"`
}

// ModuleLock locates the SPX custom module relative to the SPX repository.
type ModuleLock struct {
	Path string `json:"path"`
}

// ToolchainLock records tool versions that materially affect runtime output.
type ToolchainLock struct {
	Go         string `json:"go"`
	XGo        string `json:"xgo"`
	SCons      string `json:"scons"`
	EMSDK      string `json:"emsdk"`
	AndroidNDK string `json:"android_ndk"`
	JDK        string `json:"jdk"`
}

var (
	defaultRuntimeLock    = mustParseRuntimeLock(embeddedRuntimeLockJSON)
	runtimeLocksByVersion = mustLoadRuntimeLocks(embeddedRuntimeLocks, defaultRuntimeLock)
)

// DefaultRuntimeLock returns a copy of the runtime lock embedded in this package.
func DefaultRuntimeLock() RuntimeLock {
	return cloneRuntimeLock(defaultRuntimeLock)
}

// RuntimeLockForVersion returns the immutable build contract published for an
// atomic runtime version. Historical snapshots let consumers verify an older
// manifest after runtime.lock.json advances to a newer release.
func RuntimeLockForVersion(runtimeVersion string) (RuntimeLock, error) {
	lock, ok := runtimeLocksByVersion[runtimeVersion]
	if !ok {
		return RuntimeLock{}, fmt.Errorf("release: no runtime lock snapshot for version %q", runtimeVersion)
	}
	return cloneRuntimeLock(lock), nil
}

// RuntimeReleaseTag returns the Git tag that owns this runtime's atomic
// release assets. The tag is part of the release convention, so storing it
// beside RuntimeVersion would create two independently editable identities.
func (l RuntimeLock) RuntimeReleaseTag() string {
	return "runtime-v" + l.RuntimeVersion
}

// RuntimeAssetDownloadURL returns the immutable release URL for one asset in
// this lock's atomic runtime bundle.
func (l RuntimeLock) RuntimeAssetDownloadURL(assetName string) string {
	return "https://github.com/" + l.ReleaseRepository + "/releases/download/" + l.RuntimeReleaseTag() + "/" + assetName
}

func mustParseRuntimeLock(data []byte) RuntimeLock {
	lock, err := ParseRuntimeLock(data)
	if err != nil {
		panic("release: invalid embedded runtime lock: " + err.Error())
	}
	return lock
}

func mustLoadRuntimeLocks(fileSystem fs.FS, current RuntimeLock) map[string]RuntimeLock {
	locks, err := loadRuntimeLocks(fileSystem)
	if err != nil {
		panic("release: invalid runtime lock snapshots: " + err.Error())
	}
	snapshot, ok := locks[current.RuntimeVersion]
	if !ok {
		panic("release: current runtime lock has no snapshot")
	}
	currentJSON, currentErr := current.JSON()
	snapshotJSON, snapshotErr := snapshot.JSON()
	if currentErr != nil || snapshotErr != nil || !bytes.Equal(currentJSON, snapshotJSON) {
		panic("release: current runtime lock snapshot does not match runtime.lock.json")
	}
	return locks
}

func loadRuntimeLocks(fileSystem fs.FS) (map[string]RuntimeLock, error) {
	files, err := fs.Glob(fileSystem, "runtime_locks/*.json")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("no runtime lock snapshots")
	}

	locks := make(map[string]RuntimeLock, len(files))
	for _, file := range files {
		data, err := fs.ReadFile(fileSystem, file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		lock, err := ParseRuntimeLock(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}
		version := strings.TrimSuffix(path.Base(file), ".json")
		if lock.RuntimeVersion != version {
			return nil, fmt.Errorf("%s declares runtime version %q", file, lock.RuntimeVersion)
		}
		locks[version] = lock
	}
	return locks, nil
}

func cloneRuntimeLock(lock RuntimeLock) RuntimeLock {
	lock.RequiredAssets = append([]string(nil), lock.RequiredAssets...)
	return lock
}

// ParseRuntimeLock decodes and validates a runtime lock. Unknown JSON fields
// are rejected so misspelled release inputs cannot silently change a build.
func ParseRuntimeLock(data []byte) (RuntimeLock, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return RuntimeLock{}, fmt.Errorf("decode runtime lock: %w", err)
	}
	var lock RuntimeLock
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil {
		return RuntimeLock{}, fmt.Errorf("decode runtime lock: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return RuntimeLock{}, fmt.Errorf("decode runtime lock: %w", err)
	}
	if err := lock.Validate(); err != nil {
		return RuntimeLock{}, err
	}
	return lock, nil
}

// Validate checks the runtime lock's identity, source pins, paths, toolchains,
// and required asset names.
func (l RuntimeLock) Validate() error {
	if l.Schema != RuntimeLockSchema {
		return fmt.Errorf("release: runtime lock schema = %d, want %d", l.Schema, RuntimeLockSchema)
	}
	if !runtimeVersionPattern.MatchString(l.RuntimeVersion) {
		return fmt.Errorf("release: invalid runtime version %q", l.RuntimeVersion)
	}
	if l.RuntimeABI <= 0 {
		return fmt.Errorf("release: runtime ABI must be positive")
	}
	if !releaseRepositoryPattern.MatchString(l.ReleaseRepository) {
		return fmt.Errorf("release: invalid release repository %q", l.ReleaseRepository)
	}
	if err := validateBaseName("manifest", l.Manifest); err != nil {
		return err
	}
	if len(l.RequiredAssets) == 0 {
		return errors.New("release: required_assets must not be empty")
	}
	if !slices.IsSorted(l.RequiredAssets) {
		return errors.New("release: required_assets must be sorted by name")
	}
	seenAssets := make(map[string]struct{}, len(l.RequiredAssets))
	for _, name := range l.RequiredAssets {
		if err := validateBaseName("required asset", name); err != nil {
			return err
		}
		if name == l.Manifest {
			return fmt.Errorf("release: required asset %q conflicts with manifest", name)
		}
		if _, ok := seenAssets[name]; ok {
			return fmt.Errorf("release: duplicate required asset basename %q", name)
		}
		seenAssets[name] = struct{}{}
	}

	if !godotRepositoryPattern.MatchString(l.Godot.Repository) {
		return fmt.Errorf("release: invalid canonical Godot repository %q", l.Godot.Repository)
	}
	if l.Godot.Ref == "" || strings.IndexFunc(l.Godot.Ref, isWhitespaceOrControl) >= 0 {
		return errors.New("release: Godot ref must be non-empty without whitespace or control characters")
	}
	if !gitCommitPattern.MatchString(l.Godot.Commit) {
		return fmt.Errorf("release: Godot commit %q must be a 40-character lowercase SHA-1", l.Godot.Commit)
	}
	if l.Godot.Version == "" || strings.IndexFunc(l.Godot.Version, isWhitespaceOrControl) >= 0 {
		return errors.New("release: Godot version must be non-empty without whitespace or control characters")
	}
	if err := validateRelativePath("module path", l.Module.Path); err != nil {
		return err
	}
	return validateToolchainLock(l.Toolchain)
}

func validateToolchainLock(toolchain ToolchainLock) error {
	toolchainVersions := []struct {
		name  string
		value string
	}{
		{name: "go", value: toolchain.Go},
		{name: "xgo", value: toolchain.XGo},
		{name: "scons", value: toolchain.SCons},
		{name: "emsdk", value: toolchain.EMSDK},
		{name: "android_ndk", value: toolchain.AndroidNDK},
		{name: "jdk", value: toolchain.JDK},
	}
	for _, version := range toolchainVersions {
		if !toolchainVersionPattern.MatchString(version.value) {
			return fmt.Errorf("release: invalid %s toolchain version %q", version.name, version.value)
		}
	}
	if !jdkMajorVersionPattern.MatchString(toolchain.JDK) {
		return fmt.Errorf("release: JDK toolchain version %q must be a positive major version", toolchain.JDK)
	}
	return nil
}

// JSON returns the validated, canonical, human-readable lock representation.
func (l RuntimeLock) JSON() ([]byte, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode runtime lock: %w", err)
	}
	return append(data, '\n'), nil
}

// SHA256 returns the digest of the canonical JSON representation of the lock.
func (l RuntimeLock) SHA256() (string, error) {
	data, err := l.JSON()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values")
}

// rejectDuplicateJSONKeys walks one JSON value before typed decoding so all
// consumers reject ambiguous objects consistently. encoding/json otherwise
// accepts duplicate keys and silently keeps the last value.
func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := consumeUniqueJSONValue(decoder, "$"); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func consumeUniqueJSONValue(decoder *json.Decoder, location string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key at %s is not a string", location)
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate JSON key %q at %s", key, location)
			}
			seen[key] = struct{}{}
			if err := consumeUniqueJSONValue(decoder, location+"."+key); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("invalid object terminator at %s", location)
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := consumeUniqueJSONValue(decoder, fmt.Sprintf("%s[%d]", location, index)); err != nil {
				return err
			}
			index++
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("invalid array terminator at %s", location)
		}
	default:
		return fmt.Errorf("invalid JSON delimiter %q at %s", delim, location)
	}
	return nil
}

func validateBaseName(kind, name string) error {
	if name == "" || name != strings.TrimSpace(name) || name == "." || name == ".." || path.Base(name) != name || strings.Contains(name, `\`) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return fmt.Errorf("release: invalid %s basename %q", kind, name)
	}
	return nil
}

func validateRelativePath(kind, value string) error {
	if value == "" || value != strings.TrimSpace(value) || strings.Contains(value, `\`) || strings.HasPrefix(value, "/") {
		return fmt.Errorf("release: invalid %s %q", kind, value)
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned != value || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("release: invalid %s %q", kind, value)
	}
	for _, part := range strings.Split(value, "/") {
		if !relativePathPartPattern.MatchString(part) {
			return fmt.Errorf("release: invalid %s %q", kind, value)
		}
	}
	return nil
}

func isWhitespaceOrControl(value rune) bool {
	return unicode.IsSpace(value) || unicode.IsControl(value)
}
