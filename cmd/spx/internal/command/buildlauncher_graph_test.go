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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveSPXSourceModes(t *testing.T) {
	root := t.TempDir()
	writeCommandTestFile(t, filepath.Join(root, "cmd", "ispxnative", "main.go"), "package main\n")
	wantRoot, err := canonicalDirectory(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		module listedModule
		mode   bool
	}{
		{
			name: "module cache",
			module: listedModule{
				Path: spxModulePath, Version: "v4.1.0", Dir: root,
			},
		},
		{
			name: "local replacement",
			module: listedModule{
				Path: spxModulePath, Version: "v4.1.0", Dir: root,
				Replace: &listedModule{Path: root, Dir: root},
			},
			mode: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			gotRoot, selected, _, mode, err := resolveSPXSource(test.module)
			if err != nil {
				t.Fatal(err)
			}
			if gotRoot != wantRoot || selected != "v4.1.0" || mode != test.mode {
				t.Fatalf("source = root %q, selected %q, mode %v", gotRoot, selected, mode)
			}
		})
	}
}

func TestResolveSPXSourceWithoutModuleDirectory(t *testing.T) {
	root, selected, effective, sourceMode, err := resolveSPXSource(listedModule{
		Path: spxModulePath, Version: "v4.1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if root != "" || selected != "v4.1.0" || effective != "v4.1.0" || sourceMode {
		t.Fatalf("source = root %q, selected %q, effective %q, mode %v", root, selected, effective, sourceMode)
	}
}

func TestGraphFlagsFromEnvQuotedModfile(t *testing.T) {
	workDir := t.TempDir()
	modDir := filepath.Join(workDir, "module with spaces")
	modfile := filepath.Join(modDir, "go.mod")
	writeCommandTestFile(t, modfile, "module example.com/game\n\ngo 1.25.0\n")
	canonical, err := canonicalFile(modfile)
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"-modfile=", "--modfile="} {
		t.Run(prefix, func(t *testing.T) {
			t.Setenv("GOFLAGS", "-mod=mod '"+prefix+modfile+"' -trimpath")
			flags, err := graphFlagsFromEnv(workDir)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"-mod=mod", "-modfile=" + canonical, "-trimpath"}
			if !reflect.DeepEqual(flags, want) {
				t.Fatalf("graphFlagsFromEnv = %#v, want %#v", flags, want)
			}
		})
	}
}

func TestGraphFlagsFromEnvRejectsUnterminatedQuote(t *testing.T) {
	t.Setenv("GOFLAGS", "'-modfile=broken")
	_, err := graphFlagsFromEnv(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "parse GOFLAGS") {
		t.Fatalf("graphFlagsFromEnv error = %v, want GOFLAGS parse error", err)
	}
}

func TestLauncherGraphProtectedFilesIncludesSums(t *testing.T) {
	root := t.TempDir()
	goMod := filepath.Join(root, "go.mod")
	goWork := filepath.Join(root, "go.work")
	modfile := filepath.Join(root, "alternate.mod")
	files := launcherGraphProtectedFiles(
		[]string{goMod, goWork, filepath.Join(root, "gox.mod")},
		[]string{"-modfile=" + modfile},
	)
	got := make(map[string]bool, len(files))
	for _, file := range files {
		got[file] = true
	}
	for _, want := range []string{
		filepath.Join(root, "go.sum"),
		filepath.Join(root, "go.work.sum"),
		filepath.Join(root, "alternate.sum"),
	} {
		if !got[want] {
			t.Errorf("protected files omit %q: %#v", want, files)
		}
	}
	if got[filepath.Join(root, "gox.sum")] {
		t.Fatalf("protected files include unrelated gox.sum: %#v", files)
	}
}

func TestLauncherGraphVerifier(t *testing.T) {
	root := t.TempDir()
	goMod := filepath.Join(root, "go.mod")
	metadata := filepath.Join(root, "gox.mod")
	dependency := filepath.Join(t.TempDir(), "dependency")
	writeCommandTestFile(t, filepath.Join(dependency, "go.mod"), "module example.com/dependency\n\ngo 1.25.0\n")
	moduleText := func(version string) string {
		return "module example.com/game\n\ngo 1.25.0\n\nrequire (\n\tgithub.com/goplus/spx/v4 v4.0.0\n\texample.com/dependency " + version + "\n)\nreplace github.com/goplus/spx/v4 => " + filepath.ToSlash(findSPXRoot(t)) + "\nreplace example.com/dependency => " + filepath.ToSlash(dependency) + "\n"
	}
	writeCommandTestFile(t, goMod, moduleText("v1.0.0"))
	writeCommandTestFile(t, metadata, "xgo 1.8.0\n")
	goCommand, err := resolveGoCommand()
	if err != nil {
		t.Fatal(err)
	}
	env := launcherGraphEnvironment(os.Environ(), "off")
	_, verifier, err := resolveLauncherGraph(context.Background(), launcherGraphQuery{
		goCommand: goCommand, workDir: root, goWork: "off", env: env,
		graphFlags: []string{"-mod=mod"}, files: []string{goMod, metadata},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := verifier.verify(context.Background()); err != nil {
		t.Fatal(err)
	}
	writeCommandTestFile(t, filepath.Join(root, "go.sum"), "")
	if err := verifier.verify(context.Background()); err != nil {
		t.Fatalf("go.sum changed graph identity: %v", err)
	}
	writeCommandTestFile(t, goMod, moduleText("v1.1.0"))
	if err := verifier.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "module selection changed") {
		t.Fatalf("module selection change error = %v", err)
	}
	writeCommandTestFile(t, goMod, moduleText("v1.0.0"))
	writeCommandTestFile(t, metadata, "xgo 1.9.0\n")
	if err := verifier.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "module input changed") {
		t.Fatalf("metadata change error = %v", err)
	}
}

func TestLauncherGraphVerifierVendorMetadata(t *testing.T) {
	root := t.TempDir()
	spxRoot := filepath.Join(root, "spx")
	moduleRoot := filepath.Join(root, "game")
	writeCommandTestFile(t, filepath.Join(spxRoot, "go.mod"), "module "+spxModulePath+"\n\ngo 1.25.0\n")
	writeCommandTestFile(t, filepath.Join(spxRoot, "cmd", "ispxnative", "main.go"), "package ispxnative\n")
	goMod := filepath.Join(moduleRoot, "go.mod")
	writeCommandTestFile(t, goMod, "module example.com/game\n\ngo 1.25.0\n\nrequire "+spxModulePath+" v4.0.0\nreplace "+spxModulePath+" => "+filepath.ToSlash(spxRoot)+"\n")
	writeCommandTestFile(t, filepath.Join(moduleRoot, "main.go"), "package main\nimport _ \""+spxModulePath+"/cmd/ispxnative\"\nfunc main() {}\n")
	goCommand, err := resolveGoCommand()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(goCommand, "mod", "vendor")
	command.Dir, command.Env = moduleRoot, launcherGraphEnvironment(os.Environ(), "off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go mod vendor: %v\n%s", err, output)
	}
	_, verifier, err := resolveLauncherGraph(context.Background(), launcherGraphQuery{
		goCommand: goCommand, workDir: moduleRoot, goWork: "off",
		graphFlags: []string{"-mod=vendor"}, env: command.Env, files: []string{goMod},
	})
	if err != nil {
		t.Fatal(err)
	}
	vendorFile := filepath.Join(moduleRoot, "vendor", "modules.txt")
	file, err := os.OpenFile(vendorFile, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("# changed\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifier.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "module input changed") {
		t.Fatalf("vendor metadata change error = %v", err)
	}
}

func TestResolveSPXSourceRejectsIncompleteModules(t *testing.T) {
	root := t.TempDir()
	bridge := filepath.Join(root, "cmd", "ispxnative", "main.go")
	if err := os.MkdirAll(bridge, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		module listedModule
		want   string
	}{
		{"wrong module", listedModule{Path: "example.com/wrong"}, "Go graph selected"},
		{"local root missing", listedModule{Path: spxModulePath, Main: true}, "no source directory"},
		{"missing directory", listedModule{Path: spxModulePath, Dir: filepath.Join(root, "missing")}, "SPX module root"},
		{"missing bridge", listedModule{Path: spxModulePath, Dir: t.TempDir()}, "no cmd/ispxnative package"},
		{"directory bridge", listedModule{Path: spxModulePath, Dir: root}, "no regular cmd/ispxnative"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, _, err := resolveSPXSource(tc.module); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("source error = %v, want %q", err, tc.want)
			}
		})
	}
	writeCommandTestFile(t, filepath.Join(root, "file"), "data")
	for _, tc := range []struct {
		name string
		run  func(string) (string, error)
		path string
	}{
		{"empty directory", canonicalDirectory, ""},
		{"file as directory", canonicalDirectory, filepath.Join(root, "file")},
		{"directory as file", canonicalFile, root},
		{"missing file", canonicalFile, filepath.Join(root, "missing")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.run(tc.path); err == nil {
				t.Fatal("accepted invalid canonical path")
			}
		})
	}
}

func TestLauncherGraphQueryFailures(t *testing.T) {
	for _, tc := range []struct{ name, script, want string }{
		{"command failure", "exit 7\n", "go list module"},
		{"stderr", "echo graph-unavailable >&2; exit 7\n", "graph-unavailable"},
		{"malformed output", "echo '{'\n", "decode go list module"},
		{"wrong module", "echo '{\"Path\":\"example.com/wrong\"}'\n", "want"},
		{"missing local root", `printf '{"Path":"github.com/goplus/spx/v4","Main":true}'`, "local module"},
		{"selection failure", `case "$*" in *-json*) printf '{"Path":"github.com/goplus/spx/v4"}';; *) exit 7;; esac`, "go list module graph"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := installCommandTestTools(t, map[string]string{"go": tc.script})
			query := launcherGraphQuery{goCommand: filepath.Join(bin, "go"), workDir: t.TempDir(), env: os.Environ()}
			if _, _, err := resolveLauncherGraph(context.Background(), query); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("graph error = %v, want %q", err, tc.want)
			}
			if err := (launcherGraphVerifier{query: query}).verify(context.Background()); err == nil {
				t.Fatal("verifier accepted failed graph query")
			}
			if tc.name == "command failure" || tc.name == "stderr" {
				if _, err := queryModuleSelection(context.Background(), query); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("selection error = %v, want %q", err, tc.want)
				}
				if _, err := queryGoEnv(context.Background(), query.goCommand, query.workDir, query.env, "GOMOD"); err == nil {
					t.Fatal("environment query ignored command failure")
				}
			}
		})
	}
}

func TestResolveLauncherInputsRejectsIncompleteEnvironment(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"missing go command", "resolve Go module file"},
		{"outside module", "outside a Go module"},
		{"missing module file", "Go module file"},
		{"outside active root", "outside the active Go module"},
		{"missing metadata", "no gox.mod or gop.mod"},
		{"workspace query", "resolve Go workspace"},
		{"missing workspace", "Go workspace file"},
		{"invalid flags", "parse GOFLAGS"},
		{"graph query", "resolve module graph"},
		{"invalid source", "no cmd/ispxnative package"},
		{"empty workspace", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			mod := filepath.Join(root, "go.mod")
			writeCommandTestFile(t, mod, "module example.com/game\ngo 1.25\n")
			writeCommandTestFile(t, filepath.Join(root, "gox.mod"), "project main.spx Game github.com/goplus/spx/v4 math\npack assets index.json\n")
			writeCommandTestFile(t, filepath.Join(root, "main.spx"), "onStart => {}\n")
			t.Setenv("GOFLAGS", "")
			t.Setenv("SPX_TEST_GOMOD", mod)
			t.Setenv("SPX_TEST_GOWORK", "off")
			t.Setenv("SPX_TEST_MODULE", `{"Path":"github.com/goplus/spx/v4","Version":"v4.0.0"}`)
			t.Setenv("SPX_TEST_FAIL", "")
			bin := installCommandTestTools(t, map[string]string{"go": `
case "$1 $2" in
 "env GOMOD") printf '%s\n' "$SPX_TEST_GOMOD" ;;
 "env GOWORK") [ "$SPX_TEST_FAIL" != workspace ] || exit 7; printf '%s\n' "$SPX_TEST_GOWORK" ;;
 "list -m") [ "$SPX_TEST_FAIL" != graph ] || exit 7; printf '%s\n' "$SPX_TEST_MODULE" ;;
 *) exit 9 ;;
esac
`})
			goCommand := filepath.Join(bin, "go")
			switch tc.name {
			case "missing go command":
				goCommand += "-missing"
			case "outside module":
				t.Setenv("SPX_TEST_GOMOD", os.DevNull)
			case "missing module file":
				t.Setenv("SPX_TEST_GOMOD", filepath.Join(root, "missing.mod"))
			case "outside active root":
				root = t.TempDir()
			case "missing metadata":
				if err := os.Remove(filepath.Join(root, "gox.mod")); err != nil {
					t.Fatal(err)
				}
			case "workspace query":
				t.Setenv("SPX_TEST_FAIL", "workspace")
			case "missing workspace":
				t.Setenv("SPX_TEST_GOWORK", filepath.Join(root, "missing.work"))
			case "invalid flags":
				t.Setenv("GOFLAGS", "'unfinished")
			case "graph query":
				t.Setenv("SPX_TEST_FAIL", "graph")
			case "invalid source":
				t.Setenv("SPX_TEST_MODULE", fmt.Sprintf(`{"Path":"github.com/goplus/spx/v4","Main":true,"Dir":%q,"GoMod":%q}`, root, mod))
			case "empty workspace":
				t.Setenv("SPX_TEST_GOWORK", "")
			}
			project, source, err := resolveLauncherInputs(context.Background(), root, goCommand)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("input error = %v, want %q", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if project.dir != root || source.goWork != "off" || source.selectedVersion != "v4.0.0" || source.verifyGraph == nil {
				t.Fatalf("resolved project/source = %+v / %+v", project, source)
			}
			if err := source.verifyGraph(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLauncherGraphNormalizesFilesFlagsAndModuleIdentity(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mod := filepath.Join(root, "alternate.mod")
	writeCommandTestFile(t, mod, "module example.com/game\n")
	flags, err := graphFlagsFromEnvironment(root, []string{"GOFLAGS=-modfile=alternate.mod -trimpath"})
	if err != nil || !reflect.DeepEqual(flags, []string{"-modfile=" + mod, "-trimpath"}) {
		t.Fatalf("relative modfile = %q, %v", flags, err)
	}
	if _, err := graphFlagsFromEnvironment(root, []string{"GOFLAGS=-modfile=missing.mod"}); err == nil {
		t.Fatal("accepted missing alternate modfile")
	}
	if got, err := graphFlagsFromEnvironment(root, nil); err != nil || len(got) != 0 {
		t.Fatalf("empty environment flags = %q, %v", got, err)
	}
	if _, err := snapshotLauncherFiles([]string{"", mod, mod}); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotLauncherFiles([]string{filepath.Join(root, "missing")}); err == nil {
		t.Fatal("snapshotted missing graph input")
	}
	for _, tc := range []struct {
		name   string
		module listedModule
		want   string
	}{
		{"local directory missing", listedModule{Path: spxModulePath, Main: true}, "local module"},
		{"local go.mod missing", listedModule{Path: spxModulePath, Main: true, Dir: root}, "has no go.mod"},
		{"versioned replacement", listedModule{Path: spxModulePath, Version: "v4.0.0", Replace: &listedModule{Path: "example.com/fork", Version: "v3.1.0", Sum: "replacement-sum", GoModSum: "replacement-mod-sum"}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			identity, file, err := normalizeLauncherModule(tc.module)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("module error = %v", err)
				}
				return
			}
			if err != nil || file != "" || identity.ReplacePath != "example.com/fork" || identity.ReplaceVersion != "v3.1.0" || identity.ReplaceSum != "replacement-sum" || identity.ReplaceGoModSum != "replacement-mod-sum" {
				t.Fatalf("module identity = %+v, %q, %v", identity, file, err)
			}
		})
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := resolveGoCommand(); err == nil {
		t.Fatal("resolved Go command from empty PATH")
	}
}
