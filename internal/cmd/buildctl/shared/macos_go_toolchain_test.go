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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type fakeMacOSPaths struct {
	directories map[string]bool
	executables map[string]bool
}

func (p fakeMacOSPaths) isDirectory(path string) bool {
	return p.directories[path]
}

func (p fakeMacOSPaths) isExecutable(path string) bool {
	return p.executables[path]
}

func TestConfigureMacOSGoToolchainEnvRepairsStaleInputs(t *testing.T) {
	t.Parallel()

	const (
		staleSDK      = "/missing/MacOSX13.sdk"
		staleSpaceSDK = "/missing/Xcode 13.app/MacOSX.sdk"
		currentSDK    = "/current/Xcode.app/MacOSX.sdk"
		clang         = "/current/usr/bin/clang"
		clangXX       = "/current/usr/bin/clang++"
	)
	env := map[string]string{
		"PATH":         "/usr/bin:/bin",
		"SDKROOT":      staleSDK,
		"CC":           "/missing/usr/bin/clang",
		"CXX":          "missing-clang++",
		"CGO_CFLAGS":   "-O2 -isysroot " + staleSDK + " -I" + staleSDK + "/usr/include",
		"CGO_CPPFLAGS": "--sysroot=/missing/Other.sdk -DVALUE=1",
		"CGO_CXXFLAGS": "'-isysroot" + staleSpaceSDK + "' -std=c++17",
		"CGO_LDFLAGS":  "-Wl,-syslibroot," + staleSDK + " -L" + staleSDK + "/usr/lib -framework CoreFoundation",
	}
	paths := fakeMacOSPaths{
		directories: map[string]bool{currentSDK: true},
		executables: map[string]bool{clang: true, clangXX: true},
	}
	var calls []string
	xcrun := func(args ...string) (string, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "--show-sdk-path":
			return currentSDK, nil
		case "--find clang":
			return clang, nil
		case "--find clang++":
			return clangXX, nil
		default:
			return "", fmt.Errorf("unexpected xcrun arguments: %s", call)
		}
	}

	if err := configureMacOSGoToolchainEnv("darwin", env, xcrun, paths.isDirectory, paths.isExecutable); err != nil {
		t.Fatal(err)
	}
	if got := env["SDKROOT"]; got != currentSDK {
		t.Fatalf("SDKROOT = %q, want %q", got, currentSDK)
	}
	if got := env["CC"]; got != clang {
		t.Fatalf("CC = %q, want %q", got, clang)
	}
	if got := env["CXX"]; got != clangXX {
		t.Fatalf("CXX = %q, want %q", got, clangXX)
	}
	if got, want := calls, []string{"--show-sdk-path", "--find clang", "--find clang++"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("xcrun calls = %v, want %v", got, want)
	}

	for name, value := range map[string]string{
		"CGO_CFLAGS":   env["CGO_CFLAGS"],
		"CGO_CPPFLAGS": env["CGO_CPPFLAGS"],
		"CGO_CXXFLAGS": env["CGO_CXXFLAGS"],
		"CGO_LDFLAGS":  env["CGO_LDFLAGS"],
	} {
		if strings.Contains(value, "/missing/") {
			t.Errorf("%s still contains a missing SDK: %q", name, value)
		}
	}
	assertQuotedFields(t, env["CGO_CFLAGS"], []string{"-O2", "-isysroot", currentSDK, "-I" + currentSDK + "/usr/include"})
	assertQuotedFields(t, env["CGO_CPPFLAGS"], []string{"--sysroot=" + currentSDK, "-DVALUE=1"})
	assertQuotedFields(t, env["CGO_CXXFLAGS"], []string{"-isysroot" + currentSDK, "-std=c++17"})
	assertQuotedFields(t, env["CGO_LDFLAGS"], []string{"-Wl,-syslibroot," + currentSDK, "-L" + currentSDK + "/usr/lib", "-framework", "CoreFoundation"})
}

func TestConfigureMacOSGoToolchainEnvPreservesValidInputs(t *testing.T) {
	t.Parallel()

	const (
		sdkRoot = "/valid/MacOSX.sdk"
		clang   = "/toolchain/bin/clang"
		clangXX = "/toolchain/bin/clang++"
	)
	env := map[string]string{
		"PATH":          "/toolchain/bin:/usr/bin",
		"SDKROOT":       sdkRoot,
		"CC":            "clang -Qunused-arguments",
		"CXX":           "clang++",
		"CGO_CFLAGS":    "-isysroot " + sdkRoot + " -O3",
		"CGO_LDFLAGS":   "-Wl,-syslibroot," + sdkRoot,
		"UNRELATED_ENV": "keep-me",
	}
	want := cloneStringMap(env)
	paths := fakeMacOSPaths{
		directories: map[string]bool{sdkRoot: true},
		executables: map[string]bool{clang: true, clangXX: true},
	}
	xcrun := func(args ...string) (string, error) {
		return "", fmt.Errorf("xcrun must not be called for valid inputs: %v", args)
	}

	if err := configureMacOSGoToolchainEnv("darwin", env, xcrun, paths.isDirectory, paths.isExecutable); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(env, want) {
		t.Fatalf("environment changed:\n got %#v\nwant %#v", env, want)
	}
}

func TestConfigureMacOSGoToolchainEnvRejectsRelativeSDKROOT(t *testing.T) {
	t.Parallel()

	const (
		relativeSDK = "relative/MacOSX.sdk"
		currentSDK  = "/current/MacOSX.sdk"
		clang       = "/toolchain/bin/clang"
		clangXX     = "/toolchain/bin/clang++"
	)
	env := map[string]string{
		"SDKROOT": relativeSDK,
		"CC":      clang,
		"CXX":     clangXX,
	}
	paths := fakeMacOSPaths{
		directories: map[string]bool{relativeSDK: true, currentSDK: true},
		executables: map[string]bool{clang: true, clangXX: true},
	}
	xcrun := func(args ...string) (string, error) {
		if got := strings.Join(args, " "); got != "--show-sdk-path" {
			return "", fmt.Errorf("unexpected xcrun arguments: %s", got)
		}
		return currentSDK, nil
	}

	if err := configureMacOSGoToolchainEnv("darwin", env, xcrun, paths.isDirectory, paths.isExecutable); err != nil {
		t.Fatal(err)
	}
	if got := env["SDKROOT"]; got != currentSDK {
		t.Fatalf("SDKROOT = %q, want absolute SDK %q", got, currentSDK)
	}
}

func TestConfigureMacOSGoToolchainEnvNonDarwinIsNoop(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"SDKROOT":    "/missing/MacOSX.sdk",
		"CC":         "/missing/clang",
		"CGO_CFLAGS": "-isysroot /missing/MacOSX.sdk",
	}
	want := cloneStringMap(env)
	called := false
	xcrun := func(args ...string) (string, error) {
		called = true
		return "", fmt.Errorf("unexpected xcrun call: %v", args)
	}
	pathCheck := func(path string) bool {
		called = true
		return false
	}

	if err := configureMacOSGoToolchainEnv("linux", env, xcrun, pathCheck, pathCheck); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("non-Darwin configuration consulted macOS tools or paths")
	}
	if !reflect.DeepEqual(env, want) {
		t.Fatalf("non-Darwin environment changed:\n got %#v\nwant %#v", env, want)
	}
}

func TestConfigureMacOSGoToolchainEnvIsAtomicOnFailure(t *testing.T) {
	t.Parallel()

	env := map[string]string{"SDKROOT": "/missing/MacOSX.sdk", "CC": ""}
	want := cloneStringMap(env)
	paths := fakeMacOSPaths{directories: map[string]bool{"/current/MacOSX.sdk": true}}
	xcrun := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "--show-sdk-path" {
			return "/current/MacOSX.sdk", nil
		}
		return "relative/clang", nil
	}

	err := configureMacOSGoToolchainEnv("darwin", env, xcrun, paths.isDirectory, paths.isExecutable)
	if err == nil || !strings.Contains(err.Error(), "invalid macOS C compiler") {
		t.Fatalf("configure error = %v", err)
	}
	if !reflect.DeepEqual(env, want) {
		t.Fatalf("failed configuration partially changed environment:\n got %#v\nwant %#v", env, want)
	}
}

func TestMacOSGoToolchainShellBootstrapDropsCGOFlags(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available")
	}

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	currentSDK := filepath.Join(root, "MacOSX.sdk")
	staleSDK := filepath.Join(root, "Missing.sdk")
	clang := filepath.Join(binDir, "clang")
	clangXX := filepath.Join(binDir, "clang++")
	for _, directory := range []string{binDir, currentSDK} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable(t, clang, "#!/bin/bash\nexit 0\n")
	writeExecutable(t, clangXX, "#!/bin/bash\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "go"), `#!/bin/bash
case "$*" in
  "env GOHOSTOS") printf '%s\n' "$FAKE_GOOS" ;;
  "build --bootstrap-test")
    printf 'BUILD_CGO_CFLAGS=%s:%s\n' "${CGO_CFLAGS+x}" "${CGO_CFLAGS-}"
    printf 'BUILD_CGO_CPPFLAGS=%s:%s\n' "${CGO_CPPFLAGS+x}" "${CGO_CPPFLAGS-}"
    printf 'BUILD_CGO_CXXFLAGS=%s:%s\n' "${CGO_CXXFLAGS+x}" "${CGO_CXXFLAGS-}"
    printf 'BUILD_CGO_LDFLAGS=%s:%s\n' "${CGO_LDFLAGS+x}" "${CGO_LDFLAGS-}"
    ;;
  *) echo "unexpected go arguments: $*" >&2; exit 2 ;;
esac
`)
	writeExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/bash
case "$*" in
  "--sdk macosx --show-sdk-path") printf '%s\n' "$FAKE_SDK" ;;
  "--sdk macosx --find clang") printf '%s\n' "$FAKE_CLANG" ;;
  "--sdk macosx --find clang++") printf '%s\n' "$FAKE_CLANGXX" ;;
  *) echo "unexpected xcrun arguments: $*" >&2; exit 1 ;;
esac
`)

	command := `. "$TOOLCHAIN_SCRIPT"
configure_macos_go_toolchain
macos_go_toolchain_go_build --bootstrap-test
printf 'SDKROOT=%s\n' "$SDKROOT"
printf 'CC=%s\n' "$CC"
printf 'CXX=%s\n' "$CXX"
printf 'CGO_CFLAGS=%s\n' "$CGO_CFLAGS"
printf 'CGO_CPPFLAGS=%s\n' "$CGO_CPPFLAGS"
printf 'CGO_CXXFLAGS=%s\n' "$CGO_CXXFLAGS"
printf 'CGO_LDFLAGS=%s\n' "$CGO_LDFLAGS"
`
	cgoValues := map[string]string{
		"CGO_CFLAGS":   "-O2 -isysroot " + staleSDK,
		"CGO_CPPFLAGS": "--sysroot=" + staleSDK + " -DVALUE=1",
		"CGO_CXXFLAGS": "-isysroot" + staleSDK + " -std=c++17",
		"CGO_LDFLAGS":  "-Wl,-syslibroot," + staleSDK + " -framework CoreFoundation",
	}
	for _, tc := range []struct {
		name    string
		sdkRoot string
		cc      string
		cxx     string
	}{
		{name: "valid SDKROOT with stale CGO flags", sdkRoot: currentSDK, cc: clang, cxx: clangXX},
		{name: "empty SDKROOT with stale CGO flags", cc: filepath.Join(root, "missing-clang")},
		{name: "relative SDKROOT", sdkRoot: filepath.Base(currentSDK), cc: clang, cxx: clangXX},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bash, "-c", command)
			cmd.Dir = root
			cmd.Env = []string{
				"PATH=" + binDir + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin",
				"TOOLCHAIN_SCRIPT=" + macOSGoToolchainScriptPath(t),
				"FAKE_GOOS=darwin",
				"FAKE_SDK=" + currentSDK,
				"FAKE_CLANG=" + clang,
				"FAKE_CLANGXX=" + clangXX,
				"SDKROOT=" + tc.sdkRoot,
				"CC=" + tc.cc,
				"CXX=" + tc.cxx,
				"CGO_CFLAGS=" + cgoValues["CGO_CFLAGS"],
				"CGO_CPPFLAGS=" + cgoValues["CGO_CPPFLAGS"],
				"CGO_CXXFLAGS=" + cgoValues["CGO_CXXFLAGS"],
				"CGO_LDFLAGS=" + cgoValues["CGO_LDFLAGS"],
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("configure shell toolchain: %v\n%s", err, output)
			}
			values := parseKeyValueLines(t, string(output))
			for key, want := range map[string]string{
				"SDKROOT":            currentSDK,
				"CC":                 clang,
				"CXX":                clangXX,
				"BUILD_CGO_CFLAGS":   ":",
				"BUILD_CGO_CPPFLAGS": ":",
				"BUILD_CGO_CXXFLAGS": ":",
				"BUILD_CGO_LDFLAGS":  ":",
			} {
				if got := values[key]; got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
			for key, want := range cgoValues {
				if got := values[key]; got != want {
					t.Errorf("caller %s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestMacOSGoToolchainScriptRunsLockedGo(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available")
	}

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	currentSDK := filepath.Join(root, "MacOSX SDK.sdk")
	clang := filepath.Join(binDir, "clang")
	clangXX := filepath.Join(binDir, "clang++")
	for _, directory := range []string{binDir, currentSDK} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable(t, clang, "#!/bin/bash\nexit 0\n")
	writeExecutable(t, clangXX, "#!/bin/bash\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "go"), `#!/bin/bash
if [ "$*" = "env GOHOSTOS" ]; then
  printf 'darwin\n'
  exit 0
fi
printf 'GOTOOLCHAIN=%s\n' "$GOTOOLCHAIN"
printf 'SDKROOT=%s\n' "$SDKROOT"
printf 'CC=%s\n' "$CC"
printf 'CXX=%s\n' "$CXX"
printf 'ARG_COUNT=%s\n' "$#"
index=0
for arg in "$@"; do
  printf 'ARG_%s=%s\n' "$index" "$arg"
  index=$((index + 1))
done
`)
	writeExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/bash
case "$*" in
  "--sdk macosx --show-sdk-path") printf '%s\n' "$FAKE_SDK" ;;
  "--sdk macosx --find clang") printf '%s\n' "$FAKE_CLANG" ;;
  "--sdk macosx --find clang++") printf '%s\n' "$FAKE_CLANGXX" ;;
  *) echo "unexpected xcrun arguments: $*" >&2; exit 1 ;;
esac
`)

	cmd := exec.Command(bash, macOSGoToolchainScriptPath(t), "go1.25.8", "generate", "./path with spaces/...")
	cmd.Env = []string{
		"PATH=" + binDir + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin",
		"FAKE_SDK=" + currentSDK,
		"FAKE_CLANG=" + clang,
		"FAKE_CLANGXX=" + clangXX,
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run locked Go through shell toolchain: %v\n%s", err, output)
	}
	values := parseKeyValueLines(t, string(output))
	for key, want := range map[string]string{
		"GOTOOLCHAIN": "go1.25.8",
		"SDKROOT":     currentSDK,
		"CC":          clang,
		"CXX":         clangXX,
		"ARG_COUNT":   "2",
		"ARG_0":       "generate",
		"ARG_1":       "./path with spaces/...",
	} {
		if got := values[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestConfigureMacOSGoToolchainShellNonDarwinIsNoop(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available")
	}

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "xcrun-called")
	writeExecutable(t, filepath.Join(binDir, "go"), `#!/bin/bash
case "$*" in
  "env GOHOSTOS") printf 'linux\n' ;;
  "build --bootstrap-test") printf 'BUILD_CGO_CFLAGS=%s:%s\n' "${CGO_CFLAGS+x}" "${CGO_CFLAGS-}" ;;
  *) exit 2 ;;
esac
`)
	writeExecutable(t, filepath.Join(binDir, "xcrun"), "#!/bin/bash\nprintf called > \"$XCRUN_MARKER\"\nexit 1\n")

	command := `. "$TOOLCHAIN_SCRIPT"
configure_macos_go_toolchain
macos_go_toolchain_go_build --bootstrap-test
printf 'SDKROOT=%s\n' "$SDKROOT"
printf 'CC=%s\n' "$CC"
printf 'CGO_CFLAGS=%s\n' "$CGO_CFLAGS"
`
	cmd := exec.Command(bash, "-c", command)
	cmd.Env = []string{
		"PATH=" + binDir + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin",
		"TOOLCHAIN_SCRIPT=" + macOSGoToolchainScriptPath(t),
		"XCRUN_MARKER=" + marker,
		"SDKROOT=/missing/MacOSX.sdk",
		"CC=/missing/clang",
		"CGO_CFLAGS=-isysroot /missing/MacOSX.sdk",
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("configure non-Darwin shell toolchain: %v\n%s", err, output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("non-Darwin shell invoked xcrun; marker stat error = %v", err)
	}
	values := parseKeyValueLines(t, string(output))
	for key, want := range map[string]string{
		"SDKROOT":          "/missing/MacOSX.sdk",
		"CC":               "/missing/clang",
		"CGO_CFLAGS":       "-isysroot /missing/MacOSX.sdk",
		"BUILD_CGO_CFLAGS": "x:-isysroot /missing/MacOSX.sdk",
	} {
		if got := values[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestCommandRunnerRepairsMacOSGoToolchainForChild(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS child environment integration")
	}

	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	goBin := filepath.Join(gopath, "bin")
	toolBin := filepath.Join(root, "tools")
	currentSDK := filepath.Join(root, "MacOSX.sdk")
	staleSDK := filepath.Join(root, "Missing.sdk")
	clang := filepath.Join(toolBin, "clang")
	clangXX := filepath.Join(toolBin, "clang++")
	for _, directory := range []string{goBin, toolBin, currentSDK} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable(t, clang, "#!/bin/bash\nexit 0\n")
	writeExecutable(t, clangXX, "#!/bin/bash\nexit 0\n")
	writeExecutable(t, filepath.Join(toolBin, "xcrun"), `#!/bin/bash
case "$*" in
  "--sdk macosx --show-sdk-path") printf '%s\n' "$FAKE_SDK" ;;
  "--sdk macosx --find clang") printf '%s\n' "$FAKE_CLANG" ;;
  "--sdk macosx --find clang++") printf '%s\n' "$FAKE_CLANGXX" ;;
  *) exit 1 ;;
esac
`)
	outputPath := filepath.Join(root, "child-env")
	writeExecutable(t, filepath.Join(goBin, "capture-go-env"), `#!/bin/bash
printf 'SDKROOT=%s\n' "$SDKROOT" > "$OUTPUT_PATH"
printf 'CC=%s\n' "$CC" >> "$OUTPUT_PATH"
printf 'CXX=%s\n' "$CXX" >> "$OUTPUT_PATH"
printf 'CGO_CFLAGS=%s\n' "$CGO_CFLAGS" >> "$OUTPUT_PATH"
`)

	t.Setenv("GOPATH", gopath)
	t.Setenv("PATH", toolBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_SDK", currentSDK)
	t.Setenv("FAKE_CLANG", clang)
	t.Setenv("FAKE_CLANGXX", clangXX)
	t.Setenv("OUTPUT_PATH", outputPath)
	t.Setenv("SDKROOT", staleSDK)
	t.Setenv("CC", filepath.Join(root, "missing-clang"))
	t.Setenv("CXX", filepath.Join(root, "missing-clang++"))
	t.Setenv("CGO_CFLAGS", "-O2 -isysroot "+staleSDK)

	runner := CommandRunner{RepoRoot: root}
	if err := runner.RunCommand(".", "capture-go-env"); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	values := parseKeyValueLines(t, string(output))
	for key, want := range map[string]string{
		"SDKROOT":    currentSDK,
		"CC":         clang,
		"CXX":        clangXX,
		"CGO_CFLAGS": "-O2 -isysroot " + currentSDK,
	} {
		if got := values[key]; got != want {
			t.Errorf("child %s = %q, want %q", key, got, want)
		}
	}
}

func assertQuotedFields(t *testing.T, value string, want []string) {
	t.Helper()
	got, err := splitQuotedFields(value)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields for %q = %#v, want %#v", value, got, want)
	}
}

func macOSGoToolchainScriptPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate macos_go_toolchain_test.go")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../cmd/internal/macos_go_toolchain.sh"))
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func parseKeyValueLines(t *testing.T, output string) map[string]string {
	t.Helper()
	values := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid key/value output line %q", line)
		}
		values[key] = value
	}
	return values
}
