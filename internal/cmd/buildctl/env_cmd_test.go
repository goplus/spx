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

package main

import (
	"path/filepath"
	"strings"
	"testing"

	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
)

func TestParseEnvExportShellArgsDefault(t *testing.T) {
	cfg, err := parseEnvExportShellArgs(nil)
	if err != nil {
		t.Fatalf("parseEnvExportShellArgs returned error: %v", err)
	}
	if cfg.platform != "" {
		t.Fatalf("unexpected platform: %s", cfg.platform)
	}
}

func TestResolveJDKShellExportsIncludesPATHWhenJavaHomeExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin")

	binDir := filepath.Join(home, "custom-jdk", "bin")
	mustWriteFile(t, filepath.Join(binDir, "java"), []byte("bin"))
	t.Setenv("JAVA_HOME", filepath.Join(home, "custom-jdk"))

	exports, err := toolpkg.ResolveJDKShellExports()
	if err != nil {
		t.Fatalf("ResolveJDKShellExports returned error: %v", err)
	}
	if exports["JAVA_HOME"] == "" {
		t.Fatalf("missing JAVA_HOME export: %#v", exports)
	}
	if !strings.Contains(exports["PATH"], binDir) {
		t.Fatalf("PATH export does not include java bin: %s", exports["PATH"])
	}
}
