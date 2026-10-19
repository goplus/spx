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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

func TestCreateDefaultGoModPropagatesStatErrors(t *testing.T) {
	parentPath := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parentPath, []byte("blocker"), 0o644); err != nil {
		t.Fatalf("write parent blocker: %v", err)
	}

	err := createDefaultGoMod(parentPath, "module example.com/demo\n", false)
	if err == nil {
		t.Fatal("createDefaultGoMod returned nil, want stat error")
	}
}

func TestCreateDefaultGoModSkipsExistingFileWithoutForce(t *testing.T) {
	projectRoot := t.TempDir()
	goModPath := filepath.Join(projectRoot, "go.mod")
	original := "module example.com/original\n"
	if err := os.WriteFile(goModPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if err := createDefaultGoMod(projectRoot, "module example.com/updated\n", false); err != nil {
		t.Fatalf("createDefaultGoMod failed: %v", err)
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if string(data) != original {
		t.Fatalf("go.mod = %q, want %q", string(data), original)
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
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(fmt.Sprintf("newline_%q", newline), func(t *testing.T) {
			root := t.TempDir()
			source := "module example.com/demo\n\nrequire (\n github.com/goplus/spx/v3 v3.0.0 //xgo:class\n example.com/keep v1.0.0 // keep me\n)\n\nreplace github.com/goplus/spx/v3 v3.0.0 => ../old\nreplace example.com/keep => ../keep\n"
			path := filepath.Join(root, "go.mod")
			writeProjectFile(t, path, strings.ReplaceAll(source, "\n", newline))
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
			if strings.Contains(string(data), "xgo:class") || !strings.Contains(string(data), "// keep me") {
				t.Fatalf("wrong comment preservation: %s", data)
			}
			if newline == "\r\n" && strings.Contains(strings.ReplaceAll(string(data), "\r\n", ""), "\n") {
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
		if err := EnsureGoMod(root, ProjectOptions{}); err == nil || !strings.Contains(err.Error(), "failed to create go.mod") {
			t.Fatalf("go error=%v", err)
		}
	})
	t.Run("relative source root", func(t *testing.T) {
		err := EnsureGoMod(t.TempDir(), ProjectOptions{FindSpxRoot: func(string) string { return "relative" }})
		if err == nil || !strings.Contains(err.Error(), "failed to resolve local spx replace path") {
			t.Fatalf("replace error=%v", err)
		}
	})
}

func TestCreateDefaultGoModForceUpdatesExistingFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	writeProjectFile(t, path, "module original\n")
	if err := createDefaultGoMod(root, "module updated\n", true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "module updated\n" {
		t.Fatalf("go.mod=%q error=%v", data, err)
	}
}

func writeProjectFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
