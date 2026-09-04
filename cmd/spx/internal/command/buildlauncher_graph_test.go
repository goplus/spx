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
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/envutil"
)

func TestResolveSPXSourceModes(t *testing.T) {
	root := t.TempDir()
	writeLauncherTestFile(t, filepath.Join(root, "cmd", "ispxnative", "main.go"), "package main\n")
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
				Path: spxModulePath, Version: "v3.1.0", Dir: root,
			},
		},
		{
			name: "local replacement",
			module: listedModule{
				Path: spxModulePath, Version: "v3.1.0", Dir: root,
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
			expectedRoot := ""
			if test.mode {
				expectedRoot = wantRoot
			}
			if gotRoot != expectedRoot || selected != "v3.1.0" || mode != test.mode {
				t.Fatalf("source = root %q, selected %q, mode %v", gotRoot, selected, mode)
			}
		})
	}
}

func TestResolveSPXSourceRejectsUnsupportedPublishedIdentities(t *testing.T) {
	for _, module := range []listedModule{
		{Path: spxModulePath, Version: "v3.2.5-0.20260821120000-0123456789ab"},
		{
			Path: spxModulePath, Version: "v3.2.4",
			Replace: &listedModule{Path: spxModulePath, Version: "v3.2.3"},
		},
	} {
		if _, _, _, _, err := resolveSPXSource(module); err == nil {
			t.Fatalf("resolveSPXSource accepted %#v", module)
		}
	}
}

func TestResolveSPXSourceWithoutModuleDirectory(t *testing.T) {
	root, selected, effective, sourceMode, err := resolveSPXSource(listedModule{
		Path: spxModulePath, Version: "v3.1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if root != "" || selected != "v3.1.0" || effective != "v3.1.0" || sourceMode {
		t.Fatalf("source = root %q, selected %q, effective %q, mode %v", root, selected, effective, sourceMode)
	}
}

func TestGraphFlagsFromEnvQuotedModfile(t *testing.T) {
	workDir := t.TempDir()
	modDir := filepath.Join(workDir, "module with spaces")
	modfile := filepath.Join(modDir, "go.mod")
	writeLauncherTestFile(t, modfile, "module example.com/game\n\ngo 1.25.0\n")
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

func TestEffectiveGraphFlagsReadsPersistedGOFLAGS(t *testing.T) {
	root := t.TempDir()
	writeLauncherTestFile(t, filepath.Join(root, "go.mod"), "module example.test/base\n\ngo 1.25.0\n")
	alternate := filepath.Join(root, "alternate.mod")
	writeLauncherTestFile(t, alternate, "module example.test/alternate\n\ngo 1.25.0\n")
	canonicalAlternate, err := canonicalFile(alternate)
	if err != nil {
		t.Fatal(err)
	}
	goEnv := filepath.Join(t.TempDir(), "go.env")
	writeLauncherTestFile(t, goEnv, "GOFLAGS=-modfile=alternate.mod\n")
	goCommand, err := resolveGoCommand()
	if err != nil {
		t.Fatal(err)
	}
	base := envutil.SetMany(os.Environ(),
		envutil.Assignment{Key: "GOENV", Value: goEnv},
		envutil.Assignment{Key: "GOWORK", Value: "off"},
	)
	for _, state := range []string{"empty", "unset"} {
		t.Run(state, func(t *testing.T) {
			env := envutil.SetMany(base, envutil.Assignment{Key: "GOFLAGS"})
			if state == "unset" {
				env = envutil.Without(env, "GOFLAGS")
			}
			flags, err := effectiveGraphFlags(context.Background(), goCommand, root, env)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"-modfile=" + canonicalAlternate}
			if !reflect.DeepEqual(flags, want) {
				t.Fatalf("effective graph flags = %#v, want %#v", flags, want)
			}
			args := append([]string{"list", "-m", "-f={{.Path}}"}, flags...)
			command := exec.Command(goCommand, args...)
			command.Dir = root
			command.Env = launcherGraphEnvironment(env, "off")
			output, err := command.Output()
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(string(output)); got != "example.test/alternate" {
				t.Fatalf("selected module = %q, want example.test/alternate", got)
			}
		})
	}
}

func TestEffectiveGraphFlagsPrefersExplicitGOFLAGS(t *testing.T) {
	root := t.TempDir()
	persisted := filepath.Join(root, "persisted.mod")
	explicit := filepath.Join(root, "explicit.mod")
	writeLauncherTestFile(t, persisted, "module example.test/persisted\n\ngo 1.25.0\n")
	writeLauncherTestFile(t, explicit, "module example.test/explicit\n\ngo 1.25.0\n")
	canonicalExplicit, err := canonicalFile(explicit)
	if err != nil {
		t.Fatal(err)
	}
	goEnv := filepath.Join(t.TempDir(), "go.env")
	writeLauncherTestFile(t, goEnv, "GOFLAGS=-modfile="+persisted+"\n")
	goCommand, err := resolveGoCommand()
	if err != nil {
		t.Fatal(err)
	}
	env := envutil.SetMany(os.Environ(),
		envutil.Assignment{Key: "GOENV", Value: goEnv},
		envutil.Assignment{Key: "GOFLAGS", Value: "-modfile=" + explicit},
	)
	flags, err := effectiveGraphFlags(context.Background(), goCommand, root, env)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-modfile=" + canonicalExplicit}
	if !reflect.DeepEqual(flags, want) {
		t.Fatalf("effective graph flags = %#v, want %#v", flags, want)
	}
}

func TestEffectiveGraphFlagsRejectsDuplicateGOFLAGS(t *testing.T) {
	_, err := effectiveGraphFlags(context.Background(), "go", t.TempDir(), []string{"GOFLAGS=-mod=mod", "GOFLAGS=-mod=readonly"})
	if err == nil || !strings.Contains(err.Error(), "GOFLAGS is repeated") {
		t.Fatalf("effectiveGraphFlags error = %v, want duplicate error", err)
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

func TestSplitGOFLAGSMatchesGoQuotedFields(t *testing.T) {
	got, err := splitGOFLAGS(`-trimpath ' -modfile=module with spaces ' "-buildvcs=false"`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-trimpath", " -modfile=module with spaces ", "-buildvcs=false"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitGOFLAGS = %#v, want %#v", got, want)
	}
	if _, err := splitGOFLAGS(`'-trimpath`); err == nil {
		t.Fatal("splitGOFLAGS accepted an unterminated quote")
	}
}

func TestLauncherGraphEnvironmentUsesNeutralGOFLAGS(t *testing.T) {
	env := launcherGraphEnvironment([]string{"PATH=/bin", "GOFLAGS=-mod=vendor"}, "off")
	value, found, duplicate := envutil.Lookup(env, "GOFLAGS")
	if value != envutil.NeutralGOFLAGS || !found || duplicate {
		t.Fatalf("launcher GOFLAGS = %q, %t, %t", value, found, duplicate)
	}
}

func TestLauncherGraphVerifier(t *testing.T) {
	root := t.TempDir()
	goMod := filepath.Join(root, "go.mod")
	metadata := filepath.Join(root, "gox.mod")
	dependency := filepath.Join(t.TempDir(), "dependency")
	writeLauncherTestFile(t, filepath.Join(dependency, "go.mod"), "module example.com/dependency\n\ngo 1.25.0\n")
	moduleText := func(version string) string {
		return "module example.com/game\n\ngo 1.25.0\n\nrequire (\n\tgithub.com/goplus/spx/v3 v3.0.0\n\texample.com/dependency " + version + "\n)\nreplace github.com/goplus/spx/v3 => " + filepath.ToSlash(findSPXRoot(t)) + "\nreplace example.com/dependency => " + filepath.ToSlash(dependency) + "\n"
	}
	writeLauncherTestFile(t, goMod, moduleText("v1.0.0"))
	writeLauncherTestFile(t, metadata, "xgo 1.8.0\n")
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
	writeLauncherTestFile(t, filepath.Join(root, "go.sum"), "")
	if err := verifier.verify(context.Background()); err != nil {
		t.Fatalf("go.sum changed graph identity: %v", err)
	}
	writeLauncherTestFile(t, goMod, moduleText("v1.1.0"))
	if err := verifier.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "module selection changed") {
		t.Fatalf("module selection change error = %v", err)
	}
	writeLauncherTestFile(t, goMod, moduleText("v1.0.0"))
	writeLauncherTestFile(t, metadata, "xgo 1.9.0\n")
	if err := verifier.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "module input changed") {
		t.Fatalf("metadata change error = %v", err)
	}
}

func TestLauncherGraphVerifierVendorMetadata(t *testing.T) {
	root := t.TempDir()
	spxRoot := filepath.Join(root, "spx")
	moduleRoot := filepath.Join(root, "game")
	writeLauncherTestFile(t, filepath.Join(spxRoot, "go.mod"), "module "+spxModulePath+"\n\ngo 1.25.0\n")
	writeLauncherTestFile(t, filepath.Join(spxRoot, "cmd", "ispxnative", "main.go"), "package ispxnative\n")
	goMod := filepath.Join(moduleRoot, "go.mod")
	writeLauncherTestFile(t, goMod, "module example.com/game\n\ngo 1.25.0\n\nrequire "+spxModulePath+" v3.0.0\nreplace "+spxModulePath+" => "+filepath.ToSlash(spxRoot)+"\n")
	writeLauncherTestFile(t, filepath.Join(moduleRoot, "main.go"), "package main\nimport _ \""+spxModulePath+"/cmd/ispxnative\"\nfunc main() {}\n")
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
