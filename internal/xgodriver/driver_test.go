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
	"debug/buildinfo"
	"os"
	"os/exec"
	"path/filepath"
	runtimedebug "runtime/debug"
	"testing"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/envutil"
)

func TestVerifyBuildInfoOrigin(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	localReplacement := filepath.Join(root, "framework")
	if err := os.MkdirAll(localReplacement, 0o700); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name            string
		info            *runtimedebug.BuildInfo
		want            ModuleOrigin
		replacementPath string
		role            moduleRole
		ok              bool
	}{
		{
			name: "workspace main",
			info: &runtimedebug.BuildInfo{Main: runtimedebug.Module{Path: "example.com/driver", Version: "v1.2.4-0.20260822024039-eb9e7bbaacff"}},
			want: ModuleOrigin{Selected: ModuleRef{Path: "example.com/driver"}, Main: true}, role: moduleMain, ok: true,
		},
		{
			name: "workspace main in generated launcher dependencies",
			info: &runtimedebug.BuildInfo{
				Main: runtimedebug.Module{Path: "example.com/app", Version: "(devel)"},
				Deps: []*runtimedebug.Module{{Path: "example.com/driver", Version: "(devel)"}},
			},
			want: ModuleOrigin{Selected: ModuleRef{Path: "example.com/driver"}, Main: true}, role: moduleDependency, ok: true,
		},
		{
			name: "workspace launcher rejects versioned dependency",
			info: &runtimedebug.BuildInfo{
				Main: runtimedebug.Module{Path: "example.com/app", Version: "(devel)"},
				Deps: []*runtimedebug.Module{{Path: "example.com/driver", Version: "v1.2.3"}},
			},
			want: ModuleOrigin{Selected: ModuleRef{Path: "example.com/driver"}, Main: true}, role: moduleDependency,
		},
		{
			name: "bridge rejects dependency role",
			info: &runtimedebug.BuildInfo{
				Main: runtimedebug.Module{Path: "example.com/app", Version: "(devel)"},
				Deps: []*runtimedebug.Module{{Path: "example.com/driver", Version: "(devel)"}},
			},
			want: ModuleOrigin{Selected: ModuleRef{Path: "example.com/driver"}, Main: true}, role: moduleMain,
		},
		{
			name: "launcher rejects main role",
			info: &runtimedebug.BuildInfo{Main: runtimedebug.Module{Path: "example.com/driver", Version: "(devel)"}},
			want: ModuleOrigin{Selected: ModuleRef{Path: "example.com/driver"}, Main: true}, role: moduleDependency,
		},
		{
			name: "versioned dependency replacement",
			info: &runtimedebug.BuildInfo{Deps: []*runtimedebug.Module{{
				Path: "example.com/driver", Version: "v1.2.3",
				Replace: &runtimedebug.Module{Path: "example.com/fork", Version: "v1.4.0"},
			}}},
			want: ModuleOrigin{
				Selected: ModuleRef{Path: "example.com/driver", Version: "v1.2.3"},
				Replace:  &ModuleRef{Path: "example.com/fork", Version: "v1.4.0"},
			}, role: moduleDependency, ok: true,
		},
		{
			name: "relative local dependency replacement",
			info: &runtimedebug.BuildInfo{Deps: []*runtimedebug.Module{{
				Path: "example.com/driver", Version: "v1.2.3",
				Replace: &runtimedebug.Module{Path: "../framework"},
			}}},
			want: ModuleOrigin{
				Selected: ModuleRef{Path: "example.com/driver", Version: "v1.2.3"},
				Replace:  &ModuleRef{Path: localReplacement, Dir: localReplacement},
			},
			replacementPath: "../framework", role: moduleDependency, ok: true,
		},
		{
			name: "relative local dependency replacement devel version",
			info: &runtimedebug.BuildInfo{Deps: []*runtimedebug.Module{{
				Path: "example.com/driver", Version: "v1.2.3",
				Replace: &runtimedebug.Module{Path: "../framework", Version: "(devel)"},
			}}},
			want: ModuleOrigin{
				Selected: ModuleRef{Path: "example.com/driver", Version: "v1.2.3"},
				Replace:  &ModuleRef{Path: localReplacement, Dir: localReplacement},
			},
			replacementPath: "../framework", role: moduleDependency, ok: true,
		},
		{
			name: "relative local replacement drift",
			info: &runtimedebug.BuildInfo{Deps: []*runtimedebug.Module{{
				Path: "example.com/driver", Version: "v1.2.3",
				Replace: &runtimedebug.Module{Path: "../framework"},
			}}},
			want: ModuleOrigin{
				Selected: ModuleRef{Path: "example.com/driver", Version: "v1.2.3"},
				Replace:  &ModuleRef{Path: localReplacement, Dir: localReplacement},
			},
			replacementPath: "./framework", role: moduleDependency,
		},
		{
			name: "selected version drift",
			info: &runtimedebug.BuildInfo{Main: runtimedebug.Module{Path: "example.com/driver", Version: "v1.2.4"}},
			want: ModuleOrigin{Selected: ModuleRef{Path: "example.com/driver", Version: "v1.2.3"}}, role: moduleMain,
		},
		{
			name: "replacement drift",
			info: &runtimedebug.BuildInfo{Main: runtimedebug.Module{
				Path: "example.com/driver", Version: "v1.2.3",
				Replace: &runtimedebug.Module{Path: "example.com/other", Version: "v1.4.0"},
			}},
			want: ModuleOrigin{
				Selected: ModuleRef{Path: "example.com/driver", Version: "v1.2.3"},
				Replace:  &ModuleRef{Path: "example.com/fork", Version: "v1.4.0"},
			}, role: moduleMain,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := verifyBuildInfoOrigin(test.info, test.want, test.replacementPath, test.role)
			if (err == nil) != test.ok {
				t.Fatalf("verifyBuildInfoOrigin() error = %v, want success %t", err, test.ok)
			}
		})
	}
}

func TestVerifyBuildInfoOriginWithRealLocalReplacement(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a local replacement fixture")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	framework := filepath.Join(root, "framework")
	mustWriteDriverTestFile(t, filepath.Join(framework, "go.mod"), "module example.test/framework\n\ngo 1.25\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(framework, "framework.go"), "package framework\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(app, "go.mod"), `module example.test/app

go 1.25

require example.test/framework v1.2.3

replace example.test/framework => ../framework
`, 0o600)
	mustWriteDriverTestFile(t, filepath.Join(app, "main.go"), "package main\nimport _ \"example.test/framework\"\nfunc main() {}\n", 0o600)

	artifact := filepath.Join(root, "app.bin")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", artifact, ".")
	build.Dir = app
	build.Env = append(sanitizeEnvironment(os.Environ()), "GOFLAGS="+envutil.NeutralGOFLAGS, "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build local replacement fixture: %v\n%s", err, output)
	}
	info, err := buildinfo.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	want := ModuleOrigin{
		Selected: ModuleRef{Path: "example.test/framework", Version: "v1.2.3"},
		Replace:  &ModuleRef{Path: framework, Dir: framework},
	}
	if err := verifyBuildInfoOrigin(info, want, "../framework", moduleDependency); err != nil {
		t.Fatalf("verify real local replacement: %v", err)
	}
}

func TestVerifyDriverPackageUsesGraphWorkDir(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	framework := filepath.Join(root, "framework")
	project := filepath.Join(framework, "example")
	mustWriteDriverTestFile(t, filepath.Join(app, "go.mod"), `module example.test/app

go 1.25

require example.test/framework v1.2.3 //xgo:class

replace example.test/framework => ../framework
`, 0o600)
	mustWriteDriverTestFile(t, filepath.Join(framework, "go.mod"), "module example.test/framework\n\ngo 1.25\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(framework, "cmd", "driver", "main.go"), "package main\nfunc main() {}\n", 0o600)
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		ProjectDir: project, GraphWorkDir: app, GoCommand: goCommand, GoWork: "off",
		DriverPackage: "example.test/framework/cmd/driver",
		DriverOrigin: ModuleOrigin{
			Selected: ModuleRef{Path: "example.test/framework", Version: "v1.2.3"},
			Replace:  &ModuleRef{Path: framework, Dir: framework, GoMod: filepath.Join(framework, "go.mod")},
		},
	}
	if err := verifyDriverPackage(context.Background(), cfg, os.Environ()); err != nil {
		t.Fatalf("driver validation with caller graph work dir: %v", err)
	}
	cfg.GraphWorkDir = project
	err = verifyDriverPackage(context.Background(), cfg, os.Environ())
	if err == nil {
		t.Fatal("driver validation unexpectedly preserved the caller graph from the dependency project directory")
	}
	if !IsRequestError(err) {
		t.Fatalf("driver provenance mismatch error = %v, want request error", err)
	}
}

func TestVerifyMainDriverPackageWithPrivateModfile(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	goMod := filepath.Join(root, "go.mod")
	mustWriteDriverTestFile(t, goMod, "module "+driverbundle.SPXModulePath+"\n\ngo 1.25\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(root, "cmd", "xgodriver", "main.go"), "package main\nfunc main() {}\n", 0o600)
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		GoCommand: goCommand, GraphWorkDir: root, GoWork: "off", GraphFlags: []string{"-mod=mod"},
		DriverPackage: driverbundle.SPXModulePath + "/cmd/xgodriver",
		DriverOrigin: ModuleOrigin{
			Selected: ModuleRef{Path: driverbundle.SPXModulePath, Dir: root, GoMod: goMod}, Main: true,
		},
	}
	isolated, cleanup, err := isolateGoGraph(context.Background(), cfg, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if err := verifyDriverPackage(context.Background(), isolated, os.Environ()); err != nil {
		t.Fatalf("verify main driver with private modfile: %v", err)
	}
}

func TestVerifyPackageOriginErrorClassification(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the host Go command")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mustWriteDriverTestFile(t, filepath.Join(root, "go.mod"), "module example.test/app\n\ngo 1.25.0\n", 0o600)
	command, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	command, err = filepath.EvalSymlinks(command)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GoCommand: command, GraphWorkDir: root, GoWork: "off"}

	err = verifyPackageOrigin(context.Background(), cfg, []string{"GOPROXY=off"}, "example.test/missing/driver", "driver")
	if err == nil || IsRequestError(err) {
		t.Fatalf("go list execution error = %v, want untyped execution failure", err)
	}

	cfg.GraphFlags = []string{"-mod=vendor"}
	err = verifyPackageOrigin(context.Background(), cfg, nil, "example.test/missing/driver", "driver")
	if err == nil || !IsRequestError(err) {
		t.Fatalf("vendor validation error = %v, want request error", err)
	}
}
