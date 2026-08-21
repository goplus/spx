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

package scaffold

import (
	"fmt"
	"strings"
)

type gdExtensionLibrary struct {
	key  string
	file string
}

var desktopGDExtensionLibraries = []gdExtensionLibrary{
	{"macos.debug.x86_64", "gdspx-darwin-amd64.dylib"},
	{"macos.release.x86_64", "gdspx-darwin-amd64.dylib"},
	{"macos.debug.arm64", "gdspx-darwin-arm64.dylib"},
	{"macos.release.arm64", "gdspx-darwin-arm64.dylib"},
	{"windows.debug.x86_64", "gdspx-windows-amd64.dll"},
	{"windows.release.x86_64", "gdspx-windows-amd64.dll"},
	{"windows.debug.x86_32", "gdspx-windows-386.dll"},
	{"windows.release.x86_32", "gdspx-windows-386.dll"},
	{"linux.debug.x86_64", "gdspx-linux-amd64.so"},
	{"linux.release.x86_64", "gdspx-linux-amd64.so"},
	{"linux.debug.x86_32", "gdspx-linux-386.so"},
	{"linux.release.x86_32", "gdspx-linux-386.so"},
	{"linux.debug.arm64", "gdspx-linux-arm64.so"},
	{"linux.release.arm64", "gdspx-linux-arm64.so"},
}

var projectOnlyGDExtensionLibraries = []gdExtensionLibrary{
	{"android.debug.arm64", "libgdspx-android-arm64.so"},
	{"android.release.arm64", "libgdspx-android-arm64.so"},
	{"ios.debug", "libgdspx.ios.xcframework"},
	{"ios.release", "libgdspx.ios.xcframework"},
}

var (
	runtimeGDExtension        = renderGDExtension("", desktopGDExtensionLibraries)
	sessionRuntimeGDExtension = renderGDExtension("res://", desktopGDExtensionLibraries)
	projectGDExtension        = renderGDExtension("res://lib/", joinGDExtensionLibraries(desktopGDExtensionLibraries, projectOnlyGDExtensionLibraries))
)

const (
	runtimeExtensionList = "res://runtime.gdextension\n"
	sessionExtensionList = "res://gdspx.gdextension\n"
)

// RuntimeGDExtension returns the default runtime.gdextension template used by desktop runtime.
func RuntimeGDExtension() string {
	return runtimeGDExtension
}

// SessionRuntimeGDExtension pins bridge libraries to the session root.
func SessionRuntimeGDExtension() string {
	return sessionRuntimeGDExtension
}

// RuntimeExtensionList returns the standard Godot extension list used by the
// temporary desktop runtime project.
func RuntimeExtensionList() string {
	return runtimeExtensionList
}

// SessionExtensionList selects the session-local extension descriptor.
func SessionExtensionList() string { return sessionExtensionList }

// ProjectGDExtension returns the project gdspx.gdextension template copied by project creation flows.
func ProjectGDExtension() string {
	return projectGDExtension
}

func joinGDExtensionLibraries(groups ...[]gdExtensionLibrary) []gdExtensionLibrary {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	entries := make([]gdExtensionLibrary, 0, total)
	for _, group := range groups {
		entries = append(entries, group...)
	}
	return entries
}

func renderGDExtension(prefix string, libraries []gdExtensionLibrary) string {
	var builder strings.Builder
	builder.WriteString(`[configuration]

entry_symbol = "gdspx_init"
compatibility_minimum = 4.1

[libraries]

`)
	for _, library := range libraries {
		fmt.Fprintf(&builder, `%s = "%s%s"`+"\n", library.key, prefix, library.file)
	}
	return builder.String()
}
