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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

func runVerify(args []string) error {
	flags := flag.NewFlagSet("driverbundle verify", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var zipPath, descriptorPath string
	flags.StringVar(&zipPath, "output", "", "ZIP path to verify")
	flags.StringVar(&descriptorPath, "descriptor", "", "descriptor JSON path")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: driverbundle verify --output ZIP --descriptor JSON")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(zipPath) == "" || strings.TrimSpace(descriptorPath) == "" {
		return errors.New("output and descriptor paths are required")
	}

	descriptorData, err := readRegularFile(descriptorPath)
	if err != nil {
		return fmt.Errorf("read descriptor: %w", err)
	}
	bundle, err := driverbundle.ParseBundle(descriptorData)
	if err != nil {
		return fmt.Errorf("parse descriptor: %w", err)
	}
	return verifyBundleFile(zipPath, bundle)
}

// verifyBundleFile verifies a ZIP against a trusted bundle descriptor.  The
// descriptor may come from the packaging job or from an aggregate release
// manifest; both paths use the same byte-level checks.
func verifyBundleFile(zipPath string, bundle driverbundle.Bundle) error {
	lock := release.DefaultRuntimeLock()
	return verifyBundleFileForRuntime(zipPath, bundle, lock.RuntimeVersion)
}

func verifyBundleFileForRuntime(zipPath string, bundle driverbundle.Bundle, runtimeVersion string) error {
	if err := bundle.ValidateForRuntime(runtimeVersion); err != nil {
		return fmt.Errorf("validate descriptor: %w", err)
	}
	if filepath.Base(zipPath) != bundle.Name {
		return fmt.Errorf("ZIP basename %q does not match descriptor name %q", filepath.Base(zipPath), bundle.Name)
	}

	archiveFile, archiveInfo, err := openRegularFile(zipPath)
	if err != nil {
		return fmt.Errorf("open ZIP: %w", err)
	}
	defer archiveFile.Close()
	if archiveInfo.Size() != bundle.Size {
		return fmt.Errorf("ZIP size = %d, want descriptor size %d", archiveInfo.Size(), bundle.Size)
	}
	outerDigest := sha256.New()
	if _, err := io.Copy(outerDigest, archiveFile); err != nil {
		return fmt.Errorf("hash ZIP: %w", err)
	}
	if got := hex.EncodeToString(outerDigest.Sum(nil)); got != bundle.SHA256 {
		return fmt.Errorf("ZIP SHA-256 = %s, want descriptor %s", got, bundle.SHA256)
	}

	expected := runtimebundle.Bundle{
		Schema: runtimebundle.SchemaV1, Namespace: runtimebundle.NamespaceDriver,
		Entries: runtimeEntries(bundle.Files),
	}
	_, err = runtimebundle.VerifyZipReader(archiveFile, archiveInfo.Size(), runtimebundle.VerifyOptions{Expected: &expected})
	if err != nil {
		return fmt.Errorf("verify ZIP with runtimebundle: %w", err)
	}
	interfaceDigest, err := driverbundle.ComputeEngineInterfaceDigestFromSHA256(bundle.Files[0].SHA256, bundle.Files[1].SHA256)
	if err != nil {
		return fmt.Errorf("identify Engine interface: %w", err)
	}
	if interfaceDigest != bundle.EngineInterfaceDigest {
		return fmt.Errorf("Engine interface digest = %s, want descriptor %s", interfaceDigest, bundle.EngineInterfaceDigest)
	}
	return nil
}

func runtimeEntries(files []driverbundle.File) []runtimebundle.Entry {
	entries := make([]runtimebundle.Entry, len(files))
	for i, file := range files {
		entries[i] = runtimebundle.Entry{Name: file.Name, Mode: file.Mode, Size: file.Size, SHA256: file.SHA256}
	}
	return entries
}
