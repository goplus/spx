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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/mod/xgomod"
)

func TestGraphSelectionDistinguishesSelectedAndReplacementModules(t *testing.T) {
	root := t.TempDir()
	mainDir := filepath.Join(root, "main")
	localDir := filepath.Join(root, "local")
	cacheDir := filepath.Join(root, "cache")
	for _, test := range []struct {
		name   string
		module listedModule
		want   graphModuleSelection
	}{
		{
			name: "main module",
			module: listedModule{
				Path: "example.test/main", Main: true,
				Dir: mainDir, GoMod: filepath.Join(mainDir, "go.mod"),
			},
			want: graphModuleSelection{
				Path: "example.test/main", Main: true,
				Dir: mainDir, GoMod: filepath.Join(mainDir, "go.mod"),
			},
		},
		{
			name: "local replacement",
			module: listedModule{
				Path: "example.test/dependency", Version: "v1.0.0",
				Dir: cacheDir, GoMod: filepath.Join(cacheDir, "go.mod"),
				Replace: &listedModule{
					Path: "../local", Dir: localDir, GoMod: filepath.Join(localDir, "go.mod"),
				},
			},
			want: graphModuleSelection{
				Path: "example.test/dependency", Version: "v1.0.0",
				Replace: &graphModuleSelection{
					Path: localDir, Dir: localDir, GoMod: filepath.Join(localDir, "go.mod"),
				},
			},
		},
		{
			name: "versioned replacement",
			module: listedModule{
				Path: "example.test/dependency", Version: "v1.0.0",
				Replace: &listedModule{
					Path: "example.test/fork", Version: "v1.1.0",
					Dir: cacheDir, GoMod: filepath.Join(cacheDir, "go.mod"),
				},
			},
			want: graphModuleSelection{
				Path: "example.test/dependency", Version: "v1.0.0",
				Replace: &graphModuleSelection{Path: "example.test/fork", Version: "v1.1.0"},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := graphSelectionForSelectedModule(&test.module); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("graph selection = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSummarizeModuleGraphIsOrderIndependentAndCollectsLocalFiles(t *testing.T) {
	root := t.TempDir()
	mainGoMod := filepath.Join(root, "main", "go.mod")
	localGoMod := filepath.Join(root, "local", "go.mod")
	modules := []listedModule{
		{
			Path: "example.test/main", Main: true, Dir: filepath.Dir(mainGoMod), GoMod: mainGoMod,
			Replace: &listedModule{Path: "../local", Dir: filepath.Dir(localGoMod), GoMod: localGoMod},
		},
		{
			Path: "example.test/remote", Version: "v1.0.0",
			Replace: &listedModule{
				Path: "example.test/fork", Version: "v1.1.0",
				Dir: filepath.Join(root, "cache"), GoMod: filepath.Join(root, "cache", "go.mod"),
			},
		},
	}
	first, err := summarizeModuleGraph(modules)
	if err != nil {
		t.Fatal(err)
	}
	reversed := []listedModule{modules[1], modules[0]}
	second, err := summarizeModuleGraph(reversed)
	if err != nil {
		t.Fatal(err)
	}
	if first.selectionDigest != second.selectionDigest {
		t.Fatalf("module selection depends on go list order: %q != %q", first.selectionDigest, second.selectionDigest)
	}
	want := map[string]bool{mainGoMod: true, localGoMod: true}
	if len(first.localFiles) != len(want) {
		t.Fatalf("local graph files = %#v, want %v", first.localFiles, want)
	}
	for _, file := range first.localFiles {
		if !want[file.path] || !file.required {
			t.Errorf("unexpected local graph file: %#v", file)
		}
		delete(want, file.path)
	}
	if len(want) != 0 {
		t.Fatalf("missing local graph files: %v", want)
	}
}

func TestGraphPathSetCollectsSidecarsAndRequiredPaths(t *testing.T) {
	root := t.TempDir()
	mainMod := filepath.Join(root, "main", "go.mod")
	mainSum := filepath.Join(root, "main", "go.sum")
	altMod := filepath.Join(root, "alternate.mod")
	altSum := filepath.Join(root, "alternate.sum")
	work := filepath.Join(root, "driver.workspace")
	paths := make(graphPathSet)
	paths.addMod(mainMod, true)
	paths.addMod(mainMod, false)
	paths.addMod(altMod, true)
	paths.add(altSum, true)
	paths.addWork(work)

	want := map[string]bool{
		mainMod: true, mainSum: false,
		altMod: true, altSum: true,
		work: true, work + ".sum": false,
	}
	got := paths.sorted()
	if len(got) != len(want) {
		t.Fatalf("graph paths = %#v, want %v", got, want)
	}
	for i, item := range got {
		if i > 0 && got[i-1].path >= item.path {
			t.Fatalf("graph paths are not strictly sorted: %#v", got)
		}
		required, ok := want[item.path]
		if !ok || item.required != required {
			t.Errorf("graph path = %#v, want required=%t", item, required)
		}
		delete(want, item.path)
	}
	if len(want) != 0 {
		t.Fatalf("missing graph paths: %v", want)
	}
}

func TestSnapshotGraphFilesPinsExplicitModfile(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.mod")
	mustWriteDriverTestFile(t, first, "module example.test/first\n", 0o600)
	files, err := snapshotGraphFiles(context.Background(), Config{
		GraphFlags: []string{"-modfile=" + first},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{first: true}
	for _, file := range files {
		if file.present {
			delete(want, file.path)
		}
	}
	if len(want) != 0 {
		t.Fatalf("explicit modfile was not pinned: %v; files: %#v", want, files)
	}
}

func TestSnapshotGraphFilesPinsTargetAndSkipsPrivateModfile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target", "go.mod")
	targetSum := filepath.Join(root, "target", "go.sum")
	active := filepath.Join(root, "active", "go.mod")
	activeSum := filepath.Join(root, "active", "go.sum")
	private := filepath.Join(root, "private", "graph.mod")
	privateSum := filepath.Join(root, "private", "graph.sum")
	targetData := "module example.test/target\n"
	for path, content := range map[string]string{
		target: targetData, targetSum: "", active: "module example.test/active\n", activeSum: "",
		private: "module example.test/private\n", privateSum: "",
	} {
		mustWriteDriverTestFile(t, path, content, 0o600)
	}
	digest := sha256.Sum256([]byte(targetData))
	cfg := Config{
		TargetModFile: xgomod.FileIdentity{Path: target, SHA256: hex.EncodeToString(digest[:])},
		GraphFlags:    []string{"-modfile=" + active},
		commandGraph:  &goCommandGraph{privateModFile: private},
	}
	moduleFiles := []graphPath{{path: private, required: true}}
	files, err := snapshotGraphFiles(context.Background(), cfg, nil, moduleFiles)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool, len(files))
	for _, file := range files {
		got[file.path] = file.present
	}
	if !got[target] || !got[active] || !got[activeSum] {
		t.Fatalf("target and active module files are not pinned: %#v", got)
	}
	if _, ok := got[targetSum]; ok {
		t.Fatalf("unclaimed target sum is part of graph snapshot: %#v", got)
	}
	if _, ok := got[private]; ok {
		t.Fatalf("private modfile is part of caller graph snapshot: %#v", got)
	}
	if _, ok := got[privateSum]; ok {
		t.Fatalf("private sum is part of caller graph snapshot: %#v", got)
	}
	mustWriteDriverTestFile(t, target, "module example.test/changed\n", 0o600)
	if _, err := snapshotGraphFiles(context.Background(), cfg, nil, moduleFiles); err == nil || !strings.Contains(err.Error(), "changed after XGo resolution") {
		t.Fatalf("changed target modfile error = %v", err)
	}
}

func TestReadGraphFilePinsOpenedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	replacement := filepath.Join(root, "replacement.mod")
	mustWriteDriverTestFile(t, path, "module example.test/original\n", 0o600)
	mustWriteDriverTestFile(t, replacement, "module example.test/replaced\n", 0o600)

	lstatCalls := 0
	access := graphFileAccess{
		lstat: func(name string) (os.FileInfo, error) {
			lstatCalls++
			if lstatCalls > 1 {
				name = replacement
			}
			return os.Lstat(name)
		},
		open: os.Open,
	}
	if _, err := readGraphFile(graphPath{path: path, required: true}, access); err == nil || !strings.Contains(err.Error(), "changed while reading") {
		t.Fatalf("readGraphFile() replacement error = %v", err)
	}
}

func TestReadGraphFileRejectsOversizedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.sum")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxGraphFileBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readGraphFiles([]graphPath{{path: path, required: true}}); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("readGraphFiles() oversized error = %v", err)
	}
}

func TestSnapshotGraphFilesIncludesCustomWorkspaceSum(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "alternate.mod")
	workPath := filepath.Join(root, "driver.workspace")
	sumPath := workPath + ".sum"
	mustWriteDriverTestFile(t, modPath, "module example.test/app\n\ngo 1.25.0\n", 0o600)
	mustWriteDriverTestFile(t, workPath, "go 1.25.0\n\nuse .\n", 0o600)
	mustWriteDriverTestFile(t, sumPath, "example.test/dependency v1.0.0/go.mod h1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=\n", 0o600)
	cfg := Config{
		GraphWorkDir: root, GoWork: workPath,
		GraphFlags: []string{"-modfile=" + modPath},
	}
	files, err := snapshotGraphFiles(context.Background(), cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.path == sumPath && file.present {
			return
		}
	}
	t.Fatalf("custom workspace sum %q is missing from graph snapshot: %#v", sumPath, files)
}

func TestGraphVerifierDetectsModuleSelectionAndGoModDrift(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	root := t.TempDir()
	dependency := filepath.Join(root, "dependency")
	mustWriteDriverTestFile(t, filepath.Join(dependency, "go.mod"), "module example.test/dependency\n\ngo 1.25\n", 0o600)
	writeModule := func(version string) {
		mustWriteDriverTestFile(t, filepath.Join(root, "go.mod"), "module example.test/app\n\ngo 1.25\n\nrequire example.test/dependency "+version+"\n\nreplace example.test/dependency => ./dependency\n", 0o600)
	}
	writeModule("v1.0.0")
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GoCommand: goCommand, GraphWorkDir: root, GoWork: "off", GraphFlags: []string{"-mod=mod"}}
	verifier, err := newGraphVerifier(context.Background(), cfg, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	if err := verifier.verify(context.Background()); err != nil {
		t.Fatalf("initial graph callback: %v", err)
	}

	writeModule("v1.1.0")
	if err := verifier.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "module selection changed") {
		t.Fatalf("module selection drift error = %v", err)
	}

	writeModule("v1.0.0")
	if err := os.WriteFile(filepath.Join(root, "go.sum"), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifier.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "graph input changed") {
		t.Fatalf("go.mod/go.sum drift error = %v", err)
	}
}

func TestGraphVerifierCallbackWiring(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	root := t.TempDir()
	mustWriteDriverTestFile(t, filepath.Join(root, "go.mod"), "module example.test/app\n\ngo 1.25\n", 0o600)
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GoCommand: goCommand, GraphWorkDir: root, GoWork: "off", GraphFlags: []string{"-mod=readonly"}}
	verifier, err := newGraphVerifier(context.Background(), cfg, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	callback := verifier.verify
	if callback == nil {
		t.Fatal("graph callback is nil")
	}
	if err := callback(context.Background()); err != nil {
		t.Fatalf("wired graph callback: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/changed\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := callback(context.Background()); err == nil {
		t.Fatal("wired graph callback accepted changed module")
	}
}

func TestGraphVerifierDetectsWorkspaceAndLocalReplacementDrift(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		target string
	}{
		{name: "workspace module", target: "workspace"},
		{name: "local replacement", target: "dependency"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			app := filepath.Join(root, "app")
			workspace := filepath.Join(root, "workspace")
			dependency := filepath.Join(root, "dependency")
			mustWriteDriverTestFile(t, filepath.Join(root, "go.work"), "go 1.25.0\n\nuse (\n\t./app\n\t./workspace\n)\n", 0o600)
			mustWriteDriverTestFile(t, filepath.Join(app, "go.mod"), "module example.test/app\n\ngo 1.25.0\n\nrequire example.test/dependency v0.0.0\n\nreplace example.test/dependency => ../dependency\n", 0o600)
			mustWriteDriverTestFile(t, filepath.Join(workspace, "go.mod"), "module example.test/workspace\n\ngo 1.25.0\n", 0o600)
			mustWriteDriverTestFile(t, filepath.Join(dependency, "go.mod"), "module example.test/dependency\n\ngo 1.25.0\n", 0o600)

			cfg := Config{GoCommand: goCommand, GraphWorkDir: app, GoWork: filepath.Join(root, "go.work"), GraphFlags: []string{"-mod=readonly"}}
			verifier, err := newGraphVerifier(context.Background(), cfg, os.Environ())
			if err != nil {
				t.Fatal(err)
			}
			target := workspace
			modulePath := "example.test/workspace"
			if test.target == "dependency" {
				target = dependency
				modulePath = "example.test/dependency"
			}
			mustWriteDriverTestFile(t, filepath.Join(target, "go.mod"), "module "+modulePath+"\n\ngo 1.24.0\n", 0o600)
			if err := verifier.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "graph input changed") {
				t.Fatalf("graph metadata drift error = %v", err)
			}
		})
	}
}
