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
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/goplus/mod/xgomod"
)

// VerifyDeclaration verifies that the declaring gox.mod or gop.mod is still
// the same regular file and contains the same bytes XGo used for discovery.
// The provider never reparses ambient metadata after this identity check.
func VerifyDeclaration(declaration xgomod.FileIdentity) (err error) {
	expected, err := hex.DecodeString(declaration.SHA256)
	if err != nil || len(expected) != sha256.Size {
		return fmt.Errorf("xgoruntime: invalid declaration SHA-256 %q", declaration.SHA256)
	}

	before, err := os.Lstat(declaration.Path)
	if err != nil {
		return fmt.Errorf("xgoruntime: lstat declaring metadata %q: %w", declaration.Path, err)
	}
	if !before.Mode().IsRegular() {
		return fmt.Errorf("xgoruntime: declaring metadata %q is not a regular non-symlink file", declaration.Path)
	}

	file, err := os.Open(declaration.Path)
	if err != nil {
		return fmt.Errorf("xgoruntime: open declaring metadata %q: %w", declaration.Path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("xgoruntime: close declaring metadata %q: %w", declaration.Path, closeErr)
		}
	}()

	opened, err := file.Stat()
	if err != nil {
		return fmt.Errorf("xgoruntime: stat opened declaring metadata %q: %w", declaration.Path, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return fmt.Errorf("xgoruntime: declaring metadata %q changed while opening", declaration.Path)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("xgoruntime: hash declaring metadata %q: %w", declaration.Path, err)
	}
	after, err := os.Lstat(declaration.Path)
	if err != nil {
		return fmt.Errorf("xgoruntime: re-lstat declaring metadata %q: %w", declaration.Path, err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		return fmt.Errorf("xgoruntime: declaring metadata %q changed while reading", declaration.Path)
	}
	if subtle.ConstantTimeCompare(hasher.Sum(nil), expected) != 1 {
		return fmt.Errorf("xgoruntime: declaring metadata %q changed after XGo discovery", declaration.Path)
	}
	return nil
}
