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
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// VerifyFiles verifies the declared local files beneath manifestDir using
// no-follow/open/re-stat checks and exact size/digest comparisons.
func (m LocalRuntimeManifest) VerifyFiles(manifestDir string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	for _, item := range []struct {
		label string
		file  LocalRuntimeFile
	}{
		{label: "engine", file: m.Engine},
		{label: "pack", file: m.Pack},
	} {
		path := filepath.Join(manifestDir, item.file.Name)
		if err := verifyLocalRuntimeFile(path, item.file); err != nil {
			return fmt.Errorf("release: verify local runtime %s: %w", item.label, err)
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
	if !os.SameFile(info, opened) || info.Size() != opened.Size() {
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
	expectedSize := info.Size()
	size, err := io.CopyN(writer, file, expectedSize)
	closeErr := file.Close()
	if err != nil {
		return LocalRuntimeFile{}, err
	}
	if closeErr != nil {
		return LocalRuntimeFile{}, closeErr
	}
	after, err := os.Lstat(path)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) || after.Size() != expectedSize || size != expectedSize {
		return LocalRuntimeFile{}, fmt.Errorf("%s changed while reading", path)
	}
	return LocalRuntimeFile{Name: filepath.Base(path), Size: size, SHA256: hex.EncodeToString(hasher.Sum(nil))}, nil
}
