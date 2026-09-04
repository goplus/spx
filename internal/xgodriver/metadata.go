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

package xgodriver

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/goplus/mod/xgomod"
)

// VerifyDeclaration checks the metadata XGo used for discovery.
func VerifyDeclaration(identity xgomod.FileIdentity) error {
	return verifyFileIdentity("declaration", identity)
}

// VerifyTargetModFile checks the modfile XGo used for resolution.
func VerifyTargetModFile(identity xgomod.FileIdentity) error {
	return verifyFileIdentity("target modfile", identity)
}

func verifyFileIdentity(label string, identity xgomod.FileIdentity) (err error) {
	expected, err := hex.DecodeString(identity.SHA256)
	if err != nil || len(expected) != sha256.Size {
		return fmt.Errorf("xgodriver: invalid %s SHA-256 %q", label, identity.SHA256)
	}

	before, err := os.Lstat(identity.Path)
	if err != nil {
		return fmt.Errorf("xgodriver: lstat %s %q: %w", label, identity.Path, err)
	}
	if !before.Mode().IsRegular() {
		return fmt.Errorf("xgodriver: %s %q is not a regular non-symlink file", label, identity.Path)
	}

	file, err := os.Open(identity.Path)
	if err != nil {
		return fmt.Errorf("xgodriver: open %s %q: %w", label, identity.Path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("xgodriver: close %s %q: %w", label, identity.Path, closeErr)
		}
	}()

	opened, err := file.Stat()
	if err != nil {
		return fmt.Errorf("xgodriver: stat opened %s %q: %w", label, identity.Path, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return fmt.Errorf("xgodriver: %s %q changed while opening", label, identity.Path)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("xgodriver: hash %s %q: %w", label, identity.Path, err)
	}
	after, err := os.Lstat(identity.Path)
	if err != nil {
		return fmt.Errorf("xgodriver: re-lstat %s %q: %w", label, identity.Path, err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		return fmt.Errorf("xgodriver: %s %q changed while reading", label, identity.Path)
	}
	if subtle.ConstantTimeCompare(hasher.Sum(nil), expected) != 1 {
		return fmt.Errorf("xgodriver: %s %q changed after XGo discovery", label, identity.Path)
	}
	return nil
}
