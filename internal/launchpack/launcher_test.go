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

package launchpack

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/projectbundle"
	"github.com/goplus/spx/v4/internal/projectpolicy"
	"github.com/goplus/spx/v4/internal/release"
	"github.com/goplus/spx/v4/internal/runtimebundle"
	"github.com/goplus/spx/v4/internal/runtimepayload"
)

type launchpackErrorWriter struct{ err error }

type launcherSourceReplacingWriter struct {
	io.Writer
	source string
}

func TestGeneratedLauncherGolden(t *testing.T) {
	const want = `package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/goplus/spx/v4/x/xgolauncher"
)

//go:embed payload.spxpkg
var payload []byte

const payloadSHA256 = "payload-digest"
const manifestSHA256 = "manifest-digest"

func main() {
	status, err := xgolauncher.RunCommand(context.Background(), func(ctx context.Context) (xgolauncher.ProcessStatus, error) {
		return xgolauncher.RunContext(ctx, xgolauncher.Config{
			Payload: payload, PayloadSHA256: payloadSHA256, ManifestSHA256: manifestSHA256,
			Args: os.Args[1:], Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		})
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "xgolauncher: %v\n", err)
		status = xgolauncher.ProcessStatus{Code: 1}
	}
	xgolauncher.Exit(status)
}
`
	if got := string(renderGeneratedLauncher("payload-digest", "manifest-digest")); got != want {
		t.Fatalf("generated launcher changed:\n%s", got)
	}
}

func TestCompileLauncherFailureCleansWork(t *testing.T) {
	sentinel := errors.New("payload interrupted")
	for _, tt := range []struct {
		name, want string
		modify     func(*testing.T, *Config, *payloadBuilder)
	}{
		{"invalid graph", "unsupported build flag", func(t *testing.T, c *Config, b *payloadBuilder) { c.BuildFlags = []string{"-tags=other"} }},
		{"blocked output ancestor", "inspect staging output", func(t *testing.T, c *Config, b *payloadBuilder) {
			writeProjectTestFile(t, c.Output, "blocked")
			c.Output = filepath.Join(c.Output, "launcher")
		}},
		{"existing output", "already exists", func(t *testing.T, c *Config, b *payloadBuilder) { writeProjectTestFile(t, c.Output, "existing") }},
		{"nil builder", "nil payload builder", func(t *testing.T, c *Config, b *payloadBuilder) { *b = nil }},
		{"payload failure", "write generated payload", func(t *testing.T, c *Config, b *payloadBuilder) {
			*b = func(string, io.Writer) (string, string, error) { return "", "", sentinel }
		}},
		{"closed payload", "write generated payload", func(t *testing.T, c *Config, b *payloadBuilder) {
			*b = func(_ string, w io.Writer) (string, string, error) {
				if err := w.(*os.File).Close(); err != nil {
					t.Fatal(err)
				}
				return "p", "m", nil
			}
		}},
		{"main path blocked", "write generated launcher", func(t *testing.T, c *Config, b *payloadBuilder) {
			*b = func(dir string, w io.Writer) (string, string, error) {
				if err := os.Mkdir(filepath.Join(dir, "main.go"), 0700); err != nil {
					t.Fatal(err)
				}
				return "p", "m", nil
			}
		}},
		{"graph before", "before launcher build", func(t *testing.T, c *Config, b *payloadBuilder) {
			c.VerifyGraph = func(context.Context) error { return sentinel }
		}},
		{"command failed", "build generated launcher", func(t *testing.T, c *Config, b *payloadBuilder) { launcherGoFixture(t, c, "exit 17") }},
		{"graph after", "after launcher build", func(t *testing.T, c *Config, b *payloadBuilder) {
			calls := 0
			c.VerifyGraph = func(context.Context) error {
				calls++
				if calls == 2 {
					return sentinel
				}
				return nil
			}
		}},
		{"missing output", "inspect launcher output", func(t *testing.T, c *Config, b *payloadBuilder) { launcherGoFixture(t, c, "exit 0") }},
		{"empty output", "non-empty regular", func(t *testing.T, c *Config, b *payloadBuilder) { launcherGoFixture(t, c, ": > \"$out\"") }},
		{"invalid executable", "not a host", func(t *testing.T, c *Config, b *payloadBuilder) { launcherGoFixture(t, c, "printf 'text' > \"$out\"") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLaunchpackConfig(t)
			launcherGoFixture(t, &cfg, "printf '\\177ELFfixture' > \"$out\"")
			temp := t.TempDir()
			t.Setenv("TMPDIR", temp)
			var builder payloadBuilder = func(_ string, w io.Writer) (string, string, error) {
				_, err := io.WriteString(w, "payload")
				return "payload-hash", "manifest-hash", err
			}
			tt.modify(t, &cfg, &builder)
			err := compileLauncher(context.Background(), cfg, IO{Env: []string{}}, builder)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("compileLauncher error = %v, want %q", err, tt.want)
			}
			entries, err := os.ReadDir(temp)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("temporary build directory leaked: %v", entries)
			}
			if tt.name == "existing output" {
				assertRuntimeFile(t, cfg.Output, "existing")
			}
		})
	}
}

func TestCompileLauncherRetainsRequestedWorkAndNormalizesFlags(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fixture has an ELF header")
	}
	cfg := validLaunchpackConfig(t)
	launcherGoFixture(t, &cfg, "printf '\\177ELFfixture' > \"$out\"")
	cfg.BuildFlags = []string{"-work=true", "-v=false", "-trimpath=true", "-buildvcs=false"}
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	var log bytes.Buffer
	err := compileLauncher(context.Background(), cfg, IO{Stderr: &log, Env: []string{}}, func(_ string, w io.Writer) (string, string, error) {
		_, err := io.WriteString(w, "payload")
		return "p", "m", err
	})
	if err != nil {
		t.Fatal(err)
	}
	work := strings.TrimSpace(strings.TrimPrefix(log.String(), "SPXWORK="))
	if !pathWithin(temp, work) {
		t.Fatalf("unexpected work directory %q", work)
	}
	assertRuntimeFile(t, filepath.Join(work, "payload.spxpkg"), "payload")
	assertRuntimeFile(t, filepath.Join(work, "main.go"), string(renderGeneratedLauncher("p", "m")))
	if got := normalizedGoBuildFlags(cfg.BuildFlags); !slices.Equal(got, []string{"-work", "-trimpath", "-buildvcs=false"}) {
		t.Fatalf("normalized flags = %q", got)
	}
}

func TestBuildSourceBridgeChecksGraphAndCleansFailures(t *testing.T) {
	for _, tt := range []struct {
		name, want, body string
		graphFailure     int
		keep             bool
	}{
		{name: "success", body: "printf bridge > \"$out\""},
		{name: "retain work", body: "printf bridge > \"$out\"", keep: true},
		{name: "command fails", body: "exit 3", want: "build source interpreter bridge"},
		{name: "no output", body: "exit 0", want: "lstat source interpreter bridge"},
		{name: "graph before", body: "printf bridge > \"$out\"", graphFailure: 1, want: "before source bridge build"},
		{name: "graph after", body: "printf bridge > \"$out\"", graphFailure: 2, want: "after source bridge build"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLaunchpackConfig(t)
			launcherGoFixture(t, &cfg, tt.body)
			temp := t.TempDir()
			t.Setenv("TMPDIR", temp)
			var log bytes.Buffer
			cfg.IO.Stderr = &log
			if tt.keep {
				cfg.BuildFlags = []string{"-work"}
			}
			checks := 0
			sentinel := errors.New("graph changed")
			cfg.VerifyGraph = func(context.Context) error {
				checks++
				if checks == tt.graphFailure {
					return sentinel
				}
				return nil
			}
			path, cleanup, err := BuildSourceBridge(context.Background(), cfg)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) || cleanup != nil || path != "" {
					t.Fatalf("BuildSourceBridge = %q, %v", path, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				assertRuntimeFile(t, path, "bridge")
				cleanup()
				if tt.keep {
					if !strings.Contains(log.String(), "SPXBRIDGEWORK="+filepath.Dir(path)) {
						t.Fatalf("work log = %q", log.String())
					}
					assertRuntimeFile(t, path, "bridge")
				} else if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("bridge not removed: %v", err)
				}
			}
			entries, err := os.ReadDir(temp)
			if err != nil {
				t.Fatal(err)
			}
			if !tt.keep && len(entries) != 0 {
				t.Fatalf("temporary directory leaked: %v", entries)
			}
		})
	}
	cfg := validLaunchpackConfig(t)
	cfg.BridgePackage = ""
	if _, _, err := buildSourceBridge(context.Background(), cfg, "bridge.so", IO{}); err == nil || !strings.Contains(err.Error(), "bridge package") {
		t.Fatalf("missing bridge package: %v", err)
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		name, err := bridgeFileName(goos, "arm64")
		if err != nil || !strings.Contains(name, goos+"-arm64") {
			t.Fatalf("bridgeFileName(%q) = %q, %v", goos, name, err)
		}
	}
	if _, err := bridgeFileName("plan9", "amd64"); err == nil {
		t.Fatal("unsupported host accepted")
	}
	args := sourceBridgeBuildArgsForGOOS(cfg, "bridge.dll", "windows")
	if !slices.Contains(args, "-ldflags=-extldflags=-Wl,--allow-multiple-definition") {
		t.Fatalf("Windows linker flags = %q", args)
	}
}

func TestPinnedPayloadSourcesRejectMutationAndShortReads(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*testing.T, *pinnedFile)
	}{
		{"closed", func(t *testing.T, f *pinnedFile) {
			if err := f.file.Close(); err != nil {
				t.Fatal(err)
			}
		}},
		{"removed", func(t *testing.T, f *pinnedFile) {
			if err := os.Remove(f.path); err != nil {
				t.Fatal(err)
			}
		}},
		{"truncated", func(t *testing.T, f *pinnedFile) {
			if err := os.Truncate(f.path, 1); err != nil {
				t.Fatal(err)
			}
		}},
		{"replaced", func(t *testing.T, f *pinnedFile) {
			if err := os.Rename(f.path, f.path+".old"); err != nil {
				t.Fatal(err)
			}
			writeProjectTestFile(t, f.path, "engine")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && (tt.name == "removed" || tt.name == "replaced") {
				t.Skip("Windows does not allow deleting an open pinned file")
			}
			path := filepath.Join(t.TempDir(), "engine")
			writeProjectTestFile(t, path, "engine")
			f, err := openPinnedFile("Engine", path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.file.Close()
			tt.change(t, f)
			if err := f.verify(); err == nil {
				t.Fatal("changed pinned source accepted")
			}
		})
	}
	source := runtimepayload.FileSource{ReaderAt: bytes.NewReader([]byte("one")), Size: 4}
	if _, err := digestFileSource(source); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("short source digest: %v", err)
	}
	good := byteSource("good", 0600, []byte("one"))
	for _, pair := range [][2]runtimepayload.FileSource{{source, good}, {good, source}} {
		if _, _, _, err := localEngineSourceDigests(pair[0], pair[1]); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("short runtime digest: %v", err)
		}
	}
	sentinel := errors.New("destination failed")
	if err := copyFileSource(launchpackErrorWriter{sentinel}, good); !errors.Is(err, sentinel) {
		t.Fatalf("copy failure: %v", err)
	}
	bundle := runtimebundle.Bundle{Entries: []runtimebundle.Entry{{Name: "engine", SHA256: "hash"}}}
	if !bundleEntryHasDigest(bundle, "engine", "hash") || bundleEntryHasDigest(bundle, "engine", "other") || bundleEntryHasDigest(bundle, "missing", "hash") {
		t.Fatal("bundle digest lookup accepted wrong identity")
	}
	if _, err := openPinnedFile("directory", t.TempDir()); err == nil {
		t.Fatal("directory accepted as pinned file")
	}
}

func TestLauncherPayloadAuthenticatesAndRejectsUnsafeInputs(t *testing.T) {
	for _, tt := range []struct {
		name, want    string
		mutate        func(*testing.T, *Config, *Assets, *projectbundle.Config, string)
		failWriter    bool
		replaceSource string
	}{
		{name: "valid traced payload"},
		{name: "engine replaced during write", want: "changed while reading", replaceSource: "engine"},
		{name: "pack replaced during write", want: "changed while reading", replaceSource: "pack"},
		{name: "bridge replaced during write", want: "changed while reading", replaceSource: "bridge"},
		{name: "project archive exists", want: "create project archive", mutate: func(t *testing.T, c *Config, a *Assets, p *projectbundle.Config, work string) {
			writeProjectTestFile(t, filepath.Join(work, "project.zip"), "existing")
		}},
		{name: "project missing", want: "collect project", mutate: func(t *testing.T, c *Config, a *Assets, p *projectbundle.Config, work string) {
			p.ProjectDir = filepath.Join(work, "missing")
		}},
		{name: "engine missing", want: "lstat Engine", mutate: func(t *testing.T, c *Config, a *Assets, p *projectbundle.Config, work string) {
			a.EnginePath += ".missing"
		}},
		{name: "pack missing", want: "lstat runtime PCK", mutate: func(t *testing.T, c *Config, a *Assets, p *projectbundle.Config, work string) {
			a.PackPath += ".missing"
		}},
		{name: "bridge missing", want: "lstat interpreter bridge", mutate: func(t *testing.T, c *Config, a *Assets, p *projectbundle.Config, work string) {
			a.BridgePath += ".missing"
		}},
		{name: "duplicate engine component", want: "identify Engine bundle", mutate: func(t *testing.T, c *Config, a *Assets, p *projectbundle.Config, work string) {
			a.PackPath = a.EnginePath
		}},
		{name: "bridge manifest collision", want: "identify bridge bundle", mutate: func(t *testing.T, c *Config, a *Assets, p *projectbundle.Config, work string) {
			a.BridgePath = filepath.Join(work, "bridge-manifest.json")
			writeProjectTestFile(t, a.BridgePath, "bridge")
		}},
		{name: "destination failure", want: "build embedded payload", failWriter: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.replaceSource != "" && runtime.GOOS == "windows" {
				t.Skip("Windows keeps opened source files locked against rename")
			}
			cfg := validLaunchpackConfig(t)
			cfg.Source = SourceIdentity{SelectedPath: "github.com/goplus/spx/v4", SelectedVersion: "v4.0.0", EffectivePath: cfg.WorkDir, SourceMode: true}
			cfg.BuildFlags = []string{"-x"}
			work := t.TempDir()
			assets := Assets{EnginePath: filepath.Join(work, "engine"), PackPath: filepath.Join(work, "runtime.pck"), BridgePath: filepath.Join(work, "bridge.so"), Lock: cfg.RuntimeLock}
			for path, data := range map[string]string{assets.EnginePath: "engine", assets.PackPath: "pack", assets.BridgePath: "bridge"} {
				writeProjectTestFile(t, path, data)
			}
			project, err := prepareProjectBundleConfig(cfg, cfg.PortableConfig)
			if err != nil {
				t.Fatal(err)
			}
			if tt.mutate != nil {
				tt.mutate(t, &cfg, &assets, &project, work)
			}
			var payload, log bytes.Buffer
			var dst io.Writer = &payload
			sentinel := errors.New("write failed")
			if tt.failWriter {
				dst = launchpackErrorWriter{sentinel}
			}
			if tt.replaceSource != "" {
				path := map[string]string{"engine": assets.EnginePath, "pack": assets.PackPath, "bridge": assets.BridgePath}[tt.replaceSource]
				// Replace the pathname after source validation, while the payload
				// writer still reads from its pinned, unchanged descriptor.
				dst = &launcherSourceReplacingWriter{Writer: &payload, source: path}
			}
			digest, manifest, err := writeLauncherPayload(work, dst, cfg, assets, project, IO{Stderr: &log})
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) || digest != "" || manifest != "" {
					t.Fatalf("payload = %q, %q, %v, want %q", digest, manifest, err, tt.want)
				}
				if tt.failWriter && !errors.Is(err, sentinel) {
					t.Fatalf("lost writer cause: %v", err)
				}
				if tt.replaceSource != "" && (payload.Len() == 0 || log.Len() != 0) {
					t.Fatal("revalidation did not reject written payload before success tracing")
				}
				return
			}
			verified, err := runtimepayload.Verify(payload.Bytes(), digest, manifest, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				t.Fatal(err)
			}
			if verified.Manifest.Project.PackDirectory != "assets" || !strings.Contains(log.String(), "payload="+digest) {
				t.Fatalf("payload manifest or trace mismatch: %#v, %q", verified.Manifest, log.String())
			}
		})
	}
}

func TestBuildWorkDirectoriesReportUnavailableTemporaryStorage(t *testing.T) {
	for _, name := range []string{"launcher", "source bridge"} {
		t.Run(name, func(t *testing.T) {
			cfg := validLaunchpackConfig(t)
			blocked := filepath.Join(t.TempDir(), "blocked")
			writeProjectTestFile(t, blocked, "existing")
			for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
				t.Setenv(key, blocked)
			}
			var err error
			if name == "launcher" {
				err = compileLauncher(context.Background(), cfg, IO{}, func(string, io.Writer) (string, string, error) {
					t.Fatal("payload builder ran without a work directory")
					return "", "", nil
				})
			} else {
				path, cleanup, buildErr := buildSourceBridge(context.Background(), cfg, "bridge.so", IO{})
				err = buildErr
				if path != "" || cleanup != nil {
					t.Fatalf("failed bridge build returned path %q or cleanup", path)
				}
			}
			if err == nil || !strings.Contains(err.Error(), "create "+name+" work directory") {
				t.Fatalf("%s temporary storage failure = %v", name, err)
			}
			assertRuntimeFile(t, blocked, "existing")
			if _, err := os.Lstat(cfg.Output); !os.IsNotExist(err) {
				t.Fatalf("failed build published output: %v", err)
			}
		})
	}
	cfg := validLaunchpackConfig(t)
	cfg.BuildFlags = []string{"-unsupported"}
	if _, _, err := buildSourceBridge(context.Background(), cfg, "bridge.so", IO{}); err == nil || !strings.Contains(err.Error(), "unsupported build flag") {
		t.Fatalf("invalid source bridge graph = %v", err)
	}
}

func TestSourceBridgeEnvironmentDropsInheritedCGOFlags(t *testing.T) {
	base := []string{"PATH=/tools", "CGO_CFLAGS=-I/untrusted", "CGO_LDFLAGS=-L/untrusted", "CGO_ENABLED=0", "CGO_CPPFLAGS=-DOTHER", "GOOS=other", "GOWORK=other", "GOFLAGS=-tags=other", "CGO_WITHOUT_EQUALS"}
	original := append([]string(nil), base...)
	got := sourceBridgeEnv(Config{GoWork: "off"}, base)
	want := []string{"PATH=/tools", "CGO_WITHOUT_EQUALS", "GOFLAGS=", "GOWORK=off", "GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH, "CGO_ENABLED=1"}
	if !slices.Equal(got, want) || !slices.Equal(base, original) {
		t.Fatalf("bridge environment = %q; original = %q", got, base)
	}
}

func TestBuildLauncherEndToEnd(t *testing.T) {
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	if output, err := exec.Command(goCommand, "env", "CGO_ENABLED").Output(); err != nil || strings.TrimSpace(string(output)) != "1" {
		t.Skip("cgo toolchain unavailable")
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}

	repoRoot := launchpackRepoRoot(t)
	moduleRoot := t.TempDir()
	writeProjectTestFile(t, filepath.Join(moduleRoot, "go.mod"), "module example.com/launcher\n\ngo 1.25.0\n\nrequire github.com/goplus/spx/v4 v4.0.0\nreplace github.com/goplus/spx/v4 => "+filepath.ToSlash(repoRoot)+"\n")
	writeProjectTestFile(t, filepath.Join(moduleRoot, "bridge", "main.go"), "package main\n\n// void bridge(void) {}\nimport \"C\"\n\nfunc main() {}\n")

	projectDir := filepath.Join(moduleRoot, "game")
	projectFile := filepath.Join(projectDir, "main.spx")
	writeProjectTestFile(t, projectFile, "onStart => {}\n")
	writeProjectTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{}\n")
	snapshot, err := projectpolicy.SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}

	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	runtimeRoot := filepath.Join(moduleRoot, "runtime")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := publishLocalRuntimeTest(t, runtimeRoot, filepath.Join(runtimeRoot, "manifest.json"), spec, "engine", "pack")
	output := filepath.Join(moduleRoot, "launcher"+executableSuffix(runtime.GOOS))

	var buildLog bytes.Buffer
	graphChecks := 0
	result, err := BuildLauncher(context.Background(), Config{
		ProjectDir: projectDir, ProjectFile: projectFile, ProjectExt: ".spx",
		PackDir: "assets", PackIndex: "index.json", PortableConfig: snapshot,
		RuntimeManifestPath: manifest, RuntimeCacheRoot: filepath.Join(moduleRoot, "cache"),
		RuntimeIdentity: RuntimeIdentity{Version: lock.RuntimeVersion, ABI: lock.RuntimeABI, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		RuntimeLock:     lock,
		Source:          SourceIdentity{SelectedPath: "github.com/goplus/spx/v4", SelectedVersion: "v4.0.0", EffectivePath: repoRoot, SourceMode: true},
		GoCommand:       goCommand, WorkDir: moduleRoot, GoWork: "off", GraphFlags: []string{"-mod=mod"},
		BuildFlags: []string{"-trimpath=true", "-buildvcs=false"},
		Output:     output, BridgePackage: "./bridge", VerifyGraph: func(context.Context) error {
			graphChecks++
			return nil
		},
		IO: IO{Stdout: &buildLog, Stderr: &buildLog, Env: os.Environ()},
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, buildLog.String())
	}
	if result.Output != output || len(result.PayloadSHA256) != 64 || len(result.ManifestSHA256) != 64 || graphChecks != 4 {
		t.Fatalf("result = %#v", result)
	}
}

func TestBuildLauncherFailureReleasesBridgeWork(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		configure  func(*testing.T, *Config)
	}{
		{"uninitialized snapshot", "portable config", func(t *testing.T, c *Config) { c.PortableConfig = projectpolicy.PortableConfigSnapshot{} }},
		{"runtime unavailable", "inspect local runtime manifest", func(t *testing.T, c *Config) { c.RuntimeManifestPath += ".missing" }},
		{"bridge build fails", "build source interpreter bridge", func(t *testing.T, c *Config) { launcherGoFixture(t, c, "exit 8") }},
		{"bridge verification fails", "verify source bridge", func(t *testing.T, c *Config) {
			c.VerifyBridge = func(path string) error {
				assertRuntimeFile(t, path, "bridge")
				return errors.New("bridge provenance rejected")
			}
		}},
		{"bridge removed during verification", "interpreter bridge unavailable", func(t *testing.T, c *Config) {
			c.VerifyBridge = os.Remove
		}},
		{"project changes during build", "changed after validation", func(t *testing.T, c *Config) {
			c.VerifyBridge = func(string) error { writeProjectTestFile(t, filepath.Join(c.ProjectDir, ".config"), "{}"); return nil }
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLaunchpackConfig(t)
			launcherGoFixture(t, &cfg, "printf bridge > \"$out\"")
			spec, err := release.HostRuntimeSpecFor(cfg.RuntimeLock, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				t.Skip(err)
			}
			root := t.TempDir()
			cfg.RuntimeManifestPath = publishLocalRuntimeTest(t, root, filepath.Join(root, "manifest.json"), spec, "engine", "pack")
			cfg.RuntimeCacheRoot = filepath.Join(cfg.WorkDir, "cache")
			temp := t.TempDir()
			t.Setenv("TMPDIR", temp)
			tt.configure(t, &cfg)
			result, err := BuildLauncher(context.Background(), cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) || result != (Result{}) {
				t.Fatalf("BuildLauncher = %#v, %v, want %q", result, err, tt.want)
			}
			entries, err := os.ReadDir(temp)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("work directory leaked: %v", entries)
			}
			if _, err := os.Stat(cfg.Output); !os.IsNotExist(err) {
				t.Fatalf("failed build created output: %v", err)
			}
		})
	}
}

func (w launchpackErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func (w *launcherSourceReplacingWriter) Write(data []byte) (int, error) {
	if w.source != "" {
		if err := os.Rename(w.source, w.source+".moved"); err != nil {
			return 0, err
		}
		w.source = ""
	}
	return w.Writer.Write(data)
}

// The fixture implements only the Go build output contract; no compiler or
// network is needed to exercise packaging rollback and graph verification.
func launcherGoFixture(t *testing.T, cfg *Config, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell command fixture")
	}
	cfg.GoCommand = filepath.Join(cfg.WorkDir, "fake-go")
	script := "#!/bin/sh\nset -eu\nout=''\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = '-o' ]; then shift; out=$1; fi\n  shift\ndone\n" + body + "\n"
	if err := os.WriteFile(cfg.GoCommand, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
}

func launchpackRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join(dir, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func executableSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}
