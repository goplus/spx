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

import (
	"fmt"
	"sort"
)

const (
	legacyEngineRepository  = "goplus/godot"
	legacyRuntimeRepository = "goplus/spx"
)

// AssetRelease identifies a GitHub release that owns a set of assets.
// Repository uses the owner/name form and Tag is the exact Git tag.
type AssetRelease struct {
	Repository   string
	Tag          string
	ManifestName string
}

// DownloadURL returns the GitHub release download URL for assetName.
func (r AssetRelease) DownloadURL(assetName string) string {
	return "https://github.com/" + r.Repository + "/releases/download/" + r.Tag + "/" + assetName
}

// ReleaseMeta captures the selected SPX/runtime asset mapping.
type ReleaseMeta struct {
	SPXVersion string
	Runtime    RuntimeRelease
}

// RuntimeRelease describes the runtime bundle used by SPX. Legacy releases
// keep engine binaries and the runtime pack in separate GitHub releases. The
// current atomic release stores both roles together and publishes a manifest.
type RuntimeRelease struct {
	Version          string
	EngineAssets     AssetRelease
	RuntimeAssets    AssetRelease
	RuntimePackAsset string
}

type spxRuntimeMapping struct {
	spxVersion     string
	runtimeVersion string
}

// Legacy releases predate immutable runtime lock snapshots and therefore keep
// their historical split asset locations here. Atomic releases are derived
// from versioned snapshots below, so their repository/tag/manifest cannot
// drift from the provenance used to verify them.
var legacyRuntimeReleaseDefinitions = []RuntimeRelease{
	newLegacyRuntimeRelease("2.2.0", "v2.0.0", "gdspxrt.pck.2.2.0.zip"),
	newLegacyRuntimeRelease("2.2.1", "v2.0.1", RuntimeAssetZipName),
	newLegacyRuntimeRelease("2.2.2", "v2.0.2", RuntimeAssetZipName),
	newLegacyRuntimeRelease("2.2.3", "v2.0.3", RuntimeAssetZipName),
	newLegacyRuntimeRelease("2.2.4", "v2.0.4", RuntimeAssetZipName),
	newLegacyRuntimeRelease("2.2.6", "v3.0.0", RuntimeAssetZipName),
	newLegacyRuntimeRelease("2.3.0", "v3.1.0", RuntimeAssetZipName),
}

// Historical mappings are immutable compatibility data. The current mapping
// is derived from currentSPXVersion and runtime.lock.json, so a release bump
// cannot update one side without the other.
var historicalSPXRuntimeMappings = []spxRuntimeMapping{
	{spxVersion: "v2.0.0", runtimeVersion: "2.2.0"},
	{spxVersion: "v2.0.1", runtimeVersion: "2.2.1"},
	{spxVersion: "v2.0.2", runtimeVersion: "2.2.2"},
	{spxVersion: "v2.0.3", runtimeVersion: "2.2.3"},
	{spxVersion: "v2.0.4", runtimeVersion: "2.2.4"},
	{spxVersion: "v3.0.0", runtimeVersion: "2.2.6"},
	{spxVersion: "v3.1.0", runtimeVersion: "2.3.0"},
	{spxVersion: "v3.2.0", runtimeVersion: "2.4.0"},
}

func allRuntimeReleaseDefinitions() []RuntimeRelease {
	releases := append([]RuntimeRelease(nil), legacyRuntimeReleaseDefinitions...)
	versions := make([]string, 0, len(runtimeLocksByVersion))
	for version := range runtimeLocksByVersion {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	for _, version := range versions {
		lock := runtimeLocksByVersion[version]
		releases = append(releases, newAtomicRuntimeRelease(version, lock.ReleaseRepository, lock.Manifest))
	}
	return releases
}

func allSPXRuntimeMappings() []spxRuntimeMapping {
	mappings := append([]spxRuntimeMapping(nil), historicalSPXRuntimeMappings...)
	return append(mappings, spxRuntimeMapping{
		spxVersion:     currentSPXVersion,
		runtimeVersion: DefaultRuntimeLock().RuntimeVersion,
	})
}

func newLegacyRuntimeRelease(runtimeVersion, runtimeAssetTag, runtimePackName string) RuntimeRelease {
	return RuntimeRelease{
		Version: runtimeVersion,
		EngineAssets: AssetRelease{
			Repository: legacyEngineRepository,
			Tag:        "spx" + runtimeVersion,
		},
		RuntimeAssets: AssetRelease{
			Repository: legacyRuntimeRepository,
			Tag:        runtimeAssetTag,
		},
		RuntimePackAsset: runtimePackName,
	}
}

func newAtomicRuntimeRelease(runtimeVersion, repository, manifestName string) RuntimeRelease {
	assets := AssetRelease{
		Repository:   repository,
		Tag:          "runtime-v" + runtimeVersion,
		ManifestName: manifestName,
	}
	return RuntimeRelease{
		Version:          runtimeVersion,
		EngineAssets:     assets,
		RuntimeAssets:    assets,
		RuntimePackAsset: RuntimeAssetZipName,
	}
}

type releaseCatalog struct {
	runtimeByVersion    map[string]RuntimeRelease
	runtimeVersionBySPX map[string]string
	primarySPXByRuntime map[string]string
}

func newReleaseCatalog(runtimeReleases []RuntimeRelease, mappings []spxRuntimeMapping) (releaseCatalog, error) {
	catalog := releaseCatalog{
		runtimeByVersion:    make(map[string]RuntimeRelease, len(runtimeReleases)),
		runtimeVersionBySPX: make(map[string]string, len(mappings)),
		primarySPXByRuntime: make(map[string]string, len(runtimeReleases)),
	}
	for _, runtimeRelease := range runtimeReleases {
		if runtimeRelease.Version == "" {
			return releaseCatalog{}, fmt.Errorf("release: runtime release version must not be empty")
		}
		if _, exists := catalog.runtimeByVersion[runtimeRelease.Version]; exists {
			return releaseCatalog{}, fmt.Errorf("release: duplicate runtime release %q", runtimeRelease.Version)
		}
		catalog.runtimeByVersion[runtimeRelease.Version] = runtimeRelease
	}
	for _, mapping := range mappings {
		if mapping.spxVersion == "" {
			return releaseCatalog{}, fmt.Errorf("release: SPX version must not be empty")
		}
		if _, exists := catalog.runtimeVersionBySPX[mapping.spxVersion]; exists {
			return releaseCatalog{}, fmt.Errorf("release: duplicate SPX release %q", mapping.spxVersion)
		}
		if _, exists := catalog.runtimeByVersion[mapping.runtimeVersion]; !exists {
			return releaseCatalog{}, fmt.Errorf("release: SPX release %q references unknown runtime %q", mapping.spxVersion, mapping.runtimeVersion)
		}
		catalog.runtimeVersionBySPX[mapping.spxVersion] = mapping.runtimeVersion
		if _, exists := catalog.primarySPXByRuntime[mapping.runtimeVersion]; !exists {
			catalog.primarySPXByRuntime[mapping.runtimeVersion] = mapping.spxVersion
		}
	}
	for runtimeVersion := range catalog.runtimeByVersion {
		if _, exists := catalog.primarySPXByRuntime[runtimeVersion]; !exists {
			return releaseCatalog{}, fmt.Errorf("release: runtime %q has no SPX release mapping", runtimeVersion)
		}
	}
	return catalog, nil
}

func mustNewReleaseCatalog(runtimeReleases []RuntimeRelease, mappings []spxRuntimeMapping) releaseCatalog {
	catalog, err := newReleaseCatalog(runtimeReleases, mappings)
	if err != nil {
		panic(err)
	}
	return catalog
}

func (c releaseCatalog) resolveSPXVersion(spxVersion string) (ReleaseMeta, bool) {
	runtimeVersion, ok := c.runtimeVersionBySPX[spxVersion]
	if !ok {
		return ReleaseMeta{}, false
	}
	return ReleaseMeta{SPXVersion: spxVersion, Runtime: c.runtimeByVersion[runtimeVersion]}, true
}

func (c releaseCatalog) resolveRuntimeVersion(runtimeVersion string) (ReleaseMeta, bool) {
	runtimeRelease, ok := c.runtimeByVersion[runtimeVersion]
	if !ok {
		return ReleaseMeta{}, false
	}
	// A runtime can serve multiple SPX releases. Keep the first declared SPX
	// mapping as the stable compatibility value for this legacy reverse API.
	return ReleaseMeta{SPXVersion: c.primarySPXByRuntime[runtimeVersion], Runtime: runtimeRelease}, true
}

var defaultReleaseCatalog = mustNewReleaseCatalog(allRuntimeReleaseDefinitions(), allSPXRuntimeMappings())

// DefaultReleaseMeta returns the current configured SPX/runtime mapping.
func DefaultReleaseMeta() ReleaseMeta {
	meta, ok := defaultReleaseCatalog.resolveSPXVersion(currentSPXVersion)
	if !ok {
		panic("release: current SPX version is not mapped")
	}
	lock := DefaultRuntimeLock()
	wantAssets := AssetRelease{
		Repository:   lock.ReleaseRepository,
		Tag:          lock.RuntimeReleaseTag(),
		ManifestName: lock.Manifest,
	}
	wantRuntime := RuntimeRelease{
		Version:          lock.RuntimeVersion,
		EngineAssets:     wantAssets,
		RuntimeAssets:    wantAssets,
		RuntimePackAsset: RuntimeAssetZipName,
	}
	if meta.Runtime != wantRuntime {
		panic("release: current release mapping does not match runtime.lock.json")
	}
	return meta
}

// ResolveReleaseMetaForSPXVersion resolves published runtime metadata for an
// SPX version. "latest" is an explicit alias for the latest known release.
func ResolveReleaseMetaForSPXVersion(spxVersion string) (ReleaseMeta, error) {
	if spxVersion == "latest" {
		return DefaultReleaseMeta(), nil
	}
	if meta, ok := defaultReleaseCatalog.resolveSPXVersion(spxVersion); ok {
		return meta, nil
	}
	return ReleaseMeta{}, fmt.Errorf("release: unknown SPX version %q", spxVersion)
}

// ResolveReleaseMetaForRuntimeVersion resolves published runtime metadata for
// a runtime version. "latest" is an explicit alias for the current release.
// If several SPX releases share an exact runtime, SPXVersion is the first
// declared mapping so appending a new SPX release cannot rewrite old metadata.
func ResolveReleaseMetaForRuntimeVersion(runtimeVersion string) (ReleaseMeta, error) {
	if runtimeVersion == "latest" {
		return DefaultReleaseMeta(), nil
	}
	if meta, ok := defaultReleaseCatalog.resolveRuntimeVersion(runtimeVersion); ok {
		return meta, nil
	}
	return ReleaseMeta{}, fmt.Errorf("release: unknown runtime version %q", runtimeVersion)
}

// RuntimeBinaryTag returns the runtime executable/pck base filename.
func (m ReleaseMeta) RuntimeBinaryTag() string {
	return RuntimeTag + m.Runtime.Version
}

// RuntimeDownloadURL returns the runtime archive URL for the given zip asset name.
func (m ReleaseMeta) RuntimeDownloadURL(zipName string) string {
	return m.Runtime.EngineAssets.DownloadURL(zipName)
}

// RuntimeAssetDownloadURL returns the packaged runtime asset bundle URL for the given zip asset name.
func (m ReleaseMeta) RuntimeAssetDownloadURL(zipName string) string {
	return m.Runtime.RuntimeAssets.DownloadURL(zipName)
}

// RuntimePackAssetName returns the exact runtime pack filename published for
// this SPX version. SPX v2.0.0 predates RuntimeAssetZipName.
func (m ReleaseMeta) RuntimePackAssetName() string {
	return m.Runtime.RuntimePackAsset
}

// RequiresRuntimeManifest reports whether this mapping uses the current atomic
// runtime release contract. Legacy releases intentionally have no manifest.
func (m ReleaseMeta) RequiresRuntimeManifest() bool {
	return m.Runtime.RuntimeAssets.ManifestName != ""
}

// RuntimeManifestDownloadURL returns the manifest URL for the selected atomic
// runtime release, or an empty string for a legacy release.
func (m ReleaseMeta) RuntimeManifestDownloadURL() string {
	if !m.RequiresRuntimeManifest() {
		return ""
	}
	assets := m.Runtime.RuntimeAssets
	return assets.DownloadURL(assets.ManifestName)
}
