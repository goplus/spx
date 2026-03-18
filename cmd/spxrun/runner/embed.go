/*
 * Copyright (c) 2024 The XGo Authors (xgo.dev). All rights reserved.
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

package runner

import (
	_ "embed"
	"strings"
)

// Sync files from repository before building.
// Run: go generate ./cmd/spxrun/runner
//
//go:generate cp ../../../gox.mod gox.mod
//go:generate cp ../../gox/template/version version
//go:generate cp ../../gox/template/go.mod.template go.mod.template

// GoxModTemplate is the embedded content of gox.mod from the spx repository root.
// This template is used to create gox.mod for new spx projects.
//
//go:embed gox.mod
var GoxModTemplate string

// GoModTemplate is the embedded content of go.mod.template
// This template is used to create go.mod for spx project's Go code.
//
//go:embed go.mod.template
var GoModTemplate string

// versionFile is the embedded version from cmd/gox/template/version
//
//go:embed version
var versionFile string

// Version returns the spx runtime version (trimmed)
func Version() string {
	return strings.TrimSpace(versionFile)
}
