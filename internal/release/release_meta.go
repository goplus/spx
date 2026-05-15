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

// Package release provides shared SPX/runtime release metadata.
package release

const (
	// RuntimeURLBase is the base URL for downloading runtime executables.
	// Format: https://github.com/goplus/godot/releases/download/spx{runtimeVersion}/{platform}-{arch}.zip
	RuntimeURLBase = "https://github.com/goplus/godot/releases/download/"

	// SpxReleaseURLBase is the base URL for downloading SPX release assets.
	// Format: https://github.com/goplus/spx/releases/download/{spxTag}/{assetName}
	SpxReleaseURLBase = "https://github.com/goplus/spx/releases/download/"

	// RuntimeTag is the filename prefix for the runtime binary/pck files.
	RuntimeTag = "gdspxrt"

	// RuntimeAssetZipName is the fixed runtime asset bundle name published under each SPX release tag.
	RuntimeAssetZipName = "spx-runtime-assets.zip"
)

// ReleaseMeta captures the selected SPX/runtime asset mapping.
type ReleaseMeta struct {
	SPXVersion string
	Runtime    RuntimeRelease
}

// RuntimeRelease describes the Godot runtime bundle used by SPX.
type RuntimeRelease struct {
	Version string
}

type releaseVersionMapping struct {
	spxVersion     string
	runtimeVersion string
}

var releaseVersionMappings = []releaseVersionMapping{
	{
		spxVersion:     "v2.0.0",
		runtimeVersion: "2.2.0",
	},
	{
		spxVersion:     "v2.0.1",
		runtimeVersion: "2.2.1",
	},
}

func newReleaseMeta(mapping releaseVersionMapping) ReleaseMeta {
	return ReleaseMeta{
		SPXVersion: mapping.spxVersion,
		Runtime: RuntimeRelease{
			Version: mapping.runtimeVersion,
		},
	}
}

var releaseMetaBySPXVersion = func() map[string]ReleaseMeta {
	result := make(map[string]ReleaseMeta, len(releaseVersionMappings))
	for _, item := range releaseVersionMappings {
		result[item.spxVersion] = newReleaseMeta(item)
	}
	return result
}()

// DefaultReleaseMeta returns the latest known released metadata.
func DefaultReleaseMeta() ReleaseMeta {
	if len(releaseVersionMappings) == 0 {
		panic("release: releaseVersionMappings is empty")
	}
	item := releaseVersionMappings[len(releaseVersionMappings)-1]
	return newReleaseMeta(item)
}

// CurrentReleaseMeta is kept as a compatibility alias for the default release metadata.
func CurrentReleaseMeta() ReleaseMeta {
	return DefaultReleaseMeta()
}

// ReleaseMetaForSPXVersion resolves runtime/pck metadata for an SPX module version.
// Unknown versions fall back to the latest known runtime assets.
func ReleaseMetaForSPXVersion(spxVersion string) ReleaseMeta {
	if meta, ok := releaseMetaBySPXVersion[spxVersion]; ok {
		return meta
	}
	return DefaultReleaseMeta()
}

// RuntimeBinaryTag returns the runtime executable/pck base filename.
func (m ReleaseMeta) RuntimeBinaryTag() string {
	return RuntimeTag + m.Runtime.Version
}

// RuntimeDownloadURL returns the runtime archive URL for the given zip asset name.
func (m ReleaseMeta) RuntimeDownloadURL(zipName string) string {
	return RuntimeURLBase + "spx" + m.Runtime.Version + "/" + zipName
}

// RuntimeAssetDownloadURL returns the packaged runtime asset bundle URL for the given zip asset name.
func (m ReleaseMeta) RuntimeAssetDownloadURL(zipName string) string {
	return SpxReleaseURLBase + m.SPXVersion + "/" + zipName
}
