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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseRun(t *testing.T) {
	args := validArgs(t, ActionRun)
	args = append(args,
		"--graph-flag=-mod=readonly",
		"--build-flag=-trimpath=true",
		"--",
		"--headless", "", "a b", "--", "--output=application-value",
	)

	cfg, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cfg.Action != ActionRun {
		t.Fatalf("Action = %q, want run", cfg.Action)
	}
	if got, want := cfg.GraphFlags, []string{"-mod=readonly"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GraphFlags = %#v, want %#v", got, want)
	}
	if got, want := cfg.GraphWorkDir, filepath.Dir(cfg.ProjectDir); got != want {
		t.Fatalf("GraphWorkDir = %q, want %q", got, want)
	}
	if got, want := cfg.BuildFlags, []string{"-trimpath=true"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildFlags = %#v, want %#v", got, want)
	}
	wantAppArgs := []string{"--headless", "", "a b", "--", "--output=application-value"}
	if !reflect.DeepEqual(cfg.ApplicationArgs, wantAppArgs) {
		t.Fatalf("ApplicationArgs = %#v, want %#v", cfg.ApplicationArgs, wantAppArgs)
	}
	if cfg.Output != "" || cfg.FinalOutput != "" {
		t.Fatalf("run outputs = %q/%q, want empty", cfg.Output, cfg.FinalOutput)
	}
	if cfg.DriverOrigin.Replace != nil {
		t.Fatalf("Replace = %#v, want nil", cfg.DriverOrigin.Replace)
	}
	if cfg.DriverOrigin.Selected.Dir == "" || cfg.DriverOrigin.Selected.GoMod == "" {
		t.Fatalf("selected source is incomplete: %#v", cfg.DriverOrigin.Selected)
	}
	if cfg.Project.Extension != ".spx" || cfg.Project.FullExtension != "main.spx" {
		t.Fatalf("Project = %#v", cfg.Project)
	}
}

func TestParseBuildWithReplacement(t *testing.T) {
	args := validArgs(t, ActionBuild)
	args = removeOptions(args, "selected-dir", "selected-gomod")
	root := filepath.Dir(optionValue(args, "project-dir"))
	localSPX := filepath.Join(root, "local-spx")
	mustWriteDriverTestFile(t, filepath.Join(localSPX, "go.mod"), "module github.com/goplus/spx/v3\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(localSPX, "gox.mod"), "xgo 1.8\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(root, "alternate.mod"), "module example.com/alternate\n", 0o600)
	args = replaceOption(args, "declaration-file", filepath.Join(root, "local-spx", "gox.mod"))
	args = append(args,
		"--replace-path="+localSPX,
		"--replace-version=",
		"--replace-dir="+filepath.Join(root, "local-spx"),
		"--replace-gomod="+filepath.Join(root, "local-spx", "go.mod"),
		"--graph-flag=-modfile="+filepath.Join(root, "alternate.mod"),
		"--build-flag=-x=true",
		"--output="+filepath.Join(root, "out", "game"),
		"--final-output="+filepath.Join(root, "bin", "game"),
	)

	cfg, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cfg.DriverOrigin.Replace == nil {
		t.Fatal("Replace = nil, want local replacement")
	}
	if got, want := cfg.DriverOrigin.Selected.Version, "v3.2.0"; got != want {
		t.Fatalf("selected version = %q, want %q", got, want)
	}
	if cfg.DriverOrigin.Selected.Dir != "" || cfg.DriverOrigin.Selected.GoMod != "" {
		t.Fatalf("selected source leaked into replacement origin: %#v", cfg.DriverOrigin.Selected)
	}
	if got, want := cfg.DriverOrigin.Effective().Path, localSPX; got != want {
		t.Fatalf("effective path = %q, want %q", got, want)
	}
	wantGraph := []string{"-modfile=" + filepath.Join(root, "alternate.mod")}
	if !reflect.DeepEqual(cfg.GraphFlags, wantGraph) {
		t.Fatalf("GraphFlags = %#v, want %#v", cfg.GraphFlags, wantGraph)
	}
}

func TestParseAcceptsOfficialSplitModuleCacheGoMod(t *testing.T) {
	args := validArgs(t, ActionRun)
	root := filepath.Dir(optionValue(args, "project-dir"))
	cacheRoot := filepath.Join(root, "gomodcache")
	moduleDir := filepath.Join(cacheRoot, filepath.FromSlash("github.com/goplus/spx/v3@v3.2.0"))
	cacheGoMod := filepath.Join(cacheRoot, filepath.FromSlash("cache/download/github.com/goplus/spx/v3/@v/v3.2.0.mod"))
	mustWriteDriverTestFile(t, filepath.Join(moduleDir, "gox.mod"), "xgo 1.8\n", 0o600)
	mustWriteDriverTestFile(t, cacheGoMod, "module github.com/goplus/spx/v3\n", 0o600)
	args = replaceOption(args, "selected-dir", moduleDir)
	args = replaceOption(args, "selected-gomod", cacheGoMod)
	args = replaceOption(args, "declaration-file", filepath.Join(moduleDir, "gox.mod"))
	if _, err := Parse(append(args, "--")); err != nil {
		t.Fatalf("Parse() split module-cache source error: %v", err)
	}
}

func TestParseRejectsNonMatchingSplitModuleCacheGoMod(t *testing.T) {
	args := validArgs(t, ActionRun)
	root := filepath.Dir(optionValue(args, "project-dir"))
	cacheRoot := filepath.Join(root, "gomodcache")
	moduleDir := filepath.Join(cacheRoot, filepath.FromSlash("github.com/goplus/spx/v3@v3.2.0"))
	cacheGoMod := filepath.Join(cacheRoot, filepath.FromSlash("cache/download/github.com/goplus/spx/v3/@v/v3.2.0.mod"))
	mustWriteDriverTestFile(t, filepath.Join(moduleDir, "gox.mod"), "xgo 1.8\n", 0o600)
	mustWriteDriverTestFile(t, cacheGoMod, "module github.com/example/not-spx\n", 0o600)
	args = replaceOption(args, "selected-dir", moduleDir)
	args = replaceOption(args, "selected-gomod", cacheGoMod)
	args = replaceOption(args, "declaration-file", filepath.Join(moduleDir, "gox.mod"))
	if _, err := Parse(append(args, "--")); err == nil || !strings.Contains(err.Error(), "module-cache") {
		t.Fatalf("Parse() split module-cache mismatch error = %v", err)
	}
}

func TestParseRunRejectsMissingPackDirectory(t *testing.T) {
	args := validArgs(t, ActionRun)
	packDir := filepath.Join(optionValue(args, "project-dir"), filepath.FromSlash(optionValue(args, "pack-dir")))
	if err := os.RemoveAll(packDir); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(append(args, "--")); err == nil {
		t.Fatal("Parse() accepted project without a materialized pack directory")
	}
}

func TestParseRejectsMissingGraphInput(t *testing.T) {
	for _, name := range []string{"modfile"} {
		t.Run(name, func(t *testing.T) {
			args := validArgs(t, ActionRun)
			missing := filepath.Join(filepath.Dir(optionValue(args, "project-dir")), "missing-"+name)
			args = append(args, "--graph-flag=-"+name+"="+missing, "--")
			if _, err := Parse(args); err == nil || !strings.Contains(err.Error(), "cannot be inspected") {
				t.Fatalf("Parse() missing %s error = %v", name, err)
			}
		})
	}
}

func TestParseRejectsOverlayBeforeExecution(t *testing.T) {
	args := validArgs(t, ActionRun)
	projectDir := optionValue(args, "project-dir")
	root := filepath.Dir(projectDir)
	overlay := filepath.Join(root, "overlay.json")
	mustWriteDriverTestFile(t, overlay, "{}\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(projectDir, ".config"), `{"extasset":"../shared"}`, 0o600)
	if err := os.RemoveAll(filepath.Join(projectDir, filepath.FromSlash(optionValue(args, "pack-dir")))); err != nil {
		t.Fatal(err)
	}
	args = append(args, "--graph-flag=-overlay="+overlay, "--")
	if _, err := Parse(args); err == nil || !strings.Contains(err.Error(), "does not support -overlay") {
		t.Fatalf("Parse() overlay error = %v, want explicit unsupported error", err)
	}
}

func TestParseRejectsExtAssetOnlyForPortableDriver(t *testing.T) {
	args := validArgs(t, ActionRun)
	projectDir := optionValue(args, "project-dir")
	mustWriteDriverTestFile(t, filepath.Join(projectDir, ".config"), `{"extasset":"custom_asset"}`, 0o600)
	if _, err := Parse(append(args, "--")); err == nil || !strings.Contains(err.Error(), "unsupported extasset") {
		t.Fatalf("Parse() extasset error = %v, want portable-policy rejection", err)
	}
}

func TestParseRunAcceptsAbsentPackGroup(t *testing.T) {
	args := removeOptions(append(validArgs(t, ActionRun), "--"), "pack-dir", "pack-index")
	if _, err := Parse(args); err != nil {
		t.Fatalf("Parse() absent pack group error: %v", err)
	}
}

func TestParseRunAcceptsPackedOnlyProject(t *testing.T) {
	args := validArgs(t, ActionRun)
	projectDir := optionValue(args, "project-dir")
	packDir := filepath.Join(projectDir, filepath.FromSlash(optionValue(args, "pack-dir")))
	if err := os.Remove(filepath.Join(packDir, optionValue(args, "pack-index"))); err != nil {
		t.Fatal(err)
	}
	mustWriteDriverTestFile(t, filepath.Join(packDir, "index_pack.json"), `{"zorder":[]}`, 0o600)
	if _, err := Parse(append(args, "--")); err != nil {
		t.Fatalf("Parse() packed-only error: %v", err)
	}
}

func TestParseRejectsInvalidArgv(t *testing.T) {
	tests := []struct {
		name string
		args func(*testing.T) []string
		want string
	}{
		{"missing protocol", func(*testing.T) []string { return nil }, "requires preamble and action"},
		{"legacy runtime preamble", func(t *testing.T) []string {
			args := validArgs(t, ActionRun)
			args[0] = "xgo-runtime-v1"
			return append(args, "--")
		}, "unsupported preamble"},
		{"unknown action", func(t *testing.T) []string {
			args := validArgs(t, ActionRun)
			args[1] = "test"
			return append(args, "--")
		}, "unsupported action"},
		{"run delimiter", func(t *testing.T) []string { return validArgs(t, ActionRun) }, "requires --"},
		{"build delimiter", func(t *testing.T) []string { return append(validArgs(t, ActionBuild), "--") }, "does not accept --"},
		{"positional option", func(t *testing.T) []string { return append(validArgs(t, ActionRun), "positional", "--") }, "positional argument"},
		{"missing equals", func(t *testing.T) []string { return append(validArgs(t, ActionRun), "--graph-flag", "--") }, "--name=value"},
		{"unknown option", func(t *testing.T) []string { return append(validArgs(t, ActionRun), "--mystery=value", "--") }, "unknown option --mystery"},
		{"duplicate option", func(t *testing.T) []string { return append(validArgs(t, ActionRun), "--project-dir=/other", "--") }, "may not be repeated"},
		{"partial pack group (directory only)", func(t *testing.T) []string {
			return append(removeOptions(validArgs(t, ActionRun), "pack-index"), "--")
		}, "pack options must be supplied as a complete group"},
		{"partial pack group (index only)", func(t *testing.T) []string {
			return append(removeOptions(validArgs(t, ActionRun), "pack-dir"), "--")
		}, "pack options must be supplied as a complete group"},
		{"empty pack group", func(t *testing.T) []string {
			args := replaceOption(validArgs(t, ActionRun), "pack-dir", "")
			args = replaceOption(args, "pack-index", "")
			return append(args, "--")
		}, "pack directory"},
		{"incomplete local replacement", func(t *testing.T) []string {
			args := removeOptions(validArgs(t, ActionRun), "selected-dir", "selected-gomod")
			root := filepath.Dir(optionValue(args, "project-dir"))
			return append(args, "--replace-path="+filepath.Join(root, "local"), "--")
		}, "complete group"},
		{"replacement with selected source", func(t *testing.T) []string {
			args := validArgs(t, ActionRun)
			root := filepath.Dir(optionValue(args, "project-dir"))
			return append(args, "--replace-path=local", "--replace-version=", "--replace-dir="+filepath.Join(root, "local"), "--replace-gomod="+filepath.Join(root, "local", "go.mod"), "--")
		}, "with replacement forbids"},
		{"run output", func(t *testing.T) []string { return append(validArgs(t, ActionRun), "--output=/tmp/out", "--") }, "cannot contain output paths"},
		{"build missing output", func(t *testing.T) []string { return validArgs(t, ActionBuild) }, "requires output paths"},
		{"relative project", func(t *testing.T) []string {
			return append(replaceOption(validArgs(t, ActionRun), "project-dir", "relative"), "--")
		}, "must be absolute"},
		{"dirty project", func(t *testing.T) []string {
			args := validArgs(t, ActionRun)
			return append(replaceOption(args, "project-dir", optionValue(args, "project-dir")+string(filepath.Separator)+".."), "--")
		}, "must be clean"},
		{"symlinked project root", func(t *testing.T) []string {
			args := validArgs(t, ActionRun)
			root := filepath.Dir(optionValue(args, "project-dir"))
			realProject := optionValue(args, "project-dir")
			alias := filepath.Join(root, "project-alias")
			if err := os.Symlink(realProject, alias); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			args = replaceOption(args, "project-dir", alias)
			args = replaceOption(args, "project-file", filepath.Join(alias, "main.spx"))
			return append(args, "--")
		}, "must not be a symlink"},
		{"escaping pack", func(t *testing.T) []string {
			return append(replaceOption(validArgs(t, ActionRun), "pack-dir", "../assets"), "--")
		}, "pack directory escapes"},
		{"bad index", func(t *testing.T) []string {
			return append(replaceOption(validArgs(t, ActionRun), "pack-index", "dir/index.json"), "--")
		}, "pack index must be"},
		{"missing all pack indexes", func(t *testing.T) []string {
			args := validArgs(t, ActionRun)
			indexPath := filepath.Join(optionValue(args, "project-dir"), filepath.FromSlash(optionValue(args, "pack-dir")), optionValue(args, "pack-index"))
			if err := os.Remove(indexPath); err != nil {
				t.Fatal(err)
			}
			return append(args, "--")
		}, "index_pack.json"},
		{"bad digest", func(t *testing.T) []string {
			return append(replaceOption(validArgs(t, ActionRun), "declaration-sha256", "nope"), "--")
		}, "64 hexadecimal"},
		{"bad forwarded flag", func(t *testing.T) []string { return append(validArgs(t, ActionRun), "--graph-flag=mod=readonly", "--") }, "must use -name=value"},
		{"noncanonical graph flag", func(t *testing.T) []string { return append(validArgs(t, ActionRun), "--graph-flag=-mod", "--") }, "must use -name=value"},
		{"unsupported graph flag", func(t *testing.T) []string {
			return append(validArgs(t, ActionRun), "--graph-flag=-toolexec=/tmp/tool", "--")
		}, "-toolexec is not supported"},
		{"unsupported mod value", func(t *testing.T) []string { return append(validArgs(t, ActionRun), "--graph-flag=-mod=evil", "--") }, "unsupported value"},
		{"relative modfile", func(t *testing.T) []string {
			return append(validArgs(t, ActionRun), "--graph-flag=-modfile=alternate.mod", "--")
		}, "must be absolute"},
		{"missing graph work directory", func(t *testing.T) []string {
			args := validArgs(t, ActionRun)
			missing := filepath.Join(filepath.Dir(optionValue(args, "project-dir")), "missing-graph-work-dir")
			return append(replaceOption(args, "graph-work-dir", missing), "--")
		}, "cannot be inspected"},
		{"duplicate graph flag", func(t *testing.T) []string {
			return append(validArgs(t, ActionRun), "--graph-flag=-mod=readonly", "--graph-flag=-mod=mod", "--")
		}, "may not be repeated"},
		{"unsupported build flag", func(t *testing.T) []string { return append(validArgs(t, ActionRun), "--build-flag=-ldflags=-s", "--") }, "-ldflags is not supported"},
		{"unsafe buildvcs", func(t *testing.T) []string {
			return append(validArgs(t, ActionRun), "--build-flag=-buildvcs=true", "--")
		}, "unsupported value"},
		{"duplicate build flag", func(t *testing.T) []string {
			return append(validArgs(t, ActionRun), "--build-flag=-x=true", "--build-flag=-x=true", "--")
		}, "may not be repeated"},
		{"noncanonical bool", func(t *testing.T) []string {
			return append(replaceOption(validArgs(t, ActionRun), "origin-main", "1"), "--")
		}, "expected true or false"},
		{"invalid selected module path", func(t *testing.T) []string {
			return append(replaceOption(validArgs(t, ActionRun), "selected-path", "github.com/goplus/spx/v3@latest"), "--")
		}, "invalid module path"},
		{"invalid driver import path", func(t *testing.T) []string {
			return append(replaceOption(validArgs(t, ActionRun), "driver-package", "github.com/goplus/spx/v3/cmd/xgodriver@latest"), "--")
		}, "invalid driver package"},
		{"same build outputs", func(t *testing.T) []string {
			args := validArgs(t, ActionBuild)
			root := filepath.Dir(optionValue(args, "project-dir"))
			output := filepath.Join(root, "out", "game")
			return append(args, "--output="+output, "--final-output="+output)
		}, "must be different"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.args(t))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Parse() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func validArgs(t *testing.T, action Action) []string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(root, "project")
	selectedDir := filepath.Join(root, "spx")
	mustWriteDriverTestFile(t, filepath.Join(projectDir, "main.spx"), "onStart => {}\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{}\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(root, "go.mod"), "module example.com/project\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(selectedDir, "go.mod"), "module github.com/goplus/spx/v3\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(selectedDir, "gox.mod"), "xgo 1.8\n", 0o600)
	goCommand := filepath.Join(root, "bin", "go")
	mustWriteDriverTestFile(t, goCommand, "#!/bin/sh\n", 0o700)
	args := []string{
		ProtocolV1,
		string(action),
		"--project-dir=" + projectDir,
		"--project-file=" + filepath.Join(projectDir, "main.spx"),
		"--module-root=" + root,
		"--driver-package=github.com/goplus/spx/v3/cmd/xgodriver",
		"--selected-path=github.com/goplus/spx/v3",
		"--selected-version=v3.2.0",
		"--origin-main=false",
		"--selected-dir=" + selectedDir,
		"--selected-gomod=" + filepath.Join(selectedDir, "go.mod"),
		"--project-ext=.spx",
		"--project-full-ext=main.spx",
		"--pack-dir=assets",
		"--pack-index=index.json",
		"--declaration-file=" + filepath.Join(selectedDir, "gox.mod"),
		"--declaration-sha256=" + strings.Repeat("a", 64),
		"--target-modfile=" + filepath.Join(root, "go.mod"),
		"--target-modfile-sha256=" + strings.Repeat("b", 64),
		"--go-command=" + goCommand,
		"--graph-work-dir=" + root,
		"--go-work=off",
	}
	return args
}

func mustWriteDriverTestFile(t *testing.T, name, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(name, mode); err != nil {
		t.Fatal(err)
	}
}

func removeOptions(args []string, names ...string) []string {
	remove := make(map[string]bool, len(names))
	for _, name := range names {
		remove[name] = true
	}
	result := make([]string, 0, len(args))
	for _, arg := range args {
		name, _, ok := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if strings.HasPrefix(arg, "--") && ok && remove[name] {
			continue
		}
		result = append(result, arg)
	}
	return result
}

func replaceOption(args []string, name, value string) []string {
	result := append([]string(nil), args...)
	prefix := "--" + name + "="
	for i, arg := range result {
		if strings.HasPrefix(arg, prefix) {
			result[i] = prefix + value
			return result
		}
	}
	return append(result, prefix+value)
}

func optionValue(args []string, name string) string {
	prefix := "--" + name + "="
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}
