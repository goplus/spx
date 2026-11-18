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
	"fmt"
	"path/filepath"
)

const RuntimePackAssetName = "spx-runtime-assets.zip"

// ValidateEngineReuse binds a published source to its immutable lock snapshot
// before comparing the independent engine identity. Old manifests intentionally
// cannot seed partial reuse: they did not record this build contract.
func ValidateEngineReuse(manifest RuntimeManifest, target RuntimeLock, repoRoot, revision string) error {
	source, err := RuntimeLockForVersion(manifest.RuntimeVersion)
	if err != nil {
		return err
	}
	if err := manifest.ValidateForLock(source); err != nil {
		return err
	}
	if source.ReleaseRepository != target.ReleaseRepository {
		return fmt.Errorf("release: engine source repository differs from target")
	}
	if manifest.Provenance.EngineSourceSHA256 == "" {
		return fmt.Errorf("release: source runtime has no engine identity; build a new runtime first")
	}
	digest, err := RuntimeEngineSourceSHA256(repoRoot, revision, target)
	if err != nil {
		return err
	}
	if manifest.Provenance.EngineSourceSHA256 != digest {
		return fmt.Errorf("release: engine source digest differs from current inputs")
	}
	if err := manifest.ValidateRequiredAssets(target.RequiredAssets); err != nil {
		return err
	}
	return nil
}

// VerifyEngineFiles checks each reusable engine ZIP, not the obsolete pack.
func (m RuntimeManifest) VerifyEngineFiles(directory string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	for _, asset := range m.Assets {
		if asset.Name == RuntimePackAssetName {
			continue
		}
		if err := verifyRuntimeAssetFile(asset, filepath.Join(directory, asset.Name)); err != nil {
			return err
		}
	}
	return nil
}

// InheritEngine verifies that copied bytes retain their source identity and
// keeps the original build origin across successive pack-only releases.
func (m *RuntimeManifest) InheritEngine(source RuntimeManifest) error {
	if err := source.Validate(); err != nil {
		return err
	}
	if source.Provenance.EngineSourceSHA256 == "" || source.Provenance.EngineSourceSHA256 != m.Provenance.EngineSourceSHA256 {
		return fmt.Errorf("release: copied engine identity differs from source")
	}
	for _, asset := range m.Assets {
		if asset.Name == RuntimePackAssetName {
			continue
		}
		original, ok := source.Asset(asset.Name)
		if !ok || asset != original {
			return fmt.Errorf("release: reused engine asset %q differs from source", asset.Name)
		}
	}
	origin := source.Provenance.EngineOrigin
	if origin == nil {
		origin = &RuntimeEngineOrigin{RuntimeVersion: source.RuntimeVersion, SPXCommit: source.Provenance.SPXCommit}
	}
	m.Provenance.EngineOrigin = &RuntimeEngineOrigin{RuntimeVersion: origin.RuntimeVersion, SPXCommit: origin.SPXCommit}
	return m.Validate()
}
