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

package builderai

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

func TestEnsureGoModUsesTemplateOnlyForMissingFile(t *testing.T) {
	complete := "module example.com/template\n\nrequire " + ModulePath + " " + Version + "\n"
	if ReplacementPath != "" {
		complete += "\nreplace " + ModulePath + " => " + ReplacementPath + " " + ReplacementVersion + "\n"
	}
	for _, tt := range []struct {
		name, existing, template, wantModule string
		unchangedTemplate                    bool
	}{
		{name: "custom template", template: "module example.com/custom\n", wantModule: "example.com/custom"},
		{name: "complete template", template: complete, wantModule: "example.com/template", unchangedTemplate: true},
		{name: "complete CRLF template", template: strings.ReplaceAll(complete, "\n", "\r\n"), wantModule: "example.com/template", unchangedTemplate: true},
		{name: "existing file ignores invalid template", existing: "module example.com/existing\n", template: "invalid template", wantModule: "example.com/existing"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "go.mod")
			if tt.existing != "" {
				writeProjectFile(t, path, tt.existing)
			}
			opts := ProjectOptions{GoModTemplate: tt.template}
			if err := EnsureGoMod(root, opts); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := modfile.Parse(path, data, nil)
			if err != nil || file.Module == nil || file.Module.Mod.Path != tt.wantModule {
				t.Fatalf("go.mod=%s error=%v, want module %s", data, err, tt.wantModule)
			}
			if tt.unchangedTemplate && string(data) != tt.template {
				t.Fatalf("complete template changed: %q, want %q", data, tt.template)
			}
			stamp := time.Unix(1_600_000_000, 0)
			if err := os.Chtimes(path, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := EnsureGoMod(root, opts); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(path)
			if err != nil || !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
				t.Fatalf("unchanged go.mod was rewritten: %v", err)
			}
		})
	}
}

func TestGoxModImportPreservesLayout(t *testing.T) {
	line := `import "` + ModulePath + `"`
	for _, tt := range []struct{ name, input, want string }{
		{"empty", "", line + "\n"},
		{"no newline", "module demo", "module demo\n\n" + line + "\n"},
		{"one newline", "module demo\n", "module demo\n\n" + line + "\n"},
		{"blank line", "module demo\n\n", "module demo\n\n" + line + "\n"},
		{"project directive", "project spx\nclass Sprite\n", "project spx\n\n" + line + "\nclass Sprite\n"},
		{"windows append", "module demo\r\n", "module demo\r\n\r\n" + line + "\r\n"},
		{"windows blank line", "module demo\r\n\r\n", "module demo\r\n\r\n" + line + "\r\n"},
		{"windows line endings", "project spx\r\nclass Sprite\r\n", "project spx\r\n\r\n" + line + "\r\nclass Sprite\r\n"},
		{"existing import", "project spx\n  " + line + "\n", "project spx\n  " + line + "\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := addGoxModImport(tt.input, ModulePath)
			if got != tt.want {
				t.Fatalf("imported config=%q, want %q", got, tt.want)
			}
			if again := addGoxModImport(got, ModulePath); again != got {
				t.Fatalf("second import changed config: %q", again)
			}
		})
	}
}

func TestEnsureProjectFilesRequiresDescription(t *testing.T) {
	if HasDescription("") {
		t.Fatal("empty path has a description")
	}
	for _, tt := range []struct {
		name, description string
		directory         bool
		want              bool
	}{
		{name: "missing"},
		{name: "directory ignored", description: DescriptionFile, directory: true},
		{name: "markdown", description: DescriptionFile, want: true},
		{name: "raw", description: DescriptionFileRaw, want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.description != "" {
				path := filepath.Join(root, tt.description)
				if tt.directory {
					if err := os.Mkdir(path, 0o755); err != nil {
						t.Fatal(err)
					}
				} else {
					writeProjectFile(t, path, "description")
				}
			}
			if got := HasDescription(root); got != tt.want {
				t.Fatalf("HasDescription=%v, want %v", got, tt.want)
			}
			if err := EnsureProjectFiles(root, ProjectOptions{}); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"go.mod", "gox.mod"} {
				data, err := os.ReadFile(filepath.Join(root, name))
				if !tt.want {
					if !os.IsNotExist(err) {
						t.Fatalf("unrelated project created %s: %v", name, err)
					}
				} else if err != nil || !strings.Contains(string(data), ModulePath) {
					t.Fatalf("%s=%q error=%v, want AI module", name, data, err)
				}
			}
			if tt.want {
				if err := EnsureProjectFiles(root, ProjectOptions{}); err != nil {
					t.Fatalf("repeat setup: %v", err)
				}
			}
		})
	}
}

func TestEnsureGoModPreservesRequirementsAndLineEndings(t *testing.T) {
	for _, tt := range []struct{ name, newline, comment string }{
		{"LF class marker", "\n", "//xgo:class"},
		{"CRLF class marker", "\r\n", "//xgo:class"},
		{"LF ordinary comment", "\n", "// SPX runtime dependency"},
		{"CRLF ordinary comment", "\r\n", "// SPX runtime dependency"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			source := "module example.com/demo\n\nrequire (\n github.com/goplus/spx/v4 v4.0.0 " + tt.comment + "\n " + ModulePath + " v0.0.0\n example.com/keep v1.0.0 // keep me\n)\n\nreplace github.com/goplus/spx/v4 => ../old // local spx checkout\nreplace github.com/goplus/spx/v4 v4.0.0 => ../versioned-old\nreplace example.com/keep => ../keep\n"
			path := filepath.Join(root, "go.mod")
			writeProjectFile(t, path, strings.ReplaceAll(source, "\n", tt.newline))
			opts := ProjectOptions{FindSpxRoot: func(start string) string {
				if start != root {
					t.Fatalf("lookup path=%q, want %q", start, root)
				}
				return filepath.Join(root, "..", "local-spx")
			}}
			if err := EnsureGoMod(root, opts); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "xgo:class") || !strings.Contains(string(data), "// keep me") || !strings.Contains(string(data), "// local spx checkout") {
				t.Fatalf("wrong comment preservation: %s", data)
			}
			if tt.comment != "//xgo:class" && !strings.Contains(string(data), tt.comment) {
				t.Fatalf("SPX requirement comment was removed: %s", data)
			}
			if tt.newline == "\r\n" && strings.Contains(strings.ReplaceAll(string(data), "\r\n", ""), "\n") {
				t.Fatal("changed CRLF line endings")
			}
			file, err := modfile.Parse(path, data, nil)
			if err != nil {
				t.Fatal(err)
			}
			requires, replaces := map[string]string{}, map[string]string{}
			for _, req := range file.Require {
				requires[req.Mod.Path] = req.Mod.Version
			}
			for _, repl := range file.Replace {
				replaces[repl.Old.Path] = repl.New.Path
			}
			if requires[ModulePath] != Version || requires["example.com/keep"] != "v1.0.0" || replaces[spxModulePath] != "../local-spx" || replaces["example.com/keep"] != "../keep" {
				t.Fatalf("requires=%v replaces=%v", requires, replaces)
			}
			if err := EnsureGoMod(root, opts); err != nil {
				t.Fatal(err)
			}
			again, err := os.ReadFile(path)
			if err != nil || string(again) != string(data) {
				t.Fatalf("repeated update changed go.mod: %v\n%s", err, again)
			}
		})
	}
}

func TestEnsureGoModPropagatesBuilderAIReplacement(t *testing.T) {
	for _, tt := range []struct{ name, replacements, comment string }{
		{name: "new replacement"},
		{name: "existing local replacement", replacements: "replace " + ModulePath + " => ../local-ai // AI backend\n", comment: "// AI backend"},
		{name: "existing versioned replacement", replacements: "replace " + ModulePath + " " + Version + " => example.com/old-ai v0.1.0\n"},
		{name: "multiple old replacements", replacements: "replace " + ModulePath + " => example.com/old-ai v0.1.0 // AI backend\nreplace " + ModulePath + " v0.0.0 => ../old-ai\n", comment: "// AI backend"},
		{name: "versioned replacement before wildcard", replacements: "replace " + ModulePath + " v0.0.0 => ../old-ai\nreplace " + ModulePath + " => example.com/old-ai v0.1.0 // AI backend\n", comment: "// AI backend"},
		{name: "multiple versioned replacements", replacements: "replace " + ModulePath + " v0.1.0 => ../old-ai\nreplace " + ModulePath + " v0.2.0 => example.com/old-ai v0.3.0\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "go.mod")
			source := "module example.com/demo\nrequire " + ModulePath + " v0.0.0\nreplace example.com/keep v1.0.0 => example.com/keep-fork v1.1.0\n" + tt.replacements
			writeProjectFile(t, path, source)
			original, err := modfile.Parse(path, []byte(source), nil)
			if err != nil {
				t.Fatal(err)
			}
			want := make(map[module.Version]module.Version)
			for _, repl := range original.Replace {
				if repl.Old.Path != ModulePath || ReplacementPath == "" {
					want[repl.Old] = repl.New
				}
			}
			if ReplacementPath != "" {
				want[module.Version{Path: ModulePath}] = module.Version{Path: ReplacementPath, Version: ReplacementVersion}
			}
			if err := EnsureGoMod(root, ProjectOptions{}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := modfile.Parse(path, data, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(file.Require) != 1 || file.Require[0].Mod != (module.Version{Path: ModulePath, Version: Version}) {
				t.Fatalf("AI requirement was not upgraded: %s", data)
			}
			got := make(map[module.Version]module.Version)
			for _, repl := range file.Replace {
				got[repl.Old] = repl.New
			}
			if len(file.Replace) != len(want) || !reflect.DeepEqual(got, want) {
				t.Fatalf("replacements=%v, want %v\n%s", got, want, data)
			}
			if !strings.Contains(string(data), tt.comment) {
				t.Fatalf("replacement comment %q was removed: %s", tt.comment, data)
			}
			if err := EnsureGoMod(root, ProjectOptions{}); err != nil {
				t.Fatal(err)
			}
			again, err := os.ReadFile(path)
			if err != nil || string(again) != string(data) {
				t.Fatalf("repeated dependency update changed go.mod: %v\n%s", err, again)
			}
		})
	}
}

func TestEnsureProjectFilesReportsConfigurationErrors(t *testing.T) {
	for _, tt := range []struct {
		name, config, content, want string
		directory                   bool
	}{
		{name: "gox read", config: "gox.mod", directory: true, want: "failed to read gox.mod"},
		{name: "go read", config: "go.mod", directory: true, want: "failed to read go.mod"},
		{name: "go parse", config: "go.mod", content: "invalid config", want: "failed to parse go.mod"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeProjectFile(t, filepath.Join(root, DescriptionFile), "description")
			if tt.directory {
				if err := os.Mkdir(filepath.Join(root, tt.config), 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeProjectFile(t, filepath.Join(root, tt.config), tt.content)
			}
			if err := EnsureProjectFiles(root, ProjectOptions{}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v, want %q", err, tt.want)
			}
		})
	}
	t.Run("missing parent", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "missing")
		if err := EnsureGoxMod(root); err == nil || !strings.Contains(err.Error(), "failed to write gox.mod") {
			t.Fatalf("gox error=%v", err)
		}
		if err := EnsureGoMod(root, ProjectOptions{}); err == nil || !strings.Contains(err.Error(), "failed to write go.mod") {
			t.Fatalf("go error=%v", err)
		}
	})
}

func TestEnsureGoModValidationFailureDoesNotWrite(t *testing.T) {
	for _, tt := range []struct {
		name, existing, want string
		opts                 ProjectOptions
	}{
		{name: "invalid template", opts: ProjectOptions{GoModTemplate: "invalid template"}, want: "failed to parse go.mod"},
		{name: "invalid existing file", existing: "invalid existing file", want: "failed to parse go.mod"},
		{name: "new file with invalid local path", opts: ProjectOptions{FindSpxRoot: func(string) string { return "relative" }}, want: "failed to resolve local spx replace path"},
		{name: "existing file with invalid local path", existing: "module example.com/existing\n", opts: ProjectOptions{FindSpxRoot: func(string) string { return "relative" }}, want: "failed to resolve local spx replace path"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "go.mod")
			if tt.existing != "" {
				writeProjectFile(t, path, tt.existing)
			}
			if err := EnsureGoMod(root, tt.opts); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v, want %q", err, tt.want)
			}
			data, err := os.ReadFile(path)
			if tt.existing == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("failed validation created go.mod: %q, error=%v", data, err)
				}
			} else if err != nil || string(data) != tt.existing {
				t.Fatalf("failed validation changed go.mod: %q, error=%v", data, err)
			}
		})
	}
}

func TestEnsureGoModReportsFileErrors(t *testing.T) {
	t.Run("parent is a file", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "not-a-dir")
		writeProjectFile(t, root, "blocker")
		var pathErr *os.PathError
		if err := EnsureGoMod(root, ProjectOptions{}); !errors.As(err, &pathErr) || pathErr.Path != filepath.Join(root, "go.mod") {
			t.Fatalf("error=%v, want go.mod path error", err)
		}
		if data, err := os.ReadFile(root); err != nil || string(data) != "blocker" {
			t.Fatalf("parent file=%q error=%v", data, err)
		}
	})
	for _, tt := range []struct {
		name, want string
		mode       os.FileMode
		missing    bool
	}{
		{name: "read denied", mode: 0o000, want: "failed to read go.mod"},
		{name: "write denied", mode: 0o444, want: "failed to write go.mod"},
		{name: "create denied", mode: 0o555, missing: true, want: "failed to write go.mod"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" || os.Geteuid() == 0 {
				t.Skip("requires Unix permission enforcement")
			}
			root := t.TempDir()
			path := filepath.Join(root, "go.mod")
			protected := path
			const original = "module example.com/existing\n"
			if tt.missing {
				protected = root
			} else {
				writeProjectFile(t, path, original)
			}
			if err := os.Chmod(protected, tt.mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(protected, 0o755) })
			if err := EnsureGoMod(root, ProjectOptions{}); err == nil || !strings.Contains(err.Error(), tt.want) || !errors.Is(err, os.ErrPermission) {
				t.Fatalf("error=%v, want %q permission error", err, tt.want)
			}
			if err := os.Chmod(protected, 0o755); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if tt.missing {
				if !os.IsNotExist(err) {
					t.Fatalf("failed creation left go.mod: %q, error=%v", data, err)
				}
			} else if err != nil || string(data) != original {
				t.Fatalf("failed access changed go.mod: %q, error=%v", data, err)
			}
		})
	}
}

func writeProjectFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
