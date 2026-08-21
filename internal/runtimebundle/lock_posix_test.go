//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

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
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPlatformLockFileImplRejectsSymlink(t *testing.T) {
	rootPath := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.lock")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	const name = "runtime.bin.lock"
	if err := os.Symlink(outside, filepath.Join(rootPath, name)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := openPinnedRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	file, err := openPlatformLockFileImpl(root, name)
	if file != nil {
		_ = file.close()
	}
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("open no-follow lock symlink error = %v, want ErrUnsafeArchive", err)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "outside" {
		t.Fatalf("outside lock = %q, err=%v; want unchanged", data, err)
	}
}
