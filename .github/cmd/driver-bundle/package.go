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
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goplus/spx/v4/internal/driverbundle"
	"github.com/goplus/spx/v4/internal/release"
)

var zipEpoch = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

func runPackage(args []string) error {
	flags := flag.NewFlagSet("driver-bundle package", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var enginePath, packPath, bridgePath, outputPath, descriptorPath, goos, goarch string
	flags.StringVar(&enginePath, "engine", "", "Engine input path")
	flags.StringVar(&packPath, "pack", "", "runtime PCK input path")
	flags.StringVar(&bridgePath, "bridge", "", "interpreter bridge input path")
	flags.StringVar(&outputPath, "output", "", "output ZIP path")
	flags.StringVar(&descriptorPath, "descriptor", "", "output descriptor JSON path")
	flags.StringVar(&goos, "goos", "", "target GOOS")
	flags.StringVar(&goarch, "goarch", "", "target GOARCH")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: driver-bundle package --engine PATH --pack PATH --bridge PATH --output ZIP --descriptor JSON --goos GOOS --goarch GOARCH")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	paths := []string{enginePath, packPath, bridgePath, outputPath, descriptorPath}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			return errors.New("engine, pack, bridge, output, and descriptor paths are required")
		}
	}
	lock := release.DefaultRuntimeLock()
	spec, err := driverbundle.HostSpecFor(lock.RuntimeVersion, goos, goarch)
	if err != nil {
		return err
	}
	if err := validateInputBasenames(spec, enginePath, packPath, bridgePath, outputPath); err != nil {
		return err
	}
	if err := rejectOutputAliases(outputPath, descriptorPath, enginePath, packPath, bridgePath); err != nil {
		return err
	}
	if err := ensureParent(outputPath); err != nil {
		return fmt.Errorf("prepare ZIP output: %w", err)
	}
	if err := ensureParent(descriptorPath); err != nil {
		return fmt.Errorf("prepare descriptor output: %w", err)
	}

	temporary, err := os.CreateTemp(filepath.Dir(outputPath), "."+filepath.Base(outputPath)+".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary ZIP: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	outerDigest := sha256.New()
	outer := &digestWriter{writer: temporary, digest: outerDigest}
	archive := zip.NewWriter(outer)
	files := make([]driverbundle.File, 0, 3)

	// Keep descriptor and ZIP entries in Engine/PCK/bridge order.
	for _, input := range [...]struct {
		kind, path string
		component  driverbundle.Component
	}{
		{"Engine", enginePath, spec.Engine},
		{"PCK", packPath, spec.Pack},
		{"bridge", bridgePath, spec.Bridge},
	} {
		file, err := addZipFile(archive, input.path, input.component.Name, input.component.Mode)
		if err != nil {
			return fmt.Errorf("package %s: %w", input.kind, err)
		}
		files = append(files, file)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close ZIP: %w", err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		return fmt.Errorf("set ZIP mode: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync ZIP: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary ZIP: %w", err)
	}

	interfaceDigest, err := driverbundle.ComputeEngineInterfaceDigestFromSHA256(files[0].SHA256, files[1].SHA256)
	if err != nil {
		return fmt.Errorf("identify Engine interface: %w", err)
	}
	bundle := driverbundle.Bundle{
		GOOS: goos, GOARCH: goarch, Name: filepath.Base(outputPath),
		Size: outer.size, SHA256: hex.EncodeToString(outerDigest.Sum(nil)),
		EngineInterfaceDigest: interfaceDigest, Files: files,
	}
	if err := bundle.ValidateForRuntime(lock.RuntimeVersion); err != nil {
		return fmt.Errorf("validate generated descriptor: %w", err)
	}
	descriptor, err := bundle.JSON()
	if err != nil {
		return fmt.Errorf("encode descriptor: %w", err)
	}
	if err := os.Rename(temporaryPath, outputPath); err != nil {
		return fmt.Errorf("publish ZIP %s: %w", outputPath, err)
	}
	committed = true
	if err := atomicWrite(descriptorPath, descriptor, 0o644); err != nil {
		return fmt.Errorf("publish descriptor %s: %w", descriptorPath, err)
	}
	return nil
}

// spec is the canonical naming contract already validated by HostSpecFor.
func validateInputBasenames(spec driverbundle.HostSpec, enginePath, packPath, bridgePath, outputPath string) error {
	for _, input := range [...]struct{ kind, path, name string }{
		{"bundle", outputPath, spec.BundleName},
		{"Engine", enginePath, spec.Engine.Name},
		{"PCK", packPath, spec.Pack.Name},
		{"bridge", bridgePath, spec.Bridge.Name},
	} {
		if name := filepath.Base(input.path); name != input.name {
			return fmt.Errorf("validate input basenames: %s name = %q, want %q", input.kind, name, input.name)
		}
	}
	return nil
}

func addZipFile(archive *zip.Writer, sourcePath, name string, mode uint32) (driverbundle.File, error) {
	input, info, err := openRegularFile(sourcePath)
	if err != nil {
		return driverbundle.File{}, err
	}
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(os.FileMode(mode))
	header.SetModTime(zipEpoch)
	entry, err := archive.CreateHeader(header)
	if err != nil {
		_ = input.Close()
		return driverbundle.File{}, err
	}
	fileDigest := sha256.New()
	count, copyErr := io.Copy(io.MultiWriter(entry, fileDigest), input)
	finalInfo, statErr := input.Stat()
	closeErr := input.Close()
	if copyErr != nil {
		return driverbundle.File{}, copyErr
	}
	if statErr != nil {
		return driverbundle.File{}, statErr
	}
	if closeErr != nil {
		return driverbundle.File{}, closeErr
	}
	if count != info.Size() || finalInfo.Size() != info.Size() {
		return driverbundle.File{}, fmt.Errorf("source changed while reading (size %d, want %d)", count, info.Size())
	}
	return driverbundle.File{Name: name, Mode: mode, Size: count, SHA256: hex.EncodeToString(fileDigest.Sum(nil))}, nil
}
