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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SPXModule is the validated external module input shared by buildctl's Godot
// compilation paths. Keeping the source and profile together prevents callers
// from resolving one module directory and loading build flags from another.
type SPXModule struct {
	source  string
	profile sconsProfile
}

// Source returns the validated host path to the module.
func (module SPXModule) Source() string {
	return module.source
}

// EditorBuildArgs composes module-owned flags, editor axes, and the resolved
// custom_modules path without mutating the parsed profile.
func (module SPXModule) EditorBuildArgs(buildArgs ...string) []string {
	return composeSConsBuildArgs(module.source, buildArgs, module.profile.Common, module.profile.EditorRelease)
}

// TemplateBuildArgs composes module-owned flags, template axes, and the
// resolved custom_modules path without mutating the parsed profile.
func (module SPXModule) TemplateBuildArgs(buildArgs ...string) []string {
	return module.TemplateBuildArgsAt(module.source, buildArgs...)
}

// TemplateBuildArgsAt composes template arguments for an alternate mounted
// module path, such as the path visible inside a build container.
func (module SPXModule) TemplateBuildArgsAt(moduleSource string, buildArgs ...string) []string {
	return composeSConsBuildArgs(moduleSource, buildArgs, module.profile.Common, module.profile.TemplateRelease)
}

// ResolveSPXModule resolves SPX_MODULE_SRC once and validates the external
// module build contract at that location.
func ResolveSPXModule(repoRoot string) (SPXModule, error) {
	source, err := resolveSPXModuleSource(repoRoot)
	if err != nil {
		return SPXModule{}, err
	}
	return LoadSPXModule(source)
}

// LoadSPXModule validates the files required by Godot before parsing the
// module-owned SCons profile.
func LoadSPXModule(source string) (SPXModule, error) {
	if strings.TrimSpace(source) == "" {
		return SPXModule{}, fmt.Errorf("SPX module source must not be empty")
	}
	source = filepath.Clean(source)
	info, err := os.Stat(source)
	if err != nil {
		return SPXModule{}, fmt.Errorf("SPX module source %q: %w", source, err)
	}
	if !info.IsDir() {
		return SPXModule{}, fmt.Errorf("SPX module source %q is not a directory", source)
	}

	for _, name := range []string{"SCsub", "config.py"} {
		path := filepath.Join(source, name)
		info, err := os.Stat(path)
		if err != nil {
			return SPXModule{}, fmt.Errorf("SPX module build file %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return SPXModule{}, fmt.Errorf("SPX module build file %q is not a regular file", path)
		}
	}

	profile, err := loadSConsProfile(source)
	if err != nil {
		return SPXModule{}, err
	}
	return SPXModule{source: source, profile: profile}, nil
}
