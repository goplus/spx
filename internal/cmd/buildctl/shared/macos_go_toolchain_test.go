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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/base/quoted"
)

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

func TestRunMacOSXcrun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixture")
	}
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
if [ "$#" -ne 4 ] || [ "$1" != "--sdk" ] || [ "$2" != "macosx" ] || [ "$3" != "--find" ] || [ "$4" != "clang with spaces" ]; then
  echo "unexpected arguments: $*" >&2
  exit 90
fi
if [ "${SDKROOT+x}" = x ] || [ "$EXTRA" != "kept=value" ]; then
  echo "unexpected child environment" >&2
  exit 91
fi
if [ "$FAKE_EXIT" = 0 ]; then
  printf '%s' "$FAKE_OUTPUT"
else
  printf '%s' "$FAKE_OUTPUT" >&2
fi
exit "$FAKE_EXIT"
`)
	for _, test := range []struct {
		name, output string
		exitCode     int
	}{
		{"success", " \t/tool path/clang \n", 0},
		{"error detail", " \tSDK lookup failed \n", 7},
		{"empty error", "", 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := map[string]string{
				"PATH":        binDir,
				"SDKROOT":     "/stale/SDK",
				"EXTRA":       "kept=value",
				"FAKE_OUTPUT": test.output,
				"FAKE_EXIT":   strconv.Itoa(test.exitCode),
			}
			got, err := runMacOSXcrun(env, "--find", "clang with spaces")
			if env["SDKROOT"] != "/stale/SDK" || env["EXTRA"] != "kept=value" {
				t.Fatalf("caller environment changed: %v", env)
			}
			if test.exitCode == 0 {
				if err != nil || got != "/tool path/clang" {
					t.Fatalf("runMacOSXcrun() = %q, %v", got, err)
				}
				return
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != test.exitCode {
				t.Fatalf("error = %v, want wrapped exit status %d", err, test.exitCode)
			}
			wantErr := "xcrun --find clang with spaces: exit status 7"
			if test.output != "" {
				wantErr += ": SDK lookup failed"
			}
			if got != "" || err.Error() != wantErr {
				t.Fatalf("runMacOSXcrun() = %q, %v; want empty output and %q", got, err, wantErr)
			}
		})
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

func TestResolveMacOSXcrunPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixture")
	}
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "xcrun"), "#!/bin/sh\nprintf '%s' \"$FAKE_OUTPUT\"\nexit \"$FAKE_EXIT\"\n")
	sdk := t.TempDir()
	for _, test := range []struct {
		name, output, exitCode, wantErr string
	}{
		{"valid directory", sdk + "\n", "0", ""},
		{"empty", "", "0", "invalid macOS SDK"},
		{"relative", "MacOSX.sdk", "0", "invalid macOS SDK"},
		{"missing", filepath.Join(sdk, "missing"), "0", "invalid macOS SDK"},
		{"multiple paths", sdk + "\n" + sdk, "0", "invalid macOS SDK"},
		{"carriage return", sdk + "\rignored", "0", "invalid macOS SDK"},
		{"command error", "SDK unavailable", "9", "resolve macOS SDK: xcrun --show-sdk-path: exit status 9: SDK unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := map[string]string{"PATH": binDir, "FAKE_OUTPUT": test.output, "FAKE_EXIT": test.exitCode}
			got, err := resolveMacOSXcrunPath(env, pathIsDirectory, "SDK", "--show-sdk-path")
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) || got != "" {
					t.Fatalf("resolveMacOSXcrunPath() = %q, %v; want %q", got, err, test.wantErr)
				}
			} else if err != nil || got != sdk {
				t.Fatalf("resolveMacOSXcrunPath() = %q, %v; want %q", got, err, sdk)
			}
		})
	}
	if _, err := runMacOSXcrun(map[string]string{"PATH": t.TempDir()}, "--show-sdk-path"); err == nil || !strings.Contains(err.Error(), "resolve xcrun") {
		t.Fatalf("missing xcrun error = %v", err)
	}
}

func TestMacOSCommandIsUsable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable permission checks")
	}
	binDir := t.TempDir()
	compiler := filepath.Join(binDir, "clang with spaces")
	writeExecutable(t, compiler, "#!/bin/sh\n")
	plainFile := filepath.Join(binDir, "not-executable")
	if err := os.WriteFile(plainFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"Path": string(os.PathListSeparator) + binDir}
	for _, test := range []struct {
		name, command string
		want          bool
	}{
		{"absolute quoted compiler with arguments", strconv.Quote(compiler) + " -O2", true},
		{"compiler from PATH", "'clang with spaces' -O2", true},
		{"missing compiler", "missing-clang", false},
		{"relative path", "./clang", false},
		{"empty", "", false},
		{"invalid quoting", "'clang", false},
		{"directory", strconv.Quote(binDir), false},
		{"non-executable", strconv.Quote(plainFile), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := macOSCommandIsUsable(test.command, env); got != test.want {
				t.Fatalf("macOSCommandIsUsable(%q) = %v, want %v", test.command, got, test.want)
			}
		})
	}
	if pathIsDirectory("") || pathIsExecutable("") {
		t.Fatal("an empty path must not be usable")
	}
}

func TestNormalizeMacOSCGOFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("macOS SDK flags use Unix paths")
	}
	root := t.TempDir()
	sdk := filepath.Join(root, "Current SDK.sdk")
	preserved := filepath.Join(root, "Installed.sdk")
	for _, directory := range []string{sdk, preserved} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	missing := filepath.Join(root, "Missing.sdk")
	for _, test := range []struct {
		name, input string
		want        []string
		unchanged   bool
	}{
		{"unrelated formatting", " -O2\t'-DVALUE=two words'  ", []string{"-O2", "-DVALUE=two words"}, true},
		{"valid separate root", "--sysroot " + strconv.Quote(preserved), []string{"--sysroot", preserved}, true},
		{"stale separate root", "-O2 -isysroot " + strconv.Quote(missing), []string{"-O2", "-isysroot", sdk}, false},
		{"relative root", "--sysroot relative.sdk", []string{"--sysroot", sdk}, false},
		{"missing root argument", "-isysroot", []string{"-isysroot", sdk}, false},
		{"root followed by flag", "--sysroot -O2", []string{"--sysroot", sdk, "-O2"}, false},
		{"attached isysroot", "-isysroot" + missing, []string{"-isysroot" + sdk}, false},
		{"attached sysroot", "--sysroot" + missing, []string{"--sysroot" + sdk}, false},
		{"equals isysroot", "-isysroot=" + missing, []string{"-isysroot=" + sdk}, false},
		{"equals sysroot", "--sysroot=" + missing, []string{"--sysroot=" + sdk}, false},
		{"valid attached root", "--sysroot=" + preserved, []string{"--sysroot=" + preserved}, true},
		{"SDK include suffix", "-I" + missing + "/usr/include", []string{"-I" + sdk + "/usr/include"}, false},
		{"multiple embedded SDKs", "-Wl,-syslibroot," + missing + ",-rpath," + missing + "/usr/lib", []string{"-Wl,-syslibroot," + sdk + ",-rpath," + sdk + "/usr/lib"}, false},
		{"preserve valid embedded SDK", "-L" + preserved + "/usr/lib", []string{"-L" + preserved + "/usr/lib"}, true},
		{"SDK name without a path", "-DNAME=MacOSX.sdk", []string{"-DNAME=MacOSX.sdk"}, true},
		{"case-insensitive suffix", "-I" + filepath.Join(root, "Missing.SDK") + "/include", []string{"-I" + sdk + "/include"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeMacOSCGOFlags(test.input, sdk)
			if err != nil {
				t.Fatal(err)
			}
			fields, err := quoted.Split(got)
			if err != nil || !slices.Equal(fields, test.want) {
				t.Fatalf("normalized fields = %q, %v; want %q", fields, err, test.want)
			}
			if test.unchanged && got != test.input {
				t.Fatalf("unchanged flags reformatted: %q, want %q", got, test.input)
			}
			if again, err := normalizeMacOSCGOFlags(got, sdk); err != nil || again != got {
				t.Fatalf("normalization is not idempotent: %q, %v; want %q", again, err, got)
			}
		})
	}
	if _, err := normalizeMacOSCGOFlags("'unterminated", sdk); err == nil {
		t.Fatal("expected malformed quoted flags to fail")
	}
	if got, ok := normalizeAttachedSysroot("--sysroot", sdk); ok || got != "" {
		t.Fatalf("bare sysroot flag treated as attached: %q, %v", got, ok)
	}
}
