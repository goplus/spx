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

package xgolauncher

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

func TestMaterializeComponentRejectsNilInputs(t *testing.T) {
	for _, tt := range []struct {
		name      string
		ctx       context.Context
		nilWriter bool
		message   string
	}{
		{"nil context", nil, false, "nil materialization context"},
		{"nil writer", context.Background(), true, "nil component writer"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			workDir := t.TempDir()
			write := func(io.Writer) error {
				t.Fatal("writer called for invalid input")
				return nil
			}
			if tt.nilWriter {
				write = nil
			}
			materialized, err := materializeComponent(tt.ctx, nil, workDir, runtimebundle.NamespaceProject, "", write)
			if materialized != nil || err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("materializeComponent = %v, %v; want %q", materialized, err, tt.message)
			}
			if entries, err := os.ReadDir(workDir); err != nil || len(entries) != 0 {
				t.Fatalf("invalid input changed work directory: entries = %v, error = %v", entries, err)
			}
		})
	}
}

func TestMaterializeComponentCancellation(t *testing.T) {
	for _, when := range []string{"before writing", "during writing", "after writing"} {
		t.Run(when, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if when == "before writing" {
				cancel()
			}
			workDir := t.TempDir()
			cache := runtimebundle.NewProcessLocalCache(filepath.Join(t.TempDir(), "cache"))
			called := false
			materialized, err := materializeComponent(ctx, cache, workDir, runtimebundle.NamespaceProject, "", func(dst io.Writer) error {
				called = true
				if when == "during writing" {
					cancel()
				}
				_, err := dst.Write([]byte("partial ZIP"))
				cancel()
				return err
			})
			if materialized != nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("materializeComponent = %v, %v; want context cancellation", materialized, err)
			}
			if called != (when != "before writing") {
				t.Fatalf("writer called = %v for cancellation %s", called, when)
			}
			if _, err := os.Stat(cache.Root); !os.IsNotExist(err) {
				t.Fatalf("canceled materialization created cache storage: %v", err)
			}
		})
	}
}

func TestMaterializeComponentDoesNotOverwriteArchive(t *testing.T) {
	workDir := t.TempDir()
	zipPath := filepath.Join(workDir, "project.zip")
	want := []byte("existing archive")
	if err := os.WriteFile(zipPath, want, 0o600); err != nil {
		t.Fatal(err)
	}
	materialized, err := materializeComponent(context.Background(), nil, workDir, runtimebundle.NamespaceProject, "", func(io.Writer) error {
		t.Fatal("writer called for existing archive")
		return nil
	})
	if materialized != nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("materializeComponent = %v, %v; want existing archive error", materialized, err)
	}
	if got, err := os.ReadFile(zipPath); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("existing archive = %q, %v; want %q", got, err, want)
	}
}

func TestMaterializeComponentWriterFailureClosesArchive(t *testing.T) {
	wantErr := errors.New("component writer failed")
	var writer io.Writer
	materialized, err := materializeComponent(context.Background(), nil, t.TempDir(), runtimebundle.NamespaceProject, "", func(dst io.Writer) error {
		writer = dst
		if _, err := dst.Write([]byte("partial ZIP")); err != nil {
			t.Fatal(err)
		}
		return wantErr
	})
	if materialized != nil || !errors.Is(err, wantErr) {
		t.Fatalf("materializeComponent = %v, %v; want %v", materialized, err, wantErr)
	}
	if _, err := writer.Write([]byte("more")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after failed materialization = %v, want closed archive", err)
	}
}

func TestMaterializeComponentValidatesArchiveBeforeCaching(t *testing.T) {
	data := launcherTestZIP(t, []launcherTestFile{{Name: "main.spx", Mode: 0o644, Data: []byte("onStart => {}")}})
	for _, tt := range []struct {
		name    string
		data    []byte
		message string
	}{
		{"invalid ZIP", []byte("not a ZIP"), zip.ErrFormat.Error()},
		{"changed identity", data, "embedded project identity changed after payload verification"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cache := runtimebundle.NewProcessLocalCache(filepath.Join(t.TempDir(), "cache"))
			var writer io.Writer
			materialized, err := materializeComponent(context.Background(), cache, t.TempDir(), runtimebundle.NamespaceProject, strings.Repeat("0", 64), func(dst io.Writer) error {
				writer = dst
				_, err := dst.Write(tt.data)
				return err
			})
			if materialized != nil || err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("materializeComponent = %v, %v; want %q", materialized, err, tt.message)
			}
			if _, err := os.Stat(cache.Root); !os.IsNotExist(err) {
				t.Fatalf("invalid archive created cache storage: %v", err)
			}
			if _, err := writer.Write([]byte("more")); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("write after archive rejection = %v, want closed archive", err)
			}
		})
	}
}

func TestMaterializeComponent(t *testing.T) {
	want := []byte("onStart => {}")
	data := launcherTestZIP(t, []launcherTestFile{{Name: "main.spx", Mode: 0o644, Data: want}})
	namespace := runtimebundle.NamespaceProject
	bundle := launcherTestBundle(t, data, namespace)
	cache := runtimebundle.NewProcessLocalCache(filepath.Join(t.TempDir(), "cache"))
	var writer io.Writer
	materialized, err := materializeComponent(context.Background(), cache, t.TempDir(), namespace, bundle.Digest, func(dst io.Writer) error {
		writer = dst
		_, err := dst.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	defer materialized.Close()
	if wantPath := filepath.Join(cache.Root, string(namespace), bundle.Digest); materialized.Path != wantPath {
		t.Fatalf("materialized path = %q, want %q", materialized.Path, wantPath)
	}
	if got, err := os.ReadFile(filepath.Join(materialized.Path, "main.spx")); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("materialized file = %q, %v; want %q", got, err, want)
	}
	if _, err := writer.Write([]byte("more")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after materialization = %v, want closed archive", err)
	}
}

func TestMaterializeComponentPropagatesCacheError(t *testing.T) {
	data := launcherTestZIP(t, []launcherTestFile{{Name: "main.spx", Mode: 0o644, Data: []byte("onStart => {}")}})
	bundle := launcherTestBundle(t, data, runtimebundle.NamespaceProject)
	cache := runtimebundle.NewProcessLocalCache(filepath.Join(t.TempDir(), "cache"))
	cache.LockProvider = nil
	materialized, err := materializeComponent(context.Background(), cache, t.TempDir(), runtimebundle.NamespaceProject, bundle.Digest, func(dst io.Writer) error {
		_, err := dst.Write(data)
		return err
	})
	if materialized != nil || !errors.Is(err, runtimebundle.ErrCrossProcessLockUnsupported) {
		t.Fatalf("materializeComponent = %v, %v; want cache lock error", materialized, err)
	}
}

func TestContextWriter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var dst bytes.Buffer
	writer := contextWriter{ctx: ctx, dst: &dst}
	if n, err := writer.Write([]byte("first")); n != 5 || err != nil || dst.String() != "first" {
		t.Fatalf("Write = %d, %v, buffer = %q; want 5, nil, first", n, err, dst.String())
	}
	cancel()
	if n, err := writer.Write([]byte("second")); n != 0 || !errors.Is(err, context.Canceled) || dst.String() != "first" {
		t.Fatalf("canceled Write = %d, %v, buffer = %q; want 0, canceled, first", n, err, dst.String())
	}

	wantErr := errors.New("destination failed")
	writer = contextWriter{ctx: context.Background(), dst: materializeTestWriter(func(p []byte) (int, error) {
		if string(p) != "partial" {
			t.Fatalf("destination received %q, want partial", p)
		}
		return 2, wantErr
	})}
	if n, err := writer.Write([]byte("partial")); n != 2 || !errors.Is(err, wantErr) {
		t.Fatalf("failed Write = %d, %v; want 2, %v", n, err, wantErr)
	}
}

type materializeTestWriter func([]byte) (int, error)

func (write materializeTestWriter) Write(p []byte) (int, error) { return write(p) }
