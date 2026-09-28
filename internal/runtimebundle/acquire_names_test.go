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
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func acquireNameTestSpec(name, body string) FetchSpec {
	return FetchSpec{
		Name: name, Size: int64(len(body)), SHA256: testDigest(body),
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			_, err := io.WriteString(dst, body)
			return err
		},
	}
}

func TestAcquireFileRejectsReservedAndNonportableNamesBeforeCreatingCache(t *testing.T) {
	for _, name := range []string{
		"asset.lock", "asset.LOCK", "asset.locK", ".lock",
		"asset.lock.", "asset.lock ", "asset.lock:stream", "asset:stream",
	} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "cache")
			spec := acquireNameTestSpec(name, "payload")
			fetched := false
			spec.Fetch = func(context.Context, string, io.Writer) error {
				fetched = true
				return errors.New("fetch must not run")
			}
			file, err := AcquireFile(context.Background(), root, spec)
			if file != nil {
				file.Close()
			}
			if err == nil || fetched {
				t.Fatalf("AcquireFile(%q): error %v, fetched %v; want rejection before fetch", name, err, fetched)
			}
			if _, err := os.Lstat(root); !os.IsNotExist(err) {
				t.Fatalf("invalid name created cache: Lstat error = %v", err)
			}
		})
	}
}

func TestAcquireFileLockNameCannotInvalidateSharedLease(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireFile(context.Background(), root, acquireNameTestSpec("engine.bin", "old"))
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	lockPath := filepath.Join(root, "engine.bin.lock")
	before, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	collision, collisionErr := AcquireFile(context.Background(), root, acquireNameTestSpec("engine.bin.lock", "lock collision"))
	if collision != nil {
		defer collision.Close()
	}
	if collisionErr == nil {
		t.Error("AcquireFile accepted another asset's lock sidecar name")
	}
	after, err := os.Stat(lockPath)
	if err != nil || !os.SameFile(before, after) {
		t.Errorf("sidecar identity changed while lease was held: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	replacement, err := AcquireFile(ctx, root, acquireNameTestSpec("engine.bin", "new"))
	if replacement != nil {
		replacement.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("replacement while shared lease held = %v, want deadline exceeded", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	replacement, err = AcquireFile(context.Background(), root, acquireNameTestSpec("engine.bin", "new"))
	if err != nil {
		t.Fatalf("replacement after lease close: %v", err)
	}
	defer replacement.Close()
	data, err := io.ReadAll(replacement)
	if err != nil || string(data) != "new" {
		t.Fatalf("replacement contents = %q, %v; want new", data, err)
	}
}

func TestAcquireFileAllowsNonreservedPortableNames(t *testing.T) {
	for _, name := range []string{"asset.bin", "asset.lock.zip", ".lockfile", "ロック.bin"} {
		t.Run(name, func(t *testing.T) {
			file, err := AcquireFile(context.Background(), t.TempDir(), acquireNameTestSpec(name, "payload"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil || string(data) != "payload" {
				t.Fatalf("acquired contents = %q, %v; want payload", data, err)
			}
		})
	}
}
