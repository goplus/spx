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

package projectloader

import (
	"errors"
	"io"
	"os"
	"testing"

	spxfs "github.com/goplus/spx/v4/fs"
)

type resourceTestDir struct {
	path   string
	closed int
}

func (*resourceTestDir) Open(string) (io.ReadCloser, error) {
	return nil, os.ErrNotExist
}

func (d *resourceTestDir) Close() error {
	d.closed++
	return nil
}

func (d *resourceTestDir) GetPath() string {
	return d.path
}

type listedResourceTestDir struct {
	*resourceTestDir
	entries []spxfs.DirEntry
	readDir string
}

func (d *listedResourceTestDir) ReadDir(name string) ([]spxfs.DirEntry, error) {
	d.readDir = name
	return d.entries, nil
}

func TestEngineAssetsPreserveOptionalReadDir(t *testing.T) {
	for _, path := range []string{"", "/"} {
		base := &resourceTestDir{path: path}
		if got := withEngineAssets(base); got != base {
			t.Fatalf("directory with empty asset path %q was wrapped: %T", path, got)
		}
	}
	withoutReadDir := withEngineAssets(&resourceTestDir{path: "res://assets"})
	if _, ok := withoutReadDir.(spxfs.ReadDirer); ok {
		t.Fatalf("engine adapter unexpectedly implements spxfs.ReadDirer: %T", withoutReadDir)
	}
	if got := withEngineAssets(withoutReadDir); got != withoutReadDir {
		t.Fatalf("engine adapter was wrapped again: %T", got)
	}

	base := &listedResourceTestDir{
		resourceTestDir: &resourceTestDir{path: "res://assets"},
		entries:         []spxfs.DirEntry{{Name: "Shared", IsDir: true}},
	}
	withReadDir := withEngineAssets(base)
	reader, ok := withReadDir.(spxfs.ReadDirer)
	if !ok {
		t.Fatalf("engine adapter does not implement spxfs.ReadDirer: %T", withReadDir)
	}
	if got := withEngineAssets(withReadDir); got != withReadDir {
		t.Fatalf("engine ReadDir adapter was wrapped again: %T", got)
	}
	entries, err := reader.ReadDir("fonts")
	if err != nil {
		t.Fatal(err)
	}
	if base.readDir != "fonts" {
		t.Fatalf("delegated ReadDir path = %q, want fonts", base.readDir)
	}
	if len(entries) != 1 || entries[0] != base.entries[0] {
		t.Fatalf("delegated ReadDir entries = %#v, want %#v", entries, base.entries)
	}
}

func TestOpenResourcesRejectsUnsupportedResource(t *testing.T) {
	for _, resource := range []any{123, "projectloader-unregistered://assets"} {
		opened, err := OpenResources(resource, nil)
		if err == nil || opened.FS != nil {
			t.Fatalf("OpenResources(%v) = %+v, %v; want resource error and no directory", resource, opened, err)
		}
	}
}

type verifyingResourceTestDir struct {
	resourceTestDir
	verified string
	err      error
}

func (*verifyingResourceTestDir) Open(string) (io.ReadCloser, error) {
	panic("file verification must not open the source")
}

func (d *verifyingResourceTestDir) VerifyFile(name string) error {
	d.verified = name
	return d.err
}

func TestEngineAssetsPreserveFileVerifier(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{"success", nil},
		{"failure", errors.New("verification failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := &verifyingResourceTestDir{resourceTestDir: resourceTestDir{path: "assets"}, err: test.err}
			err := spxfs.VerifyFile(withEngineAssets(base), "font.ttf")
			if err != test.err || base.verified != "font.ttf" {
				t.Fatalf("source verification = %q, %v; want font.ttf, %v", base.verified, err, test.err)
			}
		})
	}
}
