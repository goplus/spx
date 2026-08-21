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

// Package runtimepayload implements the canonical, self-contained payload
// embedded in an XGo SPX launcher. It is internal because the driver argv,
// payload layout, and cache manifests are implementation details of SPX.
package runtimepayload

import (
	"io"
	"io/fs"
	"time"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

const (
	SchemaV1       = "spx-runtime-payload/v1"
	ProtocolV1     = "xgo-driver-v1"
	ManifestPath   = "META-INF/spx-runtime-v1.json"
	ProjectZipPath = "project/project.zip"
)

var canonicalTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// SourceIdentity records the logical and physical SPX identity selected by
// the application graph. Source mode intentionally records replacement/main
// provenance instead of pretending that a local build is a published release.
type SourceIdentity struct {
	SelectedPath     string `json:"selected_path"`
	SelectedVersion  string `json:"selected_version"`
	EffectivePath    string `json:"effective_path"`
	EffectiveVersion string `json:"effective_version"`
	Main             bool   `json:"main"`
	SourceMode       bool   `json:"source_mode"`
}

// Target identifies the host executable represented by the payload.
type Target struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

// Engine describes the verified engine component and its cache identity.
type Engine struct {
	RuntimeVersion        string `json:"runtime_version"`
	RuntimeABI            int    `json:"runtime_abi"`
	EngineInterfaceDigest string `json:"engine_interface_digest"`
	Executable            string `json:"executable"`
	Pack                  string `json:"pack"`
	BundleDigest          string `json:"bundle_digest"`
}

// Bridge describes the verified interpreter bridge component.
type Bridge struct {
	File         string `json:"file"`
	BundleDigest string `json:"bundle_digest"`
}

// Project describes the project materialized for launcher execution.
type Project struct {
	PackDirectory string `json:"pack_directory"`
	BundleDigest  string `json:"bundle_digest"`
	ArchiveSHA256 string `json:"archive_sha256"`
}

// Manifest is the only top-level payload manifest. Entries contains every ZIP
// entry except ManifestPath, avoiding a self-referential digest.
type Manifest struct {
	Schema   string                `json:"schema"`
	Protocol string                `json:"protocol"`
	SPX      SourceIdentity        `json:"spx"`
	Target   Target                `json:"target"`
	Engine   Engine                `json:"engine"`
	Bridge   Bridge                `json:"bridge"`
	Project  Project               `json:"project"`
	Entries  []runtimebundle.Entry `json:"entries"`
}

// FileSource is one repeatable, fixed-size payload entry. ReaderAt must remain
// readable and stable for the duration of BuildTo. BuildTo reads every source
// twice and rejects short reads or content changes between the two passes.
type FileSource struct {
	Name     string
	Mode     fs.FileMode
	ReaderAt io.ReaderAt
	Size     int64
}

// BuildConfig supplies the identities that cannot be inferred from entry
// bytes. Component bundle digests must be namespace-qualified identities from
// runtimebundle.Bundle.WithDigest.
type BuildConfig struct {
	SPX     SourceIdentity
	Target  Target
	Engine  Engine
	Bridge  Bridge
	Project Project
}

type preparedSource struct {
	source FileSource
	entry  runtimebundle.Entry
}
