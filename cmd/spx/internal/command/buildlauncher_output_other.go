//go:build !windows

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

package command

import (
	"os"
	"path/filepath"
	"runtime"
)

func outputPathsEqual(left, right string) bool {
	return left == right
}

func launcherOutputPathIsReparse(path string) (bool, error) {
	return false, nil
}

func launcherOutputAliasAllowed(existing, resolved string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	for _, alias := range []string{"/var", "/tmp"} {
		canonical := filepath.Join("/private", filepath.Base(alias))
		existingRelative, existingErr := filepath.Rel(alias, existing)
		resolvedRelative, resolvedErr := filepath.Rel(canonical, resolved)
		if existingErr == nil && resolvedErr == nil &&
			launcherPathWithin(alias, existing) && launcherPathWithin(canonical, resolved) &&
			existingRelative == resolvedRelative {
			return true
		}
	}
	return false
}

func launcherStageIsExecutable(info os.FileInfo) bool {
	return info.Mode().Perm()&0o111 != 0 && info.Size() > 0
}

// The stage and destination share a filesystem, so rename is atomic.
func commitLauncherOutputPlatform(stage, final string) error {
	return os.Rename(stage, final)
}
