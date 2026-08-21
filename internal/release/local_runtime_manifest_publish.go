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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func writeLocalRuntimeManifest(path string, manifest LocalRuntimeManifest) error {
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
	if err := writeLocalRuntimeManifest(path, manifest); err != nil {
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
		Schema: localRuntimeManifestSchema, Mode: "local", RuntimeVersion: lock.RuntimeVersion,
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
