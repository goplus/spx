/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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

package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPortableComponentCharacters(t *testing.T) {
	for _, tt := range []struct {
		name string
		want PathComponentProblem
	}{
		{"sprite.png", ComponentOK}, {"精灵.png", ComponentOK}, {"name.", TrailingDotOrSpace}, {"name ", TrailingDotOrSpace}, {"a:b", ReservedCharacter}, {"a?b", ReservedCharacter}, {"a\x00b", ControlCharacter}, {"a\nb", ControlCharacter},
	} {
		if got := CheckPathComponent(tt.name); got != tt.want {
			t.Errorf("%q: %v, want %v", tt.name, got, tt.want)
		}
	}
}
func TestDeviceBasenames(t *testing.T) {
	for _, name := range []string{"CON", "con", "CLOCK$", "clock$", "COM1", "com9", "LPT¹", "lpt³"} {
		if !IsDeviceBase(name) {
			t.Errorf("accepted device %q", name)
		}
	}
	for _, name := range []string{"", "CONSOLE", "COM0", "COM10", "LPT4x", "CLOCK$", "Con"} {
		if IsDeviceBase(name) {
			t.Errorf("changed caller folding policy for %q", name)
		}
	}
}

func TestCanonicalPathWithRemovedWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps the working directory open")
	}
	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.Remove(cwd); err != nil {
		t.Fatal(err)
	}
	if _, err := CanonicalPath("."); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("CanonicalPath in a removed working directory = %v, want ErrNotExist", err)
	}
}

func TestIsWithinRoot(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name, path string
		want       bool
	}{
		{"root itself", root, true},
		{"child", filepath.Join(root, "assets"), true},
		{"parent", filepath.Dir(root), false},
		{"sibling with matching prefix", root + "-other", false},
		{"relative path against absolute root", "assets", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsWithinRoot(tc.path, root); got != tc.want {
				t.Fatalf("IsWithinRoot(%q, %q) = %v, want %v", tc.path, root, got, tc.want)
			}
		})
	}
}
