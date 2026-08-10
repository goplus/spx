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

package shared

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/release"
)

func resolveSPXModuleSource(repoRoot string) (string, error) {
	return resolveSPXModuleSourcePath(repoRoot, os.Getenv("SPX_MODULE_SRC"))
}

func resolveSPXModuleSourcePath(repoRoot, override string) (string, error) {
	absRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}

	moduleSource := strings.TrimSpace(override)
	if moduleSource == "" {
		moduleSource = filepath.Join(absRepoRoot, filepath.FromSlash(release.DefaultRuntimeLock().Module.Path))
	} else if !filepath.IsAbs(moduleSource) {
		moduleSource = filepath.Join(absRepoRoot, moduleSource)
	}

	absModuleSource, err := filepath.Abs(moduleSource)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absModuleSource), nil
}
