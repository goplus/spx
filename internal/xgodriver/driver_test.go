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
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	runtimedebug "runtime/debug"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/projectbundle"
	"github.com/goplus/spx/v3/internal/projectpolicy"
	"github.com/goplus/spx/v3/internal/runtimepayload"
)

func TestLocalEngineSourceDigestsPreserveIdentity(t *testing.T) {
	engine := []byte("engine-bytes")
	pack := []byte("pack-bytes")
	interfaceDigest, engineDigest, packDigest, err := localEngineSourceDigests(
		runtimepayload.FileSource{ReaderAt: bytes.NewReader(engine), Size: int64(len(engine))},
		runtimepayload.FileSource{ReaderAt: bytes.NewReader(pack), Size: int64(len(pack))},
	)
	if err != nil {
		t.Fatal(err)
	}
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("spx-local-engine-interface/v1\x00"))
	_, _ = hasher.Write(engine)
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(pack)
	engineSum := sha256.Sum256(engine)
	packSum := sha256.Sum256(pack)
	if want := hex.EncodeToString(hasher.Sum(nil)); interfaceDigest != want {
		t.Fatalf("interface digest = %s, want %s", interfaceDigest, want)
	}
	if want := hex.EncodeToString(engineSum[:]); engineDigest != want {
		t.Fatalf("Engine digest = %s, want %s", engineDigest, want)
	}
	if want := hex.EncodeToString(packSum[:]); packDigest != want {
		t.Fatalf("pack digest = %s, want %s", packDigest, want)
	}
}

func TestVerifyBuildInfoOrigin(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	localReplacement := filepath.Join(root, "framework")
	for _, dir := range []string{localReplacement} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name            string
		info            *runtimedebug.BuildInfo
		want            ModuleOrigin
		replacementPath string
		ok              bool
	}{
		{
			name: "workspace main",
			info: &runtimedebug.BuildInfo{Main: runtimedebug.Module{Path: "example.com/driver", Version: "(devel)"}},
			want: ModuleOrigin{Selected: ModuleRef{Path: "example.com/driver"}, Main: true}, ok: true,
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
			}, ok: true,
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
			replacementPath: "../framework", ok: true,
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
			replacementPath: "../framework", ok: true,
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
			replacementPath: "./framework",
		},
		{
			name: "selected version drift",
			info: &runtimedebug.BuildInfo{Main: runtimedebug.Module{Path: "example.com/driver", Version: "v1.2.4"}},
			want: ModuleOrigin{Selected: ModuleRef{Path: "example.com/driver", Version: "v1.2.3"}},
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
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := verifyBuildInfoOrigin(test.info, test.want, test.replacementPath)
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
	build.Env = append(sanitizeEnvironment(os.Environ()), "GOFLAGS=", "GOWORK=off")
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
	if err := verifyBuildInfoOrigin(info, want, "../framework"); err != nil {
		t.Fatalf("verify real local replacement: %v", err)
	}
}

func TestSourceBridgeBuildArgsPreserveGraphAndBuildPolicy(t *testing.T) {
	cfg := Config{
		GraphFlags:   []string{"-mod=readonly"},
		BuildFlags:   []string{"-trimpath=true", "-buildvcs=false", "-v=true", "-work=true"},
		DriverOrigin: ModuleOrigin{Selected: ModuleRef{Path: "example.com/driver"}, Main: true},
	}
	base := []string{"build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-v", "-work", "-buildmode=c-shared"}
	for _, test := range []struct {
		goos       string
		linkerFlag bool
	}{
		{goos: "darwin"},
		{goos: "linux"},
		{goos: "windows", linkerFlag: true},
	} {
		t.Run(test.goos, func(t *testing.T) {
			want := append([]string(nil), base...)
			if test.linkerFlag {
				want = append(want, "-ldflags=-extldflags=-Wl,--allow-multiple-definition")
			}
			want = append(want, "-o", "/tmp/bridge", "example.com/driver/cmd/ispxnative")
			got := sourceBridgeBuildArgsForGOOS(cfg, "/tmp/bridge", test.goos)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("sourceBridgeBuildArgsForGOOS() = %#v, want %#v", got, want)
			}
		})
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
	goCommand, err = filepath.Abs(goCommand)
	if err != nil {
		t.Fatal(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		ProjectDir:    project,
		GraphWorkDir:  app,
		GoCommand:     goCommand,
		GoWork:        "off",
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
	if err := verifyDriverPackage(context.Background(), cfg, os.Environ()); err == nil {
		t.Fatal("driver validation unexpectedly preserved the caller graph from the dependency project directory")
	}
}

func TestValidateSPXRequestRequiresPackMetadata(t *testing.T) {
	if err := validateSPXRequest(Config{}); err == nil {
		t.Fatal("missing SPX pack metadata was accepted")
	}
	if err := validateSPXRequest(Config{Project: ProjectSnapshot{
		PackDirectory: "assets",
		PackIndexFile: "index.json",
	}}); err != nil {
		t.Fatalf("complete SPX pack metadata was rejected: %v", err)
	}
}

func TestMaterializePortableConfigSnapshotRejectsReplacement(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".config")
	validated := []byte(`{"name":"validated"}`)
	if err := os.WriteFile(configPath, validated, 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := projectpolicy.SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, validated, 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err = materializePortableConfigSnapshot(t.TempDir(), projectDir, snapshot)
	if err == nil || !strings.Contains(err.Error(), "changed after validation") {
		t.Fatalf("materializePortableConfigSnapshot() error = %v, want replacement rejection", err)
	}
}

func TestMaterializePortableConfigSnapshotWritesCapturedBytes(t *testing.T) {
	projectDir := t.TempDir()
	validated := []byte(`{"name":"validated"}`)
	if err := os.WriteFile(filepath.Join(projectDir, ".config"), validated, 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := projectpolicy.SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	configDir, identity, err := materializePortableConfigSnapshot(t.TempDir(), projectDir, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	wantIdentity, err := snapshot.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if identity != wantIdentity {
		t.Fatalf("materialized identity = %q, want %q", identity, wantIdentity)
	}
	got, err := os.ReadFile(filepath.Join(configDir, ".config"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, validated) {
		t.Fatalf("materialized .config = %q, want %q", got, validated)
	}
}

func TestPrepareProjectBundleConfigUsesCapturedConfigBytes(t *testing.T) {
	projectDir := t.TempDir()
	projectFile := filepath.Join(projectDir, "main.spx")
	configPath := filepath.Join(projectDir, ".config")
	validated := []byte(`{"name":"validated"}`)
	mustWriteDriverTestFile(t, projectFile, "onStart => {}\n", 0o600)
	mustWriteDriverTestFile(t, configPath, string(validated), 0o600)
	mustWriteDriverTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{}\n", 0o600)
	snapshot, err := projectpolicy.SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		ProjectDir:  projectDir,
		ProjectFile: projectFile,
		Project: ProjectSnapshot{
			Extension: ".spx", PackDirectory: "assets", PackIndexFile: "index.json",
		},
	}
	bundleConfig, err := prepareProjectBundleConfig(cfg, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"name":"live-replacement"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var archive bytes.Buffer
	if _, err := projectbundle.WriteArchive(&archive, bundleConfig); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Name != ".config" {
			continue
		}
		config, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		got, readErr := io.ReadAll(config)
		closeErr := config.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if !bytes.Equal(got, validated) {
			t.Fatalf("bundled .config = %q, want captured bytes %q", got, validated)
		}
		return
	}
	t.Fatal("project bundle is missing .config")
}

func TestPrepareProjectBundleConfigRejectsConfigDrift(t *testing.T) {
	projectDir := t.TempDir()
	projectFile := filepath.Join(projectDir, "main.spx")
	configPath := filepath.Join(projectDir, ".config")
	mustWriteDriverTestFile(t, projectFile, "onStart => {}\n", 0o600)
	mustWriteDriverTestFile(t, configPath, `{"name":"validated"}`, 0o600)
	mustWriteDriverTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{}\n", 0o600)
	snapshot, err := projectpolicy.SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"name":"replacement"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = prepareProjectBundleConfig(Config{
		ProjectDir:  projectDir,
		ProjectFile: projectFile,
		Project: ProjectSnapshot{
			Extension: ".spx", PackDirectory: "assets", PackIndexFile: "index.json",
		},
	}, snapshot)
	if err == nil || !strings.Contains(err.Error(), "changed after validation") {
		t.Fatalf("prepareProjectBundleConfig() drift error = %v", err)
	}
}

func TestSourceModeIdentity(t *testing.T) {
	tests := []struct {
		name   string
		origin ModuleOrigin
		want   bool
	}{
		{name: "workspace main", origin: ModuleOrigin{Main: true, Selected: ModuleRef{}}, want: true},
		{name: "local replace", origin: ModuleOrigin{Selected: ModuleRef{Version: "v3.2.1"}, Replace: &ModuleRef{Path: "/workspace/spx", Dir: "/workspace/spx"}}, want: true},
		{name: "versioned replace", origin: ModuleOrigin{Selected: ModuleRef{Version: "v3.2.1"}, Replace: &ModuleRef{Path: "github.com/acme/spx", Version: "v3.2.2"}}, want: false},
		{name: "pseudo version", origin: ModuleOrigin{Selected: ModuleRef{Version: "v3.2.2-0.20260817010101-0123456789ab"}}, want: false},
		{name: "release", origin: ModuleOrigin{Selected: ModuleRef{Version: "v3.2.1"}}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sourceMode(test.origin); got != test.want {
				t.Fatalf("sourceMode() = %t, want %t", got, test.want)
			}
		})
	}
}
