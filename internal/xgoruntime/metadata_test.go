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

package xgoruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/mod/xgomod"
)

func TestVerifyDeclaration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gox.mod")
	data := []byte("xgo 1.8\nproject main.spx Game example/spx\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	declaration := xgomod.FileIdentity{Path: path, SHA256: hex.EncodeToString(digest[:])}
	if err := VerifyDeclaration(declaration); err != nil {
		t.Fatalf("VerifyDeclaration() error: %v", err)
	}

	if err := os.WriteFile(path, append(data, []byte("pack assets index.json\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDeclaration(declaration); err == nil || !strings.Contains(err.Error(), "changed after XGo discovery") {
		t.Fatalf("changed metadata error = %v", err)
	}
}

func TestVerifyDeclarationRejectsInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "gox.mod")
	if err := os.WriteFile(regular, []byte("xgo 1.8\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	validDigest := sha256.Sum256([]byte("xgo 1.8\n"))

	tests := []struct {
		name        string
		declaration xgomod.FileIdentity
		want        string
	}{
		{"bad digest", xgomod.FileIdentity{Path: regular, SHA256: "bad"}, "invalid declaration SHA-256"},
		{"missing", xgomod.FileIdentity{Path: filepath.Join(dir, "missing"), SHA256: hex.EncodeToString(validDigest[:])}, "lstat declaring metadata"},
		{"directory", xgomod.FileIdentity{Path: dir, SHA256: hex.EncodeToString(validDigest[:])}, "not a regular non-symlink file"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := VerifyDeclaration(test.declaration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("VerifyDeclaration() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestVerifyDeclarationRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.mod")
	link := filepath.Join(dir, "gox.mod")
	data := []byte("xgo 1.8\n")
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	digest := sha256.Sum256(data)
	err := VerifyDeclaration(xgomod.FileIdentity{Path: link, SHA256: hex.EncodeToString(digest[:])})
	if err == nil || !strings.Contains(err.Error(), "not a regular non-symlink file") {
		t.Fatalf("VerifyDeclaration() error = %v", err)
	}
}
