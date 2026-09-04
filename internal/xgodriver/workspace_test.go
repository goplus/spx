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

package xgodriver

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/mod/modfile"
)

func TestRewriteGoWorkMakesLocalPathsAbsolute(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "go.work")
	data := []byte(`go 1.25.0

use (
	./app
	../shared
)

replace example.test/dependency => ./dependency
replace example.test/versioned v1.0.0 => example.test/fork v1.1.0
replace example.test/multi => ./all
replace example.test/multi v1.2.3 => ./one
`)
	rewritten, err := rewriteGoWork(path, data)
	if err != nil {
		t.Fatal(err)
	}
	work, err := modfile.ParseWork("private.go.work", rewritten, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantUses := map[string]bool{
		filepath.Join(root, "app"):                          true,
		filepath.Clean(filepath.Join(root, "..", "shared")): true,
	}
	for _, use := range work.Use {
		if !wantUses[use.Path] {
			t.Errorf("private use path = %q, want an absolute original-workspace path", use.Path)
		}
		delete(wantUses, use.Path)
	}
	if len(wantUses) != 0 {
		t.Fatalf("private workspace is missing use paths: %v", wantUses)
	}
	for _, replacement := range work.Replace {
		switch replacement.Old.Path {
		case "example.test/dependency":
			if want := filepath.Join(root, "dependency"); replacement.New.Path != want || replacement.New.Version != "" {
				t.Errorf("local replacement = %#v, want path %q", replacement.New, want)
			}
		case "example.test/versioned":
			if replacement.New.Path != "example.test/fork" || replacement.New.Version != "v1.1.0" {
				t.Errorf("versioned replacement changed: %#v", replacement.New)
			}
		case "example.test/multi":
			want := filepath.Join(root, "all")
			if replacement.Old.Version != "" {
				want = filepath.Join(root, "one")
			}
			if replacement.New.Path != want || replacement.New.Version != "" {
				t.Errorf("multi replacement %q = %#v, want path %q", replacement.Old.Version, replacement.New, want)
			}
		}
	}
}

func TestIsolateGoWorkspaceKeepsCallerSumPrivate(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	dependency := filepath.Join(root, "dependency")
	workPath := filepath.Join(root, "go.work")
	callerSum := []byte("golang.org/x/text v0.21.0 h1:zyQAAkrwaneQ066sspRyJaG9VNi/YJ1NfzcGB3hZ/qo=\n")
	mustWriteDriverTestFile(t, workPath, "go 1.25.0\n\nuse ./app\n\nreplace example.test/dependency => ./dependency\n", 0o600)
	mustWriteDriverTestFile(t, workPath+".sum", string(callerSum), 0o600)
	mustWriteDriverTestFile(t, filepath.Join(app, "go.mod"), "module example.test/app\n\ngo 1.25.0\n\nrequire example.test/dependency v0.0.0\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(dependency, "go.mod"), "module example.test/dependency\n\ngo 1.25.0\n", 0o600)
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		GoCommand: goCommand, GraphWorkDir: app, GoWork: workPath,
		GraphFlags: []string{"-mod=readonly"},
	}
	isolated, cleanup, err := isolateGoWorkspace(context.Background(), cfg, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	privateDir := filepath.Dir(isolated.goWorkForCommand())
	if isolated.goWorkForCommand() == workPath || privateDir == root {
		t.Fatalf("private Go workspace path = %q", isolated.goWorkForCommand())
	}
	privateSum, err := os.ReadFile(isolated.goWorkForCommand() + ".sum")
	if err != nil || !bytes.Equal(privateSum, callerSum) {
		t.Fatalf("private go.work.sum = %q, %v; want caller snapshot", privateSum, err)
	}
	if err := os.WriteFile(isolated.goWorkForCommand()+".sum", []byte("private update\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	currentCallerSum, err := os.ReadFile(workPath + ".sum")
	if err != nil || !bytes.Equal(currentCallerSum, callerSum) {
		t.Fatalf("caller go.work.sum = %q, %v; want unchanged snapshot", currentCallerSum, err)
	}
	cleanup()
	if _, err := os.Stat(privateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private Go workspace was not removed: %v", err)
	}
}

func TestIsolateGoWorkspaceBuildPreservesCallerGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	workspaceModule := filepath.Join(root, "workspace-module")
	localReplacement := filepath.Join(root, "local-replacement")
	workPath := filepath.Join(root, "go.work")
	const sum = "golang.org/x/text v0.21.0 h1:zyQAAkrwaneQ066sspRyJaG9VNi/YJ1NfzcGB3hZ/qo=\n"
	fixtureFiles := map[string]string{
		workPath: `go 1.25.0

use (
	./app
	./workspace-module
)

replace example.test/dependency => ./local-replacement
`,
		workPath + ".sum":                                sum,
		filepath.Join(app, "go.mod"):                     "module example.test/app\n\ngo 1.25.0\n\nrequire example.test/dependency v0.0.0\n",
		filepath.Join(app, "go.sum"):                     sum,
		filepath.Join(workspaceModule, "go.mod"):         "module example.test/framework\n\ngo 1.25.0\n",
		filepath.Join(workspaceModule, "go.sum"):         sum,
		filepath.Join(localReplacement, "go.mod"):        "module example.test/dependency\n\ngo 1.25.0\n",
		filepath.Join(localReplacement, "go.sum"):        sum,
		filepath.Join(app, "main.go"):                    "package main\n\nimport (\n\t_ \"example.test/dependency\"\n\t_ \"example.test/framework\"\n)\n\nfunc main() {}\n",
		filepath.Join(workspaceModule, "framework.go"):   "package framework\n",
		filepath.Join(localReplacement, "dependency.go"): "package dependency\n",
	}
	for path, content := range fixtureFiles {
		mustWriteDriverTestFile(t, path, content, 0o600)
	}
	callerGraphFiles := []string{
		workPath, workPath + ".sum",
		filepath.Join(app, "go.mod"), filepath.Join(app, "go.sum"),
		filepath.Join(workspaceModule, "go.mod"), filepath.Join(workspaceModule, "go.sum"),
		filepath.Join(localReplacement, "go.mod"), filepath.Join(localReplacement, "go.sum"),
	}
	graphSnapshots := make(map[string][]byte, len(callerGraphFiles))
	for _, path := range callerGraphFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		graphSnapshots[path] = data
	}

	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}
	isolated, cleanup, err := isolateGoWorkspace(context.Background(), Config{
		GoCommand: goCommand, GraphWorkDir: app, GoWork: workPath,
		GraphFlags: []string{"-mod=readonly"},
	}, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	privateWork := isolated.goWorkForCommand()
	if privateWork == workPath {
		t.Fatal("workspace isolation reused the caller workfile")
	}
	privateDir := filepath.Dir(privateWork)
	args := append([]string{"build"}, isolated.graphFlagsForCommand()...)
	args = append(args, "-o", filepath.Join(root, "app.bin"), ".")
	build := exec.Command(goCommand, args...)
	build.Dir = app
	build.Env = hostGoEnv(isolated, os.Environ())
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build with private workspace: %v\n%s", err, output)
	}
	for _, path := range callerGraphFiles {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, graphSnapshots[path]) {
			t.Fatalf("caller metadata %q = %q, %v; want unchanged", path, got, err)
		}
	}
	cleanup()
	if _, err := os.Stat(privateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private Go workspace was not removed: %v", err)
	}
}

func TestIsolateGoModKeepsCallerFilesPrivate(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	dependency := filepath.Join(root, "dependency")
	modPath := filepath.Join(app, "go.mod")
	sumPath := filepath.Join(app, "go.sum")
	modData := []byte("module example.test/app\n\ngo 1.25.0\n\nrequire example.test/dependency v0.0.0\n\nreplace example.test/dependency => ../dependency\n")
	sumData := []byte("golang.org/x/text v0.21.0 h1:zyQAAkrwaneQ066sspRyJaG9VNi/YJ1NfzcGB3hZ/qo=\n")
	mustWriteDriverTestFile(t, modPath, string(modData), 0o600)
	mustWriteDriverTestFile(t, sumPath, string(sumData), 0o600)
	mustWriteDriverTestFile(t, filepath.Join(dependency, "go.mod"), "module example.test/dependency\n\ngo 1.25.0\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(dependency, "dependency.go"), "package dependency\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(app, "main.go"), "package main\nimport _ \"example.test/dependency\"\nfunc main() {}\n", 0o600)
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}

	cfg := Config{GoCommand: goCommand, GraphWorkDir: app, GoWork: "off", GraphFlags: []string{"-mod=mod"}}
	isolated, cleanup, err := isolateGoGraph(context.Background(), cfg, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	privateModFile := isolated.privateModFileForCommand()
	if privateModFile == "" || filepath.Dir(privateModFile) == app {
		t.Fatalf("private modfile = %q", privateModFile)
	}
	if got := isolated.graphFlagsForCommand(); !containsGraphFlag(got, "-mod=mod") || !containsGraphModfile(got, privateModFile) {
		t.Fatalf("private graph flags = %q", got)
	}
	privateData, err := os.ReadFile(privateModFile)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := modfile.Parse(privateModFile, privateData, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantReplacement := filepath.Join(root, "dependency")
	if len(parsed.Replace) != 1 || parsed.Replace[0].New.Path != wantReplacement {
		t.Fatalf("private replacement = %#v, want %q", parsed.Replace, wantReplacement)
	}
	artifact := filepath.Join(root, "app.bin")
	buildArgs := append([]string{"build"}, isolated.graphFlagsForCommand()...)
	buildArgs = append(buildArgs, "-o", artifact, ".")
	build := exec.Command(goCommand, buildArgs...)
	build.Dir = app
	build.Env = hostGoEnv(isolated, os.Environ())
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build with private modfile: %v\n%s", err, output)
	}
	privateSum := goModSumPath(privateModFile)
	if got, err := os.ReadFile(privateSum); err != nil || !bytes.Equal(got, sumData) {
		t.Fatalf("private go.sum = %q, %v; want caller snapshot", got, err)
	}
	if err := os.WriteFile(privateSum, append(sumData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(modPath); err != nil || !bytes.Equal(got, modData) {
		t.Fatalf("caller go.mod = %q, %v; want unchanged snapshot", got, err)
	}
	if got, err := os.ReadFile(sumPath); err != nil || !bytes.Equal(got, sumData) {
		t.Fatalf("caller go.sum = %q, %v; want unchanged snapshot", got, err)
	}
}

func TestIsolateGoModUsesExplicitModfile(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	dependency := filepath.Join(root, "dependency")
	alternateMod := filepath.Join(root, "config", "nested", "alternate.mod")
	alternateSum := filepath.Join(root, "config", "nested", "alternate.sum")
	modData := []byte("module example.test/app\n\ngo 1.25.0\n\nrequire example.test/dependency v0.0.0\n\nreplace example.test/dependency => ../dependency\n")
	sumData := []byte("golang.org/x/text v0.21.0 h1:zyQAAkrwaneQ066sspRyJaG9VNi/YJ1NfzcGB3hZ/qo=\n")
	mustWriteDriverTestFile(t, filepath.Join(app, "go.mod"), "module example.test/app\n\ngo 1.25.0\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(dependency, "go.mod"), "module example.test/dependency\n\ngo 1.25.0\n", 0o600)
	mustWriteDriverTestFile(t, alternateMod, string(modData), 0o600)
	mustWriteDriverTestFile(t, alternateSum, string(sumData), 0o600)
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		GoCommand: goCommand, GraphWorkDir: app, GoWork: "off",
		GraphFlags: []string{"-mod=mod", "-modfile=" + alternateMod},
	}
	isolated, cleanup, err := isolateGoGraph(context.Background(), cfg, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	privateMod := isolated.privateModFileForCommand()
	if privateMod == "" || containsGraphModfile(isolated.graphFlagsForCommand(), alternateMod) || !containsGraphModfile(isolated.graphFlagsForCommand(), privateMod) {
		t.Fatalf("isolated graph flags = %q", isolated.graphFlagsForCommand())
	}
	privateData, err := os.ReadFile(privateMod)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := modfile.Parse(privateMod, privateData, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Replace) != 1 || parsed.Replace[0].New.Path != dependency {
		t.Fatalf("private replacement = %#v, want %q", parsed.Replace, dependency)
	}
	if got, err := os.ReadFile(goModSumPath(privateMod)); err != nil || !bytes.Equal(got, sumData) {
		t.Fatalf("private alternate sum = %q, %v", got, err)
	}
}

func containsGraphModfile(flags []string, want string) bool {
	for _, flag := range flags {
		if path, ok := graphModfilePath(flag); ok && path == want {
			return true
		}
	}
	return false
}

func TestIsolateGoWorkspaceCleansPrivateWorkspaceAfterValidationFailure(t *testing.T) {
	root := t.TempDir()
	tempRoot := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tempRoot)
	t.Setenv("TMP", tempRoot)
	t.Setenv("TEMP", tempRoot)

	app := filepath.Join(root, "app")
	workPath := filepath.Join(root, "go.work")
	mustWriteDriverTestFile(t, workPath, "go 1.25.0\n\nuse ./app\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(app, "go.mod"), "module example.test/app\n\ngo 1.25.0\n", 0o600)
	cfg := Config{
		GoCommand: filepath.Join(root, "missing-go"), GraphWorkDir: app, GoWork: workPath,
		GraphFlags: []string{"-mod=readonly"},
	}
	_, cleanup, err := isolateGoWorkspace(context.Background(), cfg, os.Environ())
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("isolateGoWorkspace succeeded with a missing Go command")
	}
	entries, readErr := os.ReadDir(tempRoot)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("private workspace leaked after validation failure: %v", entries)
	}
}
