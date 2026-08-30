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

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/release"
)

var zipEpoch = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

func runPackage(args []string) error {
	flags := flag.NewFlagSet("driverbundle package", flag.ContinueOnError)
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
		fmt.Fprintln(flags.Output(), "usage: driverbundle package --engine PATH --pack PATH --bridge PATH --output ZIP --descriptor JSON --goos GOOS --goarch GOARCH")
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

	engine, err := addZipFile(archive, enginePath, spec.Engine.Name, spec.Engine.Mode)
	if err != nil {
		return fmt.Errorf("package Engine: %w", err)
	}
	files = append(files, engine)
	pack, err := addZipFile(archive, packPath, spec.Pack.Name, spec.Pack.Mode)
	if err != nil {
		return fmt.Errorf("package PCK: %w", err)
	}
	files = append(files, pack)
	bridge, err := addZipFile(archive, bridgePath, spec.Bridge.Name, spec.Bridge.Mode)
	if err != nil {
		return fmt.Errorf("package bridge: %w", err)
	}
	files = append(files, bridge)
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

	interfaceDigest, err := driverbundle.ComputeEngineInterfaceDigestFromSHA256(engine.SHA256, pack.SHA256)
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

func validateInputBasenames(spec driverbundle.HostSpec, enginePath, packPath, bridgePath, outputPath string) error {
	validDigest := strings.Repeat("0", sha256.Size*2)
	interfaceDigest, err := driverbundle.ComputeEngineInterfaceDigestFromSHA256(validDigest, validDigest)
	if err != nil {
		return err
	}
	bundle := driverbundle.Bundle{
		GOOS: spec.GOOS, GOARCH: spec.GOARCH, Name: filepath.Base(outputPath), Size: 1,
		SHA256: validDigest, EngineInterfaceDigest: interfaceDigest,
		Files: []driverbundle.File{
			{Name: filepath.Base(enginePath), Mode: spec.Engine.Mode, Size: 1, SHA256: validDigest},
			{Name: filepath.Base(packPath), Mode: spec.Pack.Mode, Size: 1, SHA256: validDigest},
			{Name: filepath.Base(bridgePath), Mode: spec.Bridge.Mode, Size: 1, SHA256: validDigest},
		},
	}
	if err := bundle.ValidateForRuntime(spec.RuntimeVersion); err != nil {
		return fmt.Errorf("validate input basenames: %w", err)
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
