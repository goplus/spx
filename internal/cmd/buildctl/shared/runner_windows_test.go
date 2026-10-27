//go:build windows

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

package shared

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWindowsExecutableExtensions(t *testing.T) {
	defaults := []string{".com", ".exe", ".bat", ".cmd"}
	for _, tt := range []struct {
		name string
		env  map[string]string
		want []string
	}{
		{"missing", nil, defaults},
		{"empty", map[string]string{"PATHEXT": ""}, defaults},
		{"empty entries", map[string]string{"PATHEXT": ";;"}, defaults},
		{"case insensitive with empty entries", map[string]string{"pAtHeXt": ".CMD;;.EXE;.BaT;"}, []string{".cmd", ".exe", ".bat"}},
		{"canonical key takes precedence", map[string]string{"PATHEXT": ".EXE", "Pathext": ".CMD"}, []string{".exe"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := maps.Clone(tt.env)
			if got := executableExtensions(tt.env); !slices.Equal(got, tt.want) {
				t.Fatalf("executableExtensions() = %q, want %q", got, tt.want)
			}
			if !maps.Equal(tt.env, before) {
				t.Fatal("extension lookup changed caller environment")
			}
		})
	}
}

func TestWindowsResolveCommandPathPriority(t *testing.T) {
	for _, tt := range []struct {
		name, extensions, command, want string
		files                           []string
	}{
		{"PATHEXT order", ".CMD;.EXE", "tool", "first/tool.cmd", []string{"first/tool.exe", "first/tool.cmd"}},
		{"PATH directory before extension", ".CMD;.EXE", "tool", "first/tool.exe", []string{"first/tool.exe", "second/tool.cmd"}},
		{"default extension order", "", "tool", "first/tool.com", []string{"first/tool.exe", "first/tool.com", "first/tool.cmd"}},
		{"empty extension fallback", ";;", "tool", "first/tool.exe", []string{"first/tool.exe"}},
		{"case insensitive extension", ".EXE", "tool", "first/tool.exe", []string{"first/tool.EXE"}},
		{"explicit extension", ".CMD;.EXE", "tool.exe", "first/tool.exe", []string{"first/tool.exe", "first/tool.exe.cmd"}},
		{"extensionless command first", ".EXE", "tool", "first/tool", []string{"first/tool", "first/tool.exe"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
			for _, directory := range []string{first, second} {
				if err := os.Mkdir(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range tt.files {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte("fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			// Empty PATH entries must not resolve a command in the working directory.
			t.Chdir(root)
			if err := os.WriteFile("tool.cmd", []byte("not on PATH"), 0o600); err != nil {
				t.Fatal(err)
			}
			env := map[string]string{"pAtH": ";" + first + ";;" + second, "PathExt": tt.extensions}
			got, err := ResolveCommandPath(tt.command, env)
			want := filepath.Join(root, filepath.FromSlash(tt.want))
			if err != nil || got != want {
				t.Fatalf("ResolveCommandPath(%q) = %q, %v; want %q", tt.command, got, err, want)
			}
		})
	}
	t.Run("directories are not commands", func(t *testing.T) {
		root := t.TempDir()
		for _, name := range []string{"tool", "tool.exe"} {
			if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := ResolveCommandPath("tool", map[string]string{"Path": root, "PATHEXT": ".EXE"}); err == nil {
			t.Fatal("resolved a directory as a command")
		}
	})
}

func TestWindowsSetPathEnvRemovesCaseVariants(t *testing.T) {
	env := map[string]string{
		"PATH": "canonical", "Path": "mixed", "path": "lower", "pAtH": "other",
		"PATHEXT": ".EXE", "OTHER": "kept=value",
	}
	if got := pathEnvValue(env); got != "canonical" {
		t.Fatalf("canonical PATH precedence = %q", got)
	}
	setPathEnv(env, "new;path with spaces")
	want := map[string]string{"PATH": "new;path with spaces", "PATHEXT": ".EXE", "OTHER": "kept=value"}
	if !maps.Equal(env, want) {
		t.Fatalf("normalized environment = %v, want %v", env, want)
	}
	if got := pathEnvValue(map[string]string{"Path": "fallback"}); got != "fallback" {
		t.Fatalf("case-insensitive PATH fallback = %q", got)
	}
	if got := pathEnvValue(map[string]string{"PATH": "", "Path": "fallback"}); got != "" {
		t.Fatalf("empty canonical PATH was replaced by %q", got)
	}
	setPathEnv(env, "")
	if env["PATH"] != "" || len(env) != 3 {
		t.Fatalf("cleared PATH environment = %v", env)
	}
}
