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
	"path/filepath"
	"strings"
)

func launcherModfilePath(flag string) (string, bool) {
	if path, ok := strings.CutPrefix(flag, "-modfile="); ok {
		return path, true
	}
	return strings.CutPrefix(flag, "--modfile=")
}

func launcherGraphProtectedFiles(files, flags []string) []string {
	protected := append([]string(nil), files...)
	for _, path := range files {
		switch filepath.Base(path) {
		case "go.mod":
			protected = append(protected, filepath.Join(filepath.Dir(path), "go.sum"))
		case "go.work":
			protected = append(protected, filepath.Join(filepath.Dir(path), "go.work.sum"))
		}
	}
	for _, flag := range flags {
		if path, ok := launcherModfilePath(flag); ok && strings.HasSuffix(path, ".mod") {
			protected = append(protected, strings.TrimSuffix(path, ".mod")+".sum")
		}
	}
	return protected
}
