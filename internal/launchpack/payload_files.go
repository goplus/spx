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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/envutil"
	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/runtimepayload"
)

type pinnedFile struct {
	name string
	path string
	file *os.File
	info os.FileInfo
}

func openPinnedFile(name, filePath string) (*pinnedFile, error) {
	before, err := os.Lstat(filePath)
	if err != nil {
		return nil, fmt.Errorf("lstat %s %q: %w", name, filePath, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%s %q is not a regular non-symlink file", name, filePath)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open %s %q: %w", name, filePath, err)
	}
	opened, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat %s %q: %w", name, filePath, statErr)
	}
	after, err := os.Lstat(filePath)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) || before.Size() != opened.Size() || opened.Size() != after.Size() {
		_ = file.Close()
		return nil, fmt.Errorf("%s %q changed while opening", name, filePath)
	}
	return &pinnedFile{name: name, path: filePath, file: file, info: opened}, nil
}

func validatePinnedFile(name, filePath string) error {
	file, err := openPinnedFile(name, filePath)
	if err != nil {
		return err
	}
	if err := file.file.Close(); err != nil {
		return fmt.Errorf("close %s %q: %w", name, filePath, err)
	}
	return nil
}

func (f *pinnedFile) source(name string) runtimepayload.FileSource {
	return runtimepayload.FileSource{Name: name, Mode: f.info.Mode().Perm(), ReaderAt: f.file, Size: f.info.Size()}
}

func (f *pinnedFile) verify() error {
	opened, err := f.file.Stat()
	if err != nil {
		return fmt.Errorf("stat %s %q: %w", f.name, f.path, err)
	}
	after, err := os.Lstat(f.path)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(f.info, opened) || !os.SameFile(opened, after) || opened.Size() != f.info.Size() || after.Size() != f.info.Size() {
		return fmt.Errorf("%s %q changed while reading", f.name, f.path)
	}
	return nil
}

func byteSource(name string, mode os.FileMode, data []byte) runtimepayload.FileSource {
	return runtimepayload.FileSource{Name: name, Mode: mode, ReaderAt: bytes.NewReader(data), Size: int64(len(data))}
}

func digestFileSource(source runtimepayload.FileSource) (string, error) {
	hasher := sha256.New()
	if err := copyFileSource(hasher, source); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func engineSourceDigests(engine, pack runtimepayload.FileSource) (interfaceDigest, engineDigest, packDigest string, err error) {
	engineDigest, err = digestFileSource(engine)
	if err != nil {
		return "", "", "", fmt.Errorf("launchpack: hash Engine: %w", err)
	}
	packDigest, err = digestFileSource(pack)
	if err != nil {
		return "", "", "", fmt.Errorf("launchpack: hash runtime PCK: %w", err)
	}
	interfaceDigest, err = driverbundle.ComputeEngineInterfaceDigestFromSHA256(engineDigest, packDigest)
	if err != nil {
		return "", "", "", fmt.Errorf("launchpack: identify Engine interface: %w", err)
	}
	return interfaceDigest, engineDigest, packDigest, nil
}

func copyFileSource(dst io.Writer, source runtimepayload.FileSource) error {
	count, err := io.Copy(dst, io.NewSectionReader(source.ReaderAt, 0, source.Size))
	if err != nil {
		return err
	}
	if count != source.Size {
		return fmt.Errorf("short read: read %d bytes, want %d: %w", count, source.Size, io.ErrUnexpectedEOF)
	}
	return nil
}

func bundleEntryHasDigest(bundle runtimebundle.Bundle, name, digest string) bool {
	for _, entry := range bundle.Entries {
		if entry.Name == name {
			return entry.SHA256 == digest
		}
	}
	return false
}

func hasBuildFlag(flags []string, name string) bool {
	bare, enabled := "-"+name, "-"+name+"=true"
	for _, flag := range flags {
		if flag == bare || flag == enabled {
			return true
		}
	}
	return false
}

func traceEnabled(flags []string) bool { return hasBuildFlag(flags, "x") || hasBuildFlag(flags, "v") }

func hostGoEnv(cfg Config, base []string) []string {
	return envutil.HostGoEnvironment(base, cfg.GoWork, false)
}

func sourceBridgeEnv(cfg Config, base []string) []string {
	return envutil.HostGoEnvironment(envutil.WithoutPrefixes(base, "CGO_"), cfg.GoWork, true)
}
