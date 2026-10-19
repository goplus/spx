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
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
)

func TestRunEnvDispatchAndArgumentErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		wantErr  error
		contains string
	}{
		{name: "missing command", wantErr: shared.ErrUsage},
		{name: "help", args: []string{"help"}},
		{name: "export shell help", args: []string{"export-shell", "--help"}},
		{name: "unknown", args: []string{"unknown"}, contains: `unknown env command "unknown"`},
		{name: "invalid platform", args: []string{"export-shell", "--platform", "plan9"}, contains: "unsupported platform"},
		{name: "extra argument", args: []string{"export-jdk-shell", "extra"}, wantErr: shared.ErrUsage},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := runEnv(test.args)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("runEnv(%q) error = %v, want %v", test.args, err, test.wantErr)
				}
				return
			}
			if test.contains != "" {
				if err == nil || !strings.Contains(err.Error(), test.contains) {
					t.Fatalf("runEnv(%q) error = %v, want substring %q", test.args, err, test.contains)
				}
				return
			}
			if err != nil {
				t.Fatalf("runEnv(%q) returned error: %v", test.args, err)
			}
		})
	}
}

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
