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

import "testing"

func TestReleaseMetaForSPXVersionMapped(t *testing.T) {
	meta := ReleaseMetaForSPXVersion("v3.0.0")
	if meta.Runtime.Version != "2.2.6" {
		t.Fatalf("runtime version = %q, want %q", meta.Runtime.Version, "2.2.6")
	}
	if got := meta.RuntimeAssetDownloadURL(RuntimeAssetZipName); got != SpxReleaseURLBase+"v3.0.0/"+RuntimeAssetZipName {
		t.Fatalf("runtime asset download url = %q, want %q", got, SpxReleaseURLBase+"v3.0.0/"+RuntimeAssetZipName)
	}
}

func TestReleaseMetaForRuntimeVersionMapped(t *testing.T) {
	meta := ReleaseMetaForRuntimeVersion("2.2.6")
	if meta.SPXVersion != "v3.0.0" {
		t.Fatalf("spx version = %q, want %q", meta.SPXVersion, "v3.0.0")
	}
	if got := meta.RuntimeAssetDownloadURL(RuntimeAssetZipName); got != SpxReleaseURLBase+"v3.0.0/"+RuntimeAssetZipName {
		t.Fatalf("runtime asset download url = %q, want %q", got, SpxReleaseURLBase+"v3.0.0/"+RuntimeAssetZipName)
	}
}

func TestDefaultReleaseMetaUsesLatestSPXVersionForRuntimeAssets(t *testing.T) {
	meta := DefaultReleaseMeta()
	if meta.SPXVersion != "v3.0.0" {
		t.Fatalf("spx version = %q, want %q", meta.SPXVersion, "v3.0.0")
	}
	if meta.Runtime.Version != "2.2.6" {
		t.Fatalf("runtime version = %q, want %q", meta.Runtime.Version, "2.2.6")
	}
	if got := meta.RuntimeAssetDownloadURL(RuntimeAssetZipName); got != SpxReleaseURLBase+"v3.0.0/"+RuntimeAssetZipName {
		t.Fatalf("runtime asset download url = %q, want %q", got, SpxReleaseURLBase+"v3.0.0/"+RuntimeAssetZipName)
	}
}

func TestReleaseMetaForRuntimeVersionUnknownFallback(t *testing.T) {
	meta := ReleaseMetaForRuntimeVersion("2.2.0-unknown")
	defaultMeta := DefaultReleaseMeta()
	if meta.SPXVersion != defaultMeta.SPXVersion {
		t.Fatalf("spx version = %q, want default %q", meta.SPXVersion, defaultMeta.SPXVersion)
	}
	if meta.Runtime.Version != defaultMeta.Runtime.Version {
		t.Fatalf("runtime version = %q, want default %q", meta.Runtime.Version, defaultMeta.Runtime.Version)
	}
}

func TestReleaseMetaForSPXVersionLatestFallback(t *testing.T) {
	meta := ReleaseMetaForSPXVersion("latest")
	defaultMeta := DefaultReleaseMeta()
	if meta.SPXVersion != defaultMeta.SPXVersion {
		t.Fatalf("spx version = %q, want default %q", meta.SPXVersion, defaultMeta.SPXVersion)
	}
	if meta.Runtime.Version != defaultMeta.Runtime.Version {
		t.Fatalf("runtime version = %q, want default %q", meta.Runtime.Version, defaultMeta.Runtime.Version)
	}
}

func TestReleaseMetaForSPXVersionUnknownFallback(t *testing.T) {
	meta := ReleaseMetaForSPXVersion("v2.0.0-pre.99")
	defaultMeta := DefaultReleaseMeta()
	if meta.SPXVersion != defaultMeta.SPXVersion {
		t.Fatalf("spx version = %q, want default %q", meta.SPXVersion, defaultMeta.SPXVersion)
	}
	if meta.Runtime.Version != defaultMeta.Runtime.Version {
		t.Fatalf("runtime version = %q, want default %q", meta.Runtime.Version, defaultMeta.Runtime.Version)
	}
}
