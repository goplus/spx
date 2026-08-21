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

package runtimecmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/release"
)

// writeLocalRuntimeManifestIfComplete publishes source-mode metadata only
// after both local runtime files exist.
func writeLocalRuntimeManifestIfComplete(repoRoot, goBinDir string) error {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	enginePath := filepath.Join(goBinDir, spec.RuntimeName)
	if _, err := os.Lstat(enginePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect local runtime Engine: %w", err)
	}
	packPath := filepath.Join(goBinDir, spec.PackName)
	manifest, err := release.NewLocalRuntimeManifest(lock, spec.GOOS, spec.GOARCH, enginePath, packPath)
	if err != nil {
		return err
	}
	manifestPath, err := release.LocalRuntimeManifestPath(repoRoot, lock, spec.GOOS, spec.GOARCH)
	if err != nil {
		return err
	}
	if err := release.PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Local runtime manifest: %s\n", manifestPath)
	return nil
}
