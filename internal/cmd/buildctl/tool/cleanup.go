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

package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

func cleanInstalledAssets() error {
	goPath, err := shared.EnsureGoPath()
	if err != nil {
		return err
	}

	binDir := filepath.Join(goPath, "bin")
	if !shared.FileExists(binDir) {
		fmt.Fprintf(os.Stdout, "No GOPATH bin directory found at %s\n", binDir)
		return nil
	}

	patterns := []string{
		"spx",
		"spx.exe",
		"ispx",
		"ispx.wasm",
		"ispx.wasm.br",
		"runtime.gdextension",
		"gdspx*",
	}

	seen := map[string]struct{}{}
	var removed []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(binDir, pattern))
		if err != nil {
			return err
		}
		for _, match := range matches {
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			if err := os.RemoveAll(match); err != nil {
				return fmt.Errorf("remove installed asset %s: %w", match, err)
			}
			removed = append(removed, match)
		}
	}

	if len(removed) == 0 {
		fmt.Fprintf(os.Stdout, "No installed SPX assets found under %s\n", binDir)
		return nil
	}

	sort.Strings(removed)
	for _, path := range removed {
		fmt.Fprintf(os.Stdout, "Removed %s\n", path)
	}
	return nil
}
