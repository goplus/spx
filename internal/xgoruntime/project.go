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

package xgoruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goplus/spx/v3/internal/projectassets"
)

func collectProjectAllowlist(cfg Config) ([]string, error) {
	entries, err := os.ReadDir(cfg.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("xgoruntime: read project directory: %w", err)
	}
	extension := cfg.Project.Extension
	if extension != "" && !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	var projectFiles []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != extension {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("xgoruntime: inspect project source %q: %w", entry.Name(), err)
		}
		if entry.Type()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("xgoruntime: project source %q is not a regular non-symlink file", entry.Name())
		}
		projectFiles = append(projectFiles, entry.Name())
	}
	if len(projectFiles) == 0 {
		return nil, fmt.Errorf("xgoruntime: project has no top-level %s source files", extension)
	}
	projectBase := filepath.Base(cfg.ProjectFile)
	foundProject := false
	for _, name := range projectFiles {
		if name == projectBase {
			foundProject = true
			break
		}
	}
	if !foundProject {
		return nil, fmt.Errorf("xgoruntime: project file %q is not in the source allowlist", projectBase)
	}

	external, err := collectReferencedProjectFiles(cfg)
	if err != nil {
		return nil, err
	}
	projectFiles = append(projectFiles, external...)
	sort.Strings(projectFiles)
	projectFiles = compactStrings(projectFiles)
	return projectFiles, nil
}

// collectReferencedProjectFiles follows only explicit path-like values in
// project JSON. The entire PackDir is already included, so this collector is
// concerned solely with ../ and res:// references that resolve elsewhere in
// ProjectDir. Absolute and escaping references fail closed.
func collectReferencedProjectFiles(cfg Config) ([]string, error) {
	referenced, err := projectassets.Collect(projectassets.Config{
		ProjectDir: cfg.ProjectDir,
		PackDir:    cfg.Project.PackDirectory,
		PackIndex:  cfg.Project.PackIndexFile,
	})
	if err != nil {
		return nil, fmt.Errorf("xgoruntime: collect typed project resources: %w", err)
	}
	return referenced, nil
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	output := values[:1]
	for _, value := range values[1:] {
		if value != output[len(output)-1] {
			output = append(output, value)
		}
	}
	return output
}
