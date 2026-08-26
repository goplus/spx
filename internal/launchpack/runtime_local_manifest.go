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
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/envutil"
	"github.com/goplus/spx/v3/internal/release"
)

func findExplicitLocalRuntimeManifest(env []string, lock release.RuntimeLock, spec release.HostRuntimeSpec) (localRuntimeSource, bool, error) {
	path, found, duplicate := envutil.Lookup(env, runtimeLocalManifestEnv)
	if duplicate {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: duplicate %s", runtimeLocalManifestEnv)
	}
	if !found {
		return localRuntimeSource{}, false, nil
	}
	return readLocalRuntimeManifest(path, lock, spec, true)
}

func findSourceLocalRuntimeManifest(root string, lock release.RuntimeLock, spec release.HostRuntimeSpec) (localRuntimeSource, bool, error) {
	path, err := release.LocalRuntimeManifestPath(root, lock, spec.GOOS, spec.GOARCH)
	if err != nil {
		return localRuntimeSource{}, false, err
	}
	if info, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return localRuntimeSource{}, false, nil
		}
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: inspect local runtime manifest: %w", err)
	} else if !isRegularNonSymlink(info) {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: discovered local runtime manifest is not a regular non-symlink file: %s", path)
	}
	return readLocalRuntimeManifest(path, lock, spec, false)
}

func readLocalRuntimeManifest(path string, lock release.RuntimeLock, spec release.HostRuntimeSpec, strict bool) (localRuntimeSource, bool, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: %s must be an absolute clean path", runtimeLocalManifestEnv)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: inspect local runtime manifest %q: %w", path, err)
	}
	if !isRegularNonSymlink(info) {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: local runtime manifest %q is not a regular non-symlink file", path)
	}
	data, err := readRegularFile(path)
	if err != nil {
		return localRuntimeSource{}, false, err
	}
	manifest, err := release.ParseLocalRuntimeManifest(data)
	if err != nil {
		return localRuntimeSource{}, false, err
	}
	validate := manifest.ValidateForVersion
	if strict {
		validate = manifest.ValidateForLock
	}
	if err := validate(lock, spec.GOOS, spec.GOARCH); err != nil {
		return localRuntimeSource{}, false, err
	}
	directory := filepath.Dir(path)
	if err := manifest.VerifyFiles(directory); err != nil {
		return localRuntimeSource{}, false, err
	}
	return localRuntimeSource{
		manifest: manifest, bytes: data,
		enginePath: filepath.Join(directory, manifest.Engine.Name),
		packPath:   filepath.Join(directory, manifest.Pack.Name),
	}, true, nil
}
