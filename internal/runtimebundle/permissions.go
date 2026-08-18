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

package runtimebundle

import (
	"os"
	"path/filepath"
)

// Permissions is the platform security seam used while creating a cache. On
// POSIX the default implementation enforces private directory/file modes. A
// Windows implementation must enforce a current-user-only DACL; POSIX mode
// bits are not a Windows security boundary.
type Permissions interface {
	EnsureDir(string) error
	EnsureFile(string, bool) error
}

type modePermissions struct{}

func (modePermissions) EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func (modePermissions) EnsureFile(path string, executable bool) error {
	mode := os.FileMode(0o600)
	if executable {
		mode = 0o700
	}
	return os.Chmod(path, mode)
}

func defaultPermissions() Permissions {
	return platformPermissions()
}

// PrivateFileMode exposes the private mode selected for a manifest entry. It
// is useful to callers materializing a verified entry outside Cache.
func PrivateFileMode(mode uint32) os.FileMode {
	return privateFileMode(mode)
}

func mkdirPrivateRootChild(root *os.Root, name string) error {
	return platformMkdirPrivateRootChild(root, name)
}

func verifyPrivateRootPath(root *os.Root, name string) error {
	return platformVerifyPrivateRootPath(root, filepath.FromSlash(name))
}
