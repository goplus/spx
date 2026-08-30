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

package main

import (
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/driverbundle"
)

func openRegularFile(path string) (*os.File, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, nil, &notRegularError{path: path}
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if !os.SameFile(info, opened) {
		_ = file.Close()
		return nil, nil, &changedFileError{path: path}
	}
	return file, info, nil
}

func readRegularFile(path string) ([]byte, error) {
	file, _, err := openRegularFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, driverbundle.MaxManifestSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > driverbundle.MaxManifestSize {
		return nil, fmt.Errorf("descriptor exceeds %d-byte limit", driverbundle.MaxManifestSize)
	}
	return data, nil
}

func rejectOutputAliases(outputPath, descriptorPath string, inputs ...string) error {
	outputs := []string{outputPath, descriptorPath}
	absoluteOutputs := make([]string, len(outputs))
	for i, path := range outputs {
		value, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		absoluteOutputs[i] = filepath.Clean(value)
	}
	if absoluteOutputs[0] == absoluteOutputs[1] {
		return &aliasError{first: outputPath, second: descriptorPath}
	}
	for outputIndex, output := range outputs {
		for _, input := range inputs {
			inputAbs, err := filepath.Abs(input)
			if err != nil {
				return err
			}
			if absoluteOutputs[outputIndex] == filepath.Clean(inputAbs) {
				return &aliasError{first: output, second: input}
			}
			outputInfo, outputErr := os.Stat(output)
			inputInfo, inputErr := os.Stat(input)
			if outputErr == nil && inputErr == nil && os.SameFile(outputInfo, inputInfo) {
				return &aliasError{first: output, second: input}
			}
		}
	}
	return nil
}

func ensureParent(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	committed = true
	if err := syncDirectory(filepath.Dir(path)); err != nil && runtime.GOOS != "windows" {
		return err
	}
	return nil
}

func copyRegularFile(source, destination string) error {
	if filepath.Clean(source) == filepath.Clean(destination) {
		return errors.New("source and destination are the same path")
	}
	input, info, err := openRegularFile(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := ensureParent(destination); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".tmp-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	count, err := io.Copy(temporary, input)
	if err != nil {
		return err
	}
	if count != info.Size() {
		return fmt.Errorf("copied size %d, want %d", count, info.Size())
	}
	if err := temporary.Chmod(0o644); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	committed = true
	return nil
}

type digestWriter struct {
	writer io.Writer
	digest hash.Hash
	size   int64
}

func (w *digestWriter) Write(data []byte) (int, error) {
	count, err := w.writer.Write(data)
	if count > 0 {
		if _, digestErr := w.digest.Write(data[:count]); err == nil {
			err = digestErr
		}
		w.size += int64(count)
	}
	return count, err
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

type notRegularError struct{ path string }

func (e *notRegularError) Error() string { return e.path + " is not a regular non-symlink file" }

type changedFileError struct{ path string }

func (e *changedFileError) Error() string { return e.path + " changed while opening" }

type aliasError struct{ first, second string }

func (e *aliasError) Error() string { return "paths alias: " + e.first + " and " + e.second }
