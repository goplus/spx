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

import "fmt"

// ValidateEngineReuse binds a published source to its immutable lock snapshot
// before comparing the independent engine identity. Old manifests intentionally
// cannot seed partial reuse: they did not record this build contract.
func ValidateEngineReuse(manifest RuntimeManifest, target RuntimeLock, repoRoot, revision string) error {
	if err := manifest.validateEngineSource(target.ReleaseRepository); err != nil {
		return err
	}
	digest, err := RuntimeEngineSourceSHA256(repoRoot, revision, target)
	if err != nil {
		return err
	}
	if manifest.Provenance.EngineSourceSHA256 != digest {
		return fmt.Errorf("release: engine source digest differs from current inputs")
	}
	return manifest.validateRequiredAssets(target.RequiredAssets)
}

// InheritEngine verifies that copied bytes retain their source identity and
// keeps the original build origin across successive pack-only releases.
func (m *RuntimeManifest) InheritEngine(source RuntimeManifest) error {
	if err := source.validateEngineSource(m.ReleaseRepository); err != nil {
		return err
	}
	if source.Provenance.EngineSourceSHA256 != m.Provenance.EngineSourceSHA256 {
		return fmt.Errorf("release: engine source digest differs from current inputs")
	}
	if len(m.Assets) != len(source.Assets) {
		return fmt.Errorf("release: source and target asset counts differ")
	}
	// Both manifests are sorted. Only the pack may have different bytes.
	for i, asset := range m.Assets {
		original := source.Assets[i]
		if original.Name != asset.Name || (asset.Name != RuntimeAssetZipName && asset != original) {
			return fmt.Errorf("release: reused engine asset %q differs from source", asset.Name)
		}
	}
	origin := RuntimeEngineOrigin{RuntimeVersion: source.RuntimeVersion, SPXCommit: source.Provenance.SPXCommit}
	if source.Provenance.EngineOrigin != nil {
		origin = *source.Provenance.EngineOrigin
	}
	candidate := *m
	candidate.Provenance.EngineOrigin = &origin
	if err := candidate.Validate(); err != nil {
		return err
	}
	*m = candidate
	return nil
}

// Both planning and final assembly bind the source to its published lock.
func (manifest RuntimeManifest) validateEngineSource(repository string) error {
	source, err := RuntimeLockForVersion(manifest.RuntimeVersion)
	if err != nil {
		return err
	}
	if err := manifest.ValidateForLock(source); err != nil {
		return err
	}
	if source.ReleaseRepository != repository {
		return fmt.Errorf("release: engine source repository differs from target")
	}
	if manifest.Provenance.EngineSourceSHA256 == "" {
		return fmt.Errorf("release: source runtime has no engine identity; build a new runtime first")
	}
	return nil
}
