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

package launchpack

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

func expectedDriverBundle(bundle driverbundle.Bundle) (runtimebundle.Bundle, error) {
	if err := validateDriverBundleSize(bundle); err != nil {
		return runtimebundle.Bundle{}, err
	}
	entries := make([]runtimebundle.Entry, len(bundle.Files))
	for i, file := range bundle.Files {
		entries[i] = runtimebundle.Entry{Name: file.Name, Mode: file.Mode, Size: file.Size, SHA256: file.SHA256}
	}
	return (runtimebundle.Bundle{
		Schema:        runtimebundle.SchemaV1,
		Namespace:     runtimebundle.NamespaceDriver,
		ArchiveSHA256: bundle.SHA256,
		Entries:       entries,
	}).WithDigest()
}

func validateDriverBundleSize(bundle driverbundle.Bundle) error {
	if bundle.Size > runtimebundle.MaxArchiveBytes {
		return fmt.Errorf("launchpack: driver bundle %q size %d exceeds archive limit %d", bundle.Name, bundle.Size, runtimebundle.MaxArchiveBytes)
	}
	return nil
}

func publishedDriverAssets(materialized *runtimebundle.Materialized, bundle driverbundle.Bundle, manifestDigest, spxVersion string, lock release.RuntimeLock) (Assets, error) {
	if materialized == nil {
		return Assets{}, errors.New("launchpack: nil materialized published driver bundle")
	}
	if len(bundle.Files) != 3 {
		return Assets{}, errors.New("launchpack: published driver bundle does not contain exactly three files")
	}
	components, err := driverbundle.HostSpecFor(lock.RuntimeVersion, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Assets{}, err
	}
	paths := make(map[string]string, len(bundle.Files))
	for _, file := range bundle.Files {
		path := filepath.Join(materialized.Path, filepath.FromSlash(file.Name))
		if err := validateRuntimeFile(path, "published driver component"); err != nil {
			return Assets{}, err
		}
		paths[file.Name] = path
	}
	enginePath, ok := paths[components.Engine.Name]
	if !ok {
		return Assets{}, fmt.Errorf("launchpack: published driver bundle is missing %s", components.Engine.Name)
	}
	packPath, ok := paths[components.Pack.Name]
	if !ok {
		return Assets{}, fmt.Errorf("launchpack: published driver bundle is missing %s", components.Pack.Name)
	}
	bridgePath, ok := paths[components.Bridge.Name]
	if !ok {
		return Assets{}, fmt.Errorf("launchpack: published driver bundle is missing %s", components.Bridge.Name)
	}
	fileDigest := func(name string) string {
		for _, file := range bundle.Files {
			if file.Name == name {
				return file.SHA256
			}
		}
		return ""
	}
	engineDigest := fileDigest(components.Engine.Name)
	packDigest := fileDigest(components.Pack.Name)
	bridgeDigest := fileDigest(components.Bridge.Name)
	return Assets{
		EnginePath: enginePath, PackPath: packPath, BridgePath: bridgePath, Lock: lock,
		Published: &PublishedDriverIdentity{
			ManifestSHA256: manifestDigest, BundleSHA256: bundle.SHA256, BundleName: bundle.Name,
			SPXVersion:   spxVersion,
			EngineSHA256: engineDigest, PackSHA256: packDigest, BridgeSHA256: bridgeDigest,
			EngineInterfaceDigest: bundle.EngineInterfaceDigest,
		},
		Cleanup: func() { _ = materialized.Close() },
	}, nil
}
