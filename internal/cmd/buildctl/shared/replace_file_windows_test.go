//go:build windows

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

package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsPath(t *testing.T) {
	volume := filepath.VolumeName(t.TempDir())
	longTail := strings.Repeat(`segment\`, 40) + "file"

	drivePath := filepath.Join(volume+`\`, longTail)
	got, err := windowsPath(drivePath)
	if err != nil {
		t.Fatal(err)
	}
	if want := `\\?\` + drivePath; got != want {
		t.Fatalf("windowsPath(%q) = %q, want %q", drivePath, got, want)
	}

	uncPath := `\\server\share\` + longTail
	got, err = windowsPath(uncPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := `\\?\UNC\server\share\` + longTail; got != want {
		t.Fatalf("windowsPath(%q) = %q, want %q", uncPath, got, want)
	}

	extended := `\\?\` + drivePath
	if got, err := windowsPath(extended); err != nil || got != extended {
		t.Fatalf("windowsPath(%q) = %q, %v", extended, got, err)
	}
}

func TestReplaceFileWindows(t *testing.T) {
	for _, test := range []struct {
		name     string
		existing bool
		longPath bool
	}{
		{name: "new destination"},
		{name: "existing destination", existing: true},
		{name: "new long destination", longPath: true},
		{name: "existing long destination", existing: true, longPath: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if test.longPath {
				for len(filepath.Join(dir, "source")) <= 300 {
					dir = filepath.Join(dir, "segment-0123456789")
				}
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			src := filepath.Join(dir, "source")
			dst := filepath.Join(dir, "destination")
			if err := os.WriteFile(src, []byte("replacement"), 0o600); err != nil {
				t.Fatal(err)
			}
			if test.existing {
				if err := os.WriteFile(dst, []byte("existing"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if err := replaceFile(src, dst); err != nil {
				t.Fatalf("replaceFile returned error: %v", err)
			}
			if data, err := os.ReadFile(dst); err != nil || string(data) != "replacement" {
				t.Fatalf("destination content = %q, err = %v", data, err)
			}
			if _, err := os.Lstat(src); !os.IsNotExist(err) {
				t.Fatalf("source still exists: %v", err)
			}
		})
	}
}
