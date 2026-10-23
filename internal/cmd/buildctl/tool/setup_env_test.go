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

package tool

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/release"
)

func TestSConsEnvironmentCommands(t *testing.T) {
	venvDir := filepath.Join("tmp", "scons-4.8.1")
	pythonCommand, sconsCommand := sconsEnvironmentCommands(venvDir)
	if runtime.GOOS == "windows" {
		if pythonCommand != filepath.Join(venvDir, "Scripts", "python.exe") || sconsCommand != filepath.Join(venvDir, "Scripts", "scons.exe") {
			t.Fatalf("unexpected Windows SCons environment commands: %q, %q", pythonCommand, sconsCommand)
		}
		return
	}
	if pythonCommand != filepath.Join(venvDir, "bin", "python") || sconsCommand != filepath.Join(venvDir, "bin", "scons") {
		t.Fatalf("unexpected SCons environment commands: %q, %q", pythonCommand, sconsCommand)
	}
}

func TestParseJavaMajorVersion(t *testing.T) {
	cases := []struct {
		output string
		want   int
		ok     bool
	}{
		{output: "openjdk version \"17.0.8\" 2023-07-18", want: 17, ok: true},
		{output: "java version \"1.8.0_402\"", want: 8, ok: true},
		{output: "garbage", want: 0, ok: false},
		{output: "version without quotes\nversion \"17\"", want: 17, ok: true},
		{output: "version \"unterminated", ok: false},
		{output: "version \"bad\"", ok: false},
		{output: "version \"1.bad\"", ok: false},
	}

	for _, tc := range cases {
		got, ok := parseJavaMajorVersion(tc.output)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("parseJavaMajorVersion(%q) = (%d,%v), want (%d,%v)", tc.output, got, ok, tc.want, tc.ok)
		}
	}
}

func TestSelectEMSDKExports(t *testing.T) {
	before := map[string]string{
		"PATH": "/usr/bin",
		"HOME": "/tmp/home",
	}
	after := map[string]string{
		"PATH":         "/emsdk/bin:/usr/bin",
		"HOME":         "/tmp/home",
		"EMSDK":        "/tmp/emsdk",
		"EM_CONFIG":    "/tmp/.emscripten",
		"UNRELATED":    "value",
		"JAVA_HOME":    "/tmp/jdk",
		"EMSDK_PYTHON": "/tmp/python",
	}
	exports := selectEMSDKExports(before, after)
	if len(exports) != 5 {
		t.Fatalf("unexpected exports: %#v", exports)
	}
	if exports["UNRELATED"] != "" {
		t.Fatalf("unexpected unrelated export: %#v", exports)
	}
}

func TestResolveEMSDKEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))

	env, err := resolveEMSDKEnvironment()
	if err != nil {
		t.Fatalf("resolveEMSDKEnvironment returned error: %v", err)
	}

	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(env.rootDir, filepath.Join("Library", "Application Support", "emsdk")) {
			t.Fatalf("unexpected darwin emsdk root: %s", env.rootDir)
		}
	case "linux":
		if !strings.Contains(env.rootDir, filepath.Join(".local", "share", "emsdk")) {
			t.Fatalf("unexpected linux emsdk root: %s", env.rootDir)
		}
	case "windows":
		if !strings.Contains(strings.ToLower(env.rootDir), strings.ToLower(filepath.Join("AppData", "emsdk"))) {
			t.Fatalf("unexpected windows emsdk root: %s", env.rootDir)
		}
	}
}

func TestParseNullEnvOutput(t *testing.T) {
	got := parseNullEnvOutput([]byte("PATH=/bin\x00invalid\x00VALUE=contains=equals\x00EMPTY=\x00\x00"))
	want := map[string]string{"PATH": "/bin", "VALUE": "contains=equals", "EMPTY": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %#v, want %#v", got, want)
	}
}

func TestEnsureSConsUsesPinnedEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("command fixtures use POSIX shell scripts")
	}
	pythonCall := "python -c import sys; print(sys.version)"
	versionCall := "venv -c import SCons; print(SCons.__version__)"
	reuseCalls := []string{pythonCall, versionCall, "scons --version"}
	installCalls := []string{pythonCall, versionCall, "python -m venv", "venv -m pip install scons==" + requiredSConsVersion, "scons --version"}
	freshCalls := append([]string{pythonCall}, installCalls[2:]...)
	mv, _ := exec.LookPath("mv")
	for _, test := range []struct {
		name, current, fail string
		wantErr             bool
		want                []string
	}{
		{"current", requiredSConsVersion, "", false, reuseCalls},
		{"first installation", "missing", "", false, freshCalls},
		{"first installation pip failure", "missing", "pip", true, freshCalls[:3]},
		{"outdated", "old", "", false, installCalls},
		{"version probe failure rebuilds", requiredSConsVersion, "version", false, installCalls},
		{"python failure", "old", "python", true, installCalls[:1]},
		{"virtualenv failure", "old", "virtualenv", true, installCalls[:3]},
		{"pip failure", "old", "pip", true, installCalls[:4]},
		{"scons failure", requiredSConsVersion, "scons", true, reuseCalls},
		{"upgraded scons failure", "old", "scons", true, installCalls},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.current == "missing" && mv == "" {
				t.Skip("mv is required to materialize the virtualenv fixture")
			}
			root := t.TempDir()
			t.Chdir(root)
			writeToolFixture(t, filepath.Join(root, "go.mod"), "module fixture\n", 0o644)
			writeToolFixture(t, filepath.Join(root, "cmd", "spx", "install.sh"), "", 0o644)
			if err := os.MkdirAll(filepath.Join(root, "internal", "tools"), 0o755); err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(root, "bin")
			log := filepath.Join(root, "commands.log")
			t.Setenv("PATH", bin)
			t.Setenv("TOOL_LOG", log)
			t.Setenv("SCONS_CURRENT", test.current)
			t.Setenv("TOOL_FAIL", test.fail)
			t.Setenv("TOOL_VENV_SEED", "")
			t.Setenv("TOOL_MV", mv)
			writeToolFixture(t, filepath.Join(bin, "python3"), `#!/bin/sh
if [ "$1" = "-m" ]; then
  printf 'python -m venv\n' >> "$TOOL_LOG"
  [ "$TOOL_FAIL" != virtualenv ] || exit 1
  if [ -n "$TOOL_VENV_SEED" ]; then "$TOOL_MV" "$TOOL_VENV_SEED" "$3"; fi
else
  printf 'python %s\n' "$*" >> "$TOOL_LOG"
  [ "$TOOL_FAIL" != python ]
fi
`, 0o755)
			venvDir := filepath.Join(root, ".bin", "scons-"+requiredSConsVersion)
			python, scons := sconsEnvironmentCommands(venvDir)
			writeToolFixture(t, python, `#!/bin/sh
printf 'venv %s\n' "$*" >> "$TOOL_LOG"
if [ "$1" = "-c" ]; then
  [ "$TOOL_FAIL" != version ] || exit 1
  printf '%s\n' "$SCONS_CURRENT"
else
  [ "$TOOL_FAIL" != pip ]
fi
`, 0o755)
			writeToolFixture(t, scons, "#!/bin/sh\nprintf 'scons %s\\n' \"$*\" >> \"$TOOL_LOG\"\n[ \"$TOOL_FAIL\" != scons ]\n", 0o755)
			if test.current == "missing" {
				seed := filepath.Join(root, "virtualenv-fixture")
				if err := os.Rename(venvDir, seed); err != nil {
					t.Fatal(err)
				}
				t.Setenv("TOOL_VENV_SEED", seed)
			}
			got, err := EnsureSCons()
			if test.wantErr {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 || got != "" {
					t.Fatalf("EnsureSCons = %q, %v; want command exit 1 and no SCons path", got, err)
				}
			} else if err != nil || got != scons {
				t.Fatalf("EnsureSCons = %q, %v", got, err)
			}
			data, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if calls := strings.Split(strings.TrimSpace(string(data)), "\n"); !reflect.DeepEqual(calls, test.want) {
				t.Fatalf("commands = %#v, want %#v", calls, test.want)
			}
		})
	}
}

func TestDetectPythonCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("command fixtures use POSIX shell scripts")
	}
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if err := runToolSetupSCons(nil); err == nil {
		t.Fatal("missing Python was accepted")
	}
	writeToolFixture(t, filepath.Join(bin, "python"), "#!/bin/sh\nexit 0\n", 0o755)
	if got, err := detectPythonCommand(); err != nil || got != "python" {
		t.Fatalf("fallback Python = %q, %v", got, err)
	}
	writeToolFixture(t, filepath.Join(bin, "python3"), "#!/bin/sh\nexit 0\n", 0o755)
	if got, err := detectPythonCommand(); err != nil || got != "python3" {
		t.Fatalf("preferred Python = %q, %v", got, err)
	}
	t.Chdir(t.TempDir())
	if _, err := EnsureSCons(); err == nil || !strings.Contains(err.Error(), "repository root") {
		t.Fatalf("missing repository error = %v", err)
	}
}

func TestJDKShellExports(t *testing.T) {
	home := t.TempDir()
	t.Setenv("JAVA_HOME", "  "+home+"  ")
	exports, err := ResolveJDKShellExports()
	if err != nil || exports["JAVA_HOME"] != home || !strings.HasPrefix(exports["PATH"], filepath.Join(home, "bin")) {
		t.Fatalf("JDK exports = %#v, %v", exports, err)
	}
	if runtime.GOOS == "darwin" {
		return
	}
	t.Setenv("JAVA_HOME", "")
	exports, err = ResolveJDKShellExports()
	if err != nil || len(exports) != 0 {
		t.Fatalf("exports without JAVA_HOME = %#v, %v", exports, err)
	}
}

func TestSetupJDKWithLocalCommandFixtures(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux package manager selection")
	}
	for _, test := range []struct {
		name, manager, version, fail string
		wantErr                      bool
	}{
		{"already installed", "", strconv.Itoa(requiredJDKMajor), "", false},
		{"no package manager", "", "old", "", true},
		{"apt", "apt-get", "old", "", false},
		{"dnf", "dnf", "old", "", false},
		{"yum", "yum", "old", "", false},
		{"update failure", "apt-get", "old", "update", true},
		{"install failure", "apt-get", "old", "install", true},
		{"dnf install failure", "dnf", "old", "install", true},
		{"yum install failure", "yum", "old", "install", true},
		{"verification failure", "apt-get", "old", "verification", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			bin := t.TempDir()
			t.Setenv("PATH", bin)
			t.Setenv("JAVA_HOME", "")
			t.Setenv("JDK_VERSION", test.version)
			t.Setenv("JDK_REQUIRED", strconv.Itoa(requiredJDKMajor))
			t.Setenv("TOOL_FAIL", test.fail)
			t.Setenv("JDK_MARKER", filepath.Join(bin, "installed"))
			log := filepath.Join(bin, "commands.log")
			t.Setenv("TOOL_LOG", log)
			writeToolFixture(t, filepath.Join(bin, "java"), `#!/bin/sh
if [ -f "$JDK_MARKER" ]; then JDK_VERSION=$JDK_REQUIRED; fi
printf 'openjdk version "%s"\n' "$JDK_VERSION" >&2
`, 0o755)
			if test.manager != "" {
				writeToolFixture(t, filepath.Join(bin, test.manager), "#!/bin/sh\nexit 0\n", 0o755)
				writeToolFixture(t, filepath.Join(bin, "sudo"), `#!/bin/sh
printf '%s\n' "$*" >> "$TOOL_LOG"
[ "$TOOL_FAIL" != "$2" ] || exit 1
if [ "$2" = install ] && [ "$TOOL_FAIL" != verification ]; then : > "$JDK_MARKER"; fi
`, 0o755)
			}
			err := runToolSetupJDK(nil)
			if (err != nil) != test.wantErr {
				t.Fatalf("SetupJDK error = %v, want error %v", err, test.wantErr)
			}
			if test.manager != "" {
				data, err := os.ReadFile(log)
				if err != nil {
					t.Fatal(err)
				}
				want := test.manager + " install -y java-" + strconv.Itoa(requiredJDKMajor) + "-openjdk-devel\n"
				if test.manager == "apt-get" {
					want = "apt-get update\napt-get install -y openjdk-" + strconv.Itoa(requiredJDKMajor) + "-jdk\n"
				}
				if test.fail == "update" {
					want = "apt-get update\n"
				}
				if string(data) != want {
					t.Fatalf("package commands = %q, want %q", data, want)
				}
			}
		})
	}
}

func TestSetupEMSDKWithLocalInstallation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("command fixtures use POSIX shell scripts")
	}
	systemCommands := make(map[string]string)
	for _, name := range []string{"bash", "env", "head", "mv"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is required for EMSDK fixtures", name)
		}
		systemCommands[name] = path
	}
	for _, test := range []struct {
		name, version, fail string
		wantErr             bool
	}{
		{"current", requiredEMSDKVersion, "", false},
		{"new", "new", "", false},
		{"new clone failure", "new", "clone", true},
		{"new install failure", "new", "install", true},
		{"new activation failure", "new", "activate", true},
		{"outdated", "old", "", false},
		{"unrecognized", "", "", false},
		{"install failure", "old", "install", true},
		{"activation failure", requiredEMSDKVersion, "activate", true},
		{"compiler failure", requiredEMSDKVersion, "compiler", true},
		{"missing compiler", requiredEMSDKVersion, "missing compiler", true},
		{"blocked cache", requiredEMSDKVersion, "cache", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("EM_CONFIG", "")
			t.Setenv("EM_CACHE", "")
			t.Setenv("TOOL_FAIL", test.fail)
			t.Setenv("EMCC_VERSION", test.version)
			log := filepath.Join(home, "commands.log")
			t.Setenv("TOOL_LOG", log)
			env, err := resolveEMSDKEnvironment()
			if err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(env.repoDir, "upstream", "emscripten")
			writeToolFixture(t, filepath.Join(env.repoDir, "emsdk_env.sh"), "export PATH="+strconv.Quote(bin)+":\"$PATH\"\nexport EMSDK="+strconv.Quote(env.repoDir)+"\n", 0o644)
			writeToolFixture(t, filepath.Join(env.repoDir, "emsdk"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$TOOL_LOG\"\n[ \"$TOOL_FAIL\" != \"$1\" ]\n", 0o755)
			writeToolFixture(t, filepath.Join(bin, "emcc"), "#!/bin/sh\nprintf 'emcc fixture %s\\n' \"$EMCC_VERSION\"\n", 0o755)
			if test.fail != "missing compiler" {
				writeToolFixture(t, filepath.Join(bin, emscriptenCPPExecutableName()), `#!/bin/sh
[ "$TOOL_FAIL" != compiler ] || exit 1
[ -n "$EM_CONFIG" ] && [ -d "$EM_CACHE" ] || exit 2
printf 'compiler verified\n'
`, 0o755)
			}
			if test.fail == "cache" {
				writeToolFixture(t, filepath.Join(bin, "cache"), "blocked", 0o644)
			}
			fakeBin := filepath.Join(home, "bin")
			for _, name := range []string{"env", "head"} {
				writeToolFixture(t, filepath.Join(fakeBin, name), "#!/bin/sh\nexec "+strconv.Quote(systemCommands[name])+" \"$@\"\n", 0o755)
			}
			// Exercise the requested shell script without sourcing the host's login profile.
			writeToolFixture(t, filepath.Join(fakeBin, "bash"), "#!/bin/sh\n[ \"$1\" = -lc ] && [ \"$#\" = 2 ] || exit 98\nexec "+strconv.Quote(systemCommands["bash"])+" --noprofile --norc -c \"$2\"\n", 0o755)
			t.Setenv("PATH", fakeBin)
			writeToolFixture(t, filepath.Join(fakeBin, "git"), "#!/bin/sh\nexit 99\n", 0o755)
			if strings.HasPrefix(test.name, "new") {
				source := filepath.Join(home, "clone-fixture")
				if err := os.Rename(env.repoDir, source); err != nil {
					t.Fatal(err)
				}
				t.Setenv("EMSDK_FIXTURE", source)
				writeToolFixture(t, filepath.Join(fakeBin, "git"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$TOOL_LOG\"\n[ \"$TOOL_FAIL\" != clone ] || exit 1\nexec "+strconv.Quote(systemCommands["mv"])+" \"$EMSDK_FIXTURE\" emsdk\n", 0o755)
			}
			err = runToolSetupEMSDK(nil)
			if (err != nil) != test.wantErr {
				t.Fatalf("SetupEMSDK error = %v, want error %v", err, test.wantErr)
			}
			data, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			var want string
			if strings.HasPrefix(test.name, "new") {
				want = "clone https://github.com/emscripten-core/emsdk.git\n"
			}
			if test.fail != "clone" {
				if test.version != requiredEMSDKVersion {
					want += "install " + requiredEMSDKVersion + "\n"
				}
				if test.fail != "install" {
					want += "activate " + requiredEMSDKVersion + "\n"
				}
			}
			if string(data) != want {
				t.Fatalf("SDK commands = %q, want %q", data, want)
			}
			if !test.wantErr {
				exports, err := ResolveEMSDKShellExports()
				if err != nil || exports["EMSDK"] != env.repoDir || !strings.HasPrefix(exports["PATH"], bin) {
					t.Fatalf("EMSDK exports = %#v, %v", exports, err)
				}
			}
		})
	}
}

func TestToolEnvironmentResolutionFailures(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	for _, test := range []struct {
		name string
		run  func() error
	}{
		{"NDK", func() error { return setupAndroidNDK(toolSetupNDKConfig{}) }},
		{"EMSDK", SetupEMSDK},
		{"EMSDK exports", func() error { _, err := ResolveEMSDKShellExports(); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); err == nil {
				t.Fatal("missing home directory was accepted")
			}
		})
	}
}

func TestEMSDKRejectsUnavailableDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX EMSDK environment")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	env, err := resolveEMSDKEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyEMSDK(env); err == nil {
		t.Fatal("missing EMSDK directory was accepted")
	}
	if version, ok := detectEMSDKVersion(env); ok || version != "" {
		t.Fatalf("missing EMSDK version = %q, %v", version, ok)
	}
	writeToolFixture(t, env.rootDir, "blocked", 0o644)
	if err := SetupEMSDK(); err == nil {
		t.Fatal("EMSDK root regular file was accepted")
	}
}

func TestEnsureSConsRejectsBlockedEnvironmentDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Python fixture")
	}
	root, bin := t.TempDir(), t.TempDir()
	t.Chdir(root)
	writeToolFixture(t, filepath.Join(root, "go.mod"), "module fixture\n", 0600)
	writeToolFixture(t, filepath.Join(root, "cmd", "spx", "install.sh"), "", 0600)
	if err := os.MkdirAll(filepath.Join(root, "internal", "tools"), 0700); err != nil {
		t.Fatal(err)
	}
	writeToolFixture(t, filepath.Join(root, ".bin"), "blocked", 0600)
	writeToolFixture(t, filepath.Join(bin, "python3"), "#!/bin/sh\nexit 0\n", 0700)
	t.Setenv("PATH", bin)
	if _, err := EnsureSCons(); err == nil || !strings.Contains(err.Error(), ".bin") {
		t.Fatalf("blocked environment = %v", err)
	}
}

func TestDetectJavaVersionRejectsCommandFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if major, ok := detectJavaMajorVersion(map[string]string{"PATH": t.TempDir()}); ok || major != 0 {
		t.Fatalf("missing Java = %d, %v", major, ok)
	}
}

func TestToolchainComesFromRuntimeLock(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	if requiredSConsVersion != lock.Toolchain.SCons {
		t.Fatalf("SCons version = %q, want locked %q", requiredSConsVersion, lock.Toolchain.SCons)
	}
	if requiredEMSDKVersion != lock.Toolchain.EMSDK {
		t.Fatalf("EMSDK version = %q, want locked %q", requiredEMSDKVersion, lock.Toolchain.EMSDK)
	}
	if androidNDKVersion != lock.Toolchain.AndroidNDK {
		t.Fatalf("Android NDK version = %q, want locked %q", androidNDKVersion, lock.Toolchain.AndroidNDK)
	}
	if got := strconv.Itoa(requiredJDKMajor); got != lock.Toolchain.JDK {
		t.Fatalf("JDK major = %q, want locked %q", got, lock.Toolchain.JDK)
	}
}

func TestAndroidNDKReleaseMapping(t *testing.T) {
	got, err := androidNDKReleaseForVersion("23.2.8568313")
	if err != nil {
		t.Fatal(err)
	}
	if got != "r23c" {
		t.Fatalf("release = %q, want r23c", got)
	}
	if _, err := androidNDKReleaseForVersion("99.0.0"); err == nil {
		t.Fatal("unknown Android NDK version did not fail closed")
	}

	t.Run("setup rejects an unmapped lock before changing files", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("USERPROFILE", home)
		t.Setenv("ANDROID_SDK_ROOT", filepath.Join(home, "sdk"))
		previous := androidNDKVersion
		t.Cleanup(func() { androidNDKVersion = previous })
		androidNDKVersion = "99.0.0"
		err := setupAndroidNDK(toolSetupNDKConfig{manualInstall: true, ndkPath: filepath.Join(home, "missing.zip")})
		if err == nil || !strings.Contains(err.Error(), `no Android NDK archive mapping for locked version "99.0.0"`) {
			t.Fatalf("unmapped toolchain error = %v", err)
		}
		if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
			t.Fatalf("unsupported lock changed local state: %v, %v", entries, err)
		}
	})
}

func TestMustJDKMajorRejectsInvalidLockVersions(t *testing.T) {
	for _, version := range []string{"", "invalid", "0", "-1"} {
		t.Run(version, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid locked JDK version did not panic")
				}
			}()
			mustJDKMajor(version)
		})
	}
}
