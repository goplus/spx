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

func TestDefaultReleaseMetaUsesAtomicSPXRuntimeRelease(t *testing.T) {
	lock := DefaultRuntimeLock()
	meta := DefaultReleaseMeta()
	if meta.SPXVersion != currentSPXVersion {
		t.Fatalf("SPX version = %q, want %q", meta.SPXVersion, currentSPXVersion)
	}
	if meta.Runtime.Version != lock.RuntimeVersion {
		t.Fatalf("runtime version = %q, want %q", meta.Runtime.Version, lock.RuntimeVersion)
	}
	wantRelease := AssetRelease{
		Repository:   lock.ReleaseRepository,
		Tag:          lock.RuntimeReleaseTag(),
		ManifestName: lock.Manifest,
	}
	if meta.Runtime.EngineAssets != wantRelease {
		t.Fatalf("engine release = %#v, want %#v", meta.Runtime.EngineAssets, wantRelease)
	}
	if meta.Runtime.RuntimeAssets != wantRelease {
		t.Fatalf("runtime release = %#v, want %#v", meta.Runtime.RuntimeAssets, wantRelease)
	}
	if got := meta.RuntimePackAssetName(); got != RuntimeAssetZipName {
		t.Fatalf("runtime pack asset = %q, want %q", got, RuntimeAssetZipName)
	}
	if !meta.RequiresRuntimeManifest() {
		t.Fatal("current release must require a runtime manifest")
	}

	wantPrefix := wantRelease.DownloadURL("")
	if got := meta.RuntimeDownloadURL("linux-x86_64.zip"); got != wantPrefix+"linux-x86_64.zip" {
		t.Fatalf("runtime download URL = %q", got)
	}
	if got := meta.RuntimeAssetDownloadURL(RuntimeAssetZipName); got != wantPrefix+RuntimeAssetZipName {
		t.Fatalf("runtime asset URL = %q", got)
	}
	if got := meta.RuntimeManifestDownloadURL(); got != wantPrefix+lock.Manifest {
		t.Fatalf("runtime manifest URL = %q", got)
	}
}

func TestReleaseCatalogAllowsSPXReleasesToShareRuntime(t *testing.T) {
	runtimeRelease := newAtomicRuntimeRelease("2.4.0", "goplus/spx", "runtime-manifest.json")
	catalog, err := newReleaseCatalog(
		[]RuntimeRelease{runtimeRelease},
		[]spxRuntimeMapping{
			{spxVersion: "v3.2.0", runtimeVersion: "2.4.0"},
			{spxVersion: "v3.2.1", runtimeVersion: "2.4.0"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	first, ok := catalog.resolveSPXVersion("v3.2.0")
	if !ok {
		t.Fatal("v3.2.0 mapping not found")
	}
	second, ok := catalog.resolveSPXVersion("v3.2.1")
	if !ok {
		t.Fatal("v3.2.1 mapping not found")
	}
	if first.Runtime != runtimeRelease || second.Runtime != runtimeRelease {
		t.Fatalf("shared runtime changed: first=%#v second=%#v", first.Runtime, second.Runtime)
	}

	byRuntime, ok := catalog.resolveRuntimeVersion("2.4.0")
	if !ok {
		t.Fatal("runtime 2.4.0 not found")
	}
	if byRuntime.SPXVersion != "v3.2.0" || byRuntime.Runtime != runtimeRelease {
		t.Fatalf("reverse runtime mapping = %#v", byRuntime)
	}
}

func TestReleaseCatalogRejectsDuplicateSPXRelease(t *testing.T) {
	runtimeRelease := newAtomicRuntimeRelease("2.4.0", "goplus/spx", "runtime-manifest.json")
	_, err := newReleaseCatalog(
		[]RuntimeRelease{runtimeRelease},
		[]spxRuntimeMapping{
			{spxVersion: "v3.2.0", runtimeVersion: "2.4.0"},
			{spxVersion: "v3.2.0", runtimeVersion: "2.4.0"},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate SPX release") {
		t.Fatalf("duplicate SPX mapping error = %v", err)
	}
}

func TestReleaseCatalogRejectsDuplicateRuntimeRelease(t *testing.T) {
	runtimeRelease := newAtomicRuntimeRelease("2.4.0", "goplus/spx", "runtime-manifest.json")
	_, err := newReleaseCatalog(
		[]RuntimeRelease{runtimeRelease, runtimeRelease},
		[]spxRuntimeMapping{{spxVersion: "v3.2.0", runtimeVersion: "2.4.0"}},
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate runtime release") {
		t.Fatalf("duplicate runtime definition error = %v", err)
	}
}

func TestAtomicRuntimeReleasesHaveMatchingLockSnapshots(t *testing.T) {
	atomicVersions := make(map[string]struct{})
	for _, runtimeRelease := range allRuntimeReleaseDefinitions() {
		if runtimeRelease.EngineAssets.ManifestName == "" && runtimeRelease.RuntimeAssets.ManifestName == "" {
			continue
		}
		atomicVersions[runtimeRelease.Version] = struct{}{}
		t.Run(runtimeRelease.Version, func(t *testing.T) {
			if runtimeRelease.EngineAssets != runtimeRelease.RuntimeAssets {
				t.Fatalf("atomic release splits asset owners: %#v", runtimeRelease)
			}
			lock, err := RuntimeLockForVersion(runtimeRelease.Version)
			if err != nil {
				t.Fatal(err)
			}
			assets := runtimeRelease.EngineAssets
			if assets.Repository != lock.ReleaseRepository {
				t.Fatalf("release repository = %q, snapshot = %q", assets.Repository, lock.ReleaseRepository)
			}
			if assets.Tag != lock.RuntimeReleaseTag() {
				t.Fatalf("release tag = %q, snapshot = %q", assets.Tag, lock.RuntimeReleaseTag())
			}
			if assets.ManifestName != lock.Manifest {
				t.Fatalf("manifest = %q, snapshot = %q", assets.ManifestName, lock.Manifest)
			}
			if runtimeRelease.RuntimePackAsset != RuntimeAssetZipName || !slices.Contains(lock.RequiredAssets, runtimeRelease.RuntimePackAsset) {
				t.Fatalf("runtime pack %q is not declared by snapshot", runtimeRelease.RuntimePackAsset)
			}
		})
	}
	for version := range runtimeLocksByVersion {
		if _, ok := atomicVersions[version]; !ok {
			t.Errorf("runtime lock snapshot %q has no atomic runtime release definition", version)
		}
	}
}

func TestLegacyReleaseMappings(t *testing.T) {
	tests := []struct {
		spxVersion     string
		runtimeVersion string
		packName       string
	}{
		{spxVersion: "v2.0.0", runtimeVersion: "2.2.0", packName: "gdspxrt.pck.2.2.0.zip"},
		{spxVersion: "v2.0.1", runtimeVersion: "2.2.1", packName: RuntimeAssetZipName},
		{spxVersion: "v2.0.2", runtimeVersion: "2.2.2", packName: RuntimeAssetZipName},
		{spxVersion: "v2.0.3", runtimeVersion: "2.2.3", packName: RuntimeAssetZipName},
		{spxVersion: "v2.0.4", runtimeVersion: "2.2.4", packName: RuntimeAssetZipName},
		{spxVersion: "v3.0.0", runtimeVersion: "2.2.6", packName: RuntimeAssetZipName},
		{spxVersion: "v3.1.0", runtimeVersion: "2.3.0", packName: RuntimeAssetZipName},
	}

	for _, test := range tests {
		t.Run(test.spxVersion, func(t *testing.T) {
			meta, err := ResolveReleaseMetaForSPXVersion(test.spxVersion)
			if err != nil {
				t.Fatalf("ResolveReleaseMetaForSPXVersion returned error: %v", err)
			}
			if meta.SPXVersion != test.spxVersion || meta.Runtime.Version != test.runtimeVersion {
				t.Fatalf("resolved metadata = %#v", meta)
			}
			wantEngineRelease := AssetRelease{Repository: "goplus/godot", Tag: "spx" + test.runtimeVersion}
			if meta.Runtime.EngineAssets != wantEngineRelease {
				t.Fatalf("engine release = %#v, want %#v", meta.Runtime.EngineAssets, wantEngineRelease)
			}
			wantRuntimeRelease := AssetRelease{Repository: "goplus/spx", Tag: test.spxVersion}
			if meta.Runtime.RuntimeAssets != wantRuntimeRelease {
				t.Fatalf("runtime release = %#v, want %#v", meta.Runtime.RuntimeAssets, wantRuntimeRelease)
			}
			if got := meta.RuntimePackAssetName(); got != test.packName {
				t.Fatalf("runtime pack asset = %q, want %q", got, test.packName)
			}
			if meta.RequiresRuntimeManifest() || meta.RuntimeManifestDownloadURL() != "" {
				t.Fatal("legacy release must not select a runtime manifest")
			}
			if got, want := meta.RuntimeDownloadURL("linux-x86_64.zip"), wantEngineRelease.DownloadURL("linux-x86_64.zip"); got != want {
				t.Fatalf("engine URL = %q, want %q", got, want)
			}
			if got, want := meta.RuntimeAssetDownloadURL(test.packName), wantRuntimeRelease.DownloadURL(test.packName); got != want {
				t.Fatalf("runtime pack URL = %q, want %q", got, want)
			}

			byRuntime, err := ResolveReleaseMetaForRuntimeVersion(test.runtimeVersion)
			if err != nil {
				t.Fatalf("ResolveReleaseMetaForRuntimeVersion returned error: %v", err)
			}
			if byRuntime != meta {
				t.Fatalf("runtime resolver = %#v, SPX resolver = %#v", byRuntime, meta)
			}
		})
	}
}

func TestLatestReleaseAliasesResolveCurrentAtomicRelease(t *testing.T) {
	want := DefaultReleaseMeta()
	meta, err := ResolveReleaseMetaForSPXVersion("latest")
	if err != nil {
		t.Fatalf("resolve latest SPX version: %v", err)
	}
	if meta != want {
		t.Fatalf("latest SPX metadata = %#v, want %#v", meta, want)
	}
	meta, err = ResolveReleaseMetaForRuntimeVersion("latest")
	if err != nil {
		t.Fatalf("resolve latest runtime version: %v", err)
	}
	if meta != want {
		t.Fatalf("latest runtime metadata = %#v, want %#v", meta, want)
	}

	meta, err = ResolveReleaseMetaForSPXVersion(currentSPXVersion)
	if err != nil {
		t.Fatalf("resolve current SPX version %s: %v", currentSPXVersion, err)
	}
	if meta != want {
		t.Fatalf("current SPX metadata = %#v, want %#v", meta, want)
	}
	runtimeVersion := DefaultRuntimeLock().RuntimeVersion
	meta, err = ResolveReleaseMetaForRuntimeVersion(runtimeVersion)
	if err != nil {
		t.Fatalf("resolve runtime version %q: %v", runtimeVersion, err)
	}
	if meta.Runtime != want.Runtime {
		t.Fatalf("current runtime = %#v, want %#v", meta.Runtime, want.Runtime)
	}
}

func TestStrictReleaseResolversRejectUnknownVersions(t *testing.T) {
	for _, version := range []string{"", "v2.0.5", "v3.0.0-pre.1"} {
		if _, err := ResolveReleaseMetaForSPXVersion(version); err == nil {
			t.Errorf("ResolveReleaseMetaForSPXVersion(%q) succeeded", version)
		}
	}
	for _, version := range []string{"", "2.2.5", "2.2.6-pre.1"} {
		if _, err := ResolveReleaseMetaForRuntimeVersion(version); err == nil {
			t.Errorf("ResolveReleaseMetaForRuntimeVersion(%q) succeeded", version)
		}
	}
}
