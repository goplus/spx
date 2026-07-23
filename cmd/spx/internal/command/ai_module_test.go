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

package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	builderai "github.com/goplus/spx/v3/cmd/spx/internal/command/builderai"
)

func TestEnsureBuilderAIModuleFilesCreatesFilesInProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	godotProjectDir := filepath.Join(projectRoot, "project")
	if err := os.MkdirAll(godotProjectDir, 0755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, builderai.DescriptionFile), []byte("summary"), 0644); err != nil {
		t.Fatalf("write %s: %v", builderai.DescriptionFile, err)
	}

	cmd := CmdTool{
		ProjectDir: godotProjectDir,
		GoModTemplate: `module github.com/goplus/spxdemo

go 1.25.0

require github.com/goplus/spx/v3 v3.0.0-test //xgo:class
`,
	}
	if err := cmd.ensureBuilderAIModuleFiles(projectRoot); err != nil {
		t.Fatalf("ensureBuilderAIModuleFiles failed: %v", err)
	}

	goxMod := readTestFile(t, filepath.Join(projectRoot, "gox.mod"))
	if !strings.Contains(goxMod, `import "`+builderai.ModulePath+`"`) {
		t.Fatalf("gox.mod should import builder ai, got:\n%s", goxMod)
	}
	if !strings.Contains(goxMod, "runner github.com/goplus/spx/v3/cmd/spxrunner") {
		t.Fatalf("gox.mod should be based on the embedded root template, got:\n%s", goxMod)
	}

	goMod := readTestFile(t, filepath.Join(projectRoot, "go.mod"))
	if !strings.Contains(goMod, builderai.ModulePath+` `+builderai.Version) {
		t.Fatalf("go.mod should require builder ai before xgo go, got:\n%s", goMod)
	}
	if strings.Contains(goMod, "//xgo:class") {
		t.Fatalf("go.mod should use project gox.mod instead of spx module class config, got:\n%s", goMod)
	}

	if _, err := os.Stat(filepath.Join(godotProjectDir, "gox.mod")); !os.IsNotExist(err) {
		t.Fatalf("gox.mod should not be generated in project/ subdir, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(godotProjectDir, "go.mod")); !os.IsNotExist(err) {
		t.Fatalf("go.mod should not be generated in project/ subdir, stat err: %v", err)
	}
}

func TestEnsureBuilderAIModuleFilesUpdatesExistingFiles(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, builderai.DescriptionFile), []byte("summary"), 0644); err != nil {
		t.Fatalf("write %s: %v", builderai.DescriptionFile, err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "gox.mod"), []byte("project main.spx Game github.com/goplus/spx/v3 math\n"), 0644); err != nil {
		t.Fatalf("write gox.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte(`module demo

go 1.25.0

require (
	github.com/goplus/spx/v3 v3.0.0-test //xgo:class
)
`), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	cmd := CmdTool{}
	if err := cmd.ensureBuilderAIModuleFiles(projectRoot); err != nil {
		t.Fatalf("ensureBuilderAIModuleFiles failed: %v", err)
	}

	goxMod := readTestFile(t, filepath.Join(projectRoot, "gox.mod"))
	if !strings.Contains(goxMod, `import "`+builderai.ModulePath+`"`) {
		t.Fatalf("existing gox.mod should import builder ai, got:\n%s", goxMod)
	}

	goMod := readTestFile(t, filepath.Join(projectRoot, "go.mod"))
	if !strings.Contains(goMod, builderai.ModulePath+` `+builderai.Version) {
		t.Fatalf("existing go.mod should require builder ai before xgo go, got:\n%s", goMod)
	}
	if strings.Contains(goMod, "//xgo:class") {
		t.Fatalf("existing go.mod should remove spx xgo class marker, got:\n%s", goMod)
	}
}

func TestEnsureBuilderAIModuleFilesSupportsRawDescriptionFilename(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, builderai.DescriptionFileRaw), []byte("summary"), 0644); err != nil {
		t.Fatalf("write %s: %v", builderai.DescriptionFileRaw, err)
	}

	cmd := CmdTool{
		GoModTemplate: `module github.com/goplus/spxdemo

go 1.25.0

require github.com/goplus/spx/v3 v3.0.0-test //xgo:class
`,
	}
	if err := cmd.ensureBuilderAIModuleFiles(projectRoot); err != nil {
		t.Fatalf("ensureBuilderAIModuleFiles failed: %v", err)
	}

	goxMod := readTestFile(t, filepath.Join(projectRoot, "gox.mod"))
	if !strings.Contains(goxMod, `import "`+builderai.ModulePath+`"`) {
		t.Fatalf("gox.mod should import builder ai for raw description filename, got:\n%s", goxMod)
	}

	goMod := readTestFile(t, filepath.Join(projectRoot, "go.mod"))
	if !strings.Contains(goMod, builderai.ModulePath+` `+builderai.Version) {
		t.Fatalf("go.mod should require builder ai for raw description filename, got:\n%s", goMod)
	}
}

func TestEnsureBuilderAIModuleFilesReusesLocalSpxReplace(t *testing.T) {
	repoRoot := t.TempDir()
	writeLocalSpxRepoMarker(t, repoRoot)

	projectRoot := filepath.Join(repoRoot, "tutorial", "AI-Town-All")
	if err := os.MkdirAll(projectRoot, 0755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, builderai.DescriptionFile), []byte("summary"), 0644); err != nil {
		t.Fatalf("write %s: %v", builderai.DescriptionFile, err)
	}

	cmd := CmdTool{
		GoModTemplate: `module github.com/goplus/spxdemo

go 1.25.0

require github.com/goplus/spx/v3 v3.0.0-test //xgo:class
`,
	}
	if err := cmd.ensureBuilderAIModuleFiles(projectRoot); err != nil {
		t.Fatalf("ensureBuilderAIModuleFiles failed: %v", err)
	}

	goMod := readTestFile(t, filepath.Join(projectRoot, "go.mod"))
	if !strings.Contains(goMod, "replace github.com/goplus/spx/v3 => ../..") {
		t.Fatalf("go.mod should reuse local spx replace, got:\n%s", goMod)
	}
	if !strings.Contains(goMod, builderai.ModulePath+` `+builderai.Version) {
		t.Fatalf("go.mod should require builder ai before xgo go, got:\n%s", goMod)
	}
	if strings.Contains(goMod, "//xgo:class") {
		t.Fatalf("go.mod should remove spx xgo class marker, got:\n%s", goMod)
	}
}

func TestEnsureBuilderAIModuleFilesSkipsProjectsWithoutDescription(t *testing.T) {
	projectRoot := t.TempDir()
	cmd := CmdTool{GoModTemplate: "module github.com/goplus/spxdemo\n"}

	if err := cmd.ensureBuilderAIModuleFiles(projectRoot); err != nil {
		t.Fatalf("ensureBuilderAIModuleFiles failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, "gox.mod")); !os.IsNotExist(err) {
		t.Fatalf("gox.mod should not be generated without ai description, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); !os.IsNotExist(err) {
		t.Fatalf("go.mod should not be generated without ai description, stat err: %v", err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
