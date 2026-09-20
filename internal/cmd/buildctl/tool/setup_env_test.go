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
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
