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

package release

import (
	"slices"
	"strings"
	"testing"
)

func TestHostRuntimeSpecFor(t *testing.T) {
	lock := DefaultRuntimeLock()
	runtimeName := RuntimeTag + lock.RuntimeVersion
	for _, test := range []struct {
		goos         string
		goarch       string
		wantPlatform string
		wantArchive  string
		wantBinary   string
	}{
		{goos: "linux", goarch: "amd64", wantPlatform: "linux", wantArchive: "linux-x86_64.zip", wantBinary: "godot.linuxbsd.template_release.x86_64"},
		{goos: "darwin", goarch: "amd64", wantPlatform: "macos", wantArchive: "macos-x86_64.zip", wantBinary: "godot.macos.template_release.x86_64"},
		{goos: "darwin", goarch: "arm64", wantPlatform: "macos", wantArchive: "macos-arm64.zip", wantBinary: "godot.macos.template_release.arm64"},
		{goos: "windows", goarch: "amd64", wantPlatform: "windows", wantArchive: "windows-x86_64.zip", wantBinary: "godot.windows.template_release.x86_64.exe"},
	} {
		t.Run(test.goos+"/"+test.goarch, func(t *testing.T) {
			spec, err := HostRuntimeSpecFor(lock, test.goos, test.goarch)
			if err != nil {
				t.Fatal(err)
			}
			if spec.GOOS != test.goos || spec.GOARCH != test.goarch {
				t.Fatalf("host identity = %s/%s, want %s/%s", spec.GOOS, spec.GOARCH, test.goos, test.goarch)
			}
			wantRuntime := runtimeName
			if test.goos == "windows" {
				wantRuntime += ".exe"
			}
			if spec.Platform != test.wantPlatform || spec.ArchiveName != test.wantArchive || spec.BinaryName != test.wantBinary || spec.RuntimeName != wantRuntime {
				t.Fatalf("spec = %#v", spec)
			}
			if spec.PackName != runtimeName+".pck" {
				t.Fatalf("pack name = %q", spec.PackName)
			}
		})
	}
}

func TestHostRuntimeSpecForSupportsMissingReleaseAssetsWhenLocked(t *testing.T) {
	lock := DefaultRuntimeLock()
	lock.RequiredAssets = append(lock.RequiredAssets, "linux-arm64.zip", "windows-arm64.zip")
	slices.Sort(lock.RequiredAssets)

	for _, test := range []struct {
		goos        string
		goarch      string
		wantArchive string
		wantBinary  string
	}{
		{goos: "linux", goarch: "arm64", wantArchive: "linux-arm64.zip", wantBinary: "godot.linuxbsd.template_release.arm64"},
		{goos: "windows", goarch: "arm64", wantArchive: "windows-arm64.zip", wantBinary: "godot.windows.template_release.arm64.exe"},
	} {
		t.Run(test.goos+"/"+test.goarch, func(t *testing.T) {
			spec, err := HostRuntimeSpecFor(lock, test.goos, test.goarch)
			if err != nil {
				t.Fatal(err)
			}
			if spec.ArchiveName != test.wantArchive || spec.BinaryName != test.wantBinary {
				t.Fatalf("spec = %#v", spec)
			}
		})
	}
}

func TestHostRuntimeSpecForRejectsMissingAsset(t *testing.T) {
	lock := DefaultRuntimeLock()
	if _, err := HostRuntimeSpecFor(lock, "linux", "arm64"); err == nil || !strings.Contains(err.Error(), "linux-arm64.zip") {
		t.Fatalf("HostRuntimeSpecFor missing-asset error = %v", err)
	}
	if _, err := HostRuntimeSpecFor(lock, "windows", "arm64"); err == nil || !strings.Contains(err.Error(), "windows-arm64.zip") {
		t.Fatalf("HostRuntimeSpecFor missing-asset error = %v", err)
	}
}

func TestHostRuntimeSpecForRejectsUnsupportedHost(t *testing.T) {
	lock := DefaultRuntimeLock()
	for _, test := range []struct {
		goos   string
		goarch string
	}{
		{goos: "freebsd", goarch: "amd64"},
		{goos: "linux", goarch: "386"},
	} {
		t.Run(test.goos+"/"+test.goarch, func(t *testing.T) {
			if _, err := HostRuntimeSpecFor(lock, test.goos, test.goarch); err == nil || !strings.Contains(err.Error(), "unsupported host platform") {
				t.Fatalf("HostRuntimeSpecFor error = %v", err)
			}
		})
	}
}

func TestHostRuntimeSpecForValidatesLock(t *testing.T) {
	lock := DefaultRuntimeLock()
	lock.RuntimeVersion = "invalid"
	if _, err := HostRuntimeSpecFor(lock, "linux", "amd64"); err == nil || !strings.Contains(err.Error(), "invalid runtime version") {
		t.Fatalf("HostRuntimeSpecFor invalid-lock error = %v", err)
	}
}
