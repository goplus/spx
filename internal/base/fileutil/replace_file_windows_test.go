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

package fileutil

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsPath(t *testing.T) {
	volume := filepath.VolumeName(t.TempDir())
	longTail := strings.Repeat(`segment\`, 40) + "file"
	drivePath := filepath.Join(volume+`\`, longTail)
	uncPath := `\\server\share\` + longTail
	for _, test := range []struct{ path, want string }{
		{"", ""},
		{"relative", "relative"},
		{filepath.Join(volume+`\`, "short"), filepath.Join(volume+`\`, "short")},
		{drivePath, `\\?\` + drivePath},
		{uncPath, `\\?\UNC\server\share\` + longTail},
		{`\\?\` + drivePath, `\\?\` + drivePath},
		{`\\?\UNC\server\share\` + longTail, `\\?\UNC\server\share\` + longTail},
		{`\\.\` + drivePath, `\\.\` + drivePath},
		{`\??\` + drivePath, `\??\` + drivePath},
	} {
		if got, err := windowsPath(test.path); err != nil || got != test.want {
			t.Fatalf("windowsPath(%q) = %q, %v; want %q", test.path, got, err, test.want)
		}
	}
}

func TestReplaceFileWindowsLongPaths(t *testing.T) {
	dir := t.TempDir()
	for len(dir) <= 300 {
		dir = filepath.Join(dir, "segment-0123456789")
	}
	testReplaceFile(t, dir)
}

func TestReplaceFileWindowsLockedDestination(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
	writeReplacementFile(t, src, "replacement", 0o600)
	writeReplacementFile(t, dst, "existing", 0o600)
	path, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		t.Fatal(err)
	}
	// A reader that does not share deletion must prevent replacement.
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if err := ReplaceFile(src, dst); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("locked destination error = %v, want sharing violation", err)
	}
	assertReplacementContent(t, dst, "existing")
	assertReplacementContent(t, src, "replacement")
}
