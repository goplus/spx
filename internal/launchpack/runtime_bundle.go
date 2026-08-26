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
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

const engineAcquisitionManifestName = "engine-acquisition-manifest.json"

func expectedEngineBundle(acquisitionManifest []byte, spec release.HostRuntimeSpec, engineSize int64, engineSHA string, packSize int64, packSHA string) (runtimebundle.Bundle, error) {
	bundle := runtimebundle.Bundle{
		Schema: runtimebundle.SchemaV1, Namespace: runtimebundle.NamespaceEngine,
		Entries: []runtimebundle.Entry{
			{Name: engineAcquisitionManifestName, Mode: 0o600, Size: int64(len(acquisitionManifest)), SHA256: digestBytes(acquisitionManifest)},
			{Name: spec.RuntimeName, Mode: 0o700, Size: engineSize, SHA256: engineSHA},
			{Name: spec.PackName, Mode: 0o600, Size: packSize, SHA256: packSHA},
		},
	}
	return bundle.WithDigest()
}

func validateRuntimeEntry(path, name string, bundle runtimebundle.Bundle) error {
	if err := validateRuntimeFile(path, name); err != nil {
		return err
	}
	for _, entry := range bundle.Entries {
		if entry.Name == filepath.ToSlash(name) {
			return nil
		}
	}
	return fmt.Errorf("launchpack: runtime archive is missing %s", name)
}

func isRegularNonSymlink(info os.FileInfo) bool {
	return info != nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}

func validateRuntimeFile(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("launchpack: %s unavailable at %s: %w", label, path, err)
	}
	if !isRegularNonSymlink(info) {
		return fmt.Errorf("launchpack: %s %q is not a regular non-symlink file", label, path)
	}
	return nil
}

func withPinnedFile(name, path string, fn func(*pinnedFile) error) error {
	file, err := openPinnedFile(name, path)
	if err != nil {
		return err
	}
	operationErr := fn(file)
	if operationErr == nil {
		operationErr = file.verify()
	}
	closeErr := file.file.Close()
	if operationErr != nil {
		return operationErr
	}
	return closeErr
}

func hashRuntimeFile(path string) (int64, string, error) {
	var size int64
	var digest string
	err := withPinnedFile("runtime file", path, func(file *pinnedFile) error {
		hasher := sha256.New()
		var err error
		size, err = io.Copy(hasher, file.file)
		if err != nil {
			return err
		}
		if size != file.info.Size() {
			return fmt.Errorf("file changed while reading")
		}
		digest = hex.EncodeToString(hasher.Sum(nil))
		return nil
	})
	if err != nil {
		return 0, "", err
	}
	return size, digest, nil
}

func writeEngineBundle(path string, acquisitionManifest []byte, engineName, packName, enginePath, packPath string) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".spx-launchpack-engine-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	zw := zip.NewWriter(tmp)
	addBytes := func(name string, mode os.FileMode, data []byte) error {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(mode)
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	}
	if err := addBytes(engineAcquisitionManifestName, 0o600, acquisitionManifest); err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return err
	}
	if err := addFileToZip(zw, engineName, enginePath, 0o700); err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return fmt.Errorf("add Engine to bundle: %w", err)
	}
	if err := addFileToZip(zw, packName, packPath, 0o600); err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return fmt.Errorf("add runtime PCK to bundle: %w", err)
	}
	if err := zw.Close(); err != nil {
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
	return replaceRuntimeFile(tmpPath, path, 0o600)
}

func addFileToZip(zw *zip.Writer, name, path string, mode os.FileMode) error {
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(mode)
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func replaceRuntimeFile(src, dst string, mode os.FileMode) error {
	if info, err := os.Lstat(dst); err == nil {
		if !isRegularNonSymlink(info) {
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
