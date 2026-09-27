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
	platform string
	arch     string
	file     string
}

var desktopGDExtensionLibraries = []gdExtensionLibrary{
	{"macos", "x86_64", "gdspx-darwin-amd64.dylib"},
	{"macos", "arm64", "gdspx-darwin-arm64.dylib"},
	{"windows", "x86_64", "gdspx-windows-amd64.dll"},
	{"windows", "x86_32", "gdspx-windows-386.dll"},
	{"linux", "x86_64", "gdspx-linux-amd64.so"},
	{"linux", "x86_32", "gdspx-linux-386.so"},
	{"linux", "arm64", "gdspx-linux-arm64.so"},
}

var projectOnlyGDExtensionLibraries = []gdExtensionLibrary{
	{"android", "arm64", "libgdspx-android-arm64.so"},
	{"ios", "", "libgdspx.ios.xcframework"},
}

var (
	runtimeGDExtension        = renderGDExtension("", desktopGDExtensionLibraries)
	sessionRuntimeGDExtension = renderGDExtension("res://", desktopGDExtensionLibraries)
	projectGDExtension        = renderGDExtension("res://lib/", desktopGDExtensionLibraries, projectOnlyGDExtensionLibraries)
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

func renderGDExtension(prefix string, groups ...[]gdExtensionLibrary) string {
	var builder strings.Builder
	builder.WriteString(`[configuration]

entry_symbol = "gdspx_init"
compatibility_minimum = 4.1

[libraries]

`)
	for _, libraries := range groups {
		for _, library := range libraries {
			for _, mode := range []string{"debug", "release"} {
				key := library.platform + "." + mode
				if library.arch != "" {
					key += "." + library.arch
				}
				fmt.Fprintf(&builder, `%s = "%s%s"`+"\n", key, prefix, library.file)
			}
		}
	}
	return builder.String()
}
