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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/mod/modfile"
)

type launcherProject struct {
	dir          string
	file         string
	metadataFile string
	sourceFiles  []string
	extension    string
	packDir      string
	packIndex    string
}

func loadLauncherProject(moduleRoot, projectDir string) (launcherProject, error) {
	metadataPath, data, err := readLauncherMetadata(moduleRoot)
	if err != nil {
		return launcherProject{}, err
	}
	metadata, err := modfile.Parse(metadataPath, data, nil)
	if err != nil {
		return launcherProject{}, fmt.Errorf("buildlauncher: parse project metadata %q: %w", metadataPath, err)
	}
	var project *modfile.Project
	for _, candidate := range metadata.Projects {
		if candidate.Ext != ".spx" {
			continue
		}
		if project != nil {
			return launcherProject{}, fmt.Errorf("buildlauncher: project metadata declares multiple .spx projects")
		}
		project = candidate
	}
	if project == nil {
		return launcherProject{}, fmt.Errorf("buildlauncher: project metadata %q has no .spx project", metadataPath)
	}
	if project.Pack == nil || project.Pack.Directory == "" || project.Pack.IndexFile == "" {
		return launcherProject{}, fmt.Errorf("buildlauncher: project metadata %q must declare 'pack assets index.json'", metadataPath)
	}
	sourceFiles, err := listLauncherProjectFiles(projectDir, project.Ext)
	if err != nil {
		return launcherProject{}, err
	}
	projectFile, err := findLauncherProjectFile(projectDir, project, sourceFiles)
	if err != nil {
		return launcherProject{}, err
	}
	return launcherProject{
		dir: projectDir, file: projectFile, metadataFile: metadataPath,
		sourceFiles: sourceFiles, extension: project.Ext,
		packDir: project.Pack.Directory, packIndex: project.Pack.IndexFile,
	}, nil
}

func readLauncherMetadata(moduleRoot string) (string, []byte, error) {
	var found string
	for _, name := range []string{"gox.mod", "gop.mod"} {
		path := filepath.Join(moduleRoot, name)
		if info, err := os.Lstat(path); err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return "", nil, fmt.Errorf("buildlauncher: project metadata %q is not a regular non-symlink file", path)
			}
			if found != "" {
				return "", nil, fmt.Errorf("buildlauncher: module has both %q and %q", filepath.Base(found), name)
			}
			found = path
		} else if !os.IsNotExist(err) {
			return "", nil, fmt.Errorf("buildlauncher: inspect project metadata %q: %w", path, err)
		}
	}
	if found == "" {
		return "", nil, fmt.Errorf("buildlauncher: no gox.mod or gop.mod in active module %q", moduleRoot)
	}
	data, err := os.ReadFile(found)
	if err != nil {
		return "", nil, fmt.Errorf("buildlauncher: read project metadata %q: %w", found, err)
	}
	return found, data, nil
}

func listLauncherProjectFiles(projectDir, extension string) ([]string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("buildlauncher: read project directory: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != extension {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil, fmt.Errorf("buildlauncher: project source %q is not a regular non-symlink file", entry.Name())
		}
		files = append(files, filepath.Join(projectDir, entry.Name()))
	}
	return files, nil
}

func findLauncherProjectFile(projectDir string, project *modfile.Project, sourceFiles []string) (string, error) {
	if project.FullExt != "" && !strings.ContainsAny(project.FullExt, "*") {
		path := filepath.Join(projectDir, filepath.FromSlash(project.FullExt))
		if isRegularFile(path) {
			return path, nil
		}
	}
	mainName := "main" + project.Ext
	if isRegularFile(filepath.Join(projectDir, mainName)) && project.IsProj(project.Ext, mainName) {
		return filepath.Join(projectDir, mainName), nil
	}
	var matches []string
	for _, file := range sourceFiles {
		name := filepath.Base(file)
		if !project.IsProj(project.Ext, name) {
			continue
		}
		matches = append(matches, file)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("buildlauncher: no project file matching %q in %q", project.Ext, projectDir)
	}
	return "", fmt.Errorf("buildlauncher: multiple project files match metadata in %q; name main%s explicitly", projectDir, project.Ext)
}

func isRegularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}
