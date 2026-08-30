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
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/release"
)

func TestPackageAndVerifyIsDeterministic(t *testing.T) {
	directory := t.TempDir()
	runtimeVersion := release.DefaultRuntimeLock().RuntimeVersion
	enginePath := filepath.Join(directory, "gdspxrt"+runtimeVersion)
	packPath := filepath.Join(directory, "gdspxrt"+runtimeVersion+".pck")
	bridgePath := filepath.Join(directory, "gdspx-linux-amd64.so")
	for path, data := range map[string][]byte{
		enginePath: []byte("engine bytes"),
		packPath:   []byte("pack bytes"),
		bridgePath: []byte("bridge bytes"),
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outputPath := filepath.Join(directory, "spx-driver-linux-amd64.zip")
	descriptorPath := filepath.Join(directory, "bundle.json")
	args := []string{
		"--engine", enginePath, "--pack", packPath, "--bridge", bridgePath,
		"--output", outputPath, "--descriptor", descriptorPath,
		"--goos", "linux", "--goarch", "amd64",
	}
	if err := runPackage(args); err != nil {
		t.Fatalf("package: %v", err)
	}
	firstZIP, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	firstDescriptor, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := runVerify([]string{"--output", outputPath, "--descriptor", descriptorPath}); err != nil {
		t.Fatalf("verify: %v", err)
	}

	secondOutput := filepath.Join(directory, "second", "spx-driver-linux-amd64.zip")
	secondDescriptor := filepath.Join(directory, "second", "bundle.json")
	secondArgs := append([]string{}, args...)
	for i := range secondArgs {
		if secondArgs[i] == outputPath {
			secondArgs[i] = secondOutput
		}
		if secondArgs[i] == descriptorPath {
			secondArgs[i] = secondDescriptor
		}
	}
	if err := runPackage(secondArgs); err != nil {
		t.Fatalf("second package: %v", err)
	}
	secondZIP, err := os.ReadFile(secondOutput)
	if err != nil {
		t.Fatal(err)
	}
	secondDescriptorData, err := os.ReadFile(secondDescriptor)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstZIP, secondZIP) || !bytes.Equal(firstDescriptor, secondDescriptorData) {
		t.Fatal("package output is not deterministic")
	}

	bundle, err := driverbundle.ParseBundle(firstDescriptor)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Files) != 3 || bundle.Files[0].Mode != 0o755 || bundle.Files[1].Mode != 0o644 || bundle.Files[2].Mode != 0o755 {
		t.Fatalf("descriptor files = %#v", bundle.Files)
	}
	archive, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for i, entry := range archive.File {
		if entry.Name != bundle.Files[i].Name || entry.Mode().Perm() != os.FileMode(bundle.Files[i].Mode) {
			t.Fatalf("ZIP entry %d = %q mode %#o", i, entry.Name, entry.Mode().Perm())
		}
		if !entry.Modified.Equal(time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("ZIP entry %q modified = %s", entry.Name, entry.Modified)
		}
	}
}

func TestPackageRejectsNonCanonicalBasename(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"engine", "pack", "bridge"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	err := runPackage([]string{
		"--engine", filepath.Join(directory, "engine"),
		"--pack", filepath.Join(directory, "pack"),
		"--bridge", filepath.Join(directory, "bridge"),
		"--output", filepath.Join(directory, "spx-driver-linux-amd64.zip"),
		"--descriptor", filepath.Join(directory, "bundle.json"),
		"--goos", "linux", "--goarch", "amd64",
	})
	if err == nil {
		t.Fatal("accepted non-canonical input basenames")
	}
}
