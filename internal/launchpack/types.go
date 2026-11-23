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

// Package launchpack builds self-contained SPX launchers. It has no knowledge
// of XGo driver transport or provenance; callers provide those identities.
package launchpack

import (
	"context"
	"io"

	"github.com/goplus/spx/v4/internal/portableconfig"
	"github.com/goplus/spx/v4/internal/release"
	"github.com/goplus/spx/v4/internal/runtimepayload"
)

// IO describes command streams and the base environment for Go commands and
// runtime acquisition. Only a nil Env inherits the process environment.
type IO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    []string
}

// SourceIdentity records the source identity embedded in the launcher payload.
type SourceIdentity = runtimepayload.SourceIdentity

// Config is the complete input to launcher packaging.
type Config struct {
	ProjectDir  string
	ProjectFile string
	ProjectExt  string
	PackDir     string
	PackIndex   string

	PortableConfig portableconfig.PortableConfigSnapshot

	RuntimeSourceRoot   string
	RuntimeManifestPath string
	RuntimeAssetDir     string
	RuntimeCacheRoot    string
	RuntimeOffline      bool

	RuntimeLock release.RuntimeLock
	Source      SourceIdentity

	GoCommand  string
	WorkDir    string
	GoWork     string
	GraphFlags []string
	BuildFlags []string
	Output     string

	BridgePackage string
	VerifyGraph   func(context.Context) error
	VerifyBridge  func(string) error
	IO            IO
}

// Assets contains runtime and bridge files used by a launcher or direct run.
// Manifest-based acquisition also sets Lock and Cleanup; legacy installations
// have no manifest identity. The caller must invoke a non-nil Cleanup when done.
type Assets struct {
	EnginePath string
	PackPath   string
	BridgePath string
	Lock       release.RuntimeLock
	Cleanup    func()
}

// Result describes a successfully generated launcher and its embedded
// component identities.
type Result struct {
	Output         string
	PayloadSHA256  string
	ManifestSHA256 string
}
