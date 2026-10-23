//go:build !js

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

package fsutil_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/spx/v4/fs/fsutil"
)

func TestOpenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := fsutil.OpenFile(filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if data, err := io.ReadAll(file); err != nil || string(data) != "hello world" {
		t.Fatalf("ReadAll = (%q, %v), want hello world", data, err)
	}
	if offset, err := file.Seek(-5, io.SeekEnd); err != nil || offset != 6 {
		t.Fatalf("Seek = (%d, %v), want (6, nil)", offset, err)
	}
	if data, err := io.ReadAll(file); err != nil || string(data) != "world" {
		t.Fatalf("ReadAll after Seek = (%q, %v), want world", data, err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Read after Close = %v, want os.ErrClosed", err)
	}
}

func TestOpenFileMissing(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "missing.txt"))
	if _, err := fsutil.OpenFile(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenFile missing path = %v, want os.ErrNotExist", err)
	}
}
