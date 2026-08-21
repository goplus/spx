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
	"sort"
	"strings"

	"github.com/goplus/spx/v3/internal/projectassets"
	"github.com/goplus/spx/v3/internal/projectbundle"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

func collectProjectAllowlist(cfg Config) ([]string, error) {
	entries, err := os.ReadDir(cfg.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("launchpack: read project directory: %w", err)
	}
	extension := cfg.ProjectExt
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
			return nil, fmt.Errorf("launchpack: inspect project source %q: %w", entry.Name(), err)
		}
		if entry.Type()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("launchpack: project source %q is not a regular non-symlink file", entry.Name())
		}
		projectFiles = append(projectFiles, entry.Name())
	}
	if len(projectFiles) == 0 {
		return nil, fmt.Errorf("launchpack: project has no top-level %s source files", extension)
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
		return nil, fmt.Errorf("launchpack: project file %q is not in the source allowlist", projectBase)
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

func prepareProjectBundleConfig(cfg Config, snapshot projectpolicy.PortableConfigSnapshot) (projectbundle.Config, error) {
	files, err := collectProjectAllowlist(cfg)
	if err != nil {
		return projectbundle.Config{}, err
	}
	if err := snapshot.Verify(cfg.ProjectDir); err != nil {
		return projectbundle.Config{}, fmt.Errorf("launchpack: %w", err)
	}
	return projectbundle.Config{ProjectDir: cfg.ProjectDir, ProjectFiles: files,
		IncludeConfig: snapshot.Present(), ConfigBytes: snapshot.Bytes(), PackDir: cfg.PackDir,
		Output: cfg.Output}, nil
}

// collectReferencedProjectFiles resolves explicit resources outside PackDir.
func collectReferencedProjectFiles(cfg Config) ([]string, error) {
	referenced, err := projectassets.Collect(projectassets.Config{
		ProjectDir: cfg.ProjectDir,
		PackDir:    cfg.PackDir,
		PackIndex:  cfg.PackIndex,
	})
	if err != nil {
		return nil, fmt.Errorf("launchpack: collect typed project resources: %w", err)
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
