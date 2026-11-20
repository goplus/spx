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

package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func collectProjectEntries(projectDir, destination string) ([]archiveEntry, error) {
	destination, err := filepath.Abs(destination)
	if err != nil {
		return nil, err
	}
	var destinationInfo os.FileInfo
	if info, err := os.Lstat(destination); err == nil {
		destinationInfo = info
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	skipDirs := map[string]struct{}{".git": {}, "project": {}}
	entries := make([]archiveEntry, 0)
	err = filepath.Walk(projectDir, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("project entry %s must not be a symlink", name)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("project entry %s must be a regular file or directory", name)
		}
		absolute, err := filepath.Abs(name)
		if err != nil {
			return err
		}
		if absolute == destination || (destinationInfo != nil && os.SameFile(info, destinationInfo)) {
			return nil
		}
		rel, err := filepath.Rel(projectDir, name)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if strings.HasSuffix(name, ".import") {
			return nil
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) == 1 || (len(parts) == 2 && info.IsDir()) {
			if _, ok := skipDirs[info.Name()]; ok {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		zipName := filepath.ToSlash(rel)
		if strings.Contains(zipName, "\\") {
			return fmt.Errorf("project entry %s has unsafe zip name %q", name, zipName)
		}
		entries = append(entries, archiveEntry{relPath: rel, zipName: zipName, info: info})
		return nil
	})
	return entries, err
}
