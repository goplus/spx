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
	"fmt"
	"slices"
)

// HostRuntimeSpec describes the runtime files needed to execute a project on
// one host. BinaryName is the member name inside ArchiveName; the other names
// are the files materialized into the host runtime directory.
type HostRuntimeSpec struct {
	GOOS        string
	GOARCH      string
	Platform    string
	ArchiveName string
	BinaryName  string
	RuntimeName string
	PackName    string
}

// HostRuntimeSpecFor resolves the release asset and runtime filenames for a
// host. The lock is checked before deriving any names, and the host archive
// must be explicitly included in RequiredAssets.
func HostRuntimeSpecFor(lock RuntimeLock, goos, goarch string) (HostRuntimeSpec, error) {
	if err := lock.Validate(); err != nil {
		return HostRuntimeSpec{}, err
	}

	releaseArch, ok := hostReleaseArch(goarch)
	if !ok {
		return HostRuntimeSpec{}, fmt.Errorf("release: unsupported host platform %s/%s", goos, goarch)
	}

	platform, binaryPlatform, ok := hostPlatform(goos)
	if !ok {
		return HostRuntimeSpec{}, fmt.Errorf("release: unsupported host platform %s/%s", goos, goarch)
	}

	archiveName := platform + "-" + releaseArch + ".zip"
	if !slices.Contains(lock.RequiredAssets, archiveName) {
		return HostRuntimeSpec{}, fmt.Errorf("release: runtime lock does not require host asset %q", archiveName)
	}

	binaryName := fmt.Sprintf("godot.%s.template_release.%s", binaryPlatform, releaseArch)
	runtimeName := RuntimeTag + lock.RuntimeVersion
	if goos == "windows" {
		binaryName += ".exe"
		runtimeName += ".exe"
	}
	return HostRuntimeSpec{
		GOOS:        goos,
		GOARCH:      goarch,
		Platform:    platform,
		ArchiveName: archiveName,
		BinaryName:  binaryName,
		RuntimeName: runtimeName,
		PackName:    RuntimeTag + lock.RuntimeVersion + ".pck",
	}, nil
}

func hostReleaseArch(goarch string) (string, bool) {
	switch goarch {
	case "amd64":
		return "x86_64", true
	case "arm64":
		return "arm64", true
	default:
		return "", false
	}
}

func hostPlatform(goos string) (platform, binaryPlatform string, ok bool) {
	switch goos {
	case "linux":
		return "linux", "linuxbsd", true
	case "darwin":
		return "macos", "macos", true
	case "windows":
		return "windows", "windows", true
	default:
		return "", "", false
	}
}
