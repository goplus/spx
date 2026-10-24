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

package shared

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/release"
)

func TestCommandRunnerRunCommandUsesGoPathBin(t *testing.T) {
	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)

	binDir := filepath.Join(gopath, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	cmdPath := filepath.Join(binDir, "fakecmd")
	outPath := filepath.Join(root, "ran.txt")
	script := "#!/bin/sh\nprintf 'ok' > \"$OUT_PATH\"\n"
	if err := os.WriteFile(cmdPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Setenv("OUT_PATH", outPath)

	runner := CommandRunner{RepoRoot: root}
	if err := runner.RunCommand(".", "fakecmd"); err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	if !FileExists(outPath) {
		t.Fatalf("expected fake command output at %s", outPath)
	}
}

func TestCommandRunnerRunScriptUsesLockedGoToolchain(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(root, "gopath"))

	outputPath := filepath.Join(root, "toolchain.txt")
	scriptPath := filepath.Join(root, "capture-toolchain.sh")
	script := "#!/bin/bash\nset -euo pipefail\nprintf '%s' \"$GOTOOLCHAIN\" > \"$OUTPUT_PATH\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Setenv("OUTPUT_PATH", outputPath)

	runner := CommandRunner{RepoRoot: root}
	if err := runner.RunScript(filepath.Base(scriptPath)); err != nil {
		t.Fatalf("runScript returned error: %v", err)
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	want := "go" + release.DefaultRuntimeLock().Toolchain.Go
	if string(got) != want {
		t.Fatalf("script GOTOOLCHAIN = %q, want %q", got, want)
	}
}

func TestBuildctlCommandEnvPrependsGoBinDirs(t *testing.T) {
	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)

	env, err := buildctlCommandEnv()
	if err != nil {
		t.Fatalf("buildctlCommandEnv returned error: %v", err)
	}

	pathDirs := filepath.SplitList(pathEnvValue(env))
	wantDirs := []string{filepath.Join(gopath, "bin")}
	//lint:ignore SA1019 Verify that buildctl preserves the build toolchain's priority over the inherited PATH.
	if goRoot := runtime.GOROOT(); goRoot != "" {
		wantDirs = append(wantDirs, filepath.Join(goRoot, "bin"))
	}
	if len(pathDirs) < len(wantDirs) {
		t.Fatalf("PATH dirs = %v, want prefix %v", pathDirs, wantDirs)
	}
	for i, want := range wantDirs {
		if pathDirs[i] != want {
			t.Fatalf("PATH dirs = %v, want prefix %v", pathDirs, wantDirs)
		}
	}
	if got, want := env["GOTOOLCHAIN"], "go"+release.DefaultRuntimeLock().Toolchain.Go; got != want {
		t.Fatalf("GOTOOLCHAIN = %q, want %q", got, want)
	}
}

func TestResolveCommandPathUsesCaseInsensitivePathFallback(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	cmdPath := filepath.Join(binDir, "fakecmd")
	if err := os.WriteFile(cmdPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := resolveCommandPath("fakecmd", map[string]string{"Path": binDir})
	if err != nil {
		t.Fatalf("resolveCommandPath returned error: %v", err)
	}
	if got != cmdPath {
		t.Fatalf("resolveCommandPath = %q, want %q", got, cmdPath)
	}
}

func TestIsCommandFileForOSAllowsWindowsFilesWithoutUnixExecBit(t *testing.T) {
	root := t.TempDir()
	cmdPath := filepath.Join(root, "fakecmd.exe")
	if err := os.WriteFile(cmdPath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	info, err := os.Stat(cmdPath)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}

	if !isCommandFileForOS(info, "windows") {
		t.Fatal("expected regular Windows command file to be accepted without Unix exec bit")
	}
	if isCommandFileForOS(info, "linux") {
		t.Fatal("expected non-executable Unix command file to be rejected")
	}
}

func TestCommandRunnerRunCommandReturnsEnvironmentError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", "")
	t.Setenv("PATH", "")

	runner := CommandRunner{RepoRoot: root}
	err := runner.RunCommand(".", "fakecmd")
	if err == nil {
		t.Fatal("expected runCommand to fail when command environment cannot be resolved")
	}
	if !strings.Contains(err.Error(), "resolve command environment") {
		t.Fatalf("runCommand error = %v, want environment resolution context", err)
	}
}

func TestCommandRunnerRunCommandReturnsResolvePathError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(root, "gopath"))

	runner := CommandRunner{RepoRoot: root}
	err := runner.RunCommand(".", "missingcmd")
	if err == nil {
		t.Fatal("expected runCommand to fail when the command cannot be found")
	}
	if !strings.Contains(err.Error(), "resolve command path for missingcmd") {
		t.Fatalf("runCommand error = %v, want command path resolution context", err)
	}
}

func TestPrependToPathDedupes(t *testing.T) {
	got := PrependToPath(strings.Join([]string{"/usr/bin", "/bin"}, string(filepath.ListSeparator)), "/custom/bin", "/usr/bin")
	want := strings.Join([]string{"/custom/bin", "/usr/bin", "/bin"}, string(filepath.ListSeparator))
	if got != want {
		t.Fatalf("PrependToPath = %q, want %q", got, want)
	}
}

func TestCommandHelpers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "capture arguments.sh")
	writeExecutable(t, script, `#!/bin/sh
printf 'VALUE=%s\n' "$VALUE"
printf 'PWD=%s\n' "$PWD"
printf 'ARG=%s\n' "$1"
printf 'stderr detail\n' >&2
exit "$EXIT_CODE"
`)
	t.Setenv("VALUE", "inherited")
	t.Setenv("EXIT_CODE", "0")
	t.Chdir(root)
	for _, test := range []struct {
		name string
		run  func() ([]byte, error)
		want string
	}{
		{"explicit environment and directory", func() ([]byte, error) {
			return RunCommandOutputWithEnv(root, []string{"VALUE=explicit", "EXIT_CODE=0"}, script, "two words")
		}, "explicit"},
		{"inherited directory", func() ([]byte, error) {
			return RunCommandOutputWithEnv("", []string{"VALUE=explicit", "EXIT_CODE=0"}, script, "two words")
		}, "explicit"},
		{"inherited environment", func() ([]byte, error) {
			return RunCommandOutput(script, "two words")
		}, "inherited"},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := test.run()
			want := "VALUE=" + test.want + "\nPWD=" + root + "\nARG=two words\nstderr detail\n"
			if err != nil || string(output) != want {
				t.Fatalf("command output = %q, %v; want %q", output, err, want)
			}
		})
	}
	output, err := RunCommandOutputWithEnv(root, []string{"EXIT_CODE=7"}, script, "argument")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 || !strings.Contains(string(output), "stderr detail") {
		t.Fatalf("failed command output = %q, %v; want captured stderr and exit status 7", output, err)
	}

	marker := filepath.Join(root, "streamed-output")
	writeExecutable(t, script, "#!/bin/sh\nprintf '%s:%s' \"$PWD\" \"$1\" > \"$OUTPUT\"\nexit 0\n")
	t.Setenv("OUTPUT", marker)
	for _, workdir := range []string{"", root} {
		if err := RunStreamingCommand(workdir, script, "two words"); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(marker); err != nil || string(got) != root+":two words" {
			t.Fatalf("streaming command marker = %q, %v", got, err)
		}
	}
	if err := RunStreamingCommand(root, filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing streaming command must fail")
	}
}

func TestEnsureGoPathFromCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "go"), `#!/bin/sh
if [ "$*" != "env GOPATH" ]; then
  exit 99
fi
printf '%s' "$FAKE_GOPATH"
exit "$FAKE_EXIT"
`)
	t.Setenv("GOPATH", "")
	t.Setenv("PATH", binDir)
	for _, test := range []struct {
		name, output, exitCode, want, wantErr string
	}{
		{"trim whitespace", " \t/custom/go path\n", "0", "/custom/go path", ""},
		{"empty value", " \n", "0", "", "missing GOPATH"},
		{"failed command", "", "7", "", "exit status 7"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("FAKE_GOPATH", test.output)
			t.Setenv("FAKE_EXIT", test.exitCode)
			got, err := EnsureGoPath()
			if got != test.want {
				t.Fatalf("EnsureGoPath() = %q, want %q", got, test.want)
			}
			if test.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("EnsureGoPath() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestCommandRunnerListDemoDirs(t *testing.T) {
	root := t.TempDir()
	runner := CommandRunner{RepoRoot: root}
	if got := runner.RepoRootDir(); got != root {
		t.Fatalf("RepoRootDir() = %q, want %q", got, root)
	}
	if _, err := runner.ListDemoDirs(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing tutorial error = %v", err)
	}
	tutorial := filepath.Join(root, "tutorial")
	if err := os.Mkdir(tutorial, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := runner.ListDemoDirs(); err != nil || len(got) != 0 {
		t.Fatalf("empty tutorial directories = %v, %v", got, err)
	}
	for _, name := range []string{"z-last", "a-first"} {
		if err := os.Mkdir(filepath.Join(tutorial, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tutorial, "README.md"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := runner.ListDemoDirs()
	want := []string{filepath.Join("tutorial", "a-first"), filepath.Join("tutorial", "z-last")}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("ListDemoDirs() = %v, %v; want sorted directories %v", got, err, want)
	}
}

func TestResolveCommandPathExplicitAndInvalidPaths(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{filepath.Join(root, "tool"), filepath.Join("relative", "tool")} {
		if got, err := resolveCommandPath(name, nil); err != nil || got != name {
			t.Fatalf("explicit command = %q, %v; want %q", got, err, name)
		}
	}
	if _, err := resolveCommandPath("missing", nil); err == nil {
		t.Fatal("expected missing PATH to fail")
	}
	if err := os.Mkdir(filepath.Join(root, "tool"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveCommandPath("tool", map[string]string{"PATH": string(os.PathListSeparator) + root}); err == nil {
		t.Fatal("a directory in PATH must not be resolved as a command")
	}
}

func TestCommandRunnerStopWebServersUsesIsolatedTools(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixtures")
	}
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "pgrep"), `#!/bin/sh
if [ "$*" != "-f gdspx_web_server.py" ]; then
  exit 99
fi
printf '%s' "$FAKE_PIDS"
exit "$FAKE_EXIT"
`)
	t.Setenv("PATH", binDir)
	t.Setenv("OS", "")
	for _, test := range []struct {
		name, output, exitCode string
		wantErr                bool
	}{
		{"no servers", "", "1", false},
		{"empty process list", "", "0", false},
		{"malformed process identifiers", "not-a-pid 999999999999999999999999999999", "0", false},
		{"lookup failed", "", "2", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("FAKE_PIDS", test.output)
			t.Setenv("FAKE_EXIT", test.exitCode)
			err := (CommandRunner{}).StopWebServers()
			if test.wantErr {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
					t.Fatalf("StopWebServers() error = %v, want exit status 2", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}

	// Isolate the Windows compatibility branch. Only verified PIDs may be killed.
	marker := filepath.Join(binDir, "taskkill-arguments")
	writeExecutable(t, filepath.Join(binDir, "powershell"), `#!/bin/sh
case "$*" in
 *Select-Object*) printf '37\n42\n';;
 *'ProcessId = 37'*) printf 'python.exe C:\\spx\\gdspx_web_server.py';;
 *'ProcessId = 42'*) printf 'python.exe unrelated.py';;
 *) exit 99;;
esac
`)
	writeExecutable(t, filepath.Join(binDir, "taskkill"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$OUTPUT\"\n")
	t.Setenv("OUTPUT", marker)
	t.Setenv("OS", "Windows_NT")
	if err := (CommandRunner{}).StopWebServers(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(marker)
	const want = "/F /PID 37\n"
	if err != nil || string(got) != want {
		t.Fatalf("taskkill arguments = %q, %v; want %q", got, err, want)
	}
}

func TestStopWebServersChecksEveryCandidate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixtures")
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "queries")
	writeExecutable(t, filepath.Join(bin, "pgrep"), "#!/bin/sh\nprintf '37 42 99 37 0 -1 invalid\\n'\n")
	writeExecutable(t, filepath.Join(bin, "ps"), `#!/bin/sh
printf '%s\n' "$*" >> "$OUTPUT"
printf 'python unrelated.py'
`)
	t.Setenv("PATH", bin)
	t.Setenv("OS", "")
	t.Setenv("OUTPUT", log)
	if err := (CommandRunner{}).StopWebServers(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(log)
	if err != nil || string(got) != "-o args= -p 37\n-o args= -p 42\n-o args= -p 99\n" {
		t.Fatalf("verified PIDs = %q, %v", got, err)
	}
}

func TestVersionishComparisonAndMissingRepository(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want int
	}{{"2.1", "1.9", 1}, {"1.0", "1", 1}, {"1.x", "1.0", 1}, {"1", "1", 0}} {
		if got := compareVersionish(tt.a, tt.b); got != tt.want {
			t.Errorf("compareVersionish(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
	t.Chdir(t.TempDir())
	if _, err := FindRepoRoot(); err == nil || !strings.Contains(err.Error(), "could not locate repository root") {
		t.Fatalf("missing repo root = %v", err)
	}
}

func TestRunScriptRejectsUnresolvedEnvironment(t *testing.T) {
	t.Setenv("GOPATH", "")
	t.Setenv("PATH", t.TempDir())
	if err := (CommandRunner{RepoRoot: t.TempDir()}).RunScript("missing.sh"); err == nil || !strings.Contains(err.Error(), "resolve script environment") {
		t.Fatalf("script environment error = %v", err)
	}
}

func TestRepositoryResolutionReportsRemovedWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows prevents removal of the working directory")
	}
	working := filepath.Join(t.TempDir(), "working")
	if err := os.Mkdir(working, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPATH", t.TempDir())
	t.Chdir(working)
	if err := os.Remove(working); err != nil {
		t.Fatal(err)
	}
	// Some hosts can still resolve an unlinked working directory.
	if cwd, err := os.Getwd(); err == nil {
		t.Skipf("Getwd still resolves the removed directory as %q", cwd)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Getwd in removed working directory = %v", err)
	}
	for _, resolve := range []func() error{
		func() error { _, err := FindRepoRoot(); return err },
		func() error { _, err := ResolveSPXModuleSource("."); return err },
		func() error { _, err := ResolveSPXModule("."); return err },
		func() error { _, err := ResolveBuildEnvironment(".", "linux"); return err },
	} {
		if err := resolve(); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("removed working directory = %v", err)
		}
	}
}

func TestParseNoArgs(t *testing.T) {
	var output bytes.Buffer
	if err := ParseNoArgs("doctor", "Usage: buildctl doctor", nil, &output); err != nil {
		t.Fatalf("ParseNoArgs returned error: %v", err)
	}
	if err := ParseNoArgs("doctor", "Usage: buildctl doctor", []string{"extra"}, &output); !errors.Is(err, ErrUsage) {
		t.Fatalf("ParseNoArgs extra argument error = %v, want ErrUsage", err)
	}
	if !strings.Contains(output.String(), "Usage: buildctl doctor") {
		t.Fatalf("ParseNoArgs did not print usage: %q", output.String())
	}
	if err := ParseNoArgs("doctor", "Usage: buildctl doctor", []string{"--help"}, &output); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("ParseNoArgs --help error = %v, want flag.ErrHelp", err)
	}
}

func TestValidateOptionalPlatform(t *testing.T) {
	for _, platform := range []string{"", "android", "ios", "web", "linux", "windows", "macos"} {
		t.Run("accept/"+platform, func(t *testing.T) {
			if err := ValidateOptionalPlatform(platform); err != nil {
				t.Fatalf("ValidateOptionalPlatform(%q) returned error: %v", platform, err)
			}
		})
	}
	if err := ValidateOptionalPlatform("plan9"); err == nil || !strings.Contains(err.Error(), "unsupported platform: plan9") {
		t.Fatalf("ValidateOptionalPlatform(plan9) error = %v, want unsupported-platform error", err)
	}
}

func TestResolveWebModeContract(t *testing.T) {
	want := map[string]WebModeSpec{
		"normal":      {ReleaseTemplate: "web.zip", CacheSuffix: "webpack.zip", ExportCommand: "exportweb", ExportZip: "spx_web.zip"},
		"worker":      {ReleaseTemplate: "web-worker.zip", CacheSuffix: "webworker.zip", ExportCommand: "exportwebworker", ExportZip: "spx_web_worker.zip", Threaded: true},
		"minigame":    {ReleaseTemplate: "web-minigame.zip", CacheSuffix: "webminigame.zip", ExportCommand: "exportminigame", ExportZip: "spx_web_minigame.zip"},
		"miniprogram": {ReleaseTemplate: "web-miniprogram.zip", CacheSuffix: "webminiprogram.zip", ExportCommand: "exportminiprogram", ExportZip: "spx_web_miniprogram.zip"},
	}
	for mode, expected := range want {
		t.Run(mode, func(t *testing.T) {
			got, err := ResolveWebMode(mode)
			if err != nil {
				t.Fatalf("ResolveWebMode(%q) returned error: %v", mode, err)
			}
			if got != expected {
				t.Fatalf("ResolveWebMode(%q) = %#v, want %#v", mode, got, expected)
			}
			if err := ValidateWebMode(mode); err != nil {
				t.Fatalf("ValidateWebMode(%q) returned error: %v", mode, err)
			}
		})
	}
	for _, mode := range []string{"", "unknown"} {
		t.Run("reject/"+mode, func(t *testing.T) {
			if _, err := ResolveWebMode(mode); err == nil || !strings.Contains(err.Error(), "unsupported web-mode") {
				t.Fatalf("ResolveWebMode(%q) error = %v, want unsupported-web-mode error", mode, err)
			}
			if err := ValidateWebMode(mode); err == nil {
				t.Fatalf("ValidateWebMode(%q) succeeded", mode)
			}
		})
	}
}
