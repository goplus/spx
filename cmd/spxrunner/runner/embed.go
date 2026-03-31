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

	"github.com/goplus/spx/v2/internal/releasemeta"
)

// Sync files from repository before building.
// Run: go generate ./cmd/spxrunner/runner
//
//go:generate cp ../../../gop.mod gop.mod
//go:generate cp ../../gox/template/go.mod.template go.mod.template

// GopModTemplate is the embedded content of gop.mod from the SPX repository root.
// This template is used to create gop.mod for new SPX projects.
//
//go:embed gop.mod
var GopModTemplate string

// GoModTemplate is the embedded content of go.mod.template
// This template is used to create go.mod for SPX project's Go code.
//
//go:embed go.mod.template
var GoModTemplate string

// RuntimeVersion returns the default SPX runtime version.
func RuntimeVersion() string {
	return releasemeta.DefaultReleaseMeta().Runtime.Version
}

// Version returns the SPX runtime version.
// Deprecated: use RuntimeVersion instead.
func Version() string {
	return RuntimeVersion()
}
